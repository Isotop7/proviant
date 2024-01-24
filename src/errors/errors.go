// errors contains custom error definitions
package errors

import "errors"

var (
	// ErrMismatcherUserID occurs if a given user id mismatches the user id of a product owner
	ErrMismatcherUserID = errors.New("mismatching user id of requested product")

	// ErrMismatchedUsername occurs if a username of a given user (by ID) mismatches a given login data
	ErrMismatchedUsername = errors.New("mismatching username")

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

	// ErrInvalidUserData is thrown when supplied user data is invalid
	ErrInvalidUserData = errors.New("invalid user data")

	// ErrUserWithUsernameExists is thrown when a user with the same username already exists
	ErrUserWithUsernameExists = errors.New("user with this username already exists")

	// ErrUserWithMailAddressExists is thrown when a user with the same mail address already exists
	ErrUserWithMailAddressExists = errors.New("user with this mail address already exists")

	// ErrDatabaseContextNotFound is thrown if database handle can't be found in context
	ErrDatabaseContextNotFound = errors.New("failed to get database from context")

	// ErrUserIDFromToken is thrown if no user id is found in token
	ErrUserIDFromToken = errors.New("error getting user id from JWT token")

	// ErrParseBody is thrown when a body fails to parse
	ErrParseBody = errors.New("error parsing body")
)
