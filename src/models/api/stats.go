package api

// StatsMonthlyCount represents the number of products for a given month
type StatsMonthlyCount struct {
	Month string `json:"month"` // format: "2006-01"
	Count int    `json:"count"`
}

// StatsExpiringProduct represents a product expiring within a short window
type StatsExpiringProduct struct {
	ID                  uint   `json:"id"`
	ProductName         string `json:"productName"`
	ExpireAt            string `json:"expireAt"` // format: "2006-01-02"
	Quantity            int    `json:"quantity"`
	PackSize            string `json:"packSize,omitempty"`
	StorageLocationID   *uint  `json:"storageLocationId,omitempty"`
	StorageLocationName string `json:"storageLocationName,omitempty"`
	StorageLocationIcon string `json:"storageLocationIcon,omitempty"`
}

// StatsUseFirstProduct is one row of the "use me first" list: an active
// product of the household with the date it should be eaten by, where it lives
// and how many packs are there.
type StatsUseFirstProduct struct {
	ID                  uint   `json:"id"`
	ProductName         string `json:"productName"`
	ExpireAt            string `json:"expireAt"` // the effective expiry, format: "2006-01-02"; empty when the product has no date
	Quantity            int    `json:"quantity"`
	PackSize            string `json:"packSize,omitempty"`
	StorageLocationID   *uint  `json:"storageLocationId,omitempty"`
	StorageLocationName string `json:"storageLocationName,omitempty"`
	StorageLocationIcon string `json:"storageLocationIcon,omitempty"`
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
	Language                string   `json:"language"`
}

// UpdateHouseholdSettingsRequest is the request body for PATCH /api/v1/household/settings.
// Language is nil when omitted (unchanged), "" resets to automatic detection.
type UpdateHouseholdSettingsRequest struct {
	MonthlyWasteGoalType    string   `json:"monthlyWasteGoalType"`
	MonthlyWasteGoalCount   *int     `json:"monthlyWasteGoalCount"`
	MonthlyWasteGoalPercent *float64 `json:"monthlyWasteGoalPercent"`
	Language                *string  `json:"language"`
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
	UseFirst            []StatsUseFirstProduct `json:"useFirst"`
	Categories          map[string]int         `json:"categories"`
	ExpiryTrend         []StatsMonthlyCount    `json:"expiryTrend"`
}
