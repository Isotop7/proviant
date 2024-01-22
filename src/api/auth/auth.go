// auth contains authentication method handlers
package auth

import (
	"net/http"

	"gitlab.com/Isotop7/expiro/api"
	"gitlab.com/Isotop7/expiro/controllers"
	"gitlab.com/Isotop7/expiro/models/authentication"

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
	dbController := controllers.DatabaseController{DBHandle: dbHandle}

	// Parse request body to Login
	var signup authentication.Signup
	if err := ctx.ShouldBindJSON(&signup); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	// Check if signup object is valid
	validationErr := signup.IsValid()
	if validationErr != nil {
		logger.Error().Msgf("User data was invalid: '%s'", validationErr.Error())
		ctx.JSON(http.StatusBadRequest, gin.H{"Message": validationErr.Error()})
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
	if dbController.UserExists(user) {
		logger.Error().Msgf("User '%s' or mail address '%s' already exists", user.Username, user.MailAddress)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrInvalidUserData)
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
