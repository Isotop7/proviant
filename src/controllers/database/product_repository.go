package database

import (
	"crypto/sha256"
	stderrors "errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// maxDaysAfterOpening is the upper bound enforced on the DaysAfterOpening
// field on PATCH. It is also used to bound the "opened-shelf-life" OR branch
// in effectiveExpiryCandidateScope so the planner can prune candidate rows
// whose opened date is far outside the requested window. Kept as an alias of
// the model constant so the write bound and the query bound cannot drift.
const maxDaysAfterOpening = database.MaxDaysAfterOpening

// effectiveExpiryCandidateScope returns a GORM scope that selects product
// rows whose effective expiry (printed ExpireAt, OR OpenedAt + DaysAfterOpening)
// could fall within [start, end]. The "opened-shelf-life present" branch
// is bounded by maxDaysAfterOpening on the OpenedAt side: any row whose
// OpenedAt is more than maxDaysAfterOpening before start cannot possibly
// fall into the window, even with the smallest positive days-after-opening
// value, so the planner can prune it. The Go caller MUST still filter
// using EffectiveExpireAt() as a correctness safety net (the bound is a
// safe upper, not a tight lower).
//
// The OR clause prevents the planner from using the expire_at index alone,
// so this scope trades index-friendliness for correctness across both
// expiry sources. It is intentional; see the comment on
// Product.EffectiveExpireAt.
func effectiveExpiryCandidateScope(start, end time.Time) func(*gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		// A row can only enter the window via the opened branch if
		// OpenedAt + days_after_opening is in [start, end]. The
		// smallest possible days_after_opening is 1, so any row with
		// OpenedAt < start - maxDaysAfterOpening cannot qualify.
		// Similarly OpenedAt > end can never qualify (days >= 1).
		openedFloor := start.AddDate(0, 0, -maxDaysAfterOpening)
		return tx.Where(
			"(expire_at >= ? AND expire_at <= ?) OR "+
				"(opened_at IS NOT NULL AND days_after_opening > 0 "+
				"AND opened_at >= ? AND opened_at <= ?)",
			start, end, openedFloor, end,
		)
	}
}

// filterByEffectiveExpiryWindow returns the subset of products whose
// effective expiry (earlier of printed ExpireAt and OpenedAt+DaysAfterOpening)
// is in the closed window [start, end]. Rows with a zero effective expiry
// are dropped. The slice header is reused to avoid an allocation when
// the input is empty.
func filterByEffectiveExpiryWindow(products []database.Product, start, end time.Time) []database.Product {
	filtered := products[:0]
	for i := range products {
		eff := products[i].EffectiveExpireAt()
		if eff.IsZero() {
			continue
		}
		if eff.Before(start) || eff.After(end) {
			continue
		}
		filtered = append(filtered, products[i])
	}
	return filtered
}

// ImportedProduct is one row of a bulk import, paired with the storage location
// it still needs. Product.StorageLocationID is already set when the row points
// at a location the household has; NewLocationName carries the name to create
// otherwise. Keeping the location name outside the Product model lets
// CreateProductsBulk resolve it inside the same transaction as the insert, so a
// failed import leaves no orphaned locations behind.
type ImportedProduct struct {
	Product         database.Product
	NewLocationName string
	NewLocationIcon string
}

type ProductRepositoryInterface interface {
	GetUserProductsBulk(userID uint, limit int) ([]database.Product, error)
	GetUserProductsBulkSorted(userID uint, limit, offset int, sortValue, orderValue string) ([]database.Product, error)
	GetUserProductsByIDs(userID uint, ids []uint) ([]database.Product, error)
	GetUserArchivedProductsBulk(userID uint, limit, offset int) ([]database.Product, error)
	GetActiveProductProjections(userID, locationID uint) ([]database.Product, error)
	GetArchivedProductProjections(userID uint) ([]database.Product, error)
	GetUserArchivedProductsByIDs(userID uint, ids []uint) ([]database.Product, error)
	GetUserProductsBulkByBarcode(userID uint, barcode int) ([]database.Product, error)
	GetUserProductsBulkByBarcodes(userID uint, barcodes []string) ([]database.Product, error)
	FindDuplicateProduct(userID uint, draft *database.Product) (database.Product, bool, error)
	GetProductByID(productID, userID uint) (database.Product, error)
	GetProductIdentity(productID, userID uint) (database.Product, error)
	GetArchivedProductByID(productID, userID uint) (database.Product, error)
	SearchProducts(queryParam SearchParameterEnum, queryValue, sortValue, orderValue string, userID uint, limit, offset int) ([]database.Product, error)
	CountSearchProducts(queryParam SearchParameterEnum, queryValue string, userID uint) (int, error)
	SearchProductProjections(queryParam SearchParameterEnum, queryValue, sortValue, orderValue string, userID, locationID uint) ([]database.Product, error)
	CreateProduct(userID uint, product *database.Product) error
	GetProductBySyncID(userID uint, clientSyncID string) (database.Product, error)
	CreateProductsBulk(userID uint, rows []ImportedProduct) ([]string, error)
	UpdateProduct(productID uint, userID uint, product *database.ProductDTOPatch) error
	UpdateProductQuantity(productID uint, userID uint, delta int, mutationID *string) (bool, bool, error)
	ListProductsWithoutPackSize(afterID uint, limit int) ([]database.Product, error)
	SetProductPackSize(productID uint, packSize string) error
	DeleteProduct(productID uint, userID uint, archiveOnly bool) error
	RestoreProduct(productID, userID uint) error
	BulkRestoreProducts(productIDs []uint, userID uint) []BulkOperationError
	SetProductExpireAt(productID uint, userID uint, expireAt database.Timestamp) error
	SetProductNotifiedAt(productID uint) error
	GetProductsExpired(userID uint) ([]*database.Product, error)
	GetExpiredProductsCount(userID uint, now time.Time) (int, error)
	GetArchivedProductsCount(userID uint) (int, error)
	GetUniqueArchivedProductsCount(userID uint) (int, error)
	GetFrequentlyBoughtProducts(userID uint, limit int) ([]apiModel.FrequentProduct, error)
	GetActiveProductsCount(userID uint) (int, error)
	GetProductCategoryBreakdown(userID uint) (map[string]int, error)
	GetExpiryTrend(userID uint, now time.Time) ([]apiModel.StatsMonthlyCount, error)
	GetExpiringSoonProducts(userID uint, days int, now time.Time) ([]apiModel.StatsExpiringProduct, error)
	GetUseFirstProducts(userID uint, limit int) ([]apiModel.StatsUseFirstProduct, error)
	GetLastNotifiedProduct(householdID uint) (database.Product, error)
	GetExpiringInDays(userID uint, days int) ([]database.Product, error)
	GetLastInsertedProduct(householdID uint) (database.Product, error)
	UserHasProductAccess(userID uint, productID int) bool
	GetOpenFoodFactsCacheByBarcode(barcode string) (database.OpenFoodFactsCache, error)
	GetOpenFoodFactsCachesByBarcodes(barcodes []string) ([]database.OpenFoodFactsCache, error)
	CreateOpenFoodFactsCache(entry *database.OpenFoodFactsCache) error
	UpdateOpenFoodFactsCacheImageURL(barcode, imageURL string) error
	GetOpenFoodFactsCacheWithoutStorageHint() ([]database.OpenFoodFactsCache, error)
	UpdateOpenFoodFactsCacheStorageHint(barcode, storageHint string) error
	GetOpenFoodFactsCacheWithRemoteImageURL() ([]database.OpenFoodFactsCache, error)
	GetUserByID(userID uint) (authentication.User, error)
	GetUserHouseholdByID(userID uint) (uint, error)
	GetHouseholdByID(householdID uint) (database.Household, error)
	GetUserActiveProductsFiltered(userID uint, from, to *time.Time) ([]database.Product, error)
	GetUserArchivedProductsFiltered(userID uint, from, to *time.Time) ([]database.Product, error)
	GetUsersByHouseholdID(householdID uint) ([]authentication.User, error)
	GetExpiringSoonCount(userID uint, days int, now time.Time) (int, error)
	GetActiveExpiryCounts(userID uint, now time.Time, criticalDays int) (expired, critical int, err error)
	GetWasteThisMonth(userID uint, now time.Time) (int, error)
	GetExpiringProductsByHousehold(householdID uint, daysAhead int) ([]database.Product, error)
	GetProductsByHousehold(householdID uint) ([]database.Product, error)
	GetSubThresholdProducts(userID uint) ([]database.Product, error)
	ConsumeProduct(productID, userID uint) error
	ConsumeProductPartial(product *database.Product, quantity int, mutationID *string) (int, bool, bool, error)
	WasteProduct(productID, userID uint, cause string) error
	WasteProductPartial(product *database.Product, quantity int, mutationID *string, cause string) (int, bool, bool, error)
	MarkProductOpened(productID, userID uint, openedAt time.Time, force bool) (database.Product, *time.Time, bool, error)
	BulkConsumeProducts(productIDs []uint, userID uint) []BulkOperationError
	BulkWasteProducts(productIDs []uint, userID uint, cause string) []BulkOperationError
	GetExpiringProductsForMailDigest(householdID uint) (MailDigestProductGroup, error)
	GetConsumedSamples(householdID, userID uint, barcode, name string, since time.Time) ([]database.Product, error)
}

var _ ProductRepositoryInterface = (*ProductRepository)(nil)

type ProductRepository struct {
	DB *gorm.DB
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{DB: db}
}

type SearchParameterEnum int

const (
	InvalidParameter SearchParameterEnum = iota
	ProductName
	Barcode
)

func SearchParameterEnumFromString(str string) SearchParameterEnum {
	switch str {
	case "product_name":
		return ProductName
	case "barcode":
		return Barcode
	default:
		return InvalidParameter
	}
}

type BulkOperationError struct {
	productID uint
	error     error
}

// NewBulkOperationError builds a failure entry for one bulk-operation id, so
// callers outside this package (test mocks) can report failures too.
func NewBulkOperationError(productID uint, err error) BulkOperationError {
	return BulkOperationError{productID: productID, error: err}
}

func (b *BulkOperationError) Error() string {
	return fmt.Sprintf("Error bulk deleting product '%d', error: %v", b.productID, b.error)
}

// ProductID exposes which product a bulk operation error refers to, so
// callers can distinguish failed items from succeeded ones (the error list
// is sparse — it only contains failures, not one entry per requested id).
func (b *BulkOperationError) ProductID() uint {
	return b.productID
}

// Err exposes the underlying failure so callers can classify it: a
// not-found/cross-tenant miss is the client's problem, anything else is a
// server-side fault that must surface as a 500.
func (b *BulkOperationError) Err() error {
	return b.error
}

func (r *ProductRepository) getUserHouseholdID(userID uint) (uint, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return 0, err
	}
	if user.HouseholdID == 0 {
		return 0, errors.ErrInvalidUserData
	}
	return user.HouseholdID, nil
}

func (r *ProductRepository) privacyScope(userID uint) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		return db.Where("(is_private = 0 OR user_id = ?)", userID)
	}
}

// activeProductsQuery returns the query every read of the household's active
// products shares: the household's rows, the ones the caller may see, the rows
// that are not archived.
//
// The list and the count that describes it are both built from it, so the count
// cannot report a different set than the list — a count that leaked another
// household's rows or another member's private ones would be a leak of its own.
//
// The query it returns is single-use: it carries one statement, so every
// terminal call (a Find, a Count) needs a query of its own.
func (r *ProductRepository) activeProductsQuery(userID uint) (*gorm.DB, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return nil, err
	}

	return r.DB.
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull), nil
}

// archivedProductsQuery is the archived counterpart of activeProductsQuery: the
// same household and the same privacy rules, but the rows that are archived.
func (r *ProductRepository) archivedProductsQuery(userID uint) (*gorm.DB, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return nil, err
	}

	return r.DB.Unscoped().
		Scopes(r.privacyScope(userID)).
		Where(util.WhereDeletedIsNotNull).
		Where(util.QueryHouseholdId, householdID), nil
}

// pageWindow returns the rows a page covers: the rows after the offset, at
// most limit of them. A limit of zero means every remaining row, an offset
// beyond the last row means an empty page.
func pageWindow(rows []database.Product, limit, offset int) []database.Product {
	if offset > 0 {
		if offset >= len(rows) {
			return []database.Product{}
		}
		rows = rows[offset:]
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return rows
}

// applyPage limits a query to the window a page covers.
//
// An offset always carries a limit: MySQL has no OFFSET without a LIMIT and
// rejects the statement, while the SQLite driver rewrites a bare OFFSET into
// "LIMIT -1 OFFSET n" on its own. A limit of zero means "every row", so the
// offset alone becomes the largest limit the database can express.
func applyPage(query *gorm.DB, limit, offset int) *gorm.DB {
	if limit <= 0 && offset > 0 {
		limit = math.MaxInt
	}
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	return query
}

// orderProductsByID returns the rows in the order of the given ids and drops the
// rows that are not among them. The hydration reads the rows by id, so the
// order the projections were sorted in has to be restored here.
func orderProductsByID(rows []database.Product, ids []uint) []database.Product {
	byID := make(map[uint]database.Product, len(rows))
	for i := range rows {
		byID[rows[i].ID] = rows[i]
	}
	ordered := make([]database.Product, 0, len(ids))
	for _, id := range ids {
		if row, ok := byID[id]; ok {
			ordered = append(ordered, row)
		}
	}
	return ordered
}

// SortByEffectiveExpiry orders products by the earlier of the printed expiry
// and the opened-shelf-life date, with undated rows last. It is the products
// list's ordering: the repository sorts full rows with it, and the products page
// sorts the narrow projection it pages from, so both must agree exactly or page
// boundaries drift from what a full list would have shown.
//
// It must be a stable sort: two products expiring the same day are a tie, and
// an arbitrary tie order that differs between the projection pass and any later
// pass over the same data lets a product land on two pages or on none.
func SortByEffectiveExpiry(products []database.Product) {
	sortByEffectiveExpiry(products, false)
}

// SortByEffectiveExpiryDescending orders the same rows latest first. The
// undated rows stay at the end in both directions: a product without a date is
// not the freshest one, it is the one nobody knows when it goes off.
func SortByEffectiveExpiryDescending(products []database.Product) {
	sortByEffectiveExpiry(products, true)
}

// sortByEffectiveExpiry orders the rows by their effective expiry, soonest
// first or latest first. The undated rows come last either way, and the sort
// is stable, so the order a caller handed in survives as the tie order.
func sortByEffectiveExpiry(products []database.Product, descending bool) {
	sort.SliceStable(products, func(i, j int) bool {
		ti, tj := products[i].EffectiveExpireAt(), products[j].EffectiveExpireAt()
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		if descending {
			return tj.Before(ti)
		}
		return ti.Before(tj)
	})
}

// GetUserProductsBulk returns the household's active products in the order the
// products list uses: by effective expiry, soonest first.
func (r *ProductRepository) GetUserProductsBulk(userID uint, limit int) ([]database.Product, error) {
	return r.GetUserProductsBulkSorted(userID, limit, 0, SortEffectiveExpiry, "asc")
}

// GetUserProductsBulkSorted returns one page of the household's active products
// in the order the caller asked for: one of the columns the database can sort
// by, or the effective expiry, which only Go can compute.
//
// A column sort is ordered and limited by the database in one query. The
// effective expiry has no portable SQL form, so those rows are ordered in Go
// and the page is cut out there too — otherwise the database would hand back
// an arbitrary subset of the pantry and the "soonest first" order would only
// hold inside that subset.
//
// The offset is a window into the ordered set, so the order has to be the same
// on every request: resolveSortClause breaks the ties of a column sort by id,
// and the effective expiry keeps the id order the projections came in.
func (r *ProductRepository) GetUserProductsBulkSorted(userID uint, limit, offset int, sortValue, orderValue string) ([]database.Product, error) {
	sort, sortErr := resolveSortClause(sortValue, orderValue)
	if sortErr != nil {
		return []database.Product{}, sortErr
	}

	if sort.byEffectiveExpiry {
		products, err := r.getProductsByEffectiveExpiry(userID, limit, offset, sort.descending)
		if err != nil {
			return []database.Product{}, err
		}
		return products, nil
	}

	query, err := r.activeProductsQuery(userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	listQuery := applyPage(query.Preload("StorageLocation").Order(sort.clause), limit, offset)

	if findErr := listQuery.Find(&products).Error; findErr != nil {
		return []database.Product{}, findErr
	}

	return products, nil
}

// getProductsByEffectiveExpiry returns the household's active products ordered
// by the effective expiry, limited to the page that starts at the offset.
//
// The narrow projection is sorted in Go and only the rows that make up the
// page are hydrated, so the database never materialises the whole pantry as
// full rows with their storage location.
func (r *ProductRepository) getProductsByEffectiveExpiry(userID uint, limit, offset int, descending bool) ([]database.Product, error) {
	projections, err := r.GetActiveProductProjections(userID, 0)
	if err != nil {
		return nil, err
	}

	if descending {
		SortByEffectiveExpiryDescending(projections)
	} else {
		SortByEffectiveExpiry(projections)
	}

	page := pageWindow(projections, limit, offset)
	ids := make([]uint, 0, len(page))
	for i := range page {
		ids = append(ids, page[i].ID)
	}

	rows, err := r.hydrateProductsByIDs(userID, ids)
	if err != nil {
		return nil, err
	}

	return orderProductsByID(rows, ids), nil
}

// GetUserProductsByIDs returns the active products with the given IDs that belong to the user's household.
// Used for lightweight lookups of a small, known set of products (e.g. cook workflow).
func (r *ProductRepository) GetUserProductsByIDs(userID uint, ids []uint) ([]database.Product, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	queryErr := r.DB.Preload("StorageLocation").Where(util.QueryHouseholdId, householdID).Scopes(r.privacyScope(userID)).Where("id IN ?", ids).Order("id").Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

// GetUserArchivedProductsBulk returns one page of the household's archived
// products, oldest id first.
//
// The archived rows are ordered by id because they have no expiry to order
// them by: without an order the database hands the rows back in whatever order
// it finds them, and a client that walks the pages by offset would skip and
// repeat rows.
func (r *ProductRepository) GetUserArchivedProductsBulk(userID uint, limit, offset int) ([]database.Product, error) {
	query, err := r.archivedProductsQuery(userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	listQuery := applyPage(query.Preload("StorageLocation").Order("id"), limit, offset)

	queryErr := listQuery.Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

// productProjectionColumns is every column the products page reads before it has
// chosen a page: the row id it hydrates with, and the three inputs to
// EffectiveExpireAt that decide ordering and status. The page hydrates only the
// rows it renders, so the expensive part (full rows plus a StorageLocation
// preload) scales with the page size instead of with the size of the pantry.
const productProjectionColumns = "id, expire_at, opened_at, days_after_opening"

// GetActiveProductProjections returns the household's active rows narrowed to
// productProjectionColumns, ordered by id. locationID narrows the result to one
// storage location; pass 0 for every location. Callers sort, filter by status
// and slice a single page out of the result before hydrating it.
func (r *ProductRepository) GetActiveProductProjections(userID, locationID uint) ([]database.Product, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return nil, err
	}

	query := r.DB.Model(&database.Product{}).
		Select(productProjectionColumns).
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull)
	if locationID != 0 {
		query = query.Where("storage_location_id = ?", locationID)
	}

	var products []database.Product
	if err := query.Order("id").Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}

// GetArchivedProductProjections is the archived-view counterpart of
// GetActiveProductProjections. The archived view has no expiry ordering of its
// own, so id order is what keeps a row on exactly one page.
func (r *ProductRepository) GetArchivedProductProjections(userID uint) ([]database.Product, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return nil, err
	}

	var products []database.Product
	err = r.DB.Unscoped().Model(&database.Product{}).
		Select(productProjectionColumns).
		Scopes(r.privacyScope(userID)).
		Where(util.WhereDeletedIsNotNull).
		Where(util.QueryHouseholdId, householdID).
		Order("id").
		Find(&products).Error
	if err != nil {
		return nil, err
	}
	return products, nil
}

// GetUserArchivedProductsByIDs hydrates one page of the archived view: full rows
// with their StorageLocation, for the given IDs only.
func (r *ProductRepository) GetUserArchivedProductsByIDs(userID uint, ids []uint) ([]database.Product, error) {
	if len(ids) == 0 {
		return []database.Product{}, nil
	}
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	err = r.DB.Preload("StorageLocation").Unscoped().
		Scopes(r.privacyScope(userID)).
		Where(util.WhereDeletedIsNotNull).
		Where(util.QueryHouseholdId, householdID).
		Where("id IN ?", ids).
		Find(&products).Error
	if err != nil {
		return []database.Product{}, err
	}
	return products, nil
}

func (r *ProductRepository) GetUserProductsBulkByBarcode(userID uint, barcode int) ([]database.Product, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	queryErr := r.DB.Scopes(r.privacyScope(userID)).Where("household_id = ? and barcode = ?", householdID, barcode).Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

// bulkQueryChunkSize bounds the number of placeholders per IN clause. SQLite
// builds have historically capped bound variables at 999, so a full import's
// 5000 barcodes cannot go into one query.
const bulkQueryChunkSize = 500

// chunkStrings splits values into consecutive slices of at most size elements.
func chunkStrings(values []string, size int) [][]string {
	chunks := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := min(start+size, len(values))
		chunks = append(chunks, values[start:end])
	}
	return chunks
}

// GetUserProductsBulkByBarcodes returns the household's active products whose
// barcode is in the given set. Unlike GetUserProductsBulkByBarcode it takes
// the codes as strings: that variant parses them as an int, which drops
// leading zeros and cannot hold non-numeric codes.
//
// privacyScope is required, not optional. The import turns every returned
// barcode into a per-row "already exists" rejection, so without it a member
// gets an existence oracle over other members' private products and their own
// rows are rejected because of a product they cannot see.
func (r *ProductRepository) GetUserProductsBulkByBarcodes(userID uint, barcodes []string) ([]database.Product, error) {
	if len(barcodes) == 0 {
		return []database.Product{}, nil
	}

	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	for _, chunk := range chunkStrings(barcodes, bulkQueryChunkSize) {
		var batch []database.Product
		queryErr := r.DB.Scopes(r.privacyScope(userID)).
			Where(util.QueryHouseholdId, householdID).
			Where(util.WhereDeletedIsNull).
			Where("barcode IN ?", chunk).
			Find(&batch).Error
		if queryErr != nil {
			return []database.Product{}, queryErr
		}
		products = append(products, batch...)
	}
	return products, nil
}

// FindDuplicateProduct returns the household's active product that the given
// draft would duplicate, and reports whether there is one. A duplicate is a
// row with the same barcode, the same storage location and the same effective
// expiry: the same packs of the same thing in the same place.
//
// The lookup is scoped to the household and to privacyScope, so another
// member's private row is neither matched nor disclosed. Archived rows are
// skipped: they are history, not stock, and restoring one is a different
// action.
//
// The candidate set is the household's rows with that barcode, so the
// comparison runs in Go through Product.IsDuplicateOf and the rule cannot
// drift from the one the client is told about. The candidate query trims the
// stored barcode as well, because a row written with a padded barcode is the
// same barcode and must not slip past the rule.
func (r *ProductRepository) FindDuplicateProduct(userID uint, draft *database.Product) (database.Product, bool, error) {
	if draft == nil || draft.Barcode == "" {
		return database.Product{}, false, nil
	}

	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return database.Product{}, false, err
	}

	var candidates []database.Product
	queryErr := r.DB.Preload("StorageLocation").
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Where("TRIM(barcode) = ?", database.NormalizeBarcode(draft.Barcode)).
		Find(&candidates).Error
	if queryErr != nil {
		return database.Product{}, false, queryErr
	}

	for i := range candidates {
		if candidates[i].IsDuplicateOf(draft) {
			return candidates[i], true, nil
		}
	}
	return database.Product{}, false, nil
}

func (r *ProductRepository) GetProductByID(productID, userID uint) (database.Product, error) {
	if productID == 0 {
		return database.Product{}, gorm.ErrNotImplemented
	}

	var product database.Product
	getError := r.DB.Preload("StorageLocation").First(&product, productID)
	if getError.Error != nil {
		return database.Product{}, getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return database.Product{}, userErr.Error
	}

	if product.HouseholdID != user.HouseholdID {
		return database.Product{}, errors.ErrMismatcherUserID
	}
	if product.IsPrivate && product.UserID != userID {
		return database.Product{}, errors.ErrMismatcherUserID
	}
	return product, nil
}

// GetProductIdentity returns a product's identity (household + barcode +
// name + pack count + min stock + private flag + owner) without the
// StorageLocation preload. It exists for hot read paths that only need
// those fields and would otherwise pay for a useless JOIN.
func (r *ProductRepository) GetProductIdentity(productID, userID uint) (database.Product, error) {
	if productID == 0 {
		return database.Product{}, gorm.ErrNotImplemented
	}

	var product database.Product
	getError := r.DB.Select(
		"id, barcode, product_name, household_id, user_id, is_private, quantity, min_stock_amount",
	).First(&product, productID)
	if getError.Error != nil {
		return database.Product{}, getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return database.Product{}, userErr.Error
	}

	if product.HouseholdID != user.HouseholdID {
		return database.Product{}, errors.ErrMismatcherUserID
	}
	if product.IsPrivate && product.UserID != userID {
		return database.Product{}, errors.ErrMismatcherUserID
	}
	return product, nil
}

// ListProductsWithoutPackSize returns the active products that have a barcode
// but no pack size yet, starting after the given id. The backfill walks the
// table with that cursor so a barcode the data source does not know cannot
// hold the queue in place: the rows it cannot resolve are retried only after
// a full pass.
func (r *ProductRepository) ListProductsWithoutPackSize(afterID uint, limit int) ([]database.Product, error) {
	if limit <= 0 {
		limit = 50
	}
	var products []database.Product
	result := r.DB.Select("id, barcode").
		Where("(pack_size IS NULL OR pack_size = '') AND barcode <> ''").
		Where("id > ?", afterID).
		Where(util.WhereDeletedIsNull).
		Order("id").
		Limit(limit).
		Find(&products)
	if result.Error != nil {
		return nil, result.Error
	}
	return products, nil
}

// SetProductPackSize stores the size of one pack of a product.
func (r *ProductRepository) SetProductPackSize(productID uint, packSize string) error {
	return r.DB.Model(&database.Product{}).Where("id = ?", productID).
		Update("pack_size", util.PackSizeLabel(packSize)).Error
}

// GetArchivedProductByID returns the product behind productID together with
// the caller's access checks (same household, private rows only their owner).
// Despite the name it does NOT restrict the row to archived products: the
// lookup is Unscoped so it sees soft-deleted rows at all, and DeleteProduct
// shares it and must accept active rows too. A restore of an already-active
// product therefore succeeds as a no-op instead of failing.
func (r *ProductRepository) GetArchivedProductByID(productID, userID uint) (database.Product, error) {
	if productID == 0 {
		return database.Product{}, gorm.ErrNotImplemented
	}

	var product database.Product
	getError := r.DB.Preload("StorageLocation").Unscoped().First(&product, productID)
	if getError.Error != nil {
		return database.Product{}, getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return database.Product{}, userErr.Error
	}

	if product.HouseholdID != user.HouseholdID {
		return database.Product{}, errors.ErrMismatcherUserID
	}
	if product.IsPrivate && product.UserID != userID {
		return database.Product{}, errors.ErrMismatcherUserID
	}
	return product, nil
}

// productSortColumns maps a caller-supplied sort key onto the column the
// database orders by. It is the allowlist of columns permitted in a
// request-driven ORDER BY clause: sort values are interpolated verbatim into
// SQL, so every value that may reach Order() is a value resolved through this
// map — the key is what a caller sends, the value is the real column name.
var productSortColumns = map[string]string{
	"product_name": "product_name",
	"created_at":   "created_at",
	"expire_at":    "expire_at",
	"scanned_at":   "scanned_at",
	"notified_at":  "notified_at",
	"barcode":      "barcode",
}

// SortEffectiveExpiry is the sort key that orders a product list by the
// effective expiry — the earlier of the printed expiry and the
// opened-shelf-life date. It has no column: the opened-shelf-life date needs
// date arithmetic that no two database engines spell the same way, so a list
// sorted by it is ordered in Go, with SortByEffectiveExpiry.
const SortEffectiveExpiry = "effective_expire_at"

// productSort is a resolved ordering: the ORDER BY the database applies, and
// what still has to happen to the rows in Go.
type productSort struct {
	clause            string
	byEffectiveExpiry bool
	descending        bool
}

// resolveSortClause maps a caller-supplied sort key and direction onto a safe
// ordering. A column the database can sort by becomes an ORDER BY clause; the
// effective expiry becomes the id order that makes the Go sort stable, and the
// resolved sort says the rows still have to be sorted in Go. Empty values fall
// back to the defaults; any value outside the allowlist returns
// ErrDatabaseInvalidSortParameter rather than silently falling back, so misuse
// stays visible instead of masquerading as a working sort. It lives here, at
// the single Order() call site, so both the web and the API entry points are
// covered by one guard.
//
// A column sort breaks its ties by id. Two products that share the sort key
// would otherwise come back in whatever order the database finds them, and a
// client that walks the pages by offset would then skip and repeat rows
// instead of reading every one of them exactly once. The effective expiry
// breaks its ties the same way: the id order is the order the projections
// arrive in, and the Go sort keeps it.
func resolveSortClause(sortValue, orderValue string) (productSort, error) {
	if sortValue == "" {
		sortValue = "product_name"
	}
	if orderValue == "" {
		orderValue = "asc"
	}
	direction := strings.ToUpper(orderValue)
	if direction != "ASC" && direction != "DESC" {
		return productSort{}, errors.ErrDatabaseInvalidSortParameter
	}
	descending := direction == "DESC"

	if sortValue == SortEffectiveExpiry {
		return productSort{clause: "id " + direction, byEffectiveExpiry: true, descending: descending}, nil
	}

	column, ok := productSortColumns[sortValue]
	if !ok {
		return productSort{}, errors.ErrDatabaseInvalidSortParameter
	}
	return productSort{clause: column + " " + direction + ", id " + direction, descending: descending}, nil
}

// applyEffectiveExpirySort orders the rows the way the resolved sort asks for.
func applyEffectiveExpirySort(products []database.Product, sort productSort) {
	if sort.descending {
		SortByEffectiveExpiryDescending(products)
		return
	}
	SortByEffectiveExpiry(products)
}

// searchProductsQuery builds the shared predicate for both search entry points:
// the API's full-row search and the products page's projection search. Keeping
// it in one place means a new search field cannot be added to one and forgotten
// in the other. The count of the matches is built from it as well, so it
// cannot report more matches than the search returns.
//
// The household comes from the same resolution every other read uses, so a user
// without a household is an account in error: the API answers 400 and the
// products page reports the failed fetch. The search used to answer an empty
// list for such a user, which hid the broken account behind a blank page.
func (r *ProductRepository) searchProductsQuery(queryParam SearchParameterEnum, queryValue string, userID uint) (*gorm.DB, error) {
	query, err := r.activeProductsQuery(userID)
	if err != nil {
		return nil, err
	}

	likeValue := fmt.Sprintf("%%%s%%", queryValue)
	switch queryParam {
	case ProductName:
		query = query.Where("product_name LIKE ?", likeValue)
	case Barcode:
		query = query.Where("barcode LIKE ?", likeValue)
	default:
		return nil, errors.ErrDatabaseInvalidSearchParameter
	}
	return query, nil
}

// SearchProducts returns one page of the products that match the search.
//
// A column sort is paged by the database. The effective expiry has no sortable
// column, so those matches are resolved as narrow projections, ordered in Go
// and only the rows of the page are hydrated.
func (r *ProductRepository) SearchProducts(queryParam SearchParameterEnum, queryValue, sortValue, orderValue string, userID uint, limit, offset int) ([]database.Product, error) {
	sort, sortErr := resolveSortClause(sortValue, orderValue)
	if sortErr != nil {
		return []database.Product{}, sortErr
	}

	if sort.byEffectiveExpiry {
		products, searchErr := r.searchProductsByEffectiveExpiry(queryParam, queryValue, sort, userID, limit, offset)
		if searchErr != nil {
			return []database.Product{}, searchErr
		}
		return products, nil
	}

	query, queryErr := r.searchProductsQuery(queryParam, queryValue, userID)
	if queryErr != nil {
		return []database.Product{}, queryErr
	}

	var foundProducts []database.Product
	listQuery := applyPage(query.Preload("StorageLocation").Order(sort.clause), limit, offset)
	if findErr := listQuery.Find(&foundProducts); findErr.Error != nil {
		return []database.Product{}, findErr.Error
	}
	return foundProducts, nil
}

// searchProductsByEffectiveExpiry returns the page of the matches that starts
// at the offset, ordered by the effective expiry.
//
// The matches are resolved as narrow projections and ordered in Go, because the
// effective expiry has no sortable column; only the rows of the page are
// hydrated as full rows.
func (r *ProductRepository) searchProductsByEffectiveExpiry(queryParam SearchParameterEnum, queryValue string, sort productSort, userID uint, limit, offset int) ([]database.Product, error) {
	query, queryErr := r.searchProductsQuery(queryParam, queryValue, userID)
	if queryErr != nil {
		return nil, queryErr
	}

	projections, projectionErr := r.searchProductProjections(query, sort)
	if projectionErr != nil {
		return nil, projectionErr
	}

	page := pageWindow(projections, limit, offset)
	ids := make([]uint, 0, len(page))
	for i := range page {
		ids = append(ids, page[i].ID)
	}

	rows, err := r.hydrateProductsByIDs(userID, ids)
	if err != nil {
		return nil, err
	}
	return orderProductsByID(rows, ids), nil
}

// CountSearchProducts counts the products the search matches. The count uses
// the predicate of the search itself — the household's products the caller may
// see, minus the archived ones, that match the searched value — so it cannot
// describe a different set than the search returns. It counts in SQL and does
// not load the matches into memory.
//
// The count runs on a query of its own: a statement that already carries a
// LIMIT would cap the count at the page size, and one that carries an OFFSET
// would skip the count's own row and answer zero.
func (r *ProductRepository) CountSearchProducts(queryParam SearchParameterEnum, queryValue string, userID uint) (int, error) {
	query, queryErr := r.searchProductsQuery(queryParam, queryValue, userID)
	if queryErr != nil {
		return 0, queryErr
	}

	var count int64
	if err := query.Model(&database.Product{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

// SearchProductProjections is the products page's search: same predicate and the
// same whitelisted ORDER BY as SearchProducts, narrowed to
// productProjectionColumns and optionally to one storage location (0 = every
// location), so a search inside a location filter narrows instead of
// shadowing. It deliberately returns every match (no SQL LIMIT) because the
// caller still has to apply the status filter before it knows which rows make
// up the page.
func (r *ProductRepository) SearchProductProjections(
	queryParam SearchParameterEnum, queryValue, sortValue, orderValue string, userID, locationID uint,
) ([]database.Product, error) {
	query, queryErr := r.searchProductsQuery(queryParam, queryValue, userID)
	if queryErr != nil {
		return nil, queryErr
	}
	if locationID != 0 {
		query = query.Where("storage_location_id = ?", locationID)
	}

	sort, sortErr := resolveSortClause(sortValue, orderValue)
	if sortErr != nil {
		return nil, sortErr
	}

	return r.searchProductProjections(query, sort)
}

// searchProductProjections resolves the matches as narrow projections and
// orders them the way the resolved sort asks for.
func (r *ProductRepository) searchProductProjections(query *gorm.DB, sort productSort) ([]database.Product, error) {
	var foundProducts []database.Product
	if err := query.Select(productProjectionColumns).Order(sort.clause).Find(&foundProducts).Error; err != nil {
		return nil, err
	}
	if sort.byEffectiveExpiry {
		applyEffectiveExpirySort(foundProducts, sort)
	}
	return foundProducts, nil
}

func (r *ProductRepository) CreateProduct(userID uint, product *database.Product) error {
	// POST /api/v1/products binds the raw model, so OpenedAt and
	// DaysAfterOpening arrive unvalidated here — unlike PATCH, which validates
	// before it reaches the repository. The bound is not cosmetic:
	// effectiveExpiryCandidateScope prunes with maxDaysAfterOpening, so a row
	// with days_after_opening > 365 can fall inside the expiring-soon window
	// yet be pruned out of it — silently missing from the list and from
	// expiry notifications. Validate at the repository so every create path
	// is covered.
	if err := validateOpenedLifecycle(product.OpenedAt, product.DaysAfterOpening); err != nil {
		return err
	}

	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return err
	}

	product.HouseholdID = user.HouseholdID
	product.UserID = userID
	// The barcode is normalised in one place, like the pack size: every path
	// stores the same trimmed barcode, so the duplicate rule compares equal
	// barcodes whatever wrote the row.
	product.Barcode = database.NormalizeBarcode(product.Barcode)
	// The pack size is free text that several write paths fill, so it is
	// normalised in one place: every path stores the same collapsed, capped
	// label.
	product.PackSize = util.PackSizeLabel(product.PackSize)
	// The name is free text too, and its column is a bounded varchar: a name
	// that exceeds it is refused by the database rather than stored. Every
	// path stores the same collapsed, capped label.
	product.ProductName = util.ProductNameLabel(product.ProductName)
	product.Quantity = util.ClampProductQuantity(product.Quantity)
	createErr := r.DB.Create(&product)
	return createErr.Error
}

func (r *ProductRepository) GetProductBySyncID(userID uint, clientSyncID string) (database.Product, error) {
	if clientSyncID == "" {
		return database.Product{}, gorm.ErrRecordNotFound
	}
	var product database.Product
	// The household is resolved in a subquery rather than a second round trip:
	// the offline sync calls this once per queued item, and a separate user
	// fetch doubled the queries on the endpoint's hottest new path.
	// Unscoped: idx_products_household_clientsyncid spans soft-deleted rows, so
	// an archived product with this sync id has to resolve here — a miss would
	// send the caller into an insert that violates the index.
	//
	// The Preload is not optional: the callers answer a replay with
	// NewProductResponse, which projects product.StorageLocation into the body.
	if err := r.DB.Unscoped().Preload("StorageLocation").
		Where("household_id = (SELECT household_id FROM users WHERE id = ? AND deleted_at IS NULL) AND client_sync_id = ?", userID, clientSyncID).
		First(&product).Error; err != nil {
		return database.Product{}, err
	}
	return product, nil
}

// CreateProductsBulk writes a batch of imported products for one user in a
// single transaction. The household is resolved once and stamped on every row.
// Any storage location named by NewLocationName that the household does not have
// yet is created inside that same transaction, so a failed insert cannot leave
// orphaned locations behind. The returned slice holds the names of the locations
// this call created, in creation order; an empty slice means none were needed.
//
// A CSV naming more than util.CsvImportMaxNewLocations distinct new locations is
// rejected whole with errors.ErrImportTooManyLocations. Storage locations are
// permanent rows the products list, home and product-detail renders all load
// without a limit, so letting one upload add thousands of them degrades every
// later page load.
func (r *ProductRepository) CreateProductsBulk(userID uint, rows []ImportedProduct) ([]string, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}

	// Counted outside the transaction: it only seeds the SortOrder of the new
	// locations, so holding the write transaction open for it buys nothing.
	var existingCount int64
	if countErr := r.DB.Model(&database.StorageLocation{}).Where(util.QueryHouseholdId, user.HouseholdID).Count(&existingCount).Error; countErr != nil {
		return nil, countErr
	}

	var created []string
	transactionErr := r.DB.Transaction(func(tx *gorm.DB) error {
		createdIDs, locationNames, createErr := createImportedStorageLocations(tx, user.HouseholdID, int(existingCount), rows)
		if createErr != nil {
			return createErr
		}
		created = locationNames

		products := make([]database.Product, len(rows))
		for i := range rows {
			rows[i].Product.HouseholdID = user.HouseholdID
			rows[i].Product.UserID = userID
			rows[i].Product.PackSize = util.PackSizeLabel(rows[i].Product.PackSize)
			rows[i].Product.ProductName = util.ProductNameLabel(rows[i].Product.ProductName)
			rows[i].Product.Quantity = util.ClampProductQuantity(rows[i].Product.Quantity)
			products[i] = rows[i].Product
			if locationID, ok := createdIDs[strings.ToLower(rows[i].NewLocationName)]; ok {
				products[i].StorageLocationID = &locationID
			}
		}
		return tx.CreateInBatches(&products, 100).Error
	})
	if transactionErr != nil {
		return nil, transactionErr
	}
	return created, nil
}

// createImportedStorageLocations creates one storage location per distinct
// NewLocationName. It returns the id of each, keyed by the lowercased name so
// rows spelled differently still share one location, plus the created names in
// creation order. New locations sort after the existingCount locations the
// household already has. Exceeding util.CsvImportMaxNewLocations distinct names
// fails with errors.ErrImportTooManyLocations rather than truncating the file.
func createImportedStorageLocations(tx *gorm.DB, householdID uint, existingCount int, rows []ImportedProduct) (map[string]uint, []string, error) {
	ids := make(map[string]uint)
	locations := make([]database.StorageLocation, 0)
	for i := range rows {
		name := strings.TrimSpace(rows[i].NewLocationName)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, exists := ids[key]; exists {
			continue
		}
		if len(locations) >= util.CsvImportMaxNewLocations {
			return nil, nil, errors.ErrImportTooManyLocations
		}
		ids[key] = 0
		locations = append(locations, database.StorageLocation{
			HouseholdID: householdID,
			Name:        name,
			Icon:        rows[i].NewLocationIcon,
			SortOrder:   existingCount + len(locations),
		})
	}
	if len(locations) == 0 {
		return ids, nil, nil
	}

	// One batched insert instead of one round trip per name: GORM fills the
	// primary keys in place, which is what turns the placeholder ids above into
	// the ids the products are stamped with.
	if err := tx.CreateInBatches(&locations, 100).Error; err != nil {
		return nil, nil, err
	}
	names := make([]string, 0, len(locations))
	for i := range locations {
		ids[strings.ToLower(locations[i].Name)] = locations[i].ID
		names = append(names, locations[i].Name)
	}
	return ids, names, nil
}

func (r *ProductRepository) UpdateProduct(productID uint, userID uint, product *database.ProductDTOPatch) error {
	if productID == 0 {
		return gorm.ErrNotImplemented
	}

	if err := validateOpenedLifecycle(product.OpenedAt, product.DaysAfterOpening); err != nil {
		return err
	}

	var dbProduct database.Product
	getError := r.DB.First(&dbProduct, productID)
	if getError.Error != nil {
		return getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return userErr.Error
	}

	if dbProduct.HouseholdID != user.HouseholdID {
		return errors.ErrMismatcherUserID
	}
	if dbProduct.IsPrivate && dbProduct.UserID != userID {
		return errors.ErrMismatcherUserID
	}

	if dbProduct.UserID == 0 {
		dbProduct.UserID = userID
	}

	if product.Has("productName") {
		dbProduct.ProductName = product.ProductName
	}
	if product.Has("categories") {
		dbProduct.Categories = product.Categories
	}
	if product.Has("countries") {
		dbProduct.Countries = product.Countries
	}
	if product.Has("imageUrl") {
		dbProduct.ImageURL = product.ImageURL
	}
	if product.Has("expireAt") {
		dbProduct.ExpireAt = product.ExpireAt
	}
	if product.Has("quantity") {
		dbProduct.Quantity = product.Quantity
	}
	if product.Has("packSize") {
		dbProduct.PackSize = util.PackSizeLabel(product.PackSize)
	}
	if product.Has("storageLocationId") {
		dbProduct.StorageLocationID = product.StorageLocationID
	}
	if product.Has("notificationLeadDays") {
		dbProduct.NotificationLeadDays = product.NotificationLeadDays
	}
	if product.Has("minStockAmount") {
		dbProduct.MinStockAmount = product.MinStockAmount
	}
	if product.Has("isPrivate") {
		dbProduct.IsPrivate = product.IsPrivate
	}
	if product.Has("priceOverride") {
		dbProduct.PriceOverride = product.PriceOverride
	}
	if product.Has("openedAt") {
		dbProduct.OpenedAt = product.OpenedAt
	}
	if product.Has("daysAfterOpening") {
		dbProduct.DaysAfterOpening = product.DaysAfterOpening
	}
	if product.Has("categoryKey") || product.Has("categories") {
		resolver, resolveErr := LoadCategoryResolver(r.DB)
		if resolveErr != nil {
			return resolveErr
		}
		if product.Has("categoryKey") {
			categoryKey, categoryErr := CanonicalCategoryKey(resolver, product.CategoryKey)
			if categoryErr != nil {
				return categoryErr
			}
			dbProduct.CategoryKey = categoryKey
		} else if key, ok := resolver.ResolveKey(dbProduct.Categories); ok {
			// The free text names the canonical category, so a change of the
			// text changes the category unless the request named one itself.
			dbProduct.CategoryKey = &key
		} else {
			dbProduct.CategoryKey = nil
		}
		// A request that assigns a category and states no date leaves the
		// product without one, so the household's default for that category
		// fills it — the rule the create paths follow.
		if !product.Has("expireAt") {
			defaults, defaultsErr := NewHouseholdCategoryRepository(r.DB).Defaults(dbProduct.HouseholdID)
			if defaultsErr != nil {
				return defaultsErr
			}
			ApplyHouseholdShelfLife(&dbProduct, defaults, dbProduct.CategoryKey)
		}
	}

	saveResult := r.DB.Save(&dbProduct)
	return saveResult.Error
}

// validateOpenedLifecycle enforces bounds on the OpenedAt / DaysAfterOpening
// fields supplied by the client on PATCH. A future OpenedAt would let a user
// push a product's effective expiry arbitrarily far into the future and
// suppress notifications; an out-of-range DaysAfterOpening would either be
// ignored silently (when <= 0) or produce a nonsense shelf life.
func validateOpenedLifecycle(openedAt *time.Time, daysAfterOpening *int) error {
	if openedAt != nil && openedAt.After(time.Now().Add(24*time.Hour)) {
		return errors.ErrInvalidOpenedLifecycle
	}
	if daysAfterOpening != nil && (*daysAfterOpening < 0 || *daysAfterOpening > maxDaysAfterOpening) {
		return errors.ErrInvalidOpenedLifecycle
	}
	return nil
}

// ValidateOpenedLifecycle is the exported form of the create/update guard so
// the backup restore path applies the same bounds instead of trusting the file.
func ValidateOpenedLifecycle(openedAt *time.Time, daysAfterOpening *int) error {
	return validateOpenedLifecycle(openedAt, daysAfterOpening)
}

// errMutationAlreadyClaimed unwinds the transaction that lost the race for a
// clientMutationId, so the caller can report a replay instead of an error.
var errMutationAlreadyClaimed = stderrors.New("client mutation id already applied")

// errMutationClaimRefused reports a conflict on the claim insert that no claim
// row explains — MySQL's INSERT IGNORE swallows a malformed value the same way
// it swallows a duplicate, and treating that as a replay would drop a real delta.
var errMutationClaimRefused = stderrors.New("client mutation id was not claimable and no claim exists")

// UpdateProductQuantity applies delta to the product and reports it as
// (deleted, applied, err). applied is false when the mutation id was already
// applied, so callers can skip side effects a replay must not repeat twice.
// Claims outlive this call and expire through the claim sweep in proviant.go,
// which is the only thing that bounds the table: pruning per product here would
// never run for a product whose last mutation was its only claim.
// The count stays inside the range a product may hold: the delta is clamped
// there, so no path can write a count the create and the patch would refuse.
func (r *ProductRepository) UpdateProductQuantity(productID uint, userID uint, delta int, mutationID *string) (bool, applied bool, err error) {
	if productID == 0 {
		return false, false, gorm.ErrNotImplemented
	}

	// One column: the count moves through the statement below, so the row is
	// read here only to answer "is this the caller's product".
	var dbProduct database.Product
	getError := r.DB.Unscoped().Select("household_id").First(&dbProduct, productID)
	if getError.Error != nil {
		return false, false, getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return false, false, userErr.Error
	}

	if dbProduct.HouseholdID != user.HouseholdID {
		return false, false, errors.ErrMismatcherUserID
	}

	if mutationID != nil && strings.TrimSpace(*mutationID) == "" {
		mutationID = nil
	}

	// The claim and the count share one transaction: a failure between
	// the two statements would leave the delta claimed and unapplied, and
	// every retry of it would be deduped into silence.
	// A conflict is only believed once the claim row behind it exists:
	// MariaDB renders this as INSERT IGNORE, which reports a value it
	// refused to store as zero rows exactly like a duplicate.
	moveErr := r.DB.Transaction(func(tx *gorm.DB) error {
		if mutationID != nil {
			claim := database.ProductMutationClaim{ProductID: productID, MutationID: *mutationID}
			claimResult := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
			if claimResult.Error != nil {
				return claimResult.Error
			}
			if claimResult.RowsAffected == 0 {
				var claimed database.ProductMutationClaim
				if err := tx.First(&claimed, "product_id = ? AND mutation_id = ?", productID, *mutationID).Error; err != nil {
					return errMutationClaimRefused
				}
				return errMutationAlreadyClaimed
			}
		}
		return tx.Unscoped().Model(&database.Product{}).Where("id = ?", productID).
			Updates(map[string]any{
				// The count stays inside the range a product may hold: a delta
				// that would push it below zero lands on zero, one that would
				// push it past the maximum lands on the maximum. The bounds are
				// spelled as a CASE and not as MIN or LEAST because no scalar
				// minimum is portable across SQLite, MySQL and PostgreSQL.
				"quantity": gorm.Expr(
					"CASE WHEN quantity + ? < 0 THEN 0 WHEN quantity + ? > ? THEN ? ELSE quantity + ? END",
					delta, delta, util.MaxProductQuantity, util.MaxProductQuantity, delta),
			}).Error
	})
	if stderrors.Is(moveErr, errMutationAlreadyClaimed) {
		return false, false, nil
	}
	if moveErr != nil {
		return false, false, moveErr
	}

	var current database.Product
	if readErr := r.DB.Unscoped().First(&current, productID).Error; readErr != nil {
		if stderrors.Is(readErr, gorm.ErrRecordNotFound) {
			return true, true, nil
		}
		return false, false, readErr
	}
	if current.Quantity > 0 {
		return false, true, nil
	}

	// The guard belongs in the DELETE as well as in the read above: an
	// increment landing between that read and this statement would otherwise be
	// lost along with the row, leaving it archived with a positive count. Only a
	// row still at zero is eligible, so a concurrent increment cancels the delete
	// instead of being discarded.
	deleteResult := r.DB.Unscoped().
		Where("id = ? AND quantity <= 0", productID).
		Delete(&database.Product{})
	if deleteResult.Error != nil {
		return true, true, deleteResult.Error
	}
	if deleteResult.RowsAffected > 0 {
		return true, true, nil
	}
	// The guard refused the row, which means one of two things. A concurrent
	// increment raised the count and the product survives. Or a concurrent call
	// already hard-deleted the row, in which case this delta was part of the same
	// zeroing and the product really is gone — reading back tells them apart.
	var survivor database.Product
	if readErr := r.DB.Unscoped().First(&survivor, productID).Error; readErr != nil {
		if stderrors.Is(readErr, gorm.ErrRecordNotFound) {
			return true, true, nil
		}
		return false, true, readErr
	}
	return false, true, nil
}

func (r *ProductRepository) DeleteProduct(productID uint, userID uint, archiveOnly bool) error {
	_, getError := r.GetArchivedProductByID(productID, userID)
	if getError != nil {
		return getError
	}

	var deleteResult *gorm.DB
	if archiveOnly {
		deleteResult = r.DB.Delete(&database.Product{}, productID)
	} else {
		deleteResult = r.DB.Unscoped().Delete(&database.Product{}, productID)
	}
	return deleteResult.Error
}

func (r *ProductRepository) RestoreProduct(productID, userID uint) error {
	product, getError := r.GetArchivedProductByID(productID, userID)
	if getError != nil {
		return getError
	}

	product.DeletedAt = gorm.DeletedAt{}
	product.RemovalReason = ""
	product.WasteCause = ""

	saveResult := r.DB.Save(&product)
	return saveResult.Error
}

func (r *ProductRepository) BulkRestoreProducts(productIDs []uint, userID uint) []BulkOperationError {
	bulkErrors := []BulkOperationError{}
	for _, productID := range productIDs {
		product, getError := r.GetArchivedProductByID(productID, userID)
		if getError != nil {
			// Without this continue the zero-value product below would be
			// Saved: gorm writes a primary key of 0 as a fresh INSERT, so a
			// single unknown or foreign id would create a blank product row
			// (empty name, household 0) on every attempt.
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
			continue
		}
		product.DeletedAt = gorm.DeletedAt{}
		product.RemovalReason = ""
		product.WasteCause = ""
		saveResult := r.DB.Save(&product)
		if saveResult.Error != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, saveResult.Error})
		}
	}
	return bulkErrors
}

func (r *ProductRepository) SetProductExpireAt(productID uint, userID uint, expireAt database.Timestamp) error {
	var dbProduct database.Product
	getError := r.DB.First(&dbProduct, productID)
	if getError.Error != nil {
		return getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return userErr.Error
	}

	if dbProduct.HouseholdID != user.HouseholdID {
		return errors.ErrMismatcherUserID
	}

	dbProduct.ExpireAt = time.Time(expireAt.Timestamp)
	saveResult := r.DB.Save(&dbProduct)
	return saveResult.Error
}

func (r *ProductRepository) SetProductNotifiedAt(productID uint) error {
	var dbProduct database.Product
	getError := r.DB.First(&dbProduct, productID)
	if getError.Error != nil {
		return getError.Error
	}

	dbProduct.NotifiedAt = time.Now()
	saveResult := r.DB.Save(&dbProduct)
	return saveResult.Error
}

// MarkProductOpened sets the OpenedAt timestamp on a product. Returns the
// current product, the previous OpenedAt value (nil if unset), a `changed`
// flag (true if the DB row was updated by this call, false if the open
// was rejected by the conflict guard or auth check), and any error.
// When `changed` is false and `err` is nil, the row is unchanged from its
// pre-call state and the caller should surface a 409 to the user; only
// re-invoke with force=true after the user confirms.
//
// The write is a single conditional UPDATE keyed on (id, household, and
// (opened_at IS NULL OR force)) so two concurrent first-time opens cannot
// both return 200, and so authorization is re-checked at the SQL layer
// rather than trusting a stale read. On RowsAffected=0 we re-SELECT to
// distinguish "already opened" (return changed=false) from "not found /
// not authorized" (return ErrMismatcherUserID or gorm.ErrRecordNotFound).
func (r *ProductRepository) MarkProductOpened(productID, userID uint, openedAt time.Time, force bool) (database.Product, *time.Time, bool, error) {
	var user authentication.User
	if userErr := r.DB.First(&user, userID); userErr.Error != nil {
		return database.Product{}, nil, false, userErr.Error
	}

	conditionClause := "opened_at IS NULL"
	if force {
		conditionClause = "1 = 1"
	}

	tx := r.DB.Exec(
		"UPDATE products SET opened_at = ? WHERE id = ? AND household_id = ? "+
			"AND (is_private = 0 OR user_id = ?) AND "+conditionClause,
		openedAt, productID, user.HouseholdID, userID,
	)
	if tx.Error != nil {
		return database.Product{}, nil, false, tx.Error
	}

	if tx.RowsAffected == 0 {
		// Either the row is not visible to this user (not in household,
		// or private and not owned), or the open-conflict guard rejected
		// the write. Re-read to tell the two cases apart.
		var dbProduct database.Product
		if getError := r.DB.First(&dbProduct, productID); getError.Error != nil {
			return database.Product{}, nil, false, getError.Error
		}
		if dbProduct.HouseholdID != user.HouseholdID ||
			(dbProduct.IsPrivate && dbProduct.UserID != userID) {
			return database.Product{}, nil, false, errors.ErrMismatcherUserID
		}
		return dbProduct, dbProduct.OpenedAt, false, nil
	}

	var updated database.Product
	if getError := r.DB.First(&updated, productID); getError.Error != nil {
		return database.Product{}, nil, false, getError.Error
	}
	return updated, updated.OpenedAt, true, nil
}

// GetProductsExpired returns the user's products whose effective expiry lies in
// the past, matching GetExpiredProductsCount. A zero effective expiry means the
// product has no expiry set — it is never expired.
func (r *ProductRepository) GetProductsExpired(userID uint) ([]*database.Product, error) {
	userProducts, getBulkErr := r.GetUserProductsBulk(userID, 0)
	if getBulkErr != nil {
		return []*database.Product{}, getBulkErr
	}

	var expiredProducts []*database.Product
	timestamp := time.Now()
	for idx := range userProducts {
		effective := userProducts[idx].EffectiveExpireAt()
		if !effective.IsZero() && effective.Before(timestamp) {
			expiredProducts = append(expiredProducts, &userProducts[idx])
		}
	}
	return expiredProducts, nil
}

// GetExpiredProductsCount returns how many of the user's active products are
// already past their effective expiry. It follows the same shape as
// GetActiveExpiryCounts: a one-sided SQL prune of the rows that cannot possibly
// be expired, then the effective-date verdict in Go, because the date arithmetic
// (opened_at + days_after_opening) has no portable SQL form.
//
// The prune deliberately also matches rows with a zero expire_at, which the Go
// loop below skips — a product with no expiry set is never expired.
func (r *ProductRepository) GetExpiredProductsCount(userID uint, now time.Time) (int, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return 0, err
	}

	var rows []expiryCandidate
	err = r.DB.Model(&database.Product{}).
		Select("expire_at, opened_at, days_after_opening").
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Where(
			"(expire_at <= ?) OR "+
				"(opened_at IS NOT NULL AND days_after_opening > 0 AND opened_at <= ?)",
			now, now,
		).
		Find(&rows).Error
	if err != nil {
		return 0, err
	}

	count := 0
	for i := range rows {
		effective := database.EffectiveExpireAt(rows[i].ExpireAt, rows[i].OpenedAt, rows[i].DaysAfterOpening)
		if !effective.IsZero() && effective.Before(now) {
			count++
		}
	}
	return count, nil
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
func (r *ProductRepository) GetFrequentlyBoughtProducts(userID uint, limit int) ([]apiModel.FrequentProduct, error) {
	householdID, err := r.getUserHouseholdID(userID)
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

// GetActiveProductsCount counts the user's active products in SQL. It used to
// load every active row plus a StorageLocation preload just to take len(), which
// made the stats endpoint scale with the household's full stock rather than with
// the handful of numbers it returns.
//
// It counts the same set the product list returns — the household's rows the
// caller may see, minus the archived ones — so it can serve as the total of a
// paged list.
//
// The count runs on a query of its own: a statement that already carries a
// LIMIT would cap the count at the page size, and one that carries an OFFSET
// would skip the count's own row and answer zero.
func (r *ProductRepository) GetActiveProductsCount(userID uint) (int, error) {
	query, err := r.activeProductsQuery(userID)
	if err != nil {
		return 0, err
	}

	var count int64
	err = query.Model(&database.Product{}).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// GetArchivedProductsCount returns how many products the household has archived.
// It is the count-only sibling of GetUserArchivedProductsBulk, which materialises
// every archived row (and preloads StorageLocation) for callers that only need a
// number. It counts the same set the archived list returns, so it can serve as
// the total of a paged list.
//
// The count runs on a query of its own: a statement that already carries a
// LIMIT would cap the count at the page size, and one that carries an OFFSET
// would skip the count's own row and answer zero.
func (r *ProductRepository) GetArchivedProductsCount(userID uint) (int, error) {
	query, err := r.archivedProductsQuery(userID)
	if err != nil {
		return 0, err
	}

	var count int64
	err = query.Model(&database.Product{}).Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// GetUniqueArchivedProductsCount counts distinct barcodes among archived
// products. COUNT(DISTINCT barcode) returns the same number as grouping the
// rows in Go by barcode and counting the groups — an empty barcode is one group
// on both sides.
func (r *ProductRepository) GetUniqueArchivedProductsCount(userID uint) (int, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return 0, err
	}

	var count int64
	err = r.DB.Unscoped().Model(&database.Product{}).
		Scopes(r.privacyScope(userID)).
		Where(util.WhereDeletedIsNotNull).
		Where(util.QueryHouseholdId, householdID).
		Distinct("barcode").
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *ProductRepository) GetProductCategoryBreakdown(userID uint) (map[string]int, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return nil, err
	}

	// Only the categories column is read; the rows are still one per product,
	// but they no longer carry the full record or a StorageLocation preload.
	var products []database.Product
	err = r.DB.Model(&database.Product{}).
		Select("categories").
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Find(&products).Error
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for i := range products {
		raw := strings.TrimSpace(products[i].Categories)
		if raw == "" {
			counts["Uncategorized"]++
			continue
		}
		first, _, _ := strings.Cut(raw, ",")
		first = strings.TrimSpace(first)
		if idx := strings.Index(first, ":"); idx != -1 {
			first = strings.TrimSpace(first[idx+1:])
		}
		if first == "" {
			first = "Uncategorized"
		}
		counts[first]++
	}

	const maxCategories = 8
	if len(counts) <= maxCategories {
		return counts, nil
	}

	type catCount struct {
		name  string
		count int
	}
	cats := make([]catCount, 0, len(counts))
	for n, c := range counts {
		cats = append(cats, catCount{n, c})
	}
	sort.Slice(cats, func(i, j int) bool {
		return cats[i].count > cats[j].count
	})

	result := make(map[string]int, maxCategories+1)
	other := 0
	for i, c := range cats {
		if i < maxCategories {
			result[c.name] = c.count
		} else {
			other += c.count
		}
	}
	if other > 0 {
		result["Other"] = other
	}
	return result, nil
}

// GetExpiryTrend buckets the next 12 months of expiries, starting with the
// calendar month `now` falls in. The buckets follow the container's zone, so
// an expiry lands in the month the label next to it shows. Only the three
// columns EffectiveExpireAt reads are projected, so the query cost tracks the
// row count rather than the row width.
func (r *ProductRepository) GetExpiryTrend(userID uint, now time.Time) ([]apiModel.StatsMonthlyCount, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return nil, err
	}

	var products []database.Product
	err = r.DB.Model(&database.Product{}).
		Select("expire_at, opened_at, days_after_opening").
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Find(&products).Error
	if err != nil {
		return nil, err
	}

	monthKeys := util.UpcomingMonthKeys(now, 12)
	monthCounts := make(map[string]int, len(monthKeys))
	for _, key := range monthKeys {
		monthCounts[key] = 0
	}
	for i := range products {
		expiry := products[i].EffectiveExpireAt()
		if expiry.IsZero() {
			continue
		}
		month := util.MonthKey(expiry)
		if _, ok := monthCounts[month]; ok {
			monthCounts[month]++
		}
	}

	result := make([]apiModel.StatsMonthlyCount, 0, len(monthKeys))
	for _, key := range monthKeys {
		result = append(result, apiModel.StatsMonthlyCount{Month: key, Count: monthCounts[key]})
	}
	return result, nil
}

func (r *ProductRepository) GetExpiringSoonProducts(userID uint, days int, now time.Time) ([]apiModel.StatsExpiringProduct, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return nil, err
	}

	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfWindow := time.Date(now.Year(), now.Month(), now.Day()+days, 23, 59, 59, 999999999, now.Location())

	// Candidate scope is a one-sided prune: it only drops rows whose effective
	// expiry provably cannot reach the window, so the verdict below still runs
	// in Go through EffectiveExpireAt. Same pattern as GetExpiringSoonCount.
	//
	// The id and the foreign key are part of the projection because the
	// storage location is preloaded: without them the preload has neither a
	// key to collect nor the pointer to follow.
	var products []database.Product
	err = r.DB.Model(&database.Product{}).
		Select("id, storage_location_id, product_name, quantity, pack_size, expire_at, opened_at, days_after_opening").
		Preload("StorageLocation").
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Scopes(effectiveExpiryCandidateScope(startOfToday, endOfWindow)).
		Order("id").
		Find(&products).Error
	if err != nil {
		return nil, err
	}

	var result []apiModel.StatsExpiringProduct
	for i := range products {
		expirationTime := products[i].EffectiveExpireAt()
		if expirationTime.IsZero() {
			continue
		}
		if !expirationTime.Before(startOfToday) && !expirationTime.After(endOfWindow) {
			result = append(result, ExpiringProductStats(&products[i], expirationTime))
		}
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].ExpireAt < result[j].ExpireAt
	})
	return result, nil
}

// ExpiringProductStats projects a product onto the wire shape of the expiring
// list. The location is a flat name and icon pair: the list is a projection,
// not a product, so it carries no nested object. The id is there so the row
// can lead to the product it names.
//
// The backup export builds the same list from the rows it already carries, so
// the shape lives here once.
func ExpiringProductStats(product *database.Product, expirationTime time.Time) apiModel.StatsExpiringProduct {
	entry := apiModel.StatsExpiringProduct{
		ID:          product.ID,
		ProductName: product.ProductName,
		ExpireAt:    expirationTime.In(time.Local).Format(util.DefaultDateFormatParseStr),
		Quantity:    product.Quantity,
		PackSize:    product.PackSize,
	}
	if product.StorageLocation != nil {
		entry.StorageLocationID = &product.StorageLocation.ID
		entry.StorageLocationName = product.StorageLocation.Name
		entry.StorageLocationIcon = product.StorageLocation.Icon
	}
	return entry
}

// GetUseFirstProducts returns the household's active products ordered by the
// effective expiry, soonest first, with the storage location and the pack
// count of each. It answers "what do we eat next" for the whole pantry, not
// only for the products inside the warning window.
//
// The order is the one the products page uses: the narrow projection is sorted
// with SortByEffectiveExpiry and only the rows that make up the list are
// hydrated, so a product holds the same position in the tile and in the list.
// A limit of zero returns the whole pantry.
func (r *ProductRepository) GetUseFirstProducts(userID uint, limit int) ([]apiModel.StatsUseFirstProduct, error) {
	rows, err := r.getProductsByEffectiveExpiry(userID, limit, 0, false)
	if err != nil {
		return nil, err
	}
	return BuildUseFirstProducts(rows, limit), nil
}

// hydrateProductsByIDs loads the full rows for the given ids. The query binds
// one placeholder per id and SQLite caps the number of bound variables, so a
// list that covers the whole pantry is fetched in chunks.
func (r *ProductRepository) hydrateProductsByIDs(userID uint, ids []uint) ([]database.Product, error) {
	if len(ids) == 0 {
		return []database.Product{}, nil
	}
	if len(ids) <= bulkQueryChunkSize {
		return r.GetUserProductsByIDs(userID, ids)
	}

	var rows []database.Product
	for start := 0; start < len(ids); start += bulkQueryChunkSize {
		end := min(start+bulkQueryChunkSize, len(ids))
		chunk, chunkErr := r.GetUserProductsByIDs(userID, ids[start:end])
		if chunkErr != nil {
			return nil, chunkErr
		}
		rows = append(rows, chunk...)
	}
	return rows, nil
}

// BuildUseFirstProducts projects the rows of a use-first list onto the wire
// shape, ordered by the effective expiry, soonest first, with the products
// without a date last. The rows are copied, so the caller's slice keeps its
// own order. A limit of zero returns every row.
func BuildUseFirstProducts(products []database.Product, limit int) []apiModel.StatsUseFirstProduct {
	ordered := make([]database.Product, len(products))
	copy(ordered, products)
	SortByEffectiveExpiry(ordered)
	if limit > 0 && len(ordered) > limit {
		ordered = ordered[:limit]
	}

	result := make([]apiModel.StatsUseFirstProduct, 0, len(ordered))
	for i := range ordered {
		product := &ordered[i]
		entry := apiModel.StatsUseFirstProduct{
			ID:          product.ID,
			ProductName: product.ProductName,
			Quantity:    product.Quantity,
			PackSize:    product.PackSize,
		}
		if expiry := product.EffectiveExpireAt(); !expiry.IsZero() {
			entry.ExpireAt = expiry.In(time.Local).Format(util.DefaultDateFormatParseStr)
		}
		if product.StorageLocation != nil {
			entry.StorageLocationID = &product.StorageLocation.ID
			entry.StorageLocationName = product.StorageLocation.Name
			entry.StorageLocationIcon = product.StorageLocation.Icon
		}
		result = append(result, entry)
	}
	return result
}

func (r *ProductRepository) GetLastNotifiedProduct(householdID uint) (database.Product, error) {
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

func (r *ProductRepository) GetExpiringInDays(userID uint, days int) ([]database.Product, error) {
	householdID, err := r.getUserHouseholdID(userID)
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
	err = r.DB.Scopes(r.privacyScope(userID)).
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

func (r *ProductRepository) GetLastInsertedProduct(householdID uint) (database.Product, error) {
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

func (r *ProductRepository) UserHasProductAccess(userID uint, productID int) bool {
	if productID <= 0 {
		return false
	}

	var product database.Product
	getError := r.DB.Unscoped().First(&product, productID)
	if getError.Error != nil {
		return false
	}

	var user authentication.User
	getError = r.DB.First(&user, userID)
	if getError.Error != nil {
		return false
	}

	if product.HouseholdID != user.HouseholdID {
		return false
	}
	if product.IsPrivate && product.UserID != userID {
		return false
	}
	return true
}

func (r *ProductRepository) GetOpenFoodFactsCacheByBarcode(barcode string) (database.OpenFoodFactsCache, error) {
	var entry database.OpenFoodFactsCache
	result := r.DB.Where("barcode = ?", barcode).First(&entry)
	return entry, result.Error
}

// GetOpenFoodFactsCachesByBarcodes returns every cached Open Food Facts entry
// whose barcode is in the given set. It is the batch form of
// GetOpenFoodFactsCacheByBarcode, so a bulk import can resolve all of its
// fallback names with one query instead of one lookup per row.
func (r *ProductRepository) GetOpenFoodFactsCachesByBarcodes(barcodes []string) ([]database.OpenFoodFactsCache, error) {
	if len(barcodes) == 0 {
		return []database.OpenFoodFactsCache{}, nil
	}
	var entries []database.OpenFoodFactsCache
	for _, chunk := range chunkStrings(barcodes, bulkQueryChunkSize) {
		var batch []database.OpenFoodFactsCache
		queryErr := r.DB.Where("barcode IN ?", chunk).Find(&batch).Error
		if queryErr != nil {
			return []database.OpenFoodFactsCache{}, queryErr
		}
		entries = append(entries, batch...)
	}
	return entries, nil
}

func (r *ProductRepository) CreateOpenFoodFactsCache(entry *database.OpenFoodFactsCache) error {
	return r.DB.Create(entry).Error
}

func (r *ProductRepository) UpdateOpenFoodFactsCacheImageURL(barcode, imageURL string) error {
	return r.DB.Model(&database.OpenFoodFactsCache{}).Where("barcode = ?", barcode).Update("image_url", imageURL).Error
}

func (r *ProductRepository) GetOpenFoodFactsCacheWithoutStorageHint() ([]database.OpenFoodFactsCache, error) {
	var entries []database.OpenFoodFactsCache
	err := r.DB.Where("storage_hint = '' OR storage_hint IS NULL").Find(&entries).Error
	return entries, err
}

func (r *ProductRepository) UpdateOpenFoodFactsCacheStorageHint(barcode, storageHint string) error {
	return r.DB.Model(&database.OpenFoodFactsCache{}).Where("barcode = ?", barcode).Update("storage_hint", storageHint).Error
}

func (r *ProductRepository) GetOpenFoodFactsCacheWithRemoteImageURL() ([]database.OpenFoodFactsCache, error) {
	var entries []database.OpenFoodFactsCache
	err := r.DB.Where("image_url != '' AND image_url NOT LIKE '/product-images/%'").Find(&entries).Error
	return entries, err
}

func (r *ProductRepository) GetUserByID(userID uint) (authentication.User, error) {
	var user authentication.User
	selectErr := r.DB.First(&user, userID)
	return user, selectErr.Error
}

func (r *ProductRepository) GetUserHouseholdByID(userID uint) (uint, error) {
	var user authentication.User
	selectErr := r.DB.First(&user, userID)
	return user.HouseholdID, selectErr.Error
}

func (r *ProductRepository) GetHouseholdByID(householdID uint) (database.Household, error) {
	var household database.Household
	selectErr := r.DB.First(&household, householdID)
	return household, selectErr.Error
}

func (r *ProductRepository) GetUserActiveProductsFiltered(userID uint, from, to *time.Time) ([]database.Product, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	query := r.DB.Preload("StorageLocation").Scopes(r.privacyScope(userID)).Where(util.QueryHouseholdId, householdID).Where(util.WhereDeletedIsNull)

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

func (r *ProductRepository) GetUserArchivedProductsFiltered(userID uint, from, to *time.Time) ([]database.Product, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	query := r.DB.Preload("StorageLocation").Unscoped().Scopes(r.privacyScope(userID)).Where(util.WhereDeletedIsNotNull).Where(util.QueryHouseholdId, householdID)

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

func (r *ProductRepository) GetUsersByHouseholdID(householdID uint) ([]authentication.User, error) {
	var users []authentication.User
	err := r.DB.Where(util.QueryHouseholdId, householdID).Find(&users).Error
	return users, err
}

func (r *ProductRepository) GetExpiringSoonCount(userID uint, days int, now time.Time) (int, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return 0, err
	}

	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfWindow := time.Date(now.Year(), now.Month(), now.Day()+days, 23, 59, 59, 999999999, now.Location())

	// SQL filter mirrors GetExpiringInDays: either printed expiry is in the
	// window, or the row has an opened-shelf-life rule. Final effective-date
	// filter is applied in Go to keep the query portable.
	var rows []database.Product
	err = r.DB.Model(&database.Product{}).
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Scopes(effectiveExpiryCandidateScope(startOfToday, endOfWindow)).
		Find(&rows).Error
	if err != nil {
		return 0, err
	}

	return len(filterByEffectiveExpiryWindow(rows, startOfToday, endOfWindow)), nil
}

// GetActiveExpiryCounts returns how many of the user's active products are
// already past their effective expiry and how many expire within criticalDays.
// It exists for the products-page sidebar badge, which needs two integers and
// used to load every active product — full row set plus a StorageLocation
// preload — only to derive them.
//
// Only the three expiry columns are projected, and the effective-date
// classification runs in Go through database.EffectiveExpireAt. Doing it in SQL
// would need engine-specific date arithmetic on opened_at + days_after_opening,
// which the other expiry queries deliberately avoid too.
func (r *ProductRepository) GetActiveExpiryCounts(userID uint, now time.Time, criticalDays int) (expired, critical int, err error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return 0, 0, err
	}

	criticalBefore := now.AddDate(0, 0, criticalDays)

	// One-sided prune: rows whose latest possible effective expiry is beyond the
	// critical window cannot be counted. There is no lower bound — a product can
	// have been expired for years and still counts.
	var rows []expiryCandidate
	err = r.DB.Model(&database.Product{}).
		Select("expire_at, opened_at, days_after_opening").
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Where(
			"(expire_at <= ?) OR "+
				"(opened_at IS NOT NULL AND days_after_opening > 0 AND opened_at <= ?)",
			criticalBefore, criticalBefore,
		).
		Find(&rows).Error
	if err != nil {
		return 0, 0, err
	}

	for i := range rows {
		effective := database.EffectiveExpireAt(rows[i].ExpireAt, rows[i].OpenedAt, rows[i].DaysAfterOpening)
		if effective.IsZero() {
			continue
		}
		if effective.Before(now) {
			expired++
		} else if effective.Before(criticalBefore) {
			critical++
		}
	}
	return expired, critical, nil
}

// expiryCandidate is the narrow projection GetActiveExpiryCounts needs. The
// three fields are the whole input to database.EffectiveExpireAt, so the count
// cannot drift from the ranking that decides what a page renders.
type expiryCandidate struct {
	ExpireAt         time.Time
	OpenedAt         *time.Time
	DaysAfterOpening *int
}

// GetWasteThisMonth counts the waste events of the month. A waste event is one
// wasted product, full or partial. The savings ledger holds one record per
// waste, while the products table only knows the wastes that archived a
// product — a partial waste leaves the product active and would be invisible
// there, so the two sources would report different numbers for the same waste.
// The archived rows without a ledger entry are the wastes from before the
// ledger existed; they keep the count of a household that predates it.
func (r *ProductRepository) GetWasteThisMonth(userID uint, now time.Time) (int, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return 0, err
	}

	monthStart := util.MonthStart(now)
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Nanosecond)

	// The ledger is joined to the product it names, so the privacy scope still
	// applies: a member's private product is not part of another member's count.
	var ledgerCount int64
	err = r.DB.Table("savings_records AS s").
		Joins("JOIN products p ON p.id = s.product_id").
		Where("s.household_id = ?", householdID).
		Where("s.event_type = ?", database.SavingsEventTypeWasted).
		Where("s.created_at >= ?", monthStart).
		Where("s.created_at <= ?", monthEnd).
		Where("(p.is_private = 0 OR p.user_id = ?)", userID).
		Count(&ledgerCount).Error
	if err != nil {
		return 0, err
	}

	// An archived waste whose ledger entry landed in the same month is already
	// counted above; only the ones without such an entry are added here.
	var archivedCount int64
	err = r.DB.Unscoped().Model(&database.Product{}).
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNotNull).
		Where("deleted_at >= ?", monthStart).
		Where("deleted_at <= ?", monthEnd).
		Where("removal_reason = ?", database.RemovalReasonWasted).
		Where("NOT EXISTS (SELECT 1 FROM savings_records s WHERE s.product_id = products.id AND s.event_type = ? AND s.created_at >= ? AND s.created_at <= ?)",
			database.SavingsEventTypeWasted, monthStart, monthEnd).
		Count(&archivedCount).Error
	if err != nil {
		return 0, err
	}

	return int(ledgerCount + archivedCount), nil
}

// GetExpiringProductsByHousehold returns non-private products for a household
// whose effective expiry (earlier of printed ExpireAt and OpenedAt+DaysAfterOpening)
// falls in the window [startOfToday, endOfWindow].
func (r *ProductRepository) GetExpiringProductsByHousehold(householdID uint, daysAhead int) ([]database.Product, error) {
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

// GetProductsByHousehold returns all non-deleted products for a household.
func (r *ProductRepository) GetProductsByHousehold(householdID uint) ([]database.Product, error) {
	var products []database.Product
	result := r.DB.Where("household_id = ? AND deleted_at IS NULL AND is_private = 0", householdID).Find(&products)
	return products, result.Error
}

// GetHouseholdProductsForBackup returns every active household product
// including private rows. A backup exists to reproduce the household, so the
// privacy scope every user-facing query applies must not silently drop a
// member's private stock from the file.
func (r *ProductRepository) GetHouseholdProductsForBackup(householdID uint) ([]database.Product, error) {
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
func (r *ProductRepository) GetHouseholdArchivedProductsForBackup(householdID uint) ([]database.Product, error) {
	var products []database.Product
	err := r.DB.Preload("StorageLocation").Unscoped().
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNotNull).
		Find(&products).Error
	return products, err
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
func (r *ProductRepository) GetMatchableProductsByHousehold(householdID uint) ([]database.Product, error) {
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
func (r *ProductRepository) GetProductSetFingerprint(householdID uint) (string, error) {
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

func (r *ProductRepository) GetSubThresholdProducts(userID uint) ([]database.Product, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	err = r.DB.Preload("StorageLocation").
		Scopes(r.privacyScope(userID)).
		Where("household_id = ? AND deleted_at IS NULL AND quantity < min_stock_amount AND min_stock_amount > 0", householdID).
		Find(&products).Error
	if err != nil {
		return []database.Product{}, err
	}
	return products, nil
}

type MailDigestProductGroup struct {
	Today    []database.Product
	ThisWeek []database.Product
	NextWeek []database.Product
}

func (r *ProductRepository) GetExpiringProductsForMailDigest(householdID uint) (MailDigestProductGroup, error) {
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfToday := startOfToday.AddDate(0, 0, 1).Add(-time.Nanosecond)
	endOfWeek := startOfToday.AddDate(0, 0, 7).Add(-time.Nanosecond)
	endOfNextWeek := startOfToday.AddDate(0, 0, 14).Add(-time.Nanosecond)

	var allProducts []database.Product
	// SQL filter: either the printed expiry is within the 2-week window,
	// or the row has an opened-shelf-life rule. Final bucketing is done
	// in Go using EffectiveExpireAt() to keep the query portable across
	// SQLite and MariaDB.
	if err := r.DB.
		Where("household_id = ? AND deleted_at IS NULL AND is_private = 0", householdID).
		Scopes(effectiveExpiryCandidateScope(startOfToday, endOfNextWeek)).
		Find(&allProducts).Error; err != nil {
		return MailDigestProductGroup{}, err
	}

	var group MailDigestProductGroup
	for i := range allProducts {
		p := &allProducts[i]
		eff := p.EffectiveExpireAt()
		if eff.IsZero() || eff.Before(startOfToday) {
			continue
		}
		switch {
		case !eff.After(endOfToday):
			group.Today = append(group.Today, *p)
		case !eff.After(endOfWeek):
			group.ThisWeek = append(group.ThisWeek, *p)
		default:
			group.NextWeek = append(group.NextWeek, *p)
		}
	}

	sortEffectiveExpire := func(s []database.Product) {
		sort.Slice(s, func(i, j int) bool {
			ti, tj := s[i].EffectiveExpireAt(), s[j].EffectiveExpireAt()
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
	}
	sortEffectiveExpire(group.Today)
	sortEffectiveExpire(group.ThisWeek)
	sortEffectiveExpire(group.NextWeek)

	return group, nil
}

func (r *ProductRepository) ConsumeProduct(productID, userID uint) error {
	product, err := r.GetProductByID(productID, userID)
	if err != nil {
		return err
	}
	product.RemovalReason = database.RemovalReasonConsumed
	// Save and delete must succeed together — a non-transactional failure in
	// between would leave an active product already marked as consumed.
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&product).Error; err != nil {
			return err
		}
		return tx.Delete(&database.Product{}, productID).Error
	})
}

// ConsumeProductPartial removes up to quantity packs and archives the row when
// the request covers its current stock. A zero or negative quantity consumes
// the whole stock, preserving the recipe-cook contract.
func (r *ProductRepository) ConsumeProductPartial(product *database.Product, quantity int, mutationID *string) (int, bool, bool, error) {
	return r.removeProductPartial(product, quantity, database.RemovalReasonConsumed, mutationID, "")
}

// WasteProductPartial is the waste counterpart of ConsumeProductPartial.
func (r *ProductRepository) WasteProductPartial(product *database.Product, quantity int, mutationID *string, cause string) (int, bool, bool, error) {
	return r.removeProductPartial(product, quantity, database.RemovalReasonWasted, mutationID, cause)
}

// removeProductPartial compares data values only (quantity, removal_reason and
// deleted_at), never updated_at, whose driver representation is not stable on
// legacy rows. removal_reason also prevents a second request from claiming an
// archived row. A mutation claim shares the transaction with the stock change,
// so an offline replay cannot apply it twice. The cause is why the packs were
// wasted; it is written beside the removal reason when the waste archives the
// row.
func (r *ProductRepository) removeProductPartial(product *database.Product, quantity int, reason string, mutationID *string, cause string) (removed int, fullyRemoved bool, applied bool, err error) {
	if mutationID != nil && strings.TrimSpace(*mutationID) == "" {
		mutationID = nil
	}

	if mutationID == nil && quantity > 0 && quantity < product.Quantity {
		result := r.DB.Model(&database.Product{}).
			Where("id = ?", product.ID).
			Where("household_id = ?", product.HouseholdID).
			Where(util.WhereDeletedIsNull).
			Where(util.WhereRemovalReasonUnclaimed).
			Where("quantity > ?", quantity).
			Update("quantity", gorm.Expr("quantity - ?", quantity))
		if result.Error != nil {
			return 0, false, false, result.Error
		}
		if result.RowsAffected > 0 {
			return quantity, false, true, nil
		}
	}

	var txErr error
	for range 2 {
		removed, fullyRemoved, applied = 0, false, false
		txErr = r.DB.Transaction(func(tx *gorm.DB) error {
			if mutationID != nil {
				claim := database.ProductMutationClaim{ProductID: product.ID, MutationID: *mutationID}
				claimResult := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&claim)
				if claimResult.Error != nil {
					return claimResult.Error
				}
				if claimResult.RowsAffected == 0 {
					var claimed database.ProductMutationClaim
					if err := tx.First(&claimed, "product_id = ? AND mutation_id = ?", product.ID, *mutationID).Error; err != nil {
						return errMutationClaimRefused
					}
					return errMutationAlreadyClaimed
				}
			}

			var current database.Product
			if getErr := tx.Where("id = ? AND household_id = ?", product.ID, product.HouseholdID).First(&current).Error; getErr != nil {
				return getErr
			}
			if quantity > 0 && quantity < current.Quantity {
				result := tx.Model(&database.Product{}).
					Where("id = ?", current.ID).
					Where("household_id = ?", current.HouseholdID).
					Where(util.WhereDeletedIsNull).
					Where(util.WhereRemovalReasonUnclaimed).
					Where("quantity > ?", quantity).
					Update("quantity", gorm.Expr("quantity - ?", quantity))
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					return errors.ErrProductConcurrentModification
				}
				removed = quantity
				applied = true
				return nil
			}

			removed = current.Quantity
			fullyRemoved = true
			result := tx.Model(&database.Product{}).
				Where("id = ?", current.ID).
				Where("household_id = ?", current.HouseholdID).
				Where(util.WhereDeletedIsNull).
				Where(util.WhereRemovalReasonUnclaimed).
				Where("quantity = ?", current.Quantity).
				Updates(map[string]any{
					"removal_reason": reason,
					"waste_cause":    cause,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return errors.ErrProductConcurrentModification
			}
			if err := tx.Delete(&database.Product{}, current.ID).Error; err != nil {
				return err
			}
			applied = true
			return nil
		})
		if txErr != errors.ErrProductConcurrentModification {
			break
		}
	}
	if stderrors.Is(txErr, errMutationAlreadyClaimed) {
		return 0, false, false, nil
	}
	if txErr != nil {
		return 0, false, false, txErr
	}
	return removed, fullyRemoved, applied, nil
}

func (r *ProductRepository) WasteProduct(productID, userID uint, cause string) error {
	product, err := r.GetProductByID(productID, userID)
	if err != nil {
		return err
	}
	// Single transaction: the removal-reason update and the soft-delete must
	// succeed together, or neither lands. The unclaimed-row precondition fails
	// the write when a concurrent request already claimed the row, instead of
	// silently overwriting its removal reason.
	return r.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&database.Product{}).
			Where("id = ?", product.ID).
			Where("household_id = ?", product.HouseholdID).
			Where(util.WhereDeletedIsNull).
			Where(util.WhereRemovalReasonUnclaimed).
			Updates(map[string]any{
				"removal_reason": database.RemovalReasonWasted,
				"waste_cause":    cause,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return errors.ErrProductConcurrentModification
		}
		return tx.Delete(&database.Product{}, productID).Error
	})
}

func (r *ProductRepository) BulkConsumeProducts(productIDs []uint, userID uint) []BulkOperationError {
	bulkErrors := []BulkOperationError{}
	for _, productID := range productIDs {
		if err := r.ConsumeProduct(productID, userID); err != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, err})
		}
	}
	return bulkErrors
}

func (r *ProductRepository) BulkWasteProducts(productIDs []uint, userID uint, cause string) []BulkOperationError {
	bulkErrors := []BulkOperationError{}
	for _, productID := range productIDs {
		if err := r.WasteProduct(productID, userID, cause); err != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, err})
		}
	}
	return bulkErrors
}

type CalendarTokenRepositoryInterface interface {
	GetByToken(token string) (authentication.CalendarToken, error)
	DeleteByUserID(userID uint) error
	GetByUserID(userID uint) (authentication.CalendarToken, error)
	Create(ct *authentication.CalendarToken) error
	Update(ct *authentication.CalendarToken) error
}

var _ CalendarTokenRepositoryInterface = (*CalendarTokenRepository)(nil)

type CalendarTokenRepository struct {
	DB *gorm.DB
}

func NewCalendarTokenRepository(db *gorm.DB) *CalendarTokenRepository {
	return &CalendarTokenRepository{DB: db}
}

func (r *CalendarTokenRepository) GetByToken(token string) (authentication.CalendarToken, error) {
	var ct authentication.CalendarToken
	err := r.DB.Where("token = ?", token).First(&ct).Error
	if err != nil {
		return ct, err
	}
	if ct.ExpiresAt.Before(time.Now()) {
		return ct, errors.ErrTokenExpired
	}
	return ct, nil
}

func (r *CalendarTokenRepository) DeleteByUserID(userID uint) error {
	return r.DB.Where(util.QueryUserId, userID).Delete(&authentication.CalendarToken{}).Error
}

func (r *CalendarTokenRepository) GetByUserID(userID uint) (authentication.CalendarToken, error) {
	var ct authentication.CalendarToken
	err := r.DB.Where(util.QueryUserId, userID).First(&ct).Error
	return ct, err
}

func (r *CalendarTokenRepository) Create(ct *authentication.CalendarToken) error {
	return r.DB.Create(ct).Error
}

func (r *CalendarTokenRepository) Update(ct *authentication.CalendarToken) error {
	return r.DB.Save(ct).Error
}

func (r *ProductRepository) GetConsumedSamples(householdID, userID uint, barcode, name string, since time.Time) ([]database.Product, error) {
	if barcode == "" && name == "" {
		return nil, fmt.Errorf("GetConsumedSamples: barcode and name are both empty")
	}

	query := r.DB.Unscoped().
		Select("id, product_name, barcode, quantity, deleted_at, is_private, user_id, removal_reason").
		Where(util.WhereDeletedIsNotNull).
		Where(util.QueryHouseholdId, householdID).
		Where("deleted_at >= ?", since).
		Where("removal_reason = ?", database.RemovalReasonConsumed).
		Where("(is_private = ? OR user_id = ?)", false, userID)

	if barcode != "" {
		query = query.Where("barcode = ?", barcode)
	} else {
		query = query.Where("product_name = ?", name)
	}

	var products []database.Product
	if err := query.Order("deleted_at ASC").Find(&products).Error; err != nil {
		return nil, err
	}
	return products, nil
}
