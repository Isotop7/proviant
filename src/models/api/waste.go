package api

// WasteCategoryStat represents the aggregate waste impact for a single product category.
type WasteCategoryStat struct {
	CategoryKey string             `json:"categoryKey"`
	DisplayName string             `json:"displayName"`
	Count       int                `json:"count"`
	CostEUR     float64            `json:"costEur"`
	CO2Kg       float64            `json:"co2Kg"`
	Products    []WasteProductStat `json:"products,omitempty"`
}

// WasteProductStat represents the aggregate waste impact for a single product
// within a category.
type WasteProductStat struct {
	ProductName string  `json:"productName"`
	Count       int     `json:"count"`
	CostEUR     float64 `json:"costEur"`
	CO2Kg       float64 `json:"co2Kg"`
}

// WasteMonthly represents the per-month consumed vs. wasted breakdown.
type WasteMonthly struct {
	Month         string  `json:"month"` // format: "2006-01"
	ConsumedCount int     `json:"consumedCount"`
	WastedCount   int     `json:"wastedCount"`
	WastedEUR     float64 `json:"wastedEur"`
	WastedCO2Kg   float64 `json:"wastedCo2Kg"`
}

// WasteCauseStat is the waste attributed to one cause. The cause of an
// unattributed waste is the empty string.
type WasteCauseStat struct {
	Cause   string  `json:"cause"`
	Count   int     `json:"count"`
	CostEUR float64 `json:"costEur"`
	// Share is the cause's share of all wasted events in the window, in percent.
	Share float64 `json:"share"`
}

// WasteOverbuyStat names the category the household most over-buys.
type WasteOverbuyStat struct {
	CategoryKey string `json:"categoryKey"`
	DisplayName string `json:"displayName"`
	// Count is the number of over-bought waste events in the category.
	Count int `json:"count"`
	// CategoryWaste is the number of all waste events in the category.
	CategoryWaste int `json:"categoryWaste"`
	// Share is Count as a percentage of CategoryWaste.
	Share float64 `json:"share"`
}

// WasteAnalyticsResponse is the response body for GET /api/v1/stats/waste
type WasteAnalyticsResponse struct {
	Period               string              `json:"period"` // "month" | "3months" | "6months" | "12months"
	Sort                 string              `json:"sort"`   // "count" | "cost" — applied to mostWastedCategories
	ConsumedCount        int                 `json:"consumedCount"`
	WastedCount          int                 `json:"wastedCount"`
	TotalRemoved         int                 `json:"totalRemoved"`
	WastedPercent        float64             `json:"wastedPercent"`
	WastedEUR            float64             `json:"wastedEur"`
	WastedCO2Kg          float64             `json:"wastedCo2Kg"`
	Monthly              []WasteMonthly      `json:"monthly"`
	MostWastedCategories []WasteCategoryStat `json:"mostWastedCategories"`
	Trend                []StatsMonthlyCount `json:"trend"`
	CO2Source            string              `json:"co2Source"`
	// Causes breaks the wasted events down by the reason they were wasted.
	Causes []WasteCauseStat `json:"causes"`
	// Overbuy names the category the household most over-buys.
	Overbuy WasteOverbuyStat `json:"overbuy"`
	// Insight is the one-sentence insight about the household's waste, in the
	// user's language.
	Insight string `json:"insight"`
	// InsightAvailable is false when there is too little data to state an
	// insight; the Insight then holds the empty-state sentence.
	InsightAvailable bool `json:"insightAvailable"`
}
