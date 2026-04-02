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
	ErrPasswordTooShort = errors.New("password must at least be 8 characters long")

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

	// ErrUserWithUsernameExists is thrown when a user with the same username already exists
	ErrUserWithUsernameExists = errors.New("user with this username already exists")

	// ErrUserWithMailAddressExists is thrown when a user with the same mail address already exists
	ErrUserWithMailAddressExists = errors.New("user with this mail address already exists")

	// ErrUserIDFromToken is thrown if no user id is found in token
	ErrUserIDFromToken = errors.New("error getting user id from JWT token")

	// ErrProductSearchInvalidQuery is thrown if a search is ommited but no valid parameter is supplied
	ErrProductSearchInvalidQuery = errors.New("invalid search query specified")

	// ErrParseBody is thrown when a body fails to parse
	ErrParseBody = errors.New("error parsing body")

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

	/*
	 * Notification related errors
	 */
	// ErrNotificationInvalidInterval is thrown if an invalid notification interval was specified
	ErrNotificationInvalidInterval = errors.New("notification interval must be greater than 0")

	// ErrNotificationInvalidSMTPPort is thrown if an invalid SMTP port was specified
	ErrNotificationInvalidSMTPPort = errors.New("SMTP port must be greater than 0")

	// ErrNotificationEmptyFromAddress is thrown if an empty from address was specified
	ErrNotificationEmptyFromAddress = errors.New("notification from address cannot be empty")

	// ErrNotificationInvalidNtfyURL is thrown if an invalid ntfy.sh URL was specified
	ErrNotificationInvalidNtfyURL = errors.New("invalid ntfy.sh URL")

	// ErrNotificationEmptyNtfyTopic is thrown if an empty ntfy.sh topic was specified
	ErrNotificationEmptyNtfyTopic = errors.New("ntfy.sh topic cannot be empty when URL is provided")

	/*
	 * Household related errors
	 */
	// ErrHouseholdNotFound is thrown when a requested household does not exist
	ErrHouseholdNotFound = errors.New("household not found")

	// ErrNotHouseholdAdmin is thrown when a user attempts an admin action on a household they do not administrate
	ErrNotHouseholdAdmin = errors.New("user is not the admin of this household")

	// ErrApplicationAlreadyPending is thrown when a user already has a pending application for a household
	ErrApplicationAlreadyPending = errors.New("a pending application for this household already exists")

	// ErrApplicationNotFound is thrown when a requested household application does not exist
	ErrApplicationNotFound = errors.New("household application not found")
)
