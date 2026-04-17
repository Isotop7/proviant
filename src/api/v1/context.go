package v1

import (
	"fmt"
	"net/http"
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration/static"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func mustGetDB(ctx *gin.Context, logger *zerolog.Logger) (*gorm.DB, bool) {
	db, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
	}
	return db, ok
}

func mustGetUserID(ctx *gin.Context, logger *zerolog.Logger) (uint, bool) {
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return 0, false
	}
	return userID, true
}

func parseIntParam(ctx *gin.Context, logger *zerolog.Logger, paramName string) (int, bool) {
	raw := ctx.Param(paramName)
	id, err := strconv.Atoi(raw)
	if err != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, raw)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: fmt.Sprintf(errors.FormatInvalidRequestId, raw)})
		return 0, false
	}
	return id, true
}
