package errors

import (
	"testing"
)

func assertErrorDefined(t *testing.T, name string, err error, want string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s is nil", name)
		return
	}
	if err.Error() != want {
		t.Errorf("%s.Error() = %q, want %q", name, err.Error(), want)
	}
}

func TestErrorDefinitions(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"ErrMismatcherUserID", ErrMismatcherUserID, "mismatching user id of requested product"},
		{"ErrMismatchedUsername", ErrMismatchedUsername, "mismatching username"},
		{"ErrUsernameEmpty", ErrUsernameEmpty, "username can't be empty"},
		{"ErrPasswordTooShort", ErrPasswordTooShort, "password must be at least 12 characters long"},
		{"ErrUserHasNoMailAddress", ErrUserHasNoMailAddress, "user has no mail address"},
		{"ErrUserNoProductsFound", ErrUserNoProductsFound, "error getting products of user"},
		{"ErrNoBarcodeFoundInImage", ErrNoBarcodeFoundInImage, "error reading barcode from image"},
		{"ErrBarcodeDecodeTimeoutExceeded", ErrBarcodeDecodeTimeoutExceeded, "barcode decoding timeout reached"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertErrorDefined(t, tc.name, tc.err, tc.want)
		})
	}
}

func TestAuthenticationErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"ErrUserAwareAuthMiddlewareInit", ErrUserAwareAuthMiddlewareInit, "error initializing user-aware authentication middleware"},
		{"ErrAuthMiddlewareInit", ErrAuthMiddlewareInit, "error initializing authentication middleware"},
		{"ErrInvalidUserData", ErrInvalidUserData, "invalid user data"},
		{"ErrInvalidUserID", ErrInvalidUserID, "invalid user ID"},
		{"ErrUserWithUsernameExists", ErrUserWithUsernameExists, "user with this username already exists"},
		{"ErrUserWithMailAddressExists", ErrUserWithMailAddressExists, "user with this mail address already exists"},
		{"ErrUserIDFromToken", ErrUserIDFromToken, "error getting user id from JWT token"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertErrorDefined(t, tc.name, tc.err, tc.want)
		})
	}
}

func TestDatabaseErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"ErrDatabaseInvalidEngine", ErrDatabaseInvalidEngine, "no valid database engine selected"},
		{"ErrLoggerContextNotFound", ErrLoggerContextNotFound, "failed to get logger from context"},
		{"ErrDatabaseContextNotFound", ErrDatabaseContextNotFound, "failed to get database from context"},
		{"ErrDatabaseInvalidSearchParameter", ErrDatabaseInvalidSearchParameter, "invalid search parameter on database call"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertErrorDefined(t, tc.name, tc.err, tc.want)
		})
	}
}

func TestMariaDBErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"ErrDatabaseMariaDBEmptyHost", ErrDatabaseMariaDBEmptyHost, "empty MariaDB host specified"},
		{"ErrDatabaseMariaDBEmptyUser", ErrDatabaseMariaDBEmptyUser, "empty MariaDB user specified"},
		{"ErrDatabaseMariaDBEmptyPassword", ErrDatabaseMariaDBEmptyPassword, "empty MariaDB password specified"},
		{"ErrDatabaseMariaDBEmptyName", ErrDatabaseMariaDBEmptyName, "empty MariaDB database name specified"},
		{"ErrDatabaseMariaDBInvalidPort", ErrDatabaseMariaDBInvalidPort, "no valid MariaDB database port specified"},
		{"ErrDatabaseSQLiteInvalidPath", ErrDatabaseSQLiteInvalidPath, "no valid SQLite database path specified"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertErrorDefined(t, tc.name, tc.err, tc.want)
		})
	}
}

func TestOpenFoodFactsErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"ErrOpenFoodFactsAPIEmptyURL", ErrOpenFoodFactsAPIEmptyURL, "empty API URL for OpenFoodFacts specified"},
		{"ErrOpenFoodFactsAPIInvalidTimeout", ErrOpenFoodFactsAPIInvalidTimeout, "invalid timeout for OpenFoodFacts API specified"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertErrorDefined(t, tc.name, tc.err, tc.want)
		})
	}
}

func TestFormatTemplates(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"FormatGenericError", FormatGenericError, "%s: %s"},
		{"FormatInvalidRequestId", FormatInvalidRequestId, "Requested ID '%s' is invalid"},
		{"FormatProductWithIDNotFound", FormatProductWithIDNotFound, "Product with id '%d' was not found"},
		{"FormatProductForUserNotFound", FormatProductForUserNotFound, "Product with ID '%d' for user was not found"},
		{"FormatProductNotFound", FormatProductNotFound, "Product with ID '%d' was not found in database"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got == "" {
				t.Errorf("%s is empty", tc.name)
				return
			}
			if tc.got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}
