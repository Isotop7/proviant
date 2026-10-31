package controllers

import (
	"context"
	"errors"
	"sync"
	"time"

	dbController "github.com/Isotop7/proviant/controllers/database"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// packSizeBackfillInterval is how often the backfill looks for products whose
// pack size is still unknown.
const packSizeBackfillInterval = 6 * time.Hour

// packSizeBackfillBatch bounds how many products one run fills, so a large
// household cannot turn a backfill into a burst of Open Food Facts requests.
const packSizeBackfillBatch = 50

// packSizeMissCooldown is how long a barcode a lookup could not resolve is
// left alone. The walk over the table starts over at its end, so without a
// cooldown a barcode the data source does not know would be looked up again
// on every pass — a stream of outbound requests that never ends.
const packSizeMissCooldown = 7 * 24 * time.Hour

// packSizeBackfillBudget bounds one pass over the table. The pass walks the
// table in batches until it reaches its end, so the budget is what stops it
// when the data source is slow.
const packSizeBackfillBudget = 10 * time.Minute

// PackSizeController fills the pack size of products that do not have one.
// The pack size is display metadata the barcode carries, so it is read from
// Open Food Facts: the imported bulk dump first, the live API only for the
// barcodes the dump does not know.
type PackSizeController struct {
	DB     *gorm.DB
	OFFA   DatasetGetter
	Logger *zerolog.Logger

	dispatchOnce sync.Once
	mu           sync.Mutex
	stopped      bool
	cursor       uint
	misses       map[string]time.Time
	ctx          context.Context
	cancel       context.CancelFunc
	workers      sync.WaitGroup
}

// NewPackSizeController creates a pack size backfill controller.
func NewPackSizeController(db *gorm.DB, offa DatasetGetter, logger *zerolog.Logger) *PackSizeController {
	ctx, cancel := context.WithCancel(context.Background())
	return &PackSizeController{
		DB:     db,
		OFFA:   offa,
		Logger: logger,
		misses: make(map[string]time.Time),
		ctx:    ctx,
		cancel: cancel,
	}
}

// RunOnce fills the pack size of every product that is still missing one. The
// table is walked in batches until the walk reaches its end, so one pass
// covers the whole backlog instead of a single batch of it — a household with
// hundreds of unknown pack sizes would otherwise wait days for them. A barcode
// the data source does not know, or one without a pack size, leaves the product
// as it is: the pack size stays optional.
func (c *PackSizeController) RunOnce(ctx context.Context) error {
	for {
		scanned, err := c.runBatch(ctx)
		if err != nil {
			return err
		}
		if !scanned {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// runBatch fills the pack size of one batch of products and reports whether
// the batch held any, i.e. whether the walk may have more to do behind it.
func (c *PackSizeController) runBatch(ctx context.Context) (bool, error) {
	if c.DB == nil || c.OFFA == nil {
		return false, nil
	}
	repo := dbController.NewProductRepository(c.DB)
	c.mu.Lock()
	cursor := c.cursor
	c.mu.Unlock()
	products, err := repo.ListProductsWithoutPackSize(cursor, packSizeBackfillBatch)
	if err != nil {
		return false, err
	}
	if len(products) == 0 {
		// The table is walked to its end: start over, so a barcode that could
		// not be resolved gets another try once its cooldown is over.
		c.mu.Lock()
		c.cursor = 0
		c.pruneMissesLocked(time.Now())
		c.mu.Unlock()
		return false, nil
	}
	c.mu.Lock()
	c.cursor = products[len(products)-1].ID
	// The cooldowns that are over are dropped on every batch, so the map does
	// not grow with every barcode the backfill ever failed on, not even when
	// the walk over the table never reaches its end.
	c.pruneMissesLocked(time.Now())
	c.mu.Unlock()
	filled := 0
	for i := range products {
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		product := &products[i]
		if product.Barcode == "" {
			continue
		}
		if c.missed(product.Barcode) {
			continue
		}
		packSize, found := c.packSizeOf(repo, product.Barcode)
		if !found {
			c.rememberMiss(product.Barcode)
			continue
		}
		if setErr := repo.SetProductPackSize(product.ID, packSize); setErr != nil {
			if c.Logger != nil {
				c.Logger.Warn().Msgf("Pack size backfill: storing the pack size of product %d failed: %s", product.ID, setErr)
			}
			continue
		}
		filled++
	}
	if c.Logger != nil {
		c.Logger.Info().Msgf("Pack size backfill filled %d of %d products", filled, len(products))
	}
	return true, nil
}

// missed reports whether a barcode is still cooling down from a lookup that
// found no pack size.
func (c *PackSizeController) missed(barcode string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	retryAfter, known := c.misses[barcode]
	return known && time.Now().Before(retryAfter)
}

// rememberMiss puts a barcode that yielded no pack size on a cooldown, so the
// walk over the table does not look it up again on every pass.
func (c *PackSizeController) rememberMiss(barcode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.misses[barcode] = time.Now().Add(packSizeMissCooldown)
}

// pruneMissesLocked drops the cooldowns that are over, so the map does not
// grow with every barcode the backfill ever failed on.
func (c *PackSizeController) pruneMissesLocked(now time.Time) {
	for barcode, retryAfter := range c.misses {
		if !now.Before(retryAfter) {
			delete(c.misses, barcode)
		}
	}
}

// packSizeOf resolves the size of one pack for a barcode. The local cache is
// read first: a barcode the cache knows needs no outbound request, and the
// cache is the only place a lookup may read without a network call. A cache
// entry without a pack size is no answer — every entry written before the
// pack size column existed has none — so the live lookup answers it.
func (c *PackSizeController) packSizeOf(repo *dbController.ProductRepository, barcode string) (string, bool) {
	cached, cacheErr := repo.GetOpenFoodFactsCacheByBarcode(barcode)
	if cacheErr == nil && cached.PackSize != "" {
		return cached.PackSize, true
	}
	dataset, lookupErr := c.OFFA.GetDataset(barcode)
	if lookupErr != nil {
		if c.Logger != nil {
			c.Logger.Debug().Msgf("Pack size backfill: lookup failed for barcode %s: %s", barcode, lookupErr)
		}
		return "", false
	}
	return dataset.PackSize, dataset.PackSize != ""
}

// Dispatch starts the periodic backfill goroutine.
func (c *PackSizeController) Dispatch() {
	if c.OFFA == nil {
		if c.Logger != nil {
			c.Logger.Info().Msg("PackSizeController has no data source, not dispatching")
		}
		return
	}
	c.dispatchOnce.Do(func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.stopped || c.ctx.Err() != nil {
			return
		}
		c.workers.Go(func() {
			if c.Logger != nil {
				c.Logger.Info().Msgf("PackSizeController dispatched (interval: %s)", packSizeBackfillInterval)
			}
			runCtx, runCancel := context.WithTimeout(c.ctx, packSizeBackfillBudget)
			c.runPass(runCtx)
			runCancel()

			ticker := time.NewTicker(packSizeBackfillInterval)
			defer ticker.Stop()
			for {
				select {
				case <-c.ctx.Done():
					return
				case <-ticker.C:
					ctx, cancel := context.WithTimeout(c.ctx, packSizeBackfillBudget)
					c.runPass(ctx)
					cancel()
				}
			}
		})
	})
}

// runPass walks the table once and reports a failure. A pass that runs into
// its budget is not a failure: the next one picks the backlog up where this
// one stopped.
func (c *PackSizeController) runPass(ctx context.Context) {
	if err := c.RunOnce(ctx); err != nil && !errors.Is(err, context.DeadlineExceeded) && c.Logger != nil {
		c.Logger.Warn().Msgf("Pack size backfill completed with errors: %s", err)
	}
}

// Stop ends the backfill goroutine.
func (c *PackSizeController) Stop() {
	c.mu.Lock()
	c.stopped = true
	c.mu.Unlock()
	c.cancel()
	c.workers.Wait()
}
