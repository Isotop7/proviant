package auth

import (
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func VerifyEmail(ctx *gin.Context) {
	loggerValue, loggerOk := ctx.Get("logger")
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}
	logger := loggerValue.(*zerolog.Logger)

	dbHandle, ok := ctx.Get("dbHandle")
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	dbController := database.DatabaseController{DBHandle: dbHandle.(*gorm.DB)}

	token := ctx.Query("token")
	if token == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "MISSING_TOKEN", "message": "Token is required"})
		return
	}

	verification, err := dbController.GetEmailVerificationByToken(token)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_TOKEN", "message": "Invalid verification token"})
			return
		}
		logger.Error().Msgf("Error looking up email verification token: %s", err.Error())
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Internal error"})
		return
	}

	if verification.Status == dbModel.EmailVerificationStatusVerified {
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "ALREADY_VERIFIED", "message": "Email address already verified"})
		return
	}

	if verification.Status == dbModel.EmailVerificationStatusExpired || time.Now().After(verification.ExpiresAt) {
		_ = dbController.UpdateEmailVerificationStatus(token, dbModel.EmailVerificationStatusExpired)
		ctx.JSON(http.StatusBadRequest, gin.H{"code": "TOKEN_EXPIRED", "message": "Verification token has expired"})
		return
	}

	now := time.Now()
	if err := dbController.UpdateUserEmailVerified(verification.UserID, now); err != nil {
		logger.Error().Msgf("Error updating user email verified status: %s", err.Error())
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Internal error"})
		return
	}

	if err := dbController.UpdateEmailVerificationStatus(token, dbModel.EmailVerificationStatusVerified); err != nil {
		logger.Error().Msgf("Error updating email verification status: %s", err.Error())
	}

	logger.Info().Msgf("Email verified for user ID %d", verification.UserID)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Email address verified successfully"})
}
