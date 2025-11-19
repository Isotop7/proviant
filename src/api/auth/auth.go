// auth contains authentication method handlers
package auth

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models/authentication"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// Signup creates a new user object in the database
// @Summary      	Creates a new user
// @Description  	Creates a new new user in the database
// @Tags         	user
// @Accept			json
// @Produce      	json
// @Param			signup	body	authentication.Signup	true	"Signup"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/auth/signup [post]
func Signup(ctx *gin.Context) {
	// Get logger instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get database instance from context
	dbHandle, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Create database controller object
	dbController := database.DatabaseController{DBHandle: dbHandle}

	// Parse request body to Login
	var signup authentication.Signup
	if err := ctx.ShouldBindJSON(&signup); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	// Check if signup object is valid
	validationErr := signup.IsValid()
	if validationErr != nil {
		logger.Error().Msgf("User data was invalid: '%s'", validationErr.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(validationErr))
		return
	}

	// Create new user object
	user := authentication.User{
		ID:          dbController.GetNextUserID(),
		Username:    signup.Username,
		Password:    signup.Password,
		MailAddress: signup.MailAddress,
	}

	// Check if user with username already exists
	if dbController.UserExistsByUsername(&user) {
		logger.Error().Msgf("User '%s' already exists", user.Username)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserWithUsernameExists)
		return
	}

	// Check if user with mail address already exists
	if dbController.UserExistsByMailAddress(&user) {
		logger.Error().Msgf("User with mail address '%s' already exists", user.MailAddress)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserWithMailAddressExists)
		return
	}

	// Create user object in database
	createError := dbController.CreateUser(&user)
	if createError != nil {
		logger.Error().Msgf("User '%s' with ID '%d' could not be created. Error: %s", user.Username, user.ID, createError.Error())
		ctx.JSON(http.StatusBadRequest, api.ResponseErrInvalidUserData)
		return
	} else {
		logger.Info().Msgf("New User '%s' with ID '%d' created", user.Username, user.ID)
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "User was created"})
		return
	}
}
