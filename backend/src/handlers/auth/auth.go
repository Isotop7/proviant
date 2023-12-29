package auth

import (
	"net/http"

	"gitlab.com/Isotop7/expiro/controllers"
	"gitlab.com/Isotop7/expiro/models/authentication"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func Signup(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	dbController := controllers.DatabaseController{DB: db}

	var login authentication.Login
	if err := c.ShouldBindJSON(&login); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	user := authentication.User{
		ID:       dbController.GetNextUserID(),
		Username: login.Username,
		Password: login.Password,
	}

	if !user.IsValid() {
		logger.Error().Msgf("User data was invalid: ID = '%d'; Username = '%s'", user.ID, user.Username)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user data"})
		return
	}

	if dbController.UserExists(user) {
		logger.Error().Msgf("User '%s' already exists", user.Username)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user data"})
		return
	}

	createError := dbController.CreateUser(&user)
	if createError != nil {
		logger.Error().Msgf("User '%s' with ID '%d' could not be created. Error: %s", user.Username, user.ID, createError.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user data"})
	} else {
		logger.Info().Msgf("New User '%s' with ID '%d' created", user.Username, user.ID)
		c.JSON(http.StatusOK, gin.H{"message": "User was created"})
	}
}
