package models

import (
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
)

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
	// WastedEvents counts the wasted ledger events of the month. It is not the
	// same measure as WastedCount: that one counts the products that left the
	// pantry plus the ones that expired in it, this one counts the waste
	// actions the savings ledger holds.
	WastedEvents int
	// Causes breaks the month's wasted events down by the reason they were
	// wasted. A waste without a reason lands in the empty cause.
	Causes []apiModel.WasteCauseStat
	// Overbuy names the category the household most over-buys.
	Overbuy apiModel.WasteOverbuyStat
	// Insight is the one-sentence insight about the month's waste, in the
	// recipient's language.
	Insight string
	// InsightAvailable is false when there is too little data to state an
	// insight; the Insight then holds the empty-state sentence.
	InsightAvailable bool
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

// HouseholdMailDigestTarget pairs a household with users that have digest enabled.
type HouseholdMailDigestTarget struct {
	HouseholdID   uint
	HouseholdName string
	Users         []MailDigestUser
}

// MailDigestUser holds user info for digest email delivery.
type MailDigestUser struct {
	UserID              uint
	Email               string
	MailDigestFrequency string
	MailDigestToken     string
}
