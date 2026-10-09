package database

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"gorm.io/gorm"
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
	GetUserProductsByIDs(userID uint, ids []uint) ([]database.Product, error)
	GetUserArchivedProductsBulk(userID uint, limit int) ([]database.Product, error)
	GetActiveProductProjections(userID, locationID uint) ([]database.Product, error)
	GetArchivedProductProjections(userID uint) ([]database.Product, error)
	GetUserArchivedProductsByIDs(userID uint, ids []uint) ([]database.Product, error)
	GetUserProductsBulkByBarcode(userID uint, barcode int) ([]database.Product, error)
	GetUserProductsBulkByBarcodes(userID uint, barcodes []string) ([]database.Product, error)
	GetProductByID(productID, userID uint) (database.Product, error)
	GetProductIdentity(productID, userID uint) (database.Product, error)
	GetArchivedProductByID(productID, userID uint) (database.Product, error)
	SearchProducts(queryParam SearchParameterEnum, queryValue, sortValue, orderValue string, userID uint) ([]database.Product, error)
	SearchProductProjections(queryParam SearchParameterEnum, queryValue, sortValue, orderValue string, userID, locationID uint) ([]database.Product, error)
	CreateProduct(userID uint, product *database.Product) error
	CreateProductsBulk(userID uint, rows []ImportedProduct) ([]string, error)
	UpdateProduct(productID uint, userID uint, product *database.ProductDTOPatch) error
	UpdateProductAmount(productID uint, userID uint, delta int) (bool, error)
	DeleteProduct(productID uint, userID uint, archiveOnly bool) error
	RestoreProduct(productID, userID uint) error
	BulkRestoreProducts(productIDs []uint, userID uint) []BulkOperationError
	SetProductExpireAt(productID uint, userID uint, expireAt database.Timestamp) error
	SetProductNotifiedAt(productID uint) error
	GetProductsExpired(userID uint) ([]*database.Product, error)
	GetExpiredProductsCount(userID uint) (int, error)
	GetArchivedProductsCount(userID uint) (int, error)
	GetUniqueArchivedProductsCount(userID uint) (int, error)
	GetTopArchivedProducts(userID uint, limit int) ([]database.Product, error)
	GetActiveProductsCount(userID uint) (int, error)
	GetProductCategoryBreakdown(userID uint) (map[string]int, error)
	GetExpiryTrend(userID uint) ([]apiModel.StatsMonthlyCount, error)
	GetExpiringSoonProducts(userID uint, days int) ([]apiModel.StatsExpiringProduct, error)
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
	GetExpiringSoonCount(userID uint, days int) (int, error)
	GetActiveExpiryCounts(userID uint, now time.Time, criticalDays int) (expired, critical int, err error)
	GetWasteThisMonth(userID uint) (int, error)
	GetExpiringProductsByHousehold(householdID uint, daysAhead int) ([]database.Product, error)
	GetProductsByHousehold(householdID uint) ([]database.Product, error)
	GetSubThresholdProducts(userID uint) ([]database.Product, error)
	ConsumeProduct(productID, userID uint) error
	ConsumeProductPartial(product *database.Product, amount int) (int, bool, error)
	WasteProduct(productID, userID uint) error
	MarkProductOpened(productID, userID uint, openedAt time.Time, force bool) (database.Product, *time.Time, bool, error)
	BulkConsumeProducts(productIDs []uint, userID uint) []BulkOperationError
	BulkWasteProducts(productIDs []uint, userID uint) []BulkOperationError
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
	sort.SliceStable(products, func(i, j int) bool {
		ti, tj := products[i].EffectiveExpireAt(), products[j].EffectiveExpireAt()
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

func (r *ProductRepository) GetUserProductsBulk(userID uint, limit int) ([]database.Product, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	query := r.DB.Preload("StorageLocation").Where(util.QueryHouseholdId, householdID).Scopes(r.privacyScope(userID))

	if limit > 0 {
		query = query.Limit(limit)
	}

	queryErr := query.Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}

	SortByEffectiveExpiry(products)

	return products, nil
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

func (r *ProductRepository) GetUserArchivedProductsBulk(userID uint, limit int) ([]database.Product, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	query := r.DB.Preload("StorageLocation").Unscoped().Where(util.WhereDeletedIsNotNull).Where(util.QueryHouseholdId, householdID).Scopes(r.privacyScope(userID))

	if limit > 0 {
		query = query.Limit(limit)
	}

	queryErr := query.Find(&products).Error
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
		end := start + size
		if end > len(values) {
			end = len(values)
		}
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
// name + amount + unit + min stock + private flag + owner) without the
// StorageLocation preload. It exists for hot read paths that only need
// those fields and would otherwise pay for a useless JOIN.
func (r *ProductRepository) GetProductIdentity(productID, userID uint) (database.Product, error) {
	if productID == 0 {
		return database.Product{}, gorm.ErrNotImplemented
	}

	var product database.Product
	getError := r.DB.Select(
		"id, barcode, product_name, household_id, user_id, is_private, amount, unit, min_stock_amount",
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

// productSortColumns is the allowlist of columns permitted in a request-driven
// ORDER BY clause. Sort values are interpolated verbatim into SQL, so the only
// string that may reach Order() is a value resolved through this map: the key
// is what a caller sends, the value is the real column name.
var productSortColumns = map[string]string{
	"product_name": "product_name",
	"created_at":   "created_at",
	"expire_at":    "expire_at",
	"scanned_at":   "scanned_at",
	"notified_at":  "notified_at",
	"barcode":      "barcode",
}

// resolveSortClause maps a caller-supplied sort key and direction onto a safe
// ORDER BY clause. Empty values fall back to the defaults; any value outside
// the allowlist returns ErrDatabaseInvalidSortParameter rather than silently
// falling back, so misuse stays visible instead of masquerading as a working
// sort. It lives here, at the single Order() call site, so both the web and the
// API entry points are covered by one guard.
func resolveSortClause(sortValue, orderValue string) (string, error) {
	if sortValue == "" {
		sortValue = "product_name"
	}
	if orderValue == "" {
		orderValue = "asc"
	}
	column, ok := productSortColumns[sortValue]
	if !ok {
		return "", errors.ErrDatabaseInvalidSortParameter
	}
	direction := strings.ToUpper(orderValue)
	if direction != "ASC" && direction != "DESC" {
		return "", errors.ErrDatabaseInvalidSortParameter
	}
	return column + " " + direction, nil
}

// searchProductsQuery builds the shared predicate for both search entry points:
// the API's full-row search and the products page's projection search. Keeping
// it in one place means a new search field cannot be added to one and forgotten
// in the other.
func (r *ProductRepository) searchProductsQuery(queryParam SearchParameterEnum, queryValue string, userID uint) (*gorm.DB, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}

	query := r.DB.
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, user.HouseholdID).
		Where(util.WhereDeletedIsNull)

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

func (r *ProductRepository) SearchProducts(queryParam SearchParameterEnum, queryValue, sortValue, orderValue string, userID uint) ([]database.Product, error) {
	query, queryErr := r.searchProductsQuery(queryParam, queryValue, userID)
	if queryErr != nil {
		return []database.Product{}, queryErr
	}

	sortClause, sortErr := resolveSortClause(sortValue, orderValue)
	if sortErr != nil {
		return []database.Product{}, sortErr
	}

	var foundProducts []database.Product
	findErr := query.Preload("StorageLocation").Order(sortClause).Find(&foundProducts)
	if findErr.Error != nil {
		return []database.Product{}, findErr.Error
	}
	return foundProducts, nil
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

	sortClause, sortErr := resolveSortClause(sortValue, orderValue)
	if sortErr != nil {
		return nil, sortErr
	}

	var foundProducts []database.Product
	if err := query.Select(productProjectionColumns).Order(sortClause).Find(&foundProducts).Error; err != nil {
		return nil, err
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
	createErr := r.DB.Create(&product)
	return createErr.Error
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

	dbProduct.ProductName = product.ProductName
	dbProduct.Categories = product.Categories
	dbProduct.Countries = product.Countries
	dbProduct.ImageURL = product.ImageURL
	dbProduct.ExpireAt = product.ExpireAt
	dbProduct.Amount = product.Amount
	dbProduct.Unit = product.Unit
	dbProduct.StorageLocationID = product.StorageLocationID
	dbProduct.NotificationLeadDays = product.NotificationLeadDays
	dbProduct.MinStockAmount = product.MinStockAmount
	dbProduct.IsPrivate = product.IsPrivate
	dbProduct.PriceOverride = product.PriceOverride
	dbProduct.OpenedAt = product.OpenedAt
	dbProduct.DaysAfterOpening = product.DaysAfterOpening

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

func (r *ProductRepository) UpdateProductAmount(productID uint, userID uint, delta int) (bool, error) {
	if productID == 0 {
		return false, gorm.ErrNotImplemented
	}

	var dbProduct database.Product
	getError := r.DB.Unscoped().First(&dbProduct, productID)
	if getError.Error != nil {
		return false, getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return false, userErr.Error
	}

	if dbProduct.HouseholdID != user.HouseholdID {
		return false, errors.ErrMismatcherUserID
	}

	newAmount := dbProduct.Amount + delta
	if newAmount < 0 {
		newAmount = 0
	}
	dbProduct.Amount = newAmount

	if dbProduct.Amount <= 0 {
		deleteResult := r.DB.Unscoped().Delete(&database.Product{}, dbProduct.ID)
		return true, deleteResult.Error
	}

	saveResult := r.DB.Save(&dbProduct)
	if saveResult.Error != nil {
		return false, saveResult.Error
	}
	return false, nil
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

func (r *ProductRepository) GetProductsExpired(userID uint) ([]*database.Product, error) {
	userProducts, getBulkErr := r.GetUserProductsBulk(userID, 0)
	if getBulkErr != nil {
		return []*database.Product{}, getBulkErr
	}

	var expiredProducts []*database.Product
	timestamp := time.Now()
	for idx := range userProducts {
		if userProducts[idx].EffectiveExpireAt().After(timestamp) {
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
// loop below counts as expired exactly as the full-table version did.
func (r *ProductRepository) GetExpiredProductsCount(userID uint) (int, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return 0, err
	}

	now := time.Now()

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
		if database.EffectiveExpireAt(rows[i].ExpireAt, rows[i].OpenedAt, rows[i].DaysAfterOpening).Before(now) {
			count++
		}
	}
	return count, nil
}

func (r *ProductRepository) GetTopArchivedProducts(userID uint, limit int) ([]database.Product, error) {
	archivedProducts, err := r.GetUserArchivedProductsBulk(userID, -1)
	if err != nil {
		return nil, err
	}

	if len(archivedProducts) == 0 {
		return []database.Product{}, nil
	}

	barcodeCounts := make(map[string]int)
	barcodeToProduct := make(map[string]database.Product)

	for i := range archivedProducts {
		product := &archivedProducts[i]
		barcodeCounts[product.Barcode]++
		if _, exists := barcodeToProduct[product.Barcode]; !exists {
			barcodeToProduct[product.Barcode] = *product
		}
	}

	type barcodeCount struct {
		barcode string
		count   int
		product database.Product
	}

	var counts []barcodeCount
	for barcode, count := range barcodeCounts {
		counts = append(counts, barcodeCount{
			barcode: barcode,
			count:   count,
			product: barcodeToProduct[barcode],
		})
	}

	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	var result []database.Product
	for i := 0; i < len(counts) && i < limit; i++ {
		result = append(result, counts[i].product)
	}

	return result, nil
}

// GetActiveProductsCount counts the user's active products in SQL. It used to
// load every active row plus a StorageLocation preload just to take len(), which
// made the stats endpoint scale with the household's full stock rather than with
// the handful of numbers it returns.
func (r *ProductRepository) GetActiveProductsCount(userID uint) (int, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return 0, err
	}

	var count int64
	err = r.DB.Model(&database.Product{}).
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// GetArchivedProductsCount returns how many products the household has archived.
// It is the count-only sibling of GetUserArchivedProductsBulk, which materialises
// every archived row (and preloads StorageLocation) for callers that only need a
// number.
func (r *ProductRepository) GetArchivedProductsCount(userID uint) (int, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return 0, err
	}

	var count int64
	err = r.DB.Unscoped().Model(&database.Product{}).
		Scopes(r.privacyScope(userID)).
		Where(util.WhereDeletedIsNotNull).
		Where(util.QueryHouseholdId, householdID).
		Count(&count).Error
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
		first := strings.SplitN(raw, ",", 2)[0]
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

// GetExpiryTrend buckets the next 12 months of expiries. Only the three columns
// EffectiveExpireAt reads are projected, so the query cost tracks the row count
// rather than the row width.
func (r *ProductRepository) GetExpiryTrend(userID uint) ([]apiModel.StatsMonthlyCount, error) {
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

	now := time.Now()
	monthCounts := make(map[string]int, 12)
	for i := 0; i < 12; i++ {
		monthCounts[now.AddDate(0, i, 0).Format(util.DefaultDateFormatMonthStr)] = 0
	}
	for i := range products {
		expiry := products[i].EffectiveExpireAt()
		if expiry.IsZero() {
			continue
		}
		month := expiry.Format(util.DefaultDateFormatMonthStr)
		if _, ok := monthCounts[month]; ok {
			monthCounts[month]++
		}
	}

	result := make([]apiModel.StatsMonthlyCount, 0, 12)
	for i := 0; i < 12; i++ {
		month := now.AddDate(0, i, 0).Format(util.DefaultDateFormatMonthStr)
		result = append(result, apiModel.StatsMonthlyCount{Month: month, Count: monthCounts[month]})
	}
	return result, nil
}

func (r *ProductRepository) GetExpiringSoonProducts(userID uint, days int) ([]apiModel.StatsExpiringProduct, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfWindow := time.Date(now.Year(), now.Month(), now.Day()+days, 23, 59, 59, 999999999, now.Location())

	// Candidate scope is a one-sided prune: it only drops rows whose effective
	// expiry provably cannot reach the window, so the verdict below still runs
	// in Go through EffectiveExpireAt. Same pattern as GetExpiringSoonCount.
	var products []database.Product
	err = r.DB.Model(&database.Product{}).
		Select("product_name, expire_at, opened_at, days_after_opening").
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNull).
		Scopes(effectiveExpiryCandidateScope(startOfToday, endOfWindow)).
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
			result = append(result, apiModel.StatsExpiringProduct{
				ProductName: products[i].ProductName,
				ExpireAt:    expirationTime.Format(util.DefaultDateFormatParseStr),
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ExpireAt < result[j].ExpireAt
	})
	return result, nil
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

func (r *ProductRepository) GetExpiringSoonCount(userID uint, days int) (int, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return 0, err
	}

	now := time.Now()
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

func (r *ProductRepository) GetWasteThisMonth(userID uint) (int, error) {
	householdID, err := r.getUserHouseholdID(userID)
	if err != nil {
		return 0, err
	}

	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Nanosecond)

	var count int64
	err = r.DB.Unscoped().Model(&database.Product{}).
		Scopes(r.privacyScope(userID)).
		Where(util.QueryHouseholdId, householdID).
		Where(util.WhereDeletedIsNotNull).
		Where("deleted_at >= ?", monthStart).
		Where("deleted_at <= ?", monthEnd).
		Where("removal_reason = ?", database.RemovalReasonWasted).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
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
// Update("amount", ...) and Save, so scanning, cooking or editing a single
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
		Where("household_id = ? AND deleted_at IS NULL AND amount < min_stock_amount AND min_stock_amount > 0", householdID).
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

// ConsumeProductPartial reduces an active product's amount by the given
// amount, based on the product state read by the caller. A partial reduce
// is a single guarded UPDATE on the still-active row scoped to the product's
// household. A full consume (amount of 0 or negative, or one that covers the
// whole stock) re-reads the current amount inside a transaction before
// deciding, so a stale caller read never archives more than actually exists:
// if the concurrent reduction still leaves more than the requested amount,
// it falls back to a partial reduce, otherwise it soft-deletes with
// RemovalReasonConsumed and returns the current amount. fullyConsumed
// reports whether the product was archived. Both transactional writes carry
// an updated_at optimistic-lock precondition, so any concurrent row change
// (amount, opened_at, removal_reason) fails the guarded write. When a
// guarded precondition fails (the row changed between read and write inside
// the transaction)
// the transaction is retried once with a fresh read; only a second failure
// surfaces ErrProductConcurrentModification instead of silently applying.
func (r *ProductRepository) ConsumeProductPartial(product *database.Product, amount int) (consumed int, fullyConsumed bool, err error) {
	// Fast path: the caller's read shows enough stock for a plain reduce.
	// The guarded UPDATE re-validates the amount, so a concurrent change
	// surfaces as ErrProductConcurrentModification.
	if amount > 0 && amount < product.Amount {
		result := r.DB.Model(&database.Product{}).
			Where("id = ?", product.ID).
			Where("household_id = ?", product.HouseholdID).
			Where(util.WhereDeletedIsNull).
			Where("amount > ?", amount).
			Update("amount", gorm.Expr("amount - ?", amount))
		if result.Error != nil {
			return 0, false, result.Error
		}
		if result.RowsAffected > 0 {
			return amount, false, nil
		}
		// RowsAffected == 0 means a concurrent change dropped the stock
		// below the requested amount. Fall through to the transactional
		// path, which re-reads and resolves full vs partial consume.
	}

	// Full-consume path: the caller's read may be stale, so re-check the
	// current amount before deciding between full consume and partial
	// reduce. A guarded precondition failure rolls back and is retried
	// once with fresh reads before giving up.
	var txErr error
	for attempt := 0; attempt < 2; attempt++ {
		consumed, fullyConsumed = 0, false
		txErr = r.DB.Transaction(func(tx *gorm.DB) error {
			var current database.Product
			// Household-scoped re-read: a stale caller read must never let a
			// product from another household be re-read and archived here.
			if getErr := tx.Where("id = ? AND household_id = ?", product.ID, product.HouseholdID).First(&current).Error; getErr != nil {
				return getErr
			}
			if amount > 0 && amount < current.Amount {
				result := tx.Model(&database.Product{}).
					Where("id = ?", current.ID).
					Where("household_id = ?", current.HouseholdID).
					Where(util.WhereDeletedIsNull).
					Where("amount > ?", amount).
					Where("updated_at = ?", current.UpdatedAt).
					Update("amount", gorm.Expr("amount - ?", amount))
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected == 0 {
					return errors.ErrProductConcurrentModification
				}
				consumed = amount
				return nil
			}
			consumed = current.Amount
			fullyConsumed = true
			// Guarded flag-and-archive: the amount and updated_at preconditions
			// make the write fail if the row changed in any way since the read
			// (amount, opened_at, removal_reason), so a concurrent full consume
			// cannot be applied twice (double savings/activity entries).
			result := tx.Model(&database.Product{}).
				Where("id = ?", current.ID).
				Where("household_id = ?", current.HouseholdID).
				Where(util.WhereDeletedIsNull).
				Where("amount = ?", current.Amount).
				Where("updated_at = ?", current.UpdatedAt).
				Update("removal_reason", database.RemovalReasonConsumed)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return errors.ErrProductConcurrentModification
			}
			return tx.Delete(&database.Product{}, current.ID).Error
		})
		if txErr != errors.ErrProductConcurrentModification {
			break
		}
	}
	if txErr != nil {
		return 0, false, txErr
	}
	return consumed, fullyConsumed, nil
}

func (r *ProductRepository) WasteProduct(productID, userID uint) error {
	product, err := r.GetProductByID(productID, userID)
	if err != nil {
		return err
	}
	product.RemovalReason = database.RemovalReasonWasted
	if err := r.DB.Save(&product).Error; err != nil {
		return err
	}
	return r.DB.Delete(&database.Product{}, productID).Error
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

func (r *ProductRepository) BulkWasteProducts(productIDs []uint, userID uint) []BulkOperationError {
	bulkErrors := []BulkOperationError{}
	for _, productID := range productIDs {
		if err := r.WasteProduct(productID, userID); err != nil {
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
		Select("id, product_name, barcode, unit, amount, deleted_at, is_private, user_id, removal_reason").
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
