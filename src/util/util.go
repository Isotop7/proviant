// util contains helper functions and generic vars
package util

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
	ConsumptionMinSpanDays     = 7
	ConsumptionSpanFloorDays   = 1
	ConsumptionSourceRate      = "consumption_rate"
	ConsumptionSourceMinStock  = "min_stock"
	ConsumptionSourceNone      = "none"
)
