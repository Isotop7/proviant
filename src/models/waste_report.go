package models

import "time"

// WasteStats holds aggregated waste data for one household for one calendar month.
type WasteStats struct {
	Month            time.Time
	MonthLabel       string // "January 2006"
	HouseholdID      uint
	HouseholdName    string
	DeletedCount     int     // products removed from pantry during the month
	ExpiredCount     int     // products still in pantry that passed best-before during the month
	WastedCount      int     // DeletedCount + ExpiredCount
	PrevWastedCount  int
	WasteRatePct     float64 // WastedCount / active products * 100
	PrevWasteRatePct float64
	Delta            float64 // WasteRatePct - PrevWasteRatePct (positive = worse)
	DeltaColor       string  // inline CSS hex color for the trend indicator
	DeltaSymbol      string  // "▲", "▼", or "="
}

// HouseholdReportTarget pairs a household with the email addresses of opted-in members.
type HouseholdReportTarget struct {
	HouseholdID   uint
	HouseholdName string
	Recipients    []string
}
