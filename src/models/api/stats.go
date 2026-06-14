package api

// StatsMonthlyCount represents the number of products for a given month
type StatsMonthlyCount struct {
	Month string `json:"month"` // format: "2006-01"
	Count int    `json:"count"`
}

// StatsExpiringProduct represents a product expiring within a short window
type StatsExpiringProduct struct {
	ProductName string `json:"productName"`
	ExpireAt    string `json:"expireAt"` // format: "2006-01-02"
}

// ProductSummaryResponse is the response body for GET /api/v1/products/summary
type ProductSummaryResponse struct {
	ExpiringSoonCount int `json:"expiringSoonCount"`
	ExpiredCount      int `json:"expiredCount"`
	TotalActive       int `json:"totalActive"`
	WasteThisMonth    int `json:"wasteThisMonth"`
}

// StreakResponse is the response body for GET /api/v1/streak
type StreakResponse struct {
	CurrentStreak int `json:"currentStreak"`
	LongestStreak int `json:"longestStreak"`
}

// HouseholdSettingsResponse is the response body for GET /api/v1/household/settings
type HouseholdSettingsResponse struct {
	MonthlyWasteGoalType    string   `json:"monthlyWasteGoalType"`
	MonthlyWasteGoalCount   *int     `json:"monthlyWasteGoalCount"`
	MonthlyWasteGoalPercent *float64 `json:"monthlyWasteGoalPercent"`
}

// UpdateHouseholdSettingsRequest is the request body for PATCH /api/v1/household/settings
type UpdateHouseholdSettingsRequest struct {
	MonthlyWasteGoalType    string   `json:"monthlyWasteGoalType"`
	MonthlyWasteGoalCount   *int     `json:"monthlyWasteGoalCount"`
	MonthlyWasteGoalPercent *float64 `json:"monthlyWasteGoalPercent"`
}

// ProductStatsResponse is the response body for GET /api/v1/products/stats
type ProductStatsResponse struct {
	WasteCount          int                    `json:"wasteCount"`
	WastePercent        float64                `json:"wastePercent"`
	TotalActive         int                    `json:"totalActive"`
	TotalArchived       int                    `json:"totalArchived"`
	UniqueArchived      int                    `json:"uniqueArchived"`
	LastInsertedProduct string                 `json:"lastInsertedProduct"`
	ExpiringSoon        []StatsExpiringProduct `json:"expiringSoon"`
	ExpiringSoonDays    int                    `json:"expiringSoonDays"`
	Categories          map[string]int         `json:"categories"`
	ExpiryTrend         []StatsMonthlyCount    `json:"expiryTrend"`
}
