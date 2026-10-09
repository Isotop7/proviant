// errors contains custom error definitions
package errors

import "errors"

var (
	// ErrMismatcherUserID occurs if a given user id mismatches the user id of a product owner
	ErrMismatcherUserID = errors.New("mismatching user id of requested product")

	// ErrMismatchedUsername occurs if a username of a given user (by ID) mismatches a given login data
	ErrMismatchedUsername = errors.New("mismatching username")

	// ErrUsernameEmpty is thrown when the given username is too short
	ErrUsernameEmpty = errors.New("username can't be empty")

	// ErrPasswordTooShort is thrown when the given password is too short
	ErrPasswordTooShort = errors.New("password must be at least 12 characters long")

	// ErrPasswordUppercaseRequired is thrown when a password lacks uppercase letters
	ErrPasswordUppercaseRequired = errors.New("password must contain at least one uppercase letter")

	// ErrPasswordDigitRequired is thrown when a password lacks a digit
	ErrPasswordDigitRequired = errors.New("password must contain at least one digit")

	// ErrPasswordSpecialRequired is thrown when a password lacks a special character
	ErrPasswordSpecialRequired = errors.New("password must contain at least one special character")

	// ErrPasswordBreached is thrown when a password has been found in a data breach
	ErrPasswordBreached = errors.New("password has been found in a data breach, please choose a different password")

	// ErrUserHasNoMailAddress is thrown if a given user has no mail address
	ErrUserHasNoMailAddress = errors.New("user has no mail address")

	// ErrUserNoProductsFound is thrown if user products can't be retrieved
	ErrUserNoProductsFound = errors.New("error getting products of user")

	// ErrNoBarcodeFoundInImage is thrown when no barcode can be read from an image
	ErrNoBarcodeFoundInImage = errors.New("error reading barcode from image")

	// ErrBarcodeDecodeTimeoutExceeded is thrown when decoding a barcode from an image takes longer than the allowed timeout
	ErrBarcodeDecodeTimeoutExceeded = errors.New("barcode decoding timeout reached")

	// ErrUserAwareAuthMiddlewareInit is thrown when the user-aware authentication middleware fails to initialize
	ErrUserAwareAuthMiddlewareInit = errors.New("error initializing user-aware authentication middleware")

	// ErrAuthMiddlewareInit is thrown when the authentication middleware fails to initialize
	ErrAuthMiddlewareInit = errors.New("error initializing authentication middleware")

	// ErrInvalidUserData is thrown when supplied user data is invalid
	ErrInvalidUserData = errors.New("invalid user data")

	// ErrInvalidUserID is thrown when supplied user data is invalid
	ErrInvalidUserID = errors.New("invalid user ID")

	// ErrInvalidUserIDWrapper is used to interpolate a invalid user id
	ErrInvalidUserIDWrapper            = "User with id '%d' not found"
	ErrInvalidUserIDWrapperWithMessage = ErrInvalidUserIDWrapper + ": %s"

	// ErrUserWithUsernameExists is thrown when a user with the same username already exists
	ErrUserWithUsernameExists = errors.New("user with this username already exists")

	// ErrUserWithMailAddressExists is thrown when a user with the same mail address already exists
	ErrUserWithMailAddressExists = errors.New("user with this mail address already exists")

	// ErrUserIDFromToken is thrown if no user id is found in token
	ErrUserIDFromToken = errors.New("error getting user id from JWT token")

	// ErrAccountLocked is thrown when an account is temporarily locked due to too many failed login attempts
	ErrAccountLocked = errors.New("account is temporarily locked due to too many failed login attempts")

	// ErrProductSearchInvalidQuery is thrown if a search is ommited but no valid parameter is supplied
	ErrProductSearchInvalidQuery = errors.New("invalid search query specified")

	// ErrParseBody is thrown when a body fails to parse
	ErrParseBody = errors.New("error parsing body")

	// ErrParseBodyWrapper is used to interpolate a non-parseable body
	ErrParseBodyWrapper = "Error parsing body: %s"

	// Message format template for generic error
	FormatGenericError = "%s: %s"

	// Message format template for invalid ID
	FormatInvalidRequestId = "Requested ID '%s' is invalid"

	// Message format template for product not found
	FormatProductWithIDNotFound = "Product with id '%d' was not found"

	/*
	 * Database related errors
	 */
	// ErrDatabaseInvalidEngine is thrown if an invalid database engine is selected
	ErrDatabaseInvalidEngine = errors.New("no valid database engine selected")

	// ErrLoggerContextNotFound is thrown if logger handle can't be found in context
	ErrLoggerContextNotFound = errors.New("failed to get logger from context")

	// ErrDatabaseContextNotFound is thrown if database handle can't be found in context
	ErrDatabaseContextNotFound = errors.New("failed to get database from context")

	// ErrDatabaseInvalidSearchParameter is thrown if a database query contains an invalid search parameter
	ErrDatabaseInvalidSearchParameter = errors.New("invalid search parameter on database call")

	// ErrDatabaseInvalidSortParameter is thrown if a sort column or direction is not on the
	// allowlist. Sort values are interpolated into the SQL ORDER BY clause, so anything that
	// reaches the query must come from that allowlist — never from request input.
	ErrDatabaseInvalidSortParameter = errors.New("invalid sort parameter on database call")

	// ErrDatabaseMariaDBEmptyHost is thrown if an empty MariaDB host was specified
	ErrDatabaseMariaDBEmptyHost = errors.New("empty MariaDB host specified")

	// ErrDatabaseMariaDBEmptyUser is thrown if an empty MariaDB user was specified
	ErrDatabaseMariaDBEmptyUser = errors.New("empty MariaDB user specified")

	// ErrDatabaseMariaDBEmptyPassword is thrown if an empty MariaDB password was specified
	ErrDatabaseMariaDBEmptyPassword = errors.New("empty MariaDB password specified")

	// ErrDatabaseMariaDBEmptyName is thrown if an empty MariaDB database name was specified
	ErrDatabaseMariaDBEmptyName = errors.New("empty MariaDB database name specified")

	// ErrDatabaseMariaDBInvalidPort is thrown if an invalid MariaDB database port was specified
	ErrDatabaseMariaDBInvalidPort = errors.New("no valid MariaDB database port specified")

	// ErrDatabaseSQLiteInvalidPath is thrown if no valid SQLite database path was specified
	ErrDatabaseSQLiteInvalidPath = errors.New("no valid SQLite database path specified")

	// Message format template for product not found
	FormatProductNotFound = "Product with ID '%d' was not found in database"

	// Message format template for product not found for user
	FormatProductForUserNotFound = "Product with ID '%d' for user was not found"

	/*
	 * OpenFoodFacts related errors
	 */
	// ErrOpenFoodFactsAPIEmptyUrl is thrown if an empty url for the OpenFoodFacts API was specified
	ErrOpenFoodFactsAPIEmptyURL = errors.New("empty API URL for OpenFoodFacts specified")

	// ErrOpenFoodFactsAPIInvalidTimeout is thrown if an invalid API timeout was supplied
	ErrOpenFoodFactsAPIInvalidTimeout = errors.New("invalid timeout for OpenFoodFacts API specified")

	// ErrOpenFoodFactsAPIInvalidImageCachePath is thrown if image caching is enabled but no path is specified
	ErrOpenFoodFactsAPIInvalidImageCachePath = errors.New("image cache enabled but no image cache path specified")

	/*
	 * Notification related errors
	 */
	// ErrNotificationInvalidInterval is thrown if an invalid notification interval was specified
	ErrNotificationInvalidInterval = errors.New("notification interval must be greater than 0")

	// ErrNotificationInvalidThreshold is thrown if a negative notification threshold is specified
	ErrNotificationInvalidThreshold = errors.New("notification threshold must be 0 or greater")

	// ErrNotificationInvalidSMTPPort is thrown if an invalid SMTP port was specified
	ErrNotificationInvalidSMTPPort = errors.New("SMTP port must be greater than 0")

	// ErrNotificationEmptyFromAddress is thrown if an empty from address was specified
	ErrNotificationEmptyFromAddress = errors.New("notification from address cannot be empty")

	// ErrNotificationInvalidNtfyURL is thrown if an invalid ntfy.sh URL was specified
	ErrNotificationInvalidNtfyURL = errors.New("invalid ntfy.sh URL")

	// ErrNotificationEmptyNtfyTopic is thrown if an empty ntfy.sh topic was specified
	ErrNotificationEmptyNtfyTopic = errors.New("ntfy.sh topic cannot be empty when URL is provided")

	// ErrNotificationInvalidWasteReportDay is thrown if an invalid monthly waste report day is specified
	ErrNotificationInvalidWasteReportDay = errors.New("monthly waste report day must be between 1 and 28")

	// ErrNotificationInvalidWasteReportHour is thrown if an invalid monthly waste report hour is specified
	ErrNotificationInvalidWasteReportHour = errors.New("monthly waste report hour must be between 0 and 23")

	/*
	 * Household related errors
	 */
	// ErrStorageLocationNotFound is thrown when a requested storage location does not exist
	ErrStorageLocationNotFound = errors.New("storage location not found")

	// ErrStorageLocationNotOwned is thrown when a storage location does not belong to the user's household
	ErrStorageLocationNotOwned = errors.New("storage location does not belong to this household")

	// ErrHouseholdNotFound is thrown when a requested household does not exist
	ErrHouseholdNotFound = errors.New("household not found")

	// ErrHouseholdNameEmpty is thrown when a household name is empty or whitespace
	ErrHouseholdNameEmpty = errors.New("household name cannot be empty")

	// ErrNotHouseholdAdmin is thrown when a user attempts an admin action on a household they do not administrate
	ErrNotHouseholdAdmin = errors.New("user is not the admin of this household")

	// ErrInsufficientRole is thrown when a user attempts an action that requires a higher role
	ErrInsufficientRole = errors.New("insufficient role for this action")

	// ErrApplicationAlreadyPending is thrown when a user already has a pending application for a household
	ErrApplicationAlreadyPending = errors.New("a pending application for this household already exists")

	// ErrApplicationNotFound is thrown when a requested household application does not exist
	ErrApplicationNotFound = errors.New("household application not found")

	// ErrNotApplicationApplicant is thrown when a user tries to cancel an application they did not create
	ErrNotApplicationApplicant = errors.New("user is not the applicant of this application")

	// ErrCannotRemoveAdmin is thrown when an admin tries to remove themselves via the member removal endpoint
	ErrCannotRemoveAdmin = errors.New("cannot remove the household admin")

	// ErrMemberNotInHousehold is thrown when the target user is not a member of the caller's household
	ErrMemberNotInHousehold = errors.New("user is not a member of this household")

	// ErrUserNotInHousehold is thrown when an admin targets a user not in their household
	ErrUserNotInHousehold = errors.New("user is not a member of this household")

	/*
	 * Invitation related errors
	 */
	// ErrInvitationNotFound is thrown when a requested invitation does not exist
	ErrInvitationNotFound = errors.New("invitation not found")

	// ErrInvitationExpired is thrown when an invitation has passed its expiry time
	ErrInvitationExpired = errors.New("invitation has expired")

	// ErrInvitationAlreadyUsed is thrown when an invitation has already been accepted
	ErrInvitationAlreadyUsed = errors.New("invitation has already been accepted")

	// ErrInvitationCancelled is thrown when an invitation has been cancelled by the sender
	ErrInvitationCancelled = errors.New("invitation has been cancelled")

	// ErrInvitationEmailMismatch is thrown when the recipient email does not match the invitation
	ErrInvitationEmailMismatch = errors.New("email does not match invitation")

	// ErrDuplicateInvitation is thrown when a pending invitation already exists for the same email and household
	ErrDuplicateInvitation = errors.New("a pending invitation already exists for this email")

	// ErrInvitationNotAuthorized is thrown when a user tries to manage an invitation they did not create
	ErrInvitationNotAuthorized = errors.New("not authorized to manage this invitation")

	/*
	 * Email verification related errors
	 */
	// ErrEmailNotVerified is thrown when a user attempts to login without verifying their email
	ErrEmailNotVerified = errors.New("email address not verified")

	/*
	 * Password reset related errors
	 */
	// ErrPasswordResetTokenInvalid is thrown when a password reset token is missing, unknown, expired, or already used
	ErrPasswordResetTokenInvalid = errors.New("password reset link is invalid or has expired")

	// ErrPasswordResetTokenExpired is thrown when a password reset token has passed its expiry
	ErrPasswordResetTokenExpired = errors.New("password reset link has expired")

	// ErrPasswordResetTokenUsed is thrown when a password reset token has already been consumed
	ErrPasswordResetTokenUsed = errors.New("password reset link has already been used")

	/*
	 * Personal Access Token related errors
	 */
	// ErrPATNotFound is thrown when a PAT does not exist
	ErrPATNotFound = errors.New("personal access token not found")

	// ErrPATExpired is thrown when a PAT has expired
	ErrPATExpired = errors.New("personal access token expired")

	// ErrPATInvalid is thrown when a PAT is invalid
	ErrPATInvalid = errors.New("invalid personal access token")

	// ErrPATInsufficientScope is thrown when a PAT lacks required scope
	ErrPATInsufficientScope = errors.New("insufficient token scope")

	/*
	 * Webhook related errors
	 */
	// ErrWebhookNotFound is thrown when a webhook does not exist
	ErrWebhookNotFound = errors.New("webhook not found")

	// ErrWebhookURLInvalid is thrown when a webhook URL is invalid
	ErrWebhookURLInvalid = errors.New("webhook URL is invalid")

	// ErrWebhookURLPrivateIP is thrown when a webhook URL resolves to a private or internal IP address
	ErrWebhookURLPrivateIP = errors.New("webhook URL must not point to a private or internal IP address")

	// ErrWebhookURLNotHTTPS is thrown when a webhook URL does not use HTTPS
	ErrWebhookURLNotHTTPS = errors.New("webhook URL must use HTTPS")

	// ErrWebhookSecretTooShort is thrown when a webhook secret is too short
	ErrWebhookSecretTooShort = errors.New("webhook secret must be at least 16 characters")

	// ErrWebhookInvalidEvent is thrown when an invalid webhook event is specified
	ErrWebhookInvalidEvent = errors.New("invalid webhook event")

	// ErrWebhookNotOwner is thrown when a user tries to access a webhook they do not own
	ErrWebhookNotOwner = errors.New("webhook does not belong to user")

	/*
	 * Server configuration related errors
	 */
	// ErrServerEmptyTokenPassword is thrown if no JWT token password was specified
	ErrServerEmptyTokenPassword = errors.New(
		"JWT token password cannot be empty: set server.authentication.tokenPassword " +
			"(generate one with 'openssl rand -hex 32') or the " +
			"PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD environment variable")

	// ErrServerRetiredTokenPassword is thrown if the JWT token password is a
	// default that earlier releases shipped publicly in their config templates
	ErrServerRetiredTokenPassword = errors.New(
		"JWT token password is a publicly known default shipped by an earlier release: " +
			"set server.authentication.tokenPassword (generate one with 'openssl rand -hex 32') or the " +
			"PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD environment variable")

	// ErrServerInvalidTokenLifetime is thrown if an invalid JWT token lifetime was specified
	ErrServerInvalidTokenLifetime = errors.New("JWT token lifetime must be greater than 0")

	// ErrRateLimitInvalidValue is thrown if any rate limit value is zero or negative
	ErrRateLimitInvalidValue = errors.New("rate limit values must be greater than 0")

	/*
	 * CSRF related errors
	 */
	// ErrCSRFTokenInvalid is thrown when a CSRF token is missing or does not match the expected value
	ErrCSRFTokenInvalid = errors.New("CSRF token validation failed")

	/*
	 * Savings related errors
	 */
	// ErrSavingsRecordFailed is thrown when a savings record cannot be written
	ErrSavingsRecordFailed = errors.New("failed to record savings event")

	// ErrSavingsStatsUnavailable is thrown when savings statistics cannot be computed
	ErrSavingsStatsUnavailable = errors.New("savings statistics unavailable")

	/*
	 * OCR related errors
	 */
	// ErrOCRTimeout is thrown when OCR processing exceeds the configured timeout
	ErrOCRTimeout = errors.New("OCR processing timeout exceeded")

	// ErrOCRProcessing is thrown when OCR processing fails (generic)
	ErrOCRProcessing = errors.New("OCR processing failed")

	// ErrReceiptEndpoint is returned to clients when the configured vision
	// endpoint itself fails (unreachable, non-2xx status or an endpoint-reported
	// error such as 401/403 auth or config problems). Distinct from
	// ErrOCRProcessing so an upstream problem is not reported as an internal
	// server error.
	ErrReceiptEndpoint = errors.New("receipt scan endpoint failed")

	// ErrReceiptOCRDisabled is returned when receipt scanning is used while the ocr.receipt feature is disabled
	ErrReceiptOCRDisabled = errors.New("receipt scanning is not enabled on this server")

	// ErrReceiptOCRInvalidProvider is thrown when ocr.receipt is enabled with a provider
	// that is not an OpenAI-compatible vision endpoint
	ErrReceiptOCRInvalidProvider = errors.New("receipt scanning requires an OpenAI-compatible vision provider")

	// ErrReceiptOCREmptyModel is thrown when ocr.receipt is enabled without a model name
	ErrReceiptOCREmptyModel = errors.New("receipt scanning requires a model name")

	// ErrReceiptScanInvalidEndpoint is thrown when a per-user receipt scan endpoint is not an absolute http/https URL with a host
	ErrReceiptScanInvalidEndpoint = errors.New("receipt scan endpoint must be an absolute http:// or https:// URL")

	// ErrReceiptScanPrivateIP is thrown when a per-user receipt scan endpoint points to a private or internal IP address
	// (literal IPs at save time, resolved addresses at request time)
	ErrReceiptScanPrivateIP = errors.New("receipt scan endpoint must not point to a private or internal IP address")

	// ErrReceiptScanInvalidTimeout is thrown when a per-user receipt scan timeout is out of range
	ErrReceiptScanInvalidTimeout = errors.New("receipt scan timeout must be between 1 and 900 seconds")

	// ErrFileTooLarge is thrown when the uploaded image exceeds the size limit
	ErrFileTooLarge = errors.New("uploaded file too large")

	// ErrInvalidImageType is thrown when an uploaded receipt photo is not a
	// supported image format (JPEG, PNG or WebP)
	ErrInvalidImageType = errors.New("uploaded file is not a supported image")

	// ErrInvalidRequest is thrown when the request is malformed or missing required parameters
	ErrInvalidRequest = errors.New("invalid request")

	// ErrInvalidOpenedLifecycle is thrown if a product's OpenedAt / DaysAfterOpening
	// fields violate the opened-shelf-life bounds
	ErrInvalidOpenedLifecycle = errors.New(
		"daysAfterOpening must be between 0 and 365 and openedAt must not be more than one day in the future")

	// ErrTokenExpired is thrown when a calendar token has passed its expiration date
	ErrTokenExpired = errors.New("calendar token expired")

	// ErrInternalServer is thrown when an internal server error occurs
	ErrInternalServer = errors.New("internal server error")

	/*
	 * Recipe related errors
	 */
	// ErrRecipeAPIUnavailable is thrown when the recipe API is unreachable or returns an error
	ErrRecipeAPIUnavailable = errors.New("recipe service unavailable")

	// ErrRecipeCacheMiss is thrown when a cache lookup fails to find an entry
	ErrRecipeCacheMiss = errors.New("recipe cache miss")

	// ErrRecipeInvalidProvider is thrown when the configured recipe API provider is not supported
	ErrRecipeInvalidProvider = errors.New("invalid recipe API provider")

	// ErrRecipeAPIMissingAPIKey is thrown when a self-hosted recipe provider is
	// configured without the API key it requires. It is kept separate from
	// ErrRecipeInvalidProvider so startup can degrade this recoverable
	// misconfiguration instead of refusing to boot.
	ErrRecipeAPIMissingAPIKey = errors.New("missing recipe API key for configured provider")

	// ErrRecipeAPIProviderURLMismatch is thrown when a self-hosted recipe
	// provider is configured with the public TheMealDB URL. Every outbound
	// request would then carry the operator's API key and the household's
	// inventory-derived search keywords to a third party, so startup refuses
	// instead of falling back to the public API.
	ErrRecipeAPIProviderURLMismatch = errors.New(
		"recipe API URL points at the public TheMealDB host; " +
			"a self-hosted provider (mealie/tandoor) must use its own instance URL")

	// ErrRecipeAPIEmptyURL is thrown when the recipe API URL is empty
	ErrRecipeAPIEmptyURL = errors.New("empty recipe API URL")

	// ErrRecipeAPIInvalidTimeout is thrown when the recipe API timeout is invalid
	ErrRecipeAPIInvalidTimeout = errors.New("invalid timeout for recipe API")

	// ErrRecipeNoMatchesFound is thrown when no recipes match the given products
	ErrRecipeNoMatchesFound = errors.New("no matching recipes found")

	// ErrUserHasNoHousehold is thrown when a user does not belong to a household
	ErrUserHasNoHousehold = errors.New("user has no household")

	// ErrInvalidQueryParameter is thrown when a query parameter is invalid
	ErrInvalidQueryParameter = errors.New("invalid query parameter")

	// ErrDatabaseOperationFailed is thrown when a database operation fails
	ErrDatabaseOperationFailed = errors.New("database operation failed")

	// ErrProductConcurrentModification is thrown when a product was modified by another request while an update was being prepared
	ErrProductConcurrentModification = errors.New("product was modified by another request, please retry")

	/*
	 * Export related errors
	 */
	// ErrExportCSVWriteWrapper is used to interpolate csv export error
	ErrExportCSVWriteWrapper = "CSV write error: %s"

	/*
	 * CSV import related errors
	 */
	// ErrImportMissingFile is thrown when the import request carries no file part
	ErrImportMissingFile = errors.New("no CSV file uploaded")

	// ErrImportNoBarcodeColumn is thrown when the CSV header has no barcode column
	ErrImportNoBarcodeColumn = errors.New("CSV header must contain a barcode column")

	// ErrImportTooManyRows is thrown when the CSV exceeds util.CsvImportMaxRows
	ErrImportTooManyRows = errors.New("CSV contains too many rows")

	// ErrImportEmptyFile is thrown when the CSV has a header but no data rows
	ErrImportEmptyFile = errors.New("CSV contains no product rows")

	// ErrImportTooManyLocations is thrown when the CSV names more distinct new
	// storage locations than util.CsvImportMaxNewLocations
	ErrImportTooManyLocations = errors.New("CSV names too many new storage locations")

	// ErrImportRowWrapper formats a rejected CSV line for the log. The response
	// itself keeps row and reason in separate fields, so it does not need it.
	ErrImportRowWrapper = "ImportProducts: row %d rejected: %s"

	/*
	 * Stats/export shared log format strings
	 */
	FmtErrGetActiveProductsCount      = "GetActiveProductsCount: %s"
	FmtErrGetExpiredProductsCount     = "GetExpiredProductsCount: %s"
	FmtErrGetExpiringSoonProducts     = "GetExpiringSoonProducts: %s"
	FmtErrGetProductCategoryBreakdown = "GetProductCategoryBreakdown: %s"
	FmtErrGetExpiryTrend              = "GetExpiryTrend: %s"
	FmtErrGetUniqueArchivedCount      = "GetUniqueArchivedProductsCount: %s"

	/*
	 * Stats/export shared HTTP response messages
	 */
	MsgErrComputingActiveCount         = "Error computing active product count"
	MsgErrComputingWasteCount          = "Error computing waste count"
	MsgErrComputingUniqueArchivedCount = "Error computing unique archived count"
	MsgErrComputingExpiringSoon        = "Error computing expiring soon products"
	MsgErrComputingCategoryBreakdown   = "Error computing category breakdown"
	MsgErrComputingExpiryTrend         = "Error computing expiry trend"
	MsgErrGettingProducts              = "Error getting products"
)

// ReceiptEndpointError marks an OCR scan failure as originating from the
// upstream vision endpoint rather than from Proviant itself. The API layer
// maps it to 502 Bad Gateway instead of 500 via errors.As.
type ReceiptEndpointError struct {
	Err error
}

func (e *ReceiptEndpointError) Error() string { return e.Err.Error() }

func (e *ReceiptEndpointError) Unwrap() error { return e.Err }
