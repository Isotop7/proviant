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

var (
	ResponseErrInvalidUserData         = APIResponse{Message: "Invalid user data"}
	ResponseErrDatabaseContextNotFound = APIResponse{Message: "Failed to get database from context"}
	ResponseErrUserIDFromToken         = APIResponse{Message: "Error getting user id from JWT token"}
)

// APIResponse is the data model for a generic API response
type APIResponse struct {
	Message string `json:"message"`
}
