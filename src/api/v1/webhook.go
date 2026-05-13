package v1

import (
	"encoding/json"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/gin-gonic/gin"
)

// CreateWebhook creates a new webhook
// @Summary      Create a webhook
// @Description  Creates a new webhook for the authenticated user
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        request body apiModel.CreateWebhookRequest true "Webhook"
// @Success      201 {object} apiModel.WebhookResponse
// @Failure      400 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/webhooks [post]
// @Security     BearerAuth
func CreateWebhook(ctx *gin.Context, appCtx *AppContext) {
	var req apiModel.CreateWebhookRequest
	if ctx.ShouldBindJSON(&req) != nil {
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return
	}

	if err := validateWebhookURL(req.URL); err != nil {
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	for _, event := range req.Events {
		if !isValidWebhookEvent(event) {
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrWebhookInvalidEvent)
			return
		}
	}

	eventsJSON, _ := json.Marshal(req.Events)
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	webhook := dbModel.Webhook{
		UserID: appCtx.UserID,
		URL:    req.URL,
		Secret: req.Secret,
		Events: string(eventsJSON),
		Active: active,
	}

	if createErr := appCtx.Repos.Webhooks.CreateWebhook(&webhook); createErr != nil {
		appCtx.Logger.Error().Msgf("Error creating webhook: %v", createErr)
		ctx.JSON(http.StatusInternalServerError, api.CreateFailedError())
		return
	}

	ctx.JSON(http.StatusCreated, toWebhookResponse(&webhook))
}

func ListWebhooks(ctx *gin.Context, appCtx *AppContext) {
	webhooks, err := appCtx.Repos.Webhooks.GetWebhooksByUserID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error listing webhooks: %v", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	response := make([]apiModel.WebhookResponse, len(webhooks))
	for i := range webhooks {
		response[i] = toWebhookResponse(&webhooks[i])
	}

	ctx.JSON(http.StatusOK, apiModel.WebhookListResponse{Webhooks: response})
}

func GetWebhook(ctx *gin.Context, appCtx *AppContext) {
	webhookID, ok := mustGetOwnedWebhookID(ctx, appCtx.Repos, appCtx.Logger, appCtx.UserID)
	if !ok {
		return
	}

	webhook, err := appCtx.Repos.Webhooks.GetWebhookByID(webhookID)
	if err != nil {
		api.RespondError(ctx, http.StatusNotFound, errors.ErrWebhookNotFound)
		return
	}

	ctx.JSON(http.StatusOK, toWebhookResponse(&webhook))
}

func UpdateWebhook(ctx *gin.Context, appCtx *AppContext) {
	webhookID, ok := mustGetOwnedWebhookID(ctx, appCtx.Repos, appCtx.Logger, appCtx.UserID)
	if !ok {
		return
	}

	webhook, err := appCtx.Repos.Webhooks.GetWebhookByID(webhookID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, api.Error(errors.ErrWebhookNotFound))
		return
	}

	var req apiModel.UpdateWebhookRequest
	if ctx.ShouldBindJSON(&req) != nil {
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return
	}

	if req.URL != "" {
		if err := validateWebhookURL(req.URL); err != nil {
			api.RespondError(ctx, http.StatusBadRequest, err)
			return
		}
		webhook.URL = req.URL
	}
	if req.Secret != "" {
		webhook.Secret = req.Secret
	}
	if err := validateWebhookEvents(req.Events); err != nil {
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}
	if req.Events != nil {
		eventsJSON, _ := json.Marshal(req.Events)
		webhook.Events = string(eventsJSON)
	}
	if req.Active != nil {
		webhook.Active = *req.Active
	}

	if err := appCtx.Repos.Webhooks.UpdateWebhook(&webhook); err != nil {
		appCtx.Logger.Error().Msgf("Error updating webhook: %v", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, toWebhookResponse(&webhook))
}

func DeleteWebhook(ctx *gin.Context, appCtx *AppContext) {
	webhookID, ok := mustGetOwnedWebhookID(ctx, appCtx.Repos, appCtx.Logger, appCtx.UserID)
	if !ok {
		return
	}

	if err := appCtx.Repos.Webhooks.DeleteWebhook(webhookID); err != nil {
		appCtx.Logger.Error().Msgf("Error deleting webhook: %v", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Webhook deleted"})
}

func GetWebhookDeliveries(ctx *gin.Context, appCtx *AppContext) {
	webhookID, ok := mustGetOwnedWebhookID(ctx, appCtx.Repos, appCtx.Logger, appCtx.UserID)
	if !ok {
		return
	}

	logs, err := appCtx.Repos.Webhooks.GetDeliveryLogs(webhookID, 50)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting delivery logs: %v", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	response := make([]apiModel.DeliveryLogResponse, len(logs))
	for i := range logs {
		response[i] = apiModel.DeliveryLogResponse{
			ID:           logs[i].ID,
			StatusCode:   logs[i].StatusCode,
			ResponseBody: logs[i].ResponseBody,
			Error:        logs[i].Error,
			Attempt:      logs[i].Attempt,
			CreatedAt:    logs[i].CreatedAt.Format(time.RFC3339),
		}
	}

	ctx.JSON(http.StatusOK, apiModel.DeliveryLogListResponse{Deliveries: response})
}

func toWebhookResponse(webhook *dbModel.Webhook) apiModel.WebhookResponse {
	events := controllers.ParseWebhookEvents(webhook.Events)
	return apiModel.WebhookResponse{
		ID:        webhook.ID,
		URL:       webhook.URL,
		Events:    events,
		Active:    webhook.Active,
		CreatedAt: webhook.CreatedAt.Format(time.RFC3339),
		UpdatedAt: webhook.UpdatedAt.Format(time.RFC3339),
	}
}

func validateWebhookEvents(events []string) error {
	for _, event := range events {
		if !isValidWebhookEvent(event) {
			return errors.ErrWebhookInvalidEvent
		}
	}
	return nil
}

func isValidWebhookEvent(event string) bool {
	for _, e := range apiModel.ValidWebhookEvents {
		if e == event {
			return true
		}
	}
	return false
}
