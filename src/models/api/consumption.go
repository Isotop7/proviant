package api

import "time"

type ConsumptionRateResponse struct {
	ProductID    uint       `json:"productId"`
	HasEstimate  bool       `json:"hasEstimate"`
	PerWeek      float64    `json:"perWeek"`
	Unit         string     `json:"unit"`
	SampleCount  int        `json:"sampleCount"`
	DaysCovered  int        `json:"daysCovered"`
	LastConsumed *time.Time `json:"lastConsumed,omitempty"`
	Display      string     `json:"display"`
}

type RestockSuggestionResponse struct {
	ProductID      uint    `json:"productId"`
	ProductName    string  `json:"productName"`
	HasSuggestion  bool    `json:"hasSuggestion"`
	HasEstimate    bool    `json:"hasEstimate"`
	SuggestedQty   int     `json:"suggestedQty"`
	Unit           string  `json:"unit"`
	Source         string  `json:"source"`
	WeeklyRate     float64 `json:"weeklyRate"`
	SampleCount    int     `json:"sampleCount"`
	Display        string  `json:"display"`
	PerWeekDisplay string  `json:"perWeekDisplay"`
}
