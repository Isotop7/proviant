package database

import (
	"fmt"
	"strings"
	"time"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

// ProductRepositoryInterface is the product domain: a product's identity, the
// writes that change it and the guard that decides who may change it. The
// reads that page through a list live in ProductListRepositoryInterface, the
// stock movements in ProductStockRepositoryInterface.
type ProductRepositoryInterface interface {
	GetProductByID(productID, userID uint) (database.Product, error)
	GetProductIdentity(productID, userID uint) (database.Product, error)
	GetArchivedProductByID(productID, userID uint) (database.Product, error)
	FindDuplicateProduct(userID uint, draft *database.Product) (database.Product, bool, error)
	UserHasProductAccess(userID uint, productID int) bool
	CreateProduct(userID uint, product *database.Product) error
	GetProductBySyncID(userID uint, clientSyncID string) (database.Product, error)
	CreateProductsBulk(userID uint, rows []ImportedProduct) ([]string, error)
	UpdateProduct(productID uint, userID uint, product *database.ProductDTOPatch) error
	DeleteProduct(productID uint, userID uint, archiveOnly bool) error
	RestoreProduct(productID, userID uint) error
	BulkRestoreProducts(productIDs []uint, userID uint) []BulkOperationError
	SetProductExpireAt(productID uint, userID uint, expireAt database.Timestamp) error
	SetProductNotifiedAt(productID uint) error
	MarkProductOpened(productID, userID uint, openedAt time.Time, force bool) (database.Product, *time.Time, bool, error)
	ListProductsWithoutPackSize(afterID uint, limit int) ([]database.Product, error)
	SetProductPackSize(productID uint, packSize string) error
}

var _ ProductRepositoryInterface = (*ProductRepository)(nil)

// ProductRepository owns a product's identity and the writes that change it.
type ProductRepository struct {
	DB *gorm.DB
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{DB: db}
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

func getProductByID(db *gorm.DB, productID, userID uint) (database.Product, error) {
	if productID == 0 {
		return database.Product{}, gorm.ErrNotImplemented
	}

	var product database.Product
	getError := db.Preload("StorageLocation").First(&product, productID)
	if getError.Error != nil {
		return database.Product{}, getError.Error
	}

	var user authentication.User
	userErr := db.First(&user, userID)
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

func (r *ProductRepository) GetProductByID(productID, userID uint) (database.Product, error) {
	return getProductByID(r.DB, productID, userID)
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

	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return database.Product{}, false, err
	}

	var candidates []database.Product
	queryErr := r.DB.Preload("StorageLocation").
		Scopes(privacyScope(userID)).
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
