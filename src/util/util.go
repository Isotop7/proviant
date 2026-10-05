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
