package database

import (
	"database/sql"
	"fmt"
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

// GetMonthlyBreakdown returns per-month consumed and wasted aggregates, padded with zero
// buckets for months that have no data so the response is contiguous from `since`.
func (r *WasteAnalyticsRepository) GetMonthlyBreakdown(householdID uint, since time.Time, months int) ([]apiModel.WasteMonthly, error) {
	type rawRow struct {
		CreatedAt time.Time
		EventType string
		PriceEUR  float64
		CO2Kg     float64
	}

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
		key := rows[i].CreatedAt.Format(util.DefaultDateFormatMonthStr)
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
	now := time.Now()
	for i := months - 1; i >= 0; i-- {
		m := now.AddDate(0, -i, 0)
		key := m.Format(util.DefaultDateFormatMonthStr)
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

// GetTrendMonths returns wasted-count-per-month for the last `months` months (ascending).
func (r *WasteAnalyticsRepository) GetTrendMonths(householdID uint, months int) ([]apiModel.StatsMonthlyCount, error) {
	since := time.Now().AddDate(0, -months+1, 0)
	since = time.Date(since.Year(), since.Month(), 1, 0, 0, 0, 0, since.Location())

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
		byMonth[createdAts[i].Format(util.DefaultDateFormatMonthStr)]++
	}

	out := make([]apiModel.StatsMonthlyCount, 0, months)
	now := time.Now()
	for i := months - 1; i >= 0; i-- {
		m := now.AddDate(0, -i, 0)
		key := m.Format(util.DefaultDateFormatMonthStr)
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
