// util contains helper functions and generic vars
package util

import "time"

const (
	DefaultDateFormatParseStr       = "2006-01-02"
	DefaultDateFormatMonthStr       = "2006-01"
	RequestHeaderContentType        = "Content-Type"
	RequestHeaderContentDisposition = "Content-Disposition"

	// Gin context keys
	ContextKeyLogger                 = "logger"
	ContextKeyRepos                  = "repos"
	ContextKeyDBHandle               = "dbHandle"
	ContextKeyNotificationController = "notificationController"
	ContextKeyProviantConfig         = "proviantConfig"
	ContextKeyUserID                 = "userID"
	ContextKeyRequestID              = "requestID"
	ContextKeyCSPNonce               = "cspNonce"
	ContextKeyCSRFToken              = "csrfToken"
	ContextKeyHouseholdID            = "householdID"

	// Database query wrappers
	QueryId               = "id = ?"
	QueryHouseholdId      = "household_id = ?"
	QueryUserId           = "user_id = ?"
	QueryWebhookId        = "webhook_id = ?"
	WhereDeletedIsNotNull = "deleted_at IS NOT NULL"
	WhereDeletedIsNull    = "deleted_at IS NULL"

	// Mail digest
	LabelMailDigest       = "mailDigest"
	LabelMailDigestDot    = "mail_digest"
	LabelMailDigestPascal = "MailDigest"
	RouteUnsubscribe      = "/web/unsubscribe"

	// Password reset
	RouteAuth           = "/web/auth"
	RouteForgotPassword = "/web/forgot-password"
	RouteResetPassword  = "/web/reset-password" //nolint:gosec // G101: route path constant, not a credential

	// Health probes and metrics
	RouteHealth      = "/health"
	RouteHealthReady = "/health/ready"
	RouteMetrics     = "/metrics"

	// Consumption rate estimation
	ConsumptionHistoryWindowDays = 90
	ConsumptionMinSamples        = 2
	// ConsumptionMinSpanDays is the minimum observed time between the
	// first and last consumed sample required to produce a "per week"
	// estimate. Below this, the perWeek denominator (spanDays/7) is
	// not stable enough to label as a weekly rate.
	ConsumptionMinSpanDays    = 7
	ConsumptionSpanFloorDays  = 1
	ConsumptionSourceRate     = "consumption_rate"
	ConsumptionSourceMinStock = "min_stock"
	ConsumptionSourceNone     = "none"

	// Waste analytics period identifiers
	PeriodValueMonth    = "month"
	PeriodValue3Months  = "3months"
	PeriodValue6Months  = "6months"
	PeriodValue12Months = "12months"

	// CSV import column names. The canonical set mirrors the export header
	// (see api/v1/export.go) so export → import round-trips by construction.
	CsvColumnName            = "name"
	CsvColumnBarcode         = "barcode"
	CsvColumnQuantity        = "quantity"
	CsvColumnUnit            = "unit"
	CsvColumnCategory        = "category"
	CsvColumnStorageLocation = "storage_location"
	CsvColumnExpiryDate      = "expiry_date"
	CsvColumnAddedAt         = "added_at"

	// CsvImportTemplateFilename is the attachment name of the download template
	CsvImportTemplateFilename = "proviant-import-template.csv"

	// CsvImportMaxRows bounds a single import request. Exceeding it rejects the
	// whole request instead of truncating it silently.
	CsvImportMaxRows = 5000

	// CsvImportMaxNewLocations bounds how many storage locations one import may
	// bring into existence. Storage locations are permanent rows that the
	// products list, home and product-detail renders all load without a LIMIT,
	// so an uncapped import permanently degrades every later page load. A real
	// pantry is a handful of names; this leaves ample headroom.
	CsvImportMaxNewLocations = 50

	// CsvImportMaxBarcodeLength / CsvImportMinBarcodeLength bound accepted
	// barcodes. GTIN-14 is the widest common code; the lower bound rejects
	// obvious junk.
	CsvImportMaxBarcodeLength = 14
	CsvImportMinBarcodeLength = 8

	// CsvImportBarcodePattern restricts an imported barcode to the characters
	// that are safe in a database round-trip and in the path of the outbound
	// Open Food Facts request. It mirrors the sanitisation
	// OpenFoodFactsAPIController.DownloadImage already applies, so a CSV cell
	// cannot steer that request at another endpoint.
	CsvImportBarcodePattern = "^[A-Za-z0-9_-]+$"

	// BulkCreateMaxItems bounds how many products one batch-scan submission may
	// carry. Exceeding it rejects the whole request instead of truncating it
	// silently, mirroring CsvImportMaxRows for the CSV import.
	BulkCreateMaxItems = 100

	// CsvImportMultipartSlackBytes is added to the configured upload cap before
	// the request body is capped with http.MaxBytesReader, so the file part can
	// be exactly MaxUploadSizeMB without the multipart envelope tripping the
	// transport-level limit.
	CsvImportMultipartSlackBytes = 1 << 20

	// CsvImportOpenFoodFactsLookups bounds how many rows of one import may fall
	// back to a live Open Food Facts request. A CSV of barcode-only rows would
	// otherwise fan out into one outbound call per row, serially, and a single
	// upload could pin the request for hours. Rows past the budget are rejected
	// with fmtImportOffBudget instead of being fetched.
	CsvImportOpenFoodFactsLookups = 20

	// CsvImportOpenFoodFactsBudget bounds the wall-clock time one import may
	// spend on live Open Food Facts lookups. Zero means unbounded.
	CsvImportOpenFoodFactsBudget = 10 * time.Second

	// Recipe API providers
	RecipeProviderThemealDB = "themealdb"
	RecipeProviderMealie    = "mealie"
	RecipeProviderTandoor   = "tandoor"

	// DefaultTheMealDBRecipeAPIURL is the public TheMealDB endpoint. It MUST
	// stay in sync with viper.SetDefault("recipe_api.url", …) in
	// src/proviant.go: the recipe URL validator compares a self-hosted
	// provider's configured URL against this host to keep the operator's API
	// key off the public API, and a drifted copy would silently reopen the leak.
	DefaultTheMealDBRecipeAPIURL = "https://www.themealdb.com/api/json/v1/1"

	// RetiredTokenPassword is the tokenPassword value that released config
	// templates used to ship. It is public, so validation refuses to start
	// with it: anyone who knows it can forge a session token for any user.
	RetiredTokenPassword = "secret key" //nolint:gosec // G101: published placeholder being rejected, not a credential

	// Receipt scan (issue #61)
	ReceiptOCRProviderOpenAI  = "openai"
	ContextKeyReceiptCtrl     = "receiptController"
	ReceiptScanDefaultTimeout = 120

	// DefaultMaxUploadSizeMB mirrors the viper default for
	// server.maxUploadSizeMB (proviant.go). Used as a fallback where a zero or
	// negative configured value would otherwise disable the upload size check.
	DefaultMaxUploadSizeMB = 5

	// ReceiptScanMultipartSlackBytes is added to the configured upload cap
	// before the receipt scan request body is capped with http.MaxBytesReader,
	// so the image part can be exactly MaxUploadSizeMB without the multipart
	// envelope tripping the transport-level limit.
	ReceiptScanMultipartSlackBytes = 1 << 20

	// ReceiptBulkMaxItems bounds one bulk-create request. The review UI sends
	// what one receipt photo plausibly contains; a client is never expected to
	// send hundreds of items in a single call.
	ReceiptBulkMaxItems = 100

	// ReceiptBulkMaxBodyBytes caps the JSON body of one bulk-create request
	// with http.MaxBytesReader before binding. Worst-case math per item: the
	// draft field caps sum to 334 bytes (name 200, categories 100, unit 20,
	// barcode 14), but encoding/json escapes `<`, `>`, `&`, `"` and control
	// characters as \uXXXX, turning each such byte into 6 — so a fully-maxed
	// item whose payload is all-escapable expands to ~2.2KB. Budget 4KB per
	// item to cover that expansion plus per-item JSON syntax, and one extra
	// 4KB for the top-level JSON envelope. Without the transport-level cap,
	// 100 items × unbounded string fields are fully buffered in memory
	// before any per-item clamp runs.
	ReceiptBulkMaxBodyBytes = ReceiptBulkMaxItems*4096 + 4096

	// ReceiptScanMaxTokens bounds the VLM completion. 100 items at ~50 tokens
	// each exceed the previous flat 2000, which truncated large receipts
	// mid-array before parsing ever saw the full output.
	ReceiptScanMaxTokens = 5000

	// ReceiptItem limits for VLM output validation
	ReceiptItemMaxNameLength = 200
	// ReceiptItemMaxCategoriesLength bounds the free-text categories field of
	// a bulk-created product. Receipt scans emit short comma lists; the cap
	// exists so a direct API client cannot push an unbounded string to the DB.
	ReceiptItemMaxCategoriesLength = 100
	ReceiptItemMaxUnitLength       = 20
	ReceiptItemMaxAmount           = 999
	ReceiptItemMaxPrice            = 100000.0
)

// CsvImportColumnAliases maps an accepted alternative column header to its
// canonical name. The issue's proposed column list used `amount`,
// `product_name` and `categories`; rather than a second contract, those become
// aliases of the export header's names. `added_at` is listed so it is
// recognised (and then ignored — GORM owns CreatedAt) instead of silently
// treated as an unknown column.
var CsvImportColumnAliases = map[string]string{
	"product_name":           CsvColumnName,
	CsvColumnName:            CsvColumnName,
	"amount":                 CsvColumnQuantity,
	CsvColumnQuantity:        CsvColumnQuantity,
	CsvColumnUnit:            CsvColumnUnit,
	"categories":             CsvColumnCategory,
	CsvColumnCategory:        CsvColumnCategory,
	"location":               CsvColumnStorageLocation,
	"storage":                CsvColumnStorageLocation,
	CsvColumnStorageLocation: CsvColumnStorageLocation,
	"expiry":                 CsvColumnExpiryDate,
	"expires":                CsvColumnExpiryDate,
	CsvColumnExpiryDate:      CsvColumnExpiryDate,
	CsvColumnBarcode:         CsvColumnBarcode,
	CsvColumnAddedAt:         CsvColumnAddedAt,
}
