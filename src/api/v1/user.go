package v1

import (
	"fmt"
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// UpdateUser updates a user
// @Summary			Updates a user object
// @Description		Updates properties of a user
// @Tags         	user
// @Accept			json
// @Produce      	json
// @Param			user	body	authentication.User	true	"User"
// @Success      	200  {object}  authentication.User
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/user [patch]
func UpdateUser(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	// Get and parse body to user
	var user authentication.User
	if bindErr := ctx.ShouldBindJSON(&user); bindErr != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), bindErr.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(bindErr))
		return
	}

	// Check for valid user data
	validationErr := user.IsValid(true)
	if validationErr != nil {
		ctx.JSON(http.StatusBadRequest, api.Error(validationErr))
		return
	}

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}

	// Force set user id from token
	user.ID = userID

	// Update user in database
	updateErr := dbController.UpdateUser(user.ID, &user)

	switch updateErr {
	// No error => user was updated
	case nil:
		ctx.JSON(http.StatusOK, user)
		return
	// Requested user was not found
	case gorm.ErrRecordNotFound:
		logger.Error().Msgf("User with ID '%d' was not found in database", userID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("User with id '%d' was not found", userID)})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Error saving user: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(updateErr))
		return
	}
}

// UpdateUserPassword updates a user password
// @Summary			Updates a user password
// @Description		Updates password of a user
// @Tags         	user
// @Accept			json
// @Produce      	json
// @Param			login	body	authentication.Login	true	"Login"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/user/password [post]
func UpdateUserPassword(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	// Get and parse body to user
	var login authentication.Login
	if bindErr := ctx.ShouldBindJSON(&login); bindErr != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), bindErr.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(bindErr))
		return
	}

	// Check for valid login credentials
	validationErr := login.IsValid()
	if validationErr != nil {
		logger.Error().Msg(validationErr.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(validationErr))
		return
	}

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}

	// Update user in database
	updateErr := dbController.UpdateUserPassword(userID, &login)

	switch updateErr {
	// No error => password was updated
	case nil:
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Password updated"})
		return
	// Requested user was not found
	case gorm.ErrRecordNotFound:
		logger.Error().Msgf("User with ID '%d' was not found in database", userID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf("User with id '%d' was not found", userID)})
		return
	// Unspecified error
	default:
		logger.Error().Msgf("Error updating password: %s", updateErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(updateErr))
		return
	}
}
