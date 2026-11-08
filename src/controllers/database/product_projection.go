package database

import (
	"sort"

	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

// The products page pages from a narrow projection and the API answers with
// full rows. Both have to agree on the order, or a page boundary drifts from
// what the full list would have shown: a product can land on two pages or on
// none.
//
// The agreement has two halves and both live in this one file, which every
// product repository can see:
//
//   - productProjectionColumns names the columns the projection carries. The
//     sort below reads its inputs from exactly those columns, so a column the
//     projection dropped would be a column the sort could not use.
//
//   - SortByEffectiveExpiry is the Go sort both paths use. The projection path
//     sorts the projected rows with it and the full-row path sorts the rows it
//     hydrated with it, so a product holds the same position in both.
//
// The tie order is the id order the projections arrive in, and
// resolveSortClause in the search repository breaks a column sort's ties by
// id, so the Go sort stays stable and the two paths cannot disagree on a tie.

// productProjectionColumns is every column the products page reads before it has
// chosen a page: the row id it hydrates with, and the three inputs to
// EffectiveExpireAt that decide ordering and status. The page hydrates only the
// rows it renders, so the expensive part (full rows plus a StorageLocation
// preload) scales with the page size instead of with the size of the pantry.
const productProjectionColumns = "id, expire_at, opened_at, days_after_opening"

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

// activeProductProjections returns the household's active rows narrowed to
// productProjectionColumns, ordered by id. locationID narrows the result to one
// storage location; pass 0 for every location.
func activeProductProjections(db *gorm.DB, userID, locationID uint) ([]database.Product, error) {
	householdID, err := getUserHouseholdID(db, userID)
	if err != nil {
		return nil, err
	}

	query := db.Model(&database.Product{}).
		Select(productProjectionColumns).
		Scopes(privacyScope(userID)).
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

// getUserProductsByIDs returns the active products with the given IDs that belong to the user's household.
// Used for lightweight lookups of a small, known set of products (e.g. cook workflow).
func getUserProductsByIDs(db *gorm.DB, userID uint, ids []uint) ([]database.Product, error) {
	householdID, err := getUserHouseholdID(db, userID)
	if err != nil {
		return []database.Product{}, err
	}

	var products []database.Product
	queryErr := db.Preload("StorageLocation").Where(util.QueryHouseholdId, householdID).Scopes(privacyScope(userID)).Where("id IN ?", ids).Order("id").Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

// hydrateProductsByIDs loads the full rows for the given ids. The query binds
// one placeholder per id and SQLite caps the number of bound variables, so a
// list that covers the whole pantry is fetched in chunks.
func hydrateProductsByIDs(db *gorm.DB, userID uint, ids []uint) ([]database.Product, error) {
	if len(ids) == 0 {
		return []database.Product{}, nil
	}
	if len(ids) <= bulkQueryChunkSize {
		return getUserProductsByIDs(db, userID, ids)
	}

	var rows []database.Product
	for start := 0; start < len(ids); start += bulkQueryChunkSize {
		end := min(start+bulkQueryChunkSize, len(ids))
		chunk, chunkErr := getUserProductsByIDs(db, userID, ids[start:end])
		if chunkErr != nil {
			return nil, chunkErr
		}
		rows = append(rows, chunk...)
	}
	return rows, nil
}

// getProductsByEffectiveExpiry returns the household's active products ordered
// by the effective expiry, limited to the page that starts at the offset.
//
// The narrow projection is sorted in Go and only the rows that make up the
// page are hydrated, so the database never materialises the whole pantry as
// full rows with their storage location.
func getProductsByEffectiveExpiry(db *gorm.DB, userID uint, limit, offset int, descending bool) ([]database.Product, error) {
	projections, err := activeProductProjections(db, userID, 0)
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

	rows, err := hydrateProductsByIDs(db, userID, ids)
	if err != nil {
		return nil, err
	}

	return orderProductsByID(rows, ids), nil
}
