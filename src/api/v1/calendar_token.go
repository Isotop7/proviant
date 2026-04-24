package v1

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

const CalendarTokenLength = 32

func generateCalendarToken() (string, error) {
	bytes := make([]byte, CalendarTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

type CalendarTokenResponse struct {
	Token string `json:"token"`
	URL   string `json:"url"`
}

// CreateCalendarToken creates a new calendar token for CalDAV/iCal subscription
// @Summary      Create calendar token
// @Description  Creates or regenerates a personal calendar token for iCal/CalDAV subscription. Old token is invalidated.
// @Tags         calendar
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      201  {object}  CalendarTokenResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      401  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/calendar/token [post]
func CreateCalendarToken(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet("logger").(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	rawToken, genErr := generateCalendarToken()
	if genErr != nil {
		logger.Error().Msgf("generateCalendarToken: %s", genErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to generate calendar token"})
		return
	}

	calendarTokenRepo := database.NewCalendarTokenRepository(dbHandle)
	delErr := calendarTokenRepo.DeleteByUserID(userID)
	if delErr != nil {
		logger.Error().Msgf("DeleteByUserID: %s", delErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to regenerate calendar token"})
		return
	}

	calendarToken := &authentication.CalendarToken{
		UserID: userID,
		Token:  rawToken,
	}
	createErr := calendarTokenRepo.Create(calendarToken)
	if createErr != nil {
		logger.Error().Msgf("Create: %s", createErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to create calendar token"})
		return
	}

	proviantConfig, _ := ctx.MustGet("proviantConfig").(*configuration.ProviantConfiguration)
	baseURL := ""
	if proviantConfig != nil {
		baseURL = proviantConfig.Server.BaseURL
	}
	if baseURL == "" {
		baseURL = "/"
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	resp := CalendarTokenResponse{
		Token: rawToken,
		URL:   baseURL + "api/v1/calendar/export.ics?token=" + rawToken,
	}

	ctx.JSON(http.StatusCreated, resp)
}

// DeleteCalendarToken removes the user's calendar token
// @Summary      Delete calendar token
// @Description  Removes the personal calendar token, invalidating any active iCal/CalDAV subscriptions.
// @Tags         calendar
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      401  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/calendar/token [delete]
func DeleteCalendarToken(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet("logger").(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	calendarTokenRepo := database.NewCalendarTokenRepository(dbHandle)
	delErr := calendarTokenRepo.DeleteByUserID(userID)
	if delErr != nil {
		logger.Error().Msgf("DeleteByUserID: %s", delErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to delete calendar token"})
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Calendar token deleted"})
}

// GetCalendarTokenStatus returns the user's calendar token status and subscription URL
// @Summary      Get calendar token status
// @Description  Returns whether the user has a calendar token and the subscription URL for iCal/CalDAV.
// @Tags         calendar
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  api.APIResponse
// @Failure      401  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/calendar/token [get]
func GetCalendarTokenStatus(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet("logger").(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	calendarTokenRepo := database.NewCalendarTokenRepository(dbHandle)
	calendarToken, err := calendarTokenRepo.GetByUserID(userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusOK, gin.H{"hasToken": false})
			return
		}
		logger.Error().Msgf("GetByUserID: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to check calendar token status"})
		return
	}

	proviantConfig, _ := ctx.MustGet("proviantConfig").(*configuration.ProviantConfiguration)
	baseURL := ""
	if proviantConfig != nil {
		baseURL = proviantConfig.Server.BaseURL
	}
	if baseURL == "" {
		baseURL = "/"
	}

	ctx.JSON(http.StatusOK, gin.H{
		"hasToken": true,
		"url":      baseURL + "api/v1/calendar/export.ics?token=" + calendarToken.Token,
	})
}
