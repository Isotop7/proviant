package database

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

// WasteAnalyticsRepository handles aggregations of consume/waste events for analytics.
type WasteAnalyticsRepository struct {
	DB *gorm.DB
}

// NewWasteAnalyticsRepository creates a new WasteAnalyticsRepository.
func NewWasteAnalyticsRepository(db *gorm.DB) *WasteAnalyticsRepository {
	return &WasteAnalyticsRepository{DB: db}
}

// GetConsumedVsWasted returns the aggregate count/EUR/CO2 for consumed and wasted events
// in the window [since, now].
func (r *WasteAnalyticsRepository) GetConsumedVsWasted(householdID uint, since time.Time) (ConsumedVsWastedRow, error) {
	var row ConsumedVsWastedRow

	type aggRow struct {
		EventType string
		Count     int
		EUR       float64
		CO2       float64
	}

	var aggs []aggRow
	if err := r.DB.
		Table("savings_records").
		Select("event_type AS event_type, COUNT(*) AS count, COALESCE(SUM(price_eur), 0) AS eur, COALESCE(SUM(co2_kg), 0) AS co2").
		Where("household_id = ? AND created_at >= ?", householdID, since).
		Where("legacy = ?", false).
		Group("event_type").
		Scan(&aggs).Error; err != nil {
		return row, err
	}

	for _, a := range aggs {
		switch a.EventType {
		case dbModel.SavingsEventTypeConsumed:
			row.ConsumedCount = a.Count
			row.ConsumedEUR = a.EUR
			row.ConsumedCO2Kg = a.CO2
		case dbModel.SavingsEventTypeWasted:
			row.WastedCount = a.Count
			row.WastedEUR = a.EUR
			row.WastedCO2Kg = a.CO2
		}
	}
	return row, nil
}

// GetMonthlyBreakdown returns per-month consumed and wasted aggregates for the
// `months` months that end with the month `now` falls in, padded with zero
// buckets for months that have no data so the response is contiguous.
func (r *WasteAnalyticsRepository) GetMonthlyBreakdown(householdID uint, months int, now time.Time) ([]apiModel.WasteMonthly, error) {
	type rawRow struct {
		CreatedAt time.Time
		EventType string
		PriceEUR  float64
		CO2Kg     float64
	}

	if months <= 0 {
		return []apiModel.WasteMonthly{}, nil
	}

	since := util.MonthWindowStart(now, months)

	var rows []rawRow
	if err := r.DB.
		Table("savings_records").
		Select("created_at AS created_at, event_type AS event_type, price_eur AS price_eur, co2_kg AS co2_kg").
		Where("household_id = ? AND created_at >= ?", householdID, since).
		Where("legacy = ?", false).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	type bucket struct {
		ConsumedCount int
		WastedCount   int
		WastedEUR     float64
		WastedCO2Kg   float64
	}
	byMonth := make(map[string]*bucket, months)
	for i := range rows {
		key := util.MonthKey(rows[i].CreatedAt)
		b, ok := byMonth[key]
		if !ok {
			b = &bucket{}
			byMonth[key] = b
		}
		switch rows[i].EventType {
		case dbModel.SavingsEventTypeConsumed:
			b.ConsumedCount++
		case dbModel.SavingsEventTypeWasted:
			b.WastedCount++
			b.WastedEUR += rows[i].PriceEUR
			b.WastedCO2Kg += rows[i].CO2Kg
		}
	}

	out := make([]apiModel.WasteMonthly, 0, months)
	for _, key := range util.RecentMonthKeys(now, months) {
		b := byMonth[key]
		row := apiModel.WasteMonthly{Month: key}
		if b != nil {
			row.ConsumedCount = b.ConsumedCount
			row.WastedCount = b.WastedCount
			row.WastedEUR = b.WastedEUR
			row.WastedCO2Kg = b.WastedCO2Kg
		}
		out = append(out, row)
	}
	return out, nil
}

// GetMostWastedCategories returns the top `limit` categories of wasted products for the
// household in the window [since, now]. Categories are resolved from the still-existing
// Product row; if a wasted product has been hard-deleted it falls into the "Other" bucket.
// Results are sorted by `by` — "count" (default) or "cost".
func (r *WasteAnalyticsRepository) GetMostWastedCategories(householdID uint, since time.Time, limit int, by string) ([]apiModel.WasteCategoryStat, error) {
	// Pre-load the canonical categories for the display name lookup. Do this
	// in Go rather than SQL to keep the query portable across SQLite and
	// MariaDB.
	resolver, err := LoadCategoryResolver(r.DB)
	if err != nil {
		return nil, err
	}

	// Pull wasted events with the still-existing product's category (LEFT JOIN — deleted
	// products yield NULL categories which we bucket as "Other").
	type rawRow struct {
		ProductID  uint
		PriceEUR   float64
		CO2Kg      float64
		Categories sql.NullString
	}

	var raws []rawRow
	if err := r.DB.
		Table("savings_records AS s").
		Select("s.product_id AS product_id, s.price_eur AS price_eur, s.co2_kg AS co2_kg, p.categories AS categories").
		Joins("LEFT JOIN products p ON p.id = s.product_id").
		Where("s.household_id = ? AND s.event_type = ? AND s.created_at >= ?", householdID, dbModel.SavingsEventTypeWasted, since).
		Where("s.legacy = ?", false).
		Scan(&raws).Error; err != nil {
		return nil, err
	}

	agg := make(map[string]*apiModel.WasteCategoryStat)
	for _, r := range raws {
		key := wasteCategoryKey(resolver, r.Categories.String)
		bucket, ok := agg[key]
		if !ok {
			display := key
			if pc, found := resolver.Canonical(key); found {
				display = pc.DisplayName
			}
			bucket = &apiModel.WasteCategoryStat{
				CategoryKey: key,
				DisplayName: display,
			}
			agg[key] = bucket
		}
		bucket.Count++
		bucket.CostEUR += r.PriceEUR
		bucket.CO2Kg += r.CO2Kg
	}

	out := make([]apiModel.WasteCategoryStat, 0, len(agg))
	for _, v := range agg {
		out = append(out, *v)
	}

	// sort by requested key, then by the secondary key
	sortCategoryStats(out, by)

	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// WasteCauseBreakdown holds the wasted events of a household grouped by the
// reason they were wasted, plus the category the household most over-buys.
type WasteCauseBreakdown struct {
	// Causes holds one entry per cause, the unattributed waste in the entry
	// with the empty cause.
	Causes []apiModel.WasteCauseStat
	// Overbuy names the category with the most over-bought waste.
	Overbuy apiModel.WasteOverbuyStat
	// WastedEvents is the number of wasted events the breakdown covers.
	WastedEvents int
}

// GetWasteCauseBreakdown returns the wasted events of the household since the
// given instant, grouped by the reason they were wasted, together with the
// category the household most over-buys.
func (r *WasteAnalyticsRepository) GetWasteCauseBreakdown(householdID uint, since time.Time) (WasteCauseBreakdown, error) {
	return wasteCauseBreakdown(r.DB, householdID, since, time.Time{})
}

// wasteCauseBreakdown aggregates the wasted savings records of a household in
// the window [since, until) by cause and by category. A zero until leaves the
// window open. The cause of a waste is read from the savings record: the
// ledger holds one row per waste action, so a partial waste and a repeated
// waste of the same product keep their own cause, and the breakdown survives
// a hard delete of the product row.
func wasteCauseBreakdown(db *gorm.DB, householdID uint, since, until time.Time) (WasteCauseBreakdown, error) {
	var breakdown WasteCauseBreakdown

	resolver, err := LoadCategoryResolver(db)
	if err != nil {
		return breakdown, err
	}

	type rawRow struct {
		PriceEUR   float64
		Categories sql.NullString
		Cause      string
	}

	query := db.
		Table("savings_records AS s").
		Select("s.price_eur AS price_eur, p.categories AS categories, s.waste_cause AS cause").
		Joins("LEFT JOIN products p ON p.id = s.product_id").
		Where("s.household_id = ? AND s.event_type = ? AND s.created_at >= ?", householdID, dbModel.SavingsEventTypeWasted, since).
		Where("s.legacy = ?", false)
	if !until.IsZero() {
		query = query.Where("s.created_at < ?", until)
	}

	var raws []rawRow
	if err := query.Scan(&raws).Error; err != nil {
		return breakdown, err
	}

	type causeBucket struct {
		count int
		cost  float64
	}

	causes := make(map[string]*causeBucket)
	categories := make(map[string]*wasteCategoryBucket)
	total := 0

	for _, row := range raws {
		total++
		bucket, ok := causes[row.Cause]
		if !ok {
			bucket = &causeBucket{}
			causes[row.Cause] = bucket
		}
		bucket.count++
		bucket.cost += row.PriceEUR

		key := wasteCategoryKey(resolver, row.Categories.String)
		category, ok := categories[key]
		if !ok {
			category = &wasteCategoryBucket{}
			categories[key] = category
		}
		category.waste++
		if row.Cause == dbModel.WasteCauseOverbought {
			category.overbought++
		}
	}

	breakdown.WastedEvents = total
	breakdown.Causes = make([]apiModel.WasteCauseStat, 0, len(causes))
	for cause, bucket := range causes {
		share := 0.0
		if total > 0 {
			share = float64(bucket.count) / float64(total) * 100
		}
		breakdown.Causes = append(breakdown.Causes, apiModel.WasteCauseStat{
			Cause:   cause,
			Count:   bucket.count,
			CostEUR: bucket.cost,
			Share:   share,
		})
	}
	sort.Slice(breakdown.Causes, func(i, j int) bool {
		if breakdown.Causes[i].Count != breakdown.Causes[j].Count {
			return breakdown.Causes[i].Count > breakdown.Causes[j].Count
		}
		return breakdown.Causes[i].Cause < breakdown.Causes[j].Cause
	})

	breakdown.Overbuy = topOverboughtCategory(resolver, categories)
	return breakdown, nil
}

// wasteCategoryBucket counts the waste of one category, split by the over-bought
// part of it.
type wasteCategoryBucket struct {
	waste      int
	overbought int
}

// topOverboughtCategory picks the category with the most over-bought waste.
// The ties break by the category's total waste and then by its key, so the
// answer does not depend on the map's iteration order.
func topOverboughtCategory(resolver *CategoryResolver, categories map[string]*wasteCategoryBucket) apiModel.WasteOverbuyStat {
	bestKey := ""
	bestWaste := 0
	bestOverbought := 0
	for key, bucket := range categories {
		if bucket.overbought == 0 {
			continue
		}
		if bestKey != "" && !beatsOverbuy(bucket, bestOverbought, bestWaste, key, bestKey) {
			continue
		}
		bestKey = key
		bestWaste = bucket.waste
		bestOverbought = bucket.overbought
	}
	if bestKey == "" {
		return apiModel.WasteOverbuyStat{}
	}

	display := bestKey
	if canonical, found := resolver.Canonical(bestKey); found {
		display = canonical.DisplayName
	}
	share := 0.0
	if bestWaste > 0 {
		share = float64(bestOverbought) / float64(bestWaste) * 100
	}
	return apiModel.WasteOverbuyStat{
		CategoryKey:   bestKey,
		DisplayName:   display,
		Count:         bestOverbought,
		CategoryWaste: bestWaste,
		Share:         share,
	}
}

// beatsOverbuy reports whether a category takes the over-buying crown from the
// current best: more over-bought waste wins, then more waste in the category,
// then the alphabetically smaller key.
func beatsOverbuy(bucket *wasteCategoryBucket, bestOverbought, bestWaste int, key, bestKey string) bool {
	if bucket.overbought != bestOverbought {
		return bucket.overbought > bestOverbought
	}
	if bucket.waste != bestWaste {
		return bucket.waste > bestWaste
	}
	return key < bestKey
}

// GetTrendMonths returns wasted-count-per-month for the `months` months that
// end with the month `now` falls in (ascending).
func (r *WasteAnalyticsRepository) GetTrendMonths(householdID uint, months int, now time.Time) ([]apiModel.StatsMonthlyCount, error) {
	if months <= 0 {
		return []apiModel.StatsMonthlyCount{}, nil
	}

	since := util.MonthWindowStart(now, months)

	var createdAts []time.Time
	if err := r.DB.
		Table("savings_records").
		Select("created_at AS created_at").
		Where("household_id = ? AND event_type = ? AND created_at >= ?", householdID, dbModel.SavingsEventTypeWasted, since).
		Where("legacy = ?", false).
		Scan(&createdAts).Error; err != nil {
		return nil, err
	}

	byMonth := make(map[string]int, months)
	for i := range createdAts {
		byMonth[util.MonthKey(createdAts[i])]++
	}

	out := make([]apiModel.StatsMonthlyCount, 0, months)
	for _, key := range util.RecentMonthKeys(now, months) {
		out = append(out, apiModel.StatsMonthlyCount{
			Month: key,
			Count: byMonth[key],
		})
	}
	return out, nil
}

// ConsumedVsWastedRow is the aggregate over a time window.
type ConsumedVsWastedRow struct {
	ConsumedCount int
	ConsumedEUR   float64
	ConsumedCO2Kg float64
	WastedCount   int
	WastedEUR     float64
	WastedCO2Kg   float64
}

// wasteCategoryKey names the bucket a product's free-text categories fall
// into. A product that names no canonical category lands in "Other".
func wasteCategoryKey(resolver *CategoryResolver, categories string) string {
	key, ok := resolver.ResolveKey(categories)
	if !ok {
		return "Other"
	}
	return key
}

// sortCategoryStats sorts in place by the given key. Recognised keys: "count", "cost".
// Unknown keys fall back to "count" ordering. n is small (<= handful of categories).
func sortCategoryStats(s []apiModel.WasteCategoryStat, by string) {
	if by != "cost" {
		by = "count"
	}
	for i := 1; i < len(s); i++ {
		for j := i; j > 0; j-- {
			a, b := s[j-1], s[j]
			swap := false
			switch by {
			case "cost":
				if a.CostEUR < b.CostEUR || (a.CostEUR == b.CostEUR && a.Count < b.Count) {
					swap = true
				}
			default: // "count"
				if a.Count < b.Count || (a.Count == b.Count && a.CostEUR < b.CostEUR) {
					swap = true
				}
			}
			if swap {
				s[j-1], s[j] = b, a
				continue
			}
			break
		}
	}
}

// sortWasteProductStats sorts in place by count desc, then cost desc.
func sortWasteProductStats(s []apiModel.WasteProductStat) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0; j-- {
			a, b := s[j-1], s[j]
			if a.Count < b.Count || (a.Count == b.Count && a.CostEUR < b.CostEUR) {
				s[j-1], s[j] = b, a
				continue
			}
			break
		}
	}
}

// GetMostWastedProducts returns the top wasted products grouped by category for the
// household in the window [since, now]. Each category's slice is capped at perCategoryLimit.
// The category key is resolved from the still-existing Product row; if a wasted product
// has been hard-deleted it falls into the "Other" bucket.
func (r *WasteAnalyticsRepository) GetMostWastedProducts(householdID uint, since time.Time, perCategoryLimit int) (map[string][]apiModel.WasteProductStat, error) {
	resolver, err := LoadCategoryResolver(r.DB)
	if err != nil {
		return nil, err
	}

	type rawRow struct {
		ProductID   uint
		ProductName string
		PriceEUR    float64
		CO2Kg       float64
		Categories  sql.NullString
	}

	var raws []rawRow
	if err := r.DB.
		Table("savings_records AS s").
		Select("s.product_id AS product_id, s.product_name AS product_name, s.price_eur AS price_eur, s.co2_kg AS co2_kg, p.categories AS categories").
		Joins("LEFT JOIN products p ON p.id = s.product_id").
		Where("s.household_id = ? AND s.event_type = ? AND s.created_at >= ?", householdID, dbModel.SavingsEventTypeWasted, since).
		Where("s.legacy = ?", false).
		Scan(&raws).Error; err != nil {
		return nil, err
	}

	// Two-level aggregation: outer by category, inner by product name (fallback to product id).
	byCategory := make(map[string]map[string]*apiModel.WasteProductStat)
	for _, row := range raws {
		catKey := wasteCategoryKey(resolver, row.Categories.String)
		prodKey := row.ProductName
		if prodKey == "" {
			prodKey = fmt.Sprintf("product-%d", row.ProductID)
		}
		prods, ok := byCategory[catKey]
		if !ok {
			prods = make(map[string]*apiModel.WasteProductStat)
			byCategory[catKey] = prods
		}
		bucket, ok := prods[prodKey]
		if !ok {
			bucket = &apiModel.WasteProductStat{ProductName: row.ProductName}
			if row.ProductName == "" {
				bucket.ProductName = fmt.Sprintf("Product #%d", row.ProductID)
			}
			prods[prodKey] = bucket
		}
		bucket.Count++
		bucket.CostEUR += row.PriceEUR
		bucket.CO2Kg += row.CO2Kg
	}

	out := make(map[string][]apiModel.WasteProductStat, len(byCategory))
	for catKey, prods := range byCategory {
		flat := make([]apiModel.WasteProductStat, 0, len(prods))
		for _, p := range prods {
			flat = append(flat, *p)
		}
		sortWasteProductStats(flat)
		if perCategoryLimit > 0 && len(flat) > perCategoryLimit {
			flat = flat[:perCategoryLimit]
		}
		out[catKey] = flat
	}
	return out, nil
}
