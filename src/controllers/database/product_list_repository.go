package database

import (
	"time"

	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

// ProductListRepositoryInterface is the products-page read side: the lists the
// page pages through, the narrow projections it pages from and the rows it
// hydrates them with. The ordering it shares with the full-row sort lives in
// product_projection.go.
type ProductListRepositoryInterface interface {
	GetUserProductsBulk(userID uint, limit int) ([]database.Product, error)
	GetUserProductsBulkSorted(userID uint, limit, offset int, sortValue, orderValue string) ([]database.Product, error)
	GetUserProductsByIDs(userID uint, ids []uint) ([]database.Product, error)
	GetUserArchivedProductsBulk(userID uint, limit, offset int) ([]database.Product, error)
	GetActiveProductProjections(userID, locationID uint) ([]database.Product, error)
	GetArchivedProductProjections(userID uint) ([]database.Product, error)
	GetUserArchivedProductsByIDs(userID uint, ids []uint) ([]database.Product, error)
	GetUserProductsBulkByBarcode(userID uint, barcode int) ([]database.Product, error)
	GetUserProductsBulkByBarcodes(userID uint, barcodes []string) ([]database.Product, error)
	GetProductsExpired(userID uint) ([]*database.Product, error)
}

var _ ProductListRepositoryInterface = (*ProductListRepository)(nil)

// ProductListRepository answers the products-page reads: the paged lists, the
// narrow projections they are cut from and the hydration of one page.
type ProductListRepository struct {
	DB *gorm.DB
}

func NewProductListRepository(db *gorm.DB) *ProductListRepository {
	return &ProductListRepository{DB: db}
}

// GetUserProductsBulk returns the household's active products in the order the
// products list uses: by effective expiry, soonest first.
func (r *ProductListRepository) GetUserProductsBulk(userID uint, limit int) ([]database.Product, error) {
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
func (r *ProductListRepository) GetUserProductsBulkSorted(userID uint, limit, offset int, sortValue, orderValue string) ([]database.Product, error) {
	sort, sortErr := resolveSortClause(sortValue, orderValue)
	if sortErr != nil {
		return []database.Product{}, sortErr
	}

	if sort.byEffectiveExpiry {
		products, err := getProductsByEffectiveExpiry(r.DB, userID, limit, offset, sort.descending)
		if err != nil {
			return []database.Product{}, err
		}
		return products, nil
	}

	query, err := activeProductsQuery(r.DB, userID)
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

// GetUserProductsByIDs returns the active products with the given IDs that belong to the user's household.
// Used for lightweight lookups of a small, known set of products (e.g. cook workflow).
func (r *ProductListRepository) GetUserProductsByIDs(userID uint, ids []uint) ([]database.Product, error) {
	return getUserProductsByIDs(r.DB, userID, ids)
}

// GetUserArchivedProductsBulk returns one page of the household's archived
// products, oldest id first.
//
// The archived rows are ordered by id because they have no expiry to order
// them by: without an order the database hands the rows back in whatever order
// it finds them, and a client that walks the pages by offset would skip and
// repeat rows.
func (r *ProductListRepository) GetUserArchivedProductsBulk(userID uint, limit, offset int) ([]database.Product, error) {
	query, err := archivedProductsQuery(r.DB, userID)
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

// GetActiveProductProjections returns the household's active rows narrowed to
// productProjectionColumns, ordered by id. locationID narrows the result to one
// storage location; pass 0 for every location. Callers sort, filter by status
// and slice a single page out of the result before hydrating it.
func (r *ProductListRepository) GetActiveProductProjections(userID, locationID uint) ([]database.Product, error) {
	return activeProductProjections(r.DB, userID, locationID)
}

// GetArchivedProductProjections is the archived-view counterpart of
// GetActiveProductProjections. The archived view has no expiry ordering of its
// own, so id order is what keeps a row on exactly one page.
func (r *ProductListRepository) GetArchivedProductProjections(userID uint) ([]database.Product, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return nil, err
	}

	var products []database.Product
	err = r.DB.Unscoped().Model(&database.Product{}).
		Select(productProjectionColumns).
		Scopes(privacyScope(userID)).
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
func (r *ProductListRepository) GetUserArchivedProductsByIDs(userID uint, ids []uint) ([]database.Product, error) {
	if len(ids) == 0 {
		return []database.Product{}, nil
	}
	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	err = r.DB.Preload("StorageLocation").Unscoped().
		Scopes(privacyScope(userID)).
		Where(util.WhereDeletedIsNotNull).
		Where(util.QueryHouseholdId, householdID).
		Where("id IN ?", ids).
		Find(&products).Error
	if err != nil {
		return []database.Product{}, err
	}
	return products, nil
}

func (r *ProductListRepository) GetUserProductsBulkByBarcode(userID uint, barcode int) ([]database.Product, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	queryErr := r.DB.Scopes(privacyScope(userID)).Where("household_id = ? and barcode = ?", householdID, barcode).Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
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
func (r *ProductListRepository) GetUserProductsBulkByBarcodes(userID uint, barcodes []string) ([]database.Product, error) {
	if len(barcodes) == 0 {
		return []database.Product{}, nil
	}

	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	for _, chunk := range chunkStrings(barcodes, bulkQueryChunkSize) {
		var batch []database.Product
		queryErr := r.DB.Scopes(privacyScope(userID)).
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

// GetProductsExpired returns the user's products whose effective expiry lies in
// the past, matching GetExpiredProductsCount. A zero effective expiry means the
// product has no expiry set — it is never expired.
func (r *ProductListRepository) GetProductsExpired(userID uint) ([]*database.Product, error) {
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
