package database

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

// ProductHouseholdRepositoryInterface is the household's view of its stock:
// the rows a backup or an export carries, the narrow projection the recipe
// matcher reads, the staples the household buys and the user and household
// lookups that scope them.
type ProductHouseholdRepositoryInterface interface {
	GetUserActiveProductsFiltered(userID uint, from, to *time.Time) ([]database.Product, error)
	GetUserArchivedProductsFiltered(userID uint, from, to *time.Time) ([]database.Product, error)
	GetHouseholdProductsForBackup(householdID uint) ([]database.Product, error)
	GetHouseholdArchivedProductsForBackup(householdID uint) ([]database.Product, error)
	GetProductsByHousehold(householdID uint) ([]database.Product, error)
	GetMatchableProductsByHousehold(householdID uint) ([]database.Product, error)
	GetProductSetFingerprint(householdID uint) (string, error)
	GetSubThresholdProducts(userID uint) ([]database.Product, error)
	GetExpiringProductsByHousehold(householdID uint, daysAhead int) ([]database.Product, error)
	GetExpiringInDays(userID uint, days int) ([]database.Product, error)
	GetFrequentlyBoughtProducts(userID uint, limit int) ([]apiModel.FrequentProduct, error)
	GetLastNotifiedProduct(householdID uint) (database.Product, error)
	GetLastInsertedProduct(householdID uint) (database.Product, error)
}

var _ ProductHouseholdRepositoryInterface = (*ProductHouseholdRepository)(nil)

// ProductHouseholdRepository answers the reads that are scoped to a household
// rather than to a caller's product list.
type ProductHouseholdRepository struct {
	DB *gorm.DB
}

func NewProductHouseholdRepository(db *gorm.DB) *ProductHouseholdRepository {
	return &ProductHouseholdRepository{DB: db}
}

func (r *ProductHouseholdRepository) GetUserActiveProductsFiltered(userID uint, from, to *time.Time) ([]database.Product, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	query := r.DB.Preload("StorageLocation").Scopes(privacyScope(userID)).Where(util.QueryHouseholdId, householdID).Where(util.WhereDeletedIsNull)

	if from != nil {
		query = query.Where("created_at >= ?", *from)
	}
	if to != nil {
		query = query.Where("created_at <= ?", *to)
	}

	queryErr := query.Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

func (r *ProductHouseholdRepository) GetUserArchivedProductsFiltered(userID uint, from, to *time.Time) ([]database.Product, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	query := r.DB.Preload("StorageLocation").Unscoped().Scopes(privacyScope(userID)).Where(util.WhereDeletedIsNotNull).Where(util.QueryHouseholdId, householdID)

	if from != nil {
		query = query.Where("deleted_at >= ?", *from)
	}
	if to != nil {
		query = query.Where("deleted_at <= ?", *to)
	}

	queryErr := query.Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

// GetHouseholdProductsForBackup returns every active household product
// including private rows. A backup exists to reproduce the household, so the
// privacy scope every user-facing query applies must not silently drop a
// member's private stock from the file.
func (r *ProductHouseholdRepository) GetHouseholdProductsForBackup(householdID uint) ([]database.Product, error) {
	var products []database.Product
	err := r.DB.Preload("StorageLocation").
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Find(&products).Error
	return products, err
}

// GetHouseholdArchivedProductsForBackup is the archived counterpart of
// GetHouseholdProductsForBackup: every soft-deleted household row, private
// ones included.
func (r *ProductHouseholdRepository) GetHouseholdArchivedProductsForBackup(householdID uint) ([]database.Product, error) {
	var products []database.Product
	err := r.DB.Preload("StorageLocation").Unscoped().
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNotNull).
		Find(&products).Error
	return products, err
}

// GetProductsByHousehold returns all non-deleted products for a household.
func (r *ProductHouseholdRepository) GetProductsByHousehold(householdID uint) ([]database.Product, error) {
	var products []database.Product
	result := r.DB.Where("household_id = ? AND deleted_at IS NULL AND is_private = 0", householdID).Find(&products)
	return products, result.Error
}

// GetMatchableProductsByHousehold returns the household's matchable products
// with only the three columns the recipe matcher reads: ID, ProductName and
// Categories. GetProductsByHousehold loads every Product column for the whole
// household, and the matcher consumed three of them on a path that runs on every
// cache miss, so the remaining columns — barcode, notes, image, storage
// location — were materialised for nothing on an unbounded result set.
//
// The returned Products are a read projection, NOT fully populated rows: every
// field other than ID, ProductName and Categories is the zero value. Callers
// must not read anything else off them; use GetProductsByHousehold for that.
func (r *ProductHouseholdRepository) GetMatchableProductsByHousehold(householdID uint) ([]database.Product, error) {
	var products []database.Product
	result := r.DB.Model(&database.Product{}).
		Select("id, product_name, categories").
		Where("household_id = ? AND deleted_at IS NULL AND is_private = 0", householdID).
		Find(&products)
	return products, result.Error
}

// productIdentityRow holds the only product fields the recipe matcher and the
// expiry ranking read. Selecting just these keeps the fingerprint query narrow
// instead of materialising every Product column for every row of the household.
type productIdentityRow struct {
	ID               uint
	ProductName      string
	Categories       string
	ExpireAt         time.Time
	OpenedAt         *time.Time
	DaysAfterOpening *int
}

// effectiveExpireAt applies the shelf-life precedence rule to the narrow row
// above. It cannot call the Product method because the row is not a Product, so
// it delegates to the exported free function that Product.EffectiveExpireAt
// itself uses rather than restating the rule. A hand-written copy here would
// bucket the cache key differently from the ranking the payload was ordered by
// the moment the model's precedence changed.
func (row *productIdentityRow) effectiveExpireAt() time.Time {
	return database.EffectiveExpireAt(row.ExpireAt, row.OpenedAt, row.DaysAfterOpening)
}

// GetProductSetFingerprint returns a cheap signature of a household's
// matchable product set: the row count plus a hash over the identity of every
// row GetProductsByHousehold returns. It lets recipe suggestion caching notice
// that the set changed without loading every product column, which matters
// because cached suggestions carry product IDs that authorize an irreversible
// "cook" action. An add, rename, recategorisation, delete or expiry change all
// move the hash.
//
// The signature covers ProductName and Categories but deliberately not
// updated_at. Every write bumps updated_at — quantity changes go through
// Update("quantity", ...) and Save, so scanning, cooking or editing a single
// product moved it — which made the cache key change on essentially every
// request in an actively used household and turned each one into a full
// provider fan-out. Those writes cannot change what a recipe matched, so they
// must not invalidate the cache.
//
// Expiry is included as the ranking day bucket — the whole 24-hour spans between
// now and the effective expiry, clamped to the ranking horizon — and not as a raw
// timestamp or a calendar date. Ranking is expiry-proximity-first and happens at
// match time — ExpiryPoints is json:"-" — so a cached entry cannot be re-sorted on
// read, and a bucket the ranking no longer agrees with would keep serving the stale
// order for the whole TTL.
//
// The bucket is what makes the key track the ranking exactly. A calendar date does
// not: two timestamps inside one UTC date can still straddle a 24-hour bucket
// boundary, so encoding the date lets the order change while the key stays put, and
// an edit that does move the order can go unnoticed. Bucketing also keeps the key
// stable against edits that cannot move the order — a time-of-day change within one
// bucket — which is what stops quantity and scan writes from rekeying the cache.
// A product leaves its bucket at most once a day, the same cadence at which the
// expiring-ID component of the key already moves.
func (r *ProductHouseholdRepository) GetProductSetFingerprint(householdID uint) (string, error) {
	var rows []productIdentityRow
	result := r.DB.Model(&database.Product{}).
		Select("id, product_name, categories, expire_at, opened_at, days_after_opening").
		Where("household_id = ? AND deleted_at IS NULL AND is_private = 0", householdID).
		Order("id").
		Find(&rows)
	if result.Error != nil {
		return "", result.Error
	}
	// NUL separators keep the encoding unambiguous, so moving a character
	// across the name/categories/bucket boundary still changes the hash. Ordering
	// by id above makes the concatenation order, and therefore the hash,
	// deterministic.
	now := time.Now()
	var identity strings.Builder
	for i := range rows {
		identity.WriteString(strconv.FormatUint(uint64(rows[i].ID), 10))
		identity.WriteByte(0)
		identity.WriteString(rows[i].ProductName)
		identity.WriteByte(0)
		identity.WriteString(rows[i].Categories)
		identity.WriteByte(0)
		if expireAt := rows[i].effectiveExpireAt(); !expireAt.IsZero() {
			identity.WriteString(strconv.Itoa(database.ExpiryRankingDaysLeft(expireAt, now)))
		}
		identity.WriteByte(0)
	}
	sum := sha256.Sum256([]byte(identity.String()))
	return fmt.Sprintf("%d:%x", len(rows), sum), nil
}

func (r *ProductHouseholdRepository) GetSubThresholdProducts(userID uint) ([]database.Product, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	err = r.DB.Preload("StorageLocation").
		Scopes(privacyScope(userID)).
		Where("household_id = ? AND deleted_at IS NULL AND quantity < min_stock_amount AND min_stock_amount > 0", householdID).
		Find(&products).Error
	if err != nil {
		return []database.Product{}, err
	}
	return products, nil
}

// GetExpiringProductsByHousehold returns non-private products for a household
// whose effective expiry (earlier of printed ExpireAt and OpenedAt+DaysAfterOpening)
// falls in the window [startOfToday, endOfWindow].
func (r *ProductHouseholdRepository) GetExpiringProductsByHousehold(householdID uint, daysAhead int) ([]database.Product, error) {
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfWindow := time.Date(now.Year(), now.Month(), now.Day()+daysAhead, 23, 59, 59, 999999999, now.Location())

	var products []database.Product
	result := r.DB.
		Where("household_id = ? AND deleted_at IS NULL AND is_private = 0", householdID).
		Scopes(effectiveExpiryCandidateScope(startOfToday, endOfWindow)).
		Find(&products)
	if result.Error != nil {
		return []database.Product{}, result.Error
	}

	filtered := filterByEffectiveExpiryWindow(products, startOfToday, endOfWindow)
	sort.Slice(filtered, func(i, j int) bool {
		ti, tj := filtered[i].EffectiveExpireAt(), filtered[j].EffectiveExpireAt()
		if ti.IsZero() && tj.IsZero() {
			return false
		}
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		return ti.Before(tj)
	})
	return filtered, nil
}

func (r *ProductHouseholdRepository) GetExpiringInDays(userID uint, days int) ([]database.Product, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return []database.Product{}, err
	}

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfWindow := time.Date(now.Year(), now.Month(), now.Day()+days, 23, 59, 59, 999999999, now.Location())

	// SQL filter: a row qualifies if either its printed expiry is within
	// the window, OR it has an opened-shelf-life rule (OpenedAt + days).
	// The latter is the small residual that lets a product with a later
	// printed date but an earlier opened+days date still be considered.
	// The exact effective-date filter is applied in Go below to keep the
	// query portable across SQLite and MariaDB.
	var products []database.Product
	err = r.DB.Scopes(privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Scopes(effectiveExpiryCandidateScope(startOfToday, endOfWindow)).
		Find(&products).Error
	if err != nil {
		return []database.Product{}, err
	}

	filtered := filterByEffectiveExpiryWindow(products, startOfToday, endOfWindow)
	sort.Slice(filtered, func(i, j int) bool {
		ti, tj := filtered[i].EffectiveExpireAt(), filtered[j].EffectiveExpireAt()
		if ti.IsZero() && tj.IsZero() {
			return false
		}
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		return ti.Before(tj)
	})
	return filtered, nil
}

// frequentlyBoughtQuery ranks the household's archived products by how often
// it bought them and how recently, and carries the most recent row of every
// barcode so a suggestion names the product the way the household does.
//
// The archive is grouped in SQL: the query returns one row per barcode, so
// its cost does not grow with the size of the pantry.
//
// The privacy filter sits inside the grouping on purpose. A private product
// of another member must neither appear nor inflate the count of a barcode
// the household shares.
//
// The recency key is the id, not the creation date: the id is the order the
// household bought the product in, and it compares as a number on every
// database. The creation date of that purchase rides along as the date of the
// row the query joins to.
const frequentlyBoughtQuery = `
SELECT p.barcode, p.product_name, p.pack_size, p.category_key,
       p.storage_location_id, p.image_url, p.created_at,
       b.purchase_count
FROM (
    SELECT barcode,
           COUNT(*) AS purchase_count,
           MAX(id)   AS last_id
    FROM products
    WHERE household_id = ? AND deleted_at IS NOT NULL
      AND (is_private = 0 OR user_id = ?) AND barcode <> ''
    GROUP BY barcode
    HAVING COUNT(*) >= ?
) b
JOIN products p ON p.id = b.last_id
ORDER BY b.purchase_count DESC, b.last_id DESC
LIMIT ?`

// frequentProductRow is one row of the frequently-bought query. It is a
// private shape, so the soft-delete scope of GORM never touches the raw SQL.
type frequentProductRow struct {
	Barcode           string    `gorm:"column:barcode"`
	ProductName       string    `gorm:"column:product_name"`
	PackSize          string    `gorm:"column:pack_size"`
	CategoryKey       *string   `gorm:"column:category_key"`
	StorageLocationID *uint     `gorm:"column:storage_location_id"`
	ImageURL          string    `gorm:"column:image_url"`
	LastBoughtAt      time.Time `gorm:"column:created_at"`
	PurchaseCount     int       `gorm:"column:purchase_count"`
}

// GetFrequentlyBoughtProducts returns the staples the household buys
// repeatedly, most frequent first and most recent second. A product the
// household bought fewer than util.FrequentProductMinPurchases times is not a
// staple and never appears.
func (r *ProductHouseholdRepository) GetFrequentlyBoughtProducts(userID uint, limit int) ([]apiModel.FrequentProduct, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		return []apiModel.FrequentProduct{}, nil
	}

	var rows []frequentProductRow
	queryErr := r.DB.Raw(
		frequentlyBoughtQuery,
		householdID,
		userID,
		util.FrequentProductMinPurchases,
		limit,
	).Scan(&rows).Error
	if queryErr != nil {
		return nil, queryErr
	}

	products := make([]apiModel.FrequentProduct, 0, len(rows))
	for i := range rows {
		products = append(products, apiModel.FrequentProduct{
			Barcode:           rows[i].Barcode,
			ProductName:       rows[i].ProductName,
			PackSize:          rows[i].PackSize,
			CategoryKey:       rows[i].CategoryKey,
			StorageLocationID: rows[i].StorageLocationID,
			ImageURL:          rows[i].ImageURL,
			PurchaseCount:     rows[i].PurchaseCount,
			LastBoughtAt:      rows[i].LastBoughtAt,
		})
	}
	return products, nil
}

func (r *ProductHouseholdRepository) GetLastNotifiedProduct(householdID uint) (database.Product, error) {
	var lastNotifiedProduct database.Product
	getNotifiedError := r.DB.
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Where("(is_private = 0 OR user_id = (SELECT id FROM users WHERE household_id = ? LIMIT 1))", householdID).
		Order("notified_at DESC").
		Limit(1).
		Find(&lastNotifiedProduct)

	return lastNotifiedProduct, getNotifiedError.Error
}

func (r *ProductHouseholdRepository) GetLastInsertedProduct(householdID uint) (database.Product, error) {
	var lastProduct database.Product
	getError := r.DB.
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Where("(is_private = 0 OR user_id = (SELECT id FROM users WHERE household_id = ? LIMIT 1))", householdID).
		Order("created_at DESC").
		Limit(1).
		Find(&lastProduct)

	return lastProduct, getError.Error
}
