package api

// SavingsStatsResponse is the response body for GET /api/v1/savings/stats.
type SavingsStatsResponse struct {
	SavedEURThisMonth    float64 `json:"savedEurThisMonth"`
	SavedCO2KgThisMonth  float64 `json:"savedCo2KgThisMonth"`
	WastedEURThisMonth   float64 `json:"wastedEurThisMonth"`
	WastedCO2KgThisMonth float64 `json:"wastedCo2KgThisMonth"`
	SavedEURLifetime     float64 `json:"savedEurLifetime"`
	SavedCO2KgLifetime   float64 `json:"savedCo2KgLifetime"`
	WastedEURLifetime    float64 `json:"wastedEurLifetime"`
	WastedCO2KgLifetime  float64 `json:"wastedCo2KgLifetime"`
	// LegacyEvents counts the records written before the count model. Their
	// figures are left out of the totals above, because they were scaled by the
	// net content of the product rather than by a count of packs.
	LegacyEvents int    `json:"legacyEvents"`
	CO2Source    string `json:"co2Source"`
}
