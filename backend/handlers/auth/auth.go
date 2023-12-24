package auth

import (
	"expiro/backend/controllers"
	"expiro/backend/models/auth"
	"net/http"
	"time"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func AuthorizationMiddleware(db *gorm.DB) (*jwt.GinJWTMiddleware, error) {
	return jwt.New(&jwt.GinJWTMiddleware{
		Realm:       "expiro",
		Key:         []byte("secret key"),
		Timeout:     time.Hour,
		MaxRefresh:  time.Hour,
		IdentityKey: "id",
		PayloadFunc: func(data interface{}) jwt.MapClaims {
			if v, ok := data.(auth.User); ok {
				return jwt.MapClaims{
					"id":       v.ID,
					"username": v.Username,
				}
			}
			return jwt.MapClaims{}
		},
		IdentityHandler: func(c *gin.Context) interface{} {
			claims := jwt.ExtractClaims(c)
			return &auth.User{
				ID:       uint(claims["id"].(float64)),
				Username: claims["username"].(string),
			}
		},
		Authenticator: func(c *gin.Context) (interface{}, error) {
			var loginVals auth.Login
			if err := c.ShouldBind(&loginVals); err != nil {
				return "", jwt.ErrMissingLoginValues
			}

			dbController := controllers.DatabaseController{DB: db}
			user, err := dbController.FindUserByUsername(loginVals.Username)
			if err != nil {
				return nil, jwt.ErrFailedAuthentication
			}

			if dbController.VerifyPassword(&user, loginVals.Password) {
				return user, nil
			}

			return nil, jwt.ErrFailedAuthentication
		},
		Authorizator: func(data interface{}, c *gin.Context) bool {
			/*if v, ok := data.(*auth.User); ok && v.Username == "admin" {
				return true
			}

			return false*/

			// TODO: Implement RBAC based on user property
			return true
		},
		Unauthorized: func(c *gin.Context, code int, message string) {
			c.JSON(code, gin.H{
				"code":    code,
				"message": message,
			})
		},

		TokenLookup:   "header: Authorization, query: token, cookie: jwt",
		TokenHeadName: "Bearer",
		TimeFunc:      time.Now,
	})
}

func Signup(c *gin.Context) {
	logger, _ := c.MustGet("logger").(*zerolog.Logger)

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		logger.Error().Msg("Failed to get database from context")
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get database from context"})
		return
	}

	dbController := controllers.DatabaseController{DB: db}

	var login auth.Login
	if err := c.ShouldBindJSON(&login); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	user := auth.User{
		ID:       dbController.GetNextUserID(),
		Username: login.Username,
		Password: login.Password,
	}

	if !user.IsValid() {
		logger.Error().Msgf("User data was invalid: ID = '%d'; Username = '%s'", user.ID, user.Username)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user data"})
		return
	}

	if dbController.Exists(user) {
		logger.Error().Msgf("User '%s' with ID '%d' already exists", user.Username, user.ID)
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user data"})
		return
	}

	createError := dbController.Create(&user)
	if createError != nil {
		logger.Error().Msgf("User '%s' with ID '%d' could not be created. Error: %s", user.Username, user.ID, createError.Error())
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid user data"})
	} else {
		logger.Info().Msgf("New User '%s' with ID '%d' created", user.Username, user.ID)
		c.JSON(http.StatusOK, gin.H{"message": "User was created"})
	}
}
