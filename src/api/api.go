// api contains the complete API definitions
// @title           proviant
// @version         0.2.0
// @description     proviant is a simple and intuitive application to track your bought products and their expiration date to prevent waste of food.

// @contact.name   Isotop7
// @contact.url    https://github.com/Isotop7/proviant
// @contact.email  hendrik@hr94.de

// @license.name  MIT
// @license.url   https://github.com/Isotop7/proviant/blob/main/LICENSE

// @host      localhost:5050
// @BasePath  /

// @securityDefinitions.basic  JSON Web Token

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
package api

import (
	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"

	"github.com/gin-gonic/gin"
)

var (
	ResponseErrInvalidUserData           = APIResponse{Message: errors.ErrInvalidUserData.Error()}
	ResponseErrUserWithUsernameExists    = APIResponse{Message: errors.ErrUserWithUsernameExists.Error()}
	ResponseErrUserWithMailAddressExists = APIResponse{Message: errors.ErrUserWithMailAddressExists.Error()}
	ResponseErrDatabaseContextNotFound   = APIResponse{Message: errors.ErrDatabaseContextNotFound.Error()}
	ResponseErrLoggerContextNotFound     = APIResponse{Message: errors.ErrLoggerContextNotFound.Error()}
	ResponseErrUserIDFromToken           = APIResponse{Message: errors.ErrUserIDFromToken.Error()}
	ResponseErrUserNoProductsFound       = APIResponse{Message: errors.ErrUserNoProductsFound.Error()}
)

const ActionTryAgain = "Please try again later"

// APIResponse is the data model for a generic API response
type APIResponse struct {
	Message string `json:"message"`
	Action  string `json:"action,omitempty"`
}

// BulkActionResponse is the response of a bulk action (restore, consume,
// waste). FailedIDs carries the requested ids that did not land, so a partial
// failure — which is an HTTP 200 — stays machine-readable for the client
// instead of hiding in the message string.
type BulkActionResponse struct {
	APIResponse
	FailedIDs []uint `json:"failedIds,omitempty"`
}

// ProductActionResponse is the response of a consume or waste action. Removed
// counts the packs that actually left the stock, which is fewer than the
// caller asked for when the stock shrank in between.
type ProductActionResponse struct {
	APIResponse
	Removed int `json:"removed"`
}

// ProductConflictResponse is the body of a 409 from a product create.
//
// The duplicate is set when the refusal is a duplicate row: the client offers
// the user to add the pack to that row, to create a separate product, or to
// cancel. It is absent when the refusal is a clientSyncId another member's
// private row already holds — there the caller learns nothing about the row.
type ProductConflictResponse struct {
	APIResponse
	Duplicate *apiModel.ProductResponse `json:"duplicate,omitempty"`
}

// Error returns an API response object from a error object
func Error(err error) APIResponse {
	return APIResponse{Message: err.Error()}
}

func InternalError() APIResponse {
	return APIResponse{
		Message: "An error occurred. Please try again or contact support if the problem persists.",
		Action:  ActionTryAgain,
	}
}

func InvalidInputError() APIResponse {
	return APIResponse{
		Message: "The submitted data is invalid. Please check your input and try again.",
		Action:  "Please verify your input and try again",
	}
}

func InvalidInputErrorWithDetail(detail string) APIResponse {
	return APIResponse{
		Message: "The submitted data is invalid: " + detail + ". Please check your input and try again.",
		Action:  "Please verify your input and try again",
	}
}

func CreateFailedError() APIResponse {
	return APIResponse{
		Message: "Failed to save your data. Please try again.",
		Action:  ActionTryAgain,
	}
}

func UpdateFailedError() APIResponse {
	return APIResponse{
		Message: "Failed to update. Please try again.",
		Action:  ActionTryAgain,
	}
}

func DeleteFailedError() APIResponse {
	return APIResponse{
		Message: "Failed to delete. Please try again.",
		Action:  ActionTryAgain,
	}
}

func RestoreFailedError() APIResponse {
	return APIResponse{
		Message: "Failed to restore. Please try again.",
		Action:  ActionTryAgain,
	}
}

func RespondError(ctx *gin.Context, status int, err error) {
	ctx.JSON(status, APIResponse{Message: err.Error()})
}
