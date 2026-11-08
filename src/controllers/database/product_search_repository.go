package database

import (
	"fmt"
	"strings"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/database"

	"gorm.io/gorm"
)

// ProductSearchRepositoryInterface is the search, in the two shapes the
// application asks for it: the API's full-row search and the products page's
// projection search. Both are built from one predicate, so a match cannot
// differ between them.
type ProductSearchRepositoryInterface interface {
	SearchProducts(queryParam SearchParameterEnum, queryValue, sortValue, orderValue string, userID uint, limit, offset int) ([]database.Product, error)
	CountSearchProducts(queryParam SearchParameterEnum, queryValue string, userID uint) (int, error)
	SearchProductProjections(queryParam SearchParameterEnum, queryValue, sortValue, orderValue string, userID, locationID uint) ([]database.Product, error)
}

var _ ProductSearchRepositoryInterface = (*ProductSearchRepository)(nil)

// ProductSearchRepository answers the searches over the household's products.
type ProductSearchRepository struct {
	DB *gorm.DB
}

func NewProductSearchRepository(db *gorm.DB) *ProductSearchRepository {
	return &ProductSearchRepository{DB: db}
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
func (r *ProductSearchRepository) searchProductsQuery(queryParam SearchParameterEnum, queryValue string, userID uint) (*gorm.DB, error) {
	query, err := activeProductsQuery(r.DB, userID)
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
func (r *ProductSearchRepository) SearchProducts(queryParam SearchParameterEnum, queryValue, sortValue, orderValue string, userID uint, limit, offset int) ([]database.Product, error) {
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
func (r *ProductSearchRepository) searchProductsByEffectiveExpiry(queryParam SearchParameterEnum, queryValue string, sort productSort, userID uint, limit, offset int) ([]database.Product, error) {
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

	rows, err := hydrateProductsByIDs(r.DB, userID, ids)
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
func (r *ProductSearchRepository) CountSearchProducts(queryParam SearchParameterEnum, queryValue string, userID uint) (int, error) {
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
func (r *ProductSearchRepository) SearchProductProjections(
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
func (r *ProductSearchRepository) searchProductProjections(query *gorm.DB, sort productSort) ([]database.Product, error) {
	var foundProducts []database.Product
	if err := query.Select(productProjectionColumns).Order(sort.clause).Find(&foundProducts).Error; err != nil {
		return nil, err
	}
	if sort.byEffectiveExpiry {
		applyEffectiveExpirySort(foundProducts, sort)
	}
	return foundProducts, nil
}
