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
}
