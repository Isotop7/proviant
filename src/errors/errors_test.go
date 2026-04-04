package errors

import (
	"testing"
)

func TestErrorDefinitions(t *testing.T) {
	t.Run("ErrMismatcherUserID is defined", func(t *testing.T) {
		if ErrMismatcherUserID == nil {
			t.Error("ErrMismatcherUserID is nil")
		}
		if ErrMismatcherUserID.Error() != "mismatching user id of requested product" {
			t.Errorf("ErrMismatcherUserID.Error() = %v, want mismatching user id of requested product", ErrMismatcherUserID.Error())
		}
	})

	t.Run("ErrMismatchedUsername is defined", func(t *testing.T) {
		if ErrMismatchedUsername == nil {
			t.Error("ErrMismatchedUsername is nil")
		}
		if ErrMismatchedUsername.Error() != "mismatching username" {
			t.Errorf("ErrMismatchedUsername.Error() = %v, want mismatching username", ErrMismatchedUsername.Error())
		}
	})

	t.Run("ErrUsernameEmpty is defined", func(t *testing.T) {
		if ErrUsernameEmpty == nil {
			t.Error("ErrUsernameEmpty is nil")
		}
		if ErrUsernameEmpty.Error() != "username can't be empty" {
			t.Errorf("ErrUsernameEmpty.Error() = %v, want username can't be empty", ErrUsernameEmpty.Error())
		}
	})

	t.Run("ErrPasswordTooShort is defined", func(t *testing.T) {
		if ErrPasswordTooShort == nil {
			t.Error("ErrPasswordTooShort is nil")
		}
		if ErrPasswordTooShort.Error() != "password must at least be 8 characters long" {
			t.Errorf("ErrPasswordTooShort.Error() = %v, want password must at least be 8 characters long", ErrPasswordTooShort.Error())
		}
	})

	t.Run("ErrUserHasNoMailAddress is defined", func(t *testing.T) {
		if ErrUserHasNoMailAddress == nil {
			t.Error("ErrUserHasNoMailAddress is nil")
		}
		if ErrUserHasNoMailAddress.Error() != "user has no mail address" {
			t.Errorf("ErrUserHasNoMailAddress.Error() = %v, want user has no mail address", ErrUserHasNoMailAddress.Error())
		}
	})

	t.Run("ErrUserNoProductsFound is defined", func(t *testing.T) {
		if ErrUserNoProductsFound == nil {
			t.Error("ErrUserNoProductsFound is nil")
		}
		if ErrUserNoProductsFound.Error() != "error getting products of user" {
			t.Errorf("ErrUserNoProductsFound.Error() = %v, want error getting products of user", ErrUserNoProductsFound.Error())
		}
	})

	t.Run("ErrNoBarcodeFoundInImage is defined", func(t *testing.T) {
		if ErrNoBarcodeFoundInImage == nil {
			t.Error("ErrNoBarcodeFoundInImage is nil")
		}
		if ErrNoBarcodeFoundInImage.Error() != "error reading barcode from image" {
			t.Errorf("ErrNoBarcodeFoundInImage.Error() = %v, want error reading barcode from image", ErrNoBarcodeFoundInImage.Error())
		}
	})

	t.Run("ErrBarcodeDecodeTimeoutExceeded is defined", func(t *testing.T) {
		if ErrBarcodeDecodeTimeoutExceeded == nil {
			t.Error("ErrBarcodeDecodeTimeoutExceeded is nil")
		}
		if ErrBarcodeDecodeTimeoutExceeded.Error() != "barcode decoding timeout reached" {
			t.Errorf("ErrBarcodeDecodeTimeoutExceeded.Error() = %v, want barcode decoding timeout reached", ErrBarcodeDecodeTimeoutExceeded.Error())
		}
	})
}

func TestAuthenticationErrors(t *testing.T) {
	t.Run("ErrUserAwareAuthMiddlewareInit is defined", func(t *testing.T) {
		if ErrUserAwareAuthMiddlewareInit == nil {
			t.Error("ErrUserAwareAuthMiddlewareInit is nil")
		}
		if ErrUserAwareAuthMiddlewareInit.Error() != "error initializing user-aware authentication middleware" {
			t.Errorf("ErrUserAwareAuthMiddlewareInit.Error() = %v, want error initializing user-aware authentication middleware", ErrUserAwareAuthMiddlewareInit.Error())
		}
	})

	t.Run("ErrAuthMiddlewareInit is defined", func(t *testing.T) {
		if ErrAuthMiddlewareInit == nil {
			t.Error("ErrAuthMiddlewareInit is nil")
		}
		if ErrAuthMiddlewareInit.Error() != "error initializing authentication middleware" {
			t.Errorf("ErrAuthMiddlewareInit.Error() = %v, want error initializing authentication middleware", ErrAuthMiddlewareInit.Error())
		}
	})

	t.Run("ErrInvalidUserData is defined", func(t *testing.T) {
		if ErrInvalidUserData == nil {
			t.Error("ErrInvalidUserData is nil")
		}
		if ErrInvalidUserData.Error() != "invalid user data" {
			t.Errorf("ErrInvalidUserData.Error() = %v, want invalid user data", ErrInvalidUserData.Error())
		}
	})

	t.Run("ErrInvalidUserID is defined", func(t *testing.T) {
		if ErrInvalidUserID == nil {
			t.Error("ErrInvalidUserID is nil")
		}
		if ErrInvalidUserID.Error() != "invalid user ID" {
			t.Errorf("ErrInvalidUserID.Error() = %v, want invalid user ID", ErrInvalidUserID.Error())
		}
	})

	t.Run("ErrUserWithUsernameExists is defined", func(t *testing.T) {
		if ErrUserWithUsernameExists == nil {
			t.Error("ErrUserWithUsernameExists is nil")
		}
		if ErrUserWithUsernameExists.Error() != "user with this username already exists" {
			t.Errorf("ErrUserWithUsernameExists.Error() = %v, want user with this username already exists", ErrUserWithUsernameExists.Error())
		}
	})

	t.Run("ErrUserWithMailAddressExists is defined", func(t *testing.T) {
		if ErrUserWithMailAddressExists == nil {
			t.Error("ErrUserWithMailAddressExists is nil")
		}
		if ErrUserWithMailAddressExists.Error() != "user with this mail address already exists" {
			t.Errorf("ErrUserWithMailAddressExists.Error() = %v, want user with this mail address already exists", ErrUserWithMailAddressExists.Error())
		}
	})

	t.Run("ErrUserIDFromToken is defined", func(t *testing.T) {
		if ErrUserIDFromToken == nil {
			t.Error("ErrUserIDFromToken is nil")
		}
		if ErrUserIDFromToken.Error() != "error getting user id from JWT token" {
			t.Errorf("ErrUserIDFromToken.Error() = %v, want error getting user id from JWT token", ErrUserIDFromToken.Error())
		}
	})
}

func TestDatabaseErrors(t *testing.T) {
	t.Run("ErrDatabaseInvalidEngine is defined", func(t *testing.T) {
		if ErrDatabaseInvalidEngine == nil {
			t.Error("ErrDatabaseInvalidEngine is nil")
		}
		if ErrDatabaseInvalidEngine.Error() != "no valid database engine selected" {
			t.Errorf("ErrDatabaseInvalidEngine.Error() = %v, want no valid database engine selected", ErrDatabaseInvalidEngine.Error())
		}
	})

	t.Run("ErrLoggerContextNotFound is defined", func(t *testing.T) {
		if ErrLoggerContextNotFound == nil {
			t.Error("ErrLoggerContextNotFound is nil")
		}
		if ErrLoggerContextNotFound.Error() != "failed to get logger from context" {
			t.Errorf("ErrLoggerContextNotFound.Error() = %v, want failed to get logger from context", ErrLoggerContextNotFound.Error())
		}
	})

	t.Run("ErrDatabaseContextNotFound is defined", func(t *testing.T) {
		if ErrDatabaseContextNotFound == nil {
			t.Error("ErrDatabaseContextNotFound is nil")
		}
		if ErrDatabaseContextNotFound.Error() != "failed to get database from context" {
			t.Errorf("ErrDatabaseContextNotFound.Error() = %v, want failed to get database from context", ErrDatabaseContextNotFound.Error())
		}
	})

	t.Run("ErrDatabaseInvalidSearchParameter is defined", func(t *testing.T) {
		if ErrDatabaseInvalidSearchParameter == nil {
			t.Error("ErrDatabaseInvalidSearchParameter is nil")
		}
		if ErrDatabaseInvalidSearchParameter.Error() != "invalid search parameter on database call" {
			t.Errorf("ErrDatabaseInvalidSearchParameter.Error() = %v, want invalid search parameter on database call", ErrDatabaseInvalidSearchParameter.Error())
		}
	})
}

func TestMariaDBErrors(t *testing.T) {
	t.Run("ErrDatabaseMariaDBEmptyHost is defined", func(t *testing.T) {
		if ErrDatabaseMariaDBEmptyHost == nil {
			t.Error("ErrDatabaseMariaDBEmptyHost is nil")
		}
		if ErrDatabaseMariaDBEmptyHost.Error() != "empty MariaDB host specified" {
			t.Errorf("ErrDatabaseMariaDBEmptyHost.Error() = %v, want empty MariaDB host specified", ErrDatabaseMariaDBEmptyHost.Error())
		}
	})

	t.Run("ErrDatabaseMariaDBEmptyUser is defined", func(t *testing.T) {
		if ErrDatabaseMariaDBEmptyUser == nil {
			t.Error("ErrDatabaseMariaDBEmptyUser is nil")
		}
		if ErrDatabaseMariaDBEmptyUser.Error() != "empty MariaDB user specified" {
			t.Errorf("ErrDatabaseMariaDBEmptyUser.Error() = %v, want empty MariaDB user specified", ErrDatabaseMariaDBEmptyUser.Error())
		}
	})

	t.Run("ErrDatabaseMariaDBEmptyPassword is defined", func(t *testing.T) {
		if ErrDatabaseMariaDBEmptyPassword == nil {
			t.Error("ErrDatabaseMariaDBEmptyPassword is nil")
		}
		if ErrDatabaseMariaDBEmptyPassword.Error() != "empty MariaDB password specified" {
			t.Errorf("ErrDatabaseMariaDBEmptyPassword.Error() = %v, want empty MariaDB password specified", ErrDatabaseMariaDBEmptyPassword.Error())
		}
	})

	t.Run("ErrDatabaseMariaDBEmptyName is defined", func(t *testing.T) {
		if ErrDatabaseMariaDBEmptyName == nil {
			t.Error("ErrDatabaseMariaDBEmptyName is nil")
		}
		if ErrDatabaseMariaDBEmptyName.Error() != "empty MariaDB database name specified" {
			t.Errorf("ErrDatabaseMariaDBEmptyName.Error() = %v, want empty MariaDB database name specified", ErrDatabaseMariaDBEmptyName.Error())
		}
	})

	t.Run("ErrDatabaseMariaDBInvalidPort is defined", func(t *testing.T) {
		if ErrDatabaseMariaDBInvalidPort == nil {
			t.Error("ErrDatabaseMariaDBInvalidPort is nil")
		}
		if ErrDatabaseMariaDBInvalidPort.Error() != "no valid MariaDB database port specified" {
			t.Errorf("ErrDatabaseMariaDBInvalidPort.Error() = %v, want no valid MariaDB database port specified", ErrDatabaseMariaDBInvalidPort.Error())
		}
	})

	t.Run("ErrDatabaseSQLiteInvalidPath is defined", func(t *testing.T) {
		if ErrDatabaseSQLiteInvalidPath == nil {
			t.Error("ErrDatabaseSQLiteInvalidPath is nil")
		}
		if ErrDatabaseSQLiteInvalidPath.Error() != "no valid SQLite database path specified" {
			t.Errorf("ErrDatabaseSQLiteInvalidPath.Error() = %v, want no valid SQLite database path specified", ErrDatabaseSQLiteInvalidPath.Error())
		}
	})
}

func TestOpenFoodFactsErrors(t *testing.T) {
	t.Run("ErrOpenFoodFactsAPIEmptyUrl is defined", func(t *testing.T) {
		if ErrOpenFoodFactsAPIEmptyURL == nil {
			t.Error("ErrOpenFoodFactsAPIEmptyUrl is nil")
		}
		if ErrOpenFoodFactsAPIEmptyURL.Error() != "empty API URL for OpenFoodFacts specified" {
			t.Errorf("ErrOpenFoodFactsAPIEmptyUrl.Error() = %v, want empty API URL for OpenFoodFacts specified", ErrOpenFoodFactsAPIEmptyURL.Error())
		}
	})

	t.Run("ErrOpenFoodFactsAPIInvalidTimeout is defined", func(t *testing.T) {
		if ErrOpenFoodFactsAPIInvalidTimeout == nil {
			t.Error("ErrOpenFoodFactsAPIInvalidTimeout is nil")
		}
		if ErrOpenFoodFactsAPIInvalidTimeout.Error() != "invalid timeout for OpenFoodFacts API specified" {
			t.Errorf("ErrOpenFoodFactsAPIInvalidTimeout.Error() = %v, want invalid timeout for OpenFoodFacts API specified", ErrOpenFoodFactsAPIInvalidTimeout.Error())
		}
	})
}

func TestFormatTemplates(t *testing.T) {
	t.Run("FormatGenericError is defined", func(t *testing.T) {
		if FormatGenericError == "" {
			t.Error("FormatGenericError is empty")
		}
		if FormatGenericError != "%s: %s" {
			t.Errorf("FormatGenericError = %v, want '%%s: %%s'", FormatGenericError)
		}
	})

	t.Run("FormatInvalidRequestId is defined", func(t *testing.T) {
		if FormatInvalidRequestId == "" {
			t.Error("FormatInvalidRequestId is empty")
		}
		if FormatInvalidRequestId != "Requested ID '%s' is invalid" {
			t.Errorf("FormatInvalidRequestId = %v, want Requested ID '%%s' is invalid", FormatInvalidRequestId)
		}
	})

	t.Run("FormatProductWithIDNotFound is defined", func(t *testing.T) {
		if FormatProductWithIDNotFound == "" {
			t.Error("FormatProductWithIDNotFound is empty")
		}
		if FormatProductWithIDNotFound != "Product with id '%d' was not found" {
			t.Errorf("FormatProductWithIDNotFound = %v, want Product with id '%%d' was not found", FormatProductWithIDNotFound)
		}
	})

	t.Run("FormatProductForUserNotFound is defined", func(t *testing.T) {
		if FormatProductForUserNotFound == "" {
			t.Error("FormatProductForUserNotFound is empty")
		}
		if FormatProductForUserNotFound != "Product with ID '%d' for user was not found" {
			t.Errorf("FormatProductForUserNotFound = %v, want Product with ID '%%d' for user was not found", FormatProductForUserNotFound)
		}
	})

	t.Run("FormatProductNotFound is defined", func(t *testing.T) {
		if FormatProductNotFound == "" {
			t.Error("FormatProductNotFound is empty")
		}
		if FormatProductNotFound != "Product with ID '%d' was not found in database" {
			t.Errorf("FormatProductNotFound = %v, want Product with ID '%%d' was not found in database", FormatProductNotFound)
		}
	})
}
