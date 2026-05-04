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

	// Database query wrappers
	QueryId               = "id = ?"
	QueryHouseholdId      = "household_id = ?"
	QueryUserId           = "user_id = ?"
	QueryWebhookId        = "webhook_id = ?"
	WhereDeletedIsNotNull = "deleted_at IS NOT NULL"
	WhereDeletedIsNull    = "deleted_at IS NULL"
)
