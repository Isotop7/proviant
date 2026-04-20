package models

import "time"

// WasteStats holds aggregated waste data for one household for one calendar month.
type WasteStats struct {
	Month            time.Time
	MonthLabel       string // "January 2006"
	HouseholdID      uint
	HouseholdName    string
	DeletedCount     int // products removed from pantry during the month
	ExpiredCount     int // products still in pantry that passed best-before during the month
	WastedCount      int // DeletedCount + ExpiredCount
	PrevWastedCount  int
	WasteRatePct     float64 // WastedCount / active products * 100
	PrevWasteRatePct float64
	Delta            float64 // WasteRatePct - PrevWasteRatePct (positive = worse)
	DeltaColor       string  // inline CSS hex color for the trend indicator
	DeltaSymbol      string  // "▲", "▼", or "="
}

// TelegramRecipient pairs a Telegram chat ID with the user's own bot token.
type TelegramRecipient struct {
	ChatID   string
	BotToken string
}

// HouseholdReportTarget pairs a household with the opted-in members for email and Telegram.
type HouseholdReportTarget struct {
	HouseholdID        uint
	HouseholdName      string
	Recipients         []string            // email addresses
	TelegramRecipients []TelegramRecipient // per-user bot token + chat ID pairs
}
