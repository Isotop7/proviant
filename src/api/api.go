// api contains the complete API definitions
// @title           expiro
// @version         0.2.0
// @description     expiro is a simple and intuitive application to track your bought products and their expiration date to prevent waste of food.

// @contact.name   Isotop7
// @contact.url    https://gitlab.com/Isotop7/expiro
// @contact.email  hendrik@hr94.de

// @license.name  MIT
// @license.url   https://gitlab.com/Isotop7/expiro/-/blob/main/LICENSE

// @host      localhost:5050
// @BasePath  /

// @securityDefinitions.basic  JSON Web Token

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
package api

import "gitlab.com/Isotop7/expiro/errors"

var (
	ResponseErrInvalidUserData           = APIResponse{Message: errors.ErrInvalidUserData.Error()}
	ResponseErrUserWithUsernameExists    = APIResponse{Message: errors.ErrUserWithUsernameExists.Error()}
	ResponseErrUserWithMailAddressExists = APIResponse{Message: errors.ErrUserWithMailAddressExists.Error()}
	ResponseErrDatabaseContextNotFound   = APIResponse{Message: errors.ErrDatabaseContextNotFound.Error()}
	ResponseErrUserIDFromToken           = APIResponse{Message: errors.ErrUserIDFromToken.Error()}
	ResponseErrUserNoProductsFound       = APIResponse{Message: errors.ErrUserNoProductsFound.Error()}
)

// APIResponse is the data model for a generic API response
type APIResponse struct {
	Message string `json:"message"`
}

// Error returns an API response object from a error object
func Error(err error) APIResponse {
	return APIResponse{Message: err.Error()}
}
