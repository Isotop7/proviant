package database

import (
	"sort"
	"strings"
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

// ProductStatsRepositoryInterface is the statistics the dashboard reads: the
// counts, the breakdowns and the trends. Every method here answers a number
// or a projection, never a page of rows.
type ProductStatsRepositoryInterface interface {
	GetActiveProductsCount(userID uint) (int, error)
	GetArchivedProductsCount(userID uint) (int, error)
	GetUniqueArchivedProductsCount(userID uint) (int, error)
	GetExpiredProductsCount(userID uint, now time.Time) (int, error)
	GetExpiringSoonCount(userID uint, days int, now time.Time) (int, error)
	GetActiveExpiryCounts(userID uint, now time.Time, criticalDays int) (expired, critical int, err error)
	GetWasteThisMonth(userID uint, now time.Time) (int, error)
	GetProductCategoryBreakdown(userID uint) (map[string]int, error)
	GetExpiryTrend(userID uint, now time.Time) ([]apiModel.StatsMonthlyCount, error)
	GetExpiringSoonProducts(userID uint, days int, now time.Time) ([]apiModel.StatsExpiringProduct, error)
	GetUseFirstProducts(userID uint, limit int) ([]apiModel.StatsUseFirstProduct, error)
}

var _ ProductStatsRepositoryInterface = (*ProductStatsRepository)(nil)

// ProductStatsRepository answers the dashboard's questions about the
// household's stock.
type ProductStatsRepository struct {
	DB *gorm.DB
}

func NewProductStatsRepository(db *gorm.DB) *ProductStatsRepository {
	return &ProductStatsRepository{DB: db}
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
func (r *ProductStatsRepository) GetActiveProductsCount(userID uint) (int, error) {
	query, err := activeProductsQuery(r.DB, userID)
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
func (r *ProductStatsRepository) GetArchivedProductsCount(userID uint) (int, error) {
	query, err := archivedProductsQuery(r.DB, userID)
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
func (r *ProductStatsRepository) GetUniqueArchivedProductsCount(userID uint) (int, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return 0, err
	}

	var count int64
	err = r.DB.Unscoped().Model(&database.Product{}).
		Scopes(privacyScope(userID)).
		Where(util.WhereDeletedIsNotNull).
		Where(util.QueryHouseholdId, householdID).
		Distinct("barcode").
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// GetExpiredProductsCount returns how many of the user's active products are
// already past their effective expiry. It follows the same shape as
// GetActiveExpiryCounts: a one-sided SQL prune of the rows that cannot possibly
// be expired, then the effective-date verdict in Go, because the date arithmetic
// (opened_at + days_after_opening) has no portable SQL form.
//
// The prune deliberately also matches rows with a zero expire_at, which the Go
// loop below skips — a product with no expiry set is never expired.
func (r *ProductStatsRepository) GetExpiredProductsCount(userID uint, now time.Time) (int, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return 0, err
	}

	var rows []expiryCandidate
	err = r.DB.Model(&database.Product{}).
		Select("expire_at, opened_at, days_after_opening").
		Scopes(privacyScope(userID)).
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

func (r *ProductStatsRepository) GetExpiringSoonCount(userID uint, days int, now time.Time) (int, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
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
		Scopes(privacyScope(userID)).
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
func (r *ProductStatsRepository) GetActiveExpiryCounts(userID uint, now time.Time, criticalDays int) (expired, critical int, err error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
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
		Scopes(privacyScope(userID)).
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
func (r *ProductStatsRepository) GetWasteThisMonth(userID uint, now time.Time) (int, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
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
		Scopes(privacyScope(userID)).
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

func (r *ProductStatsRepository) GetProductCategoryBreakdown(userID uint) (map[string]int, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return nil, err
	}

	// Only the categories column is read; the rows are still one per product,
	// but they no longer carry the full record or a StorageLocation preload.
	var products []database.Product
	err = r.DB.Model(&database.Product{}).
		Select("categories").
		Scopes(privacyScope(userID)).
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
func (r *ProductStatsRepository) GetExpiryTrend(userID uint, now time.Time) ([]apiModel.StatsMonthlyCount, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
	if err != nil {
		return nil, err
	}

	var products []database.Product
	err = r.DB.Model(&database.Product{}).
		Select("expire_at, opened_at, days_after_opening").
		Scopes(privacyScope(userID)).
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

func (r *ProductStatsRepository) GetExpiringSoonProducts(userID uint, days int, now time.Time) ([]apiModel.StatsExpiringProduct, error) {
	householdID, err := getUserHouseholdID(r.DB, userID)
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
		Scopes(privacyScope(userID)).
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
func (r *ProductStatsRepository) GetUseFirstProducts(userID uint, limit int) ([]apiModel.StatsUseFirstProduct, error) {
	rows, err := getProductsByEffectiveExpiry(r.DB, userID, limit, 0, false)
	if err != nil {
		return nil, err
	}
	return BuildUseFirstProducts(rows, limit), nil
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
