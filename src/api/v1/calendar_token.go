package v1

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
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
func CreateCalendarToken(ctx *gin.Context, appCtx *AppContext) {
	rawToken, genErr := generateCalendarToken()
	if genErr != nil {
		appCtx.Logger.Error().Msgf("generateCalendarToken: %s", genErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to generate calendar token"})
		return
	}

	delErr := appCtx.Repos.CalendarTokens.DeleteByUserID(appCtx.UserID)
	if delErr != nil {
		appCtx.Logger.Error().Msgf("DeleteByUserID: %s", delErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to regenerate calendar token"})
		return
	}

	calendarToken := &authentication.CalendarToken{
		UserID: appCtx.UserID,
		Token:  rawToken,
	}
	createErr := appCtx.Repos.CalendarTokens.Create(calendarToken)
	if createErr != nil {
		appCtx.Logger.Error().Msgf("Create: %s", createErr)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to create calendar token"})
		return
	}

	proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
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
func DeleteCalendarToken(ctx *gin.Context, appCtx *AppContext) {
	delErr := appCtx.Repos.CalendarTokens.DeleteByUserID(appCtx.UserID)
	if delErr != nil {
		appCtx.Logger.Error().Msgf("DeleteByUserID: %s", delErr)
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
func GetCalendarTokenStatus(ctx *gin.Context, appCtx *AppContext) {
	calendarToken, err := appCtx.Repos.CalendarTokens.GetByUserID(appCtx.UserID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusOK, gin.H{"hasToken": false})
			return
		}
		appCtx.Logger.Error().Msgf("GetByUserID: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to check calendar token status"})
		return
	}

	proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
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

	ctx.JSON(http.StatusOK, gin.H{
		"hasToken": true,
		"url":      baseURL + "api/v1/calendar/export.ics?token=" + calendarToken.Token,
	})
}
