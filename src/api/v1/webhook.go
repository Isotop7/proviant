package v1

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
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
func CreateWebhook(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	var req apiModel.CreateWebhookRequest
	if bindErr := ctx.ShouldBindJSON(&req); bindErr != nil {
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return
	}

	for _, event := range req.Events {
		if !isValidWebhookEvent(event) {
			ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrWebhookInvalidEvent))
			return
		}
	}

	eventsJSON, _ := json.Marshal(req.Events)
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	webhook := dbModel.Webhook{
		UserID: userID,
		URL:    req.URL,
		Secret: req.Secret,
		Events: string(eventsJSON),
		Active: active,
	}

	if createErr := repos.Webhooks.CreateWebhook(&webhook); createErr != nil {
		logger.Error().Msgf("Error creating webhook: %v", createErr)
		ctx.JSON(http.StatusInternalServerError, api.CreateFailedError())
		return
	}

	ctx.JSON(http.StatusCreated, toWebhookResponse(&webhook))
}

// ListWebhooks returns all webhooks for the authenticated user
// @Summary      List webhooks
// @Description  Returns all webhooks for the authenticated user
// @Tags         webhook
// @Produce      json
// @Success      200 {object} apiModel.WebhookListResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/webhooks [get]
// @Security     BearerAuth
func ListWebhooks(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	webhooks, err := repos.Webhooks.GetWebhooksByUserID(userID)
	if err != nil {
		logger.Error().Msgf("Error listing webhooks: %v", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	response := make([]apiModel.WebhookResponse, len(webhooks))
	for i := range webhooks {
		response[i] = toWebhookResponse(&webhooks[i])
	}

	ctx.JSON(http.StatusOK, apiModel.WebhookListResponse{Webhooks: response})
}

// GetWebhook returns a webhook by ID
// @Summary      Get a webhook
// @Description  Returns a webhook by ID
// @Tags         webhook
// @Produce      json
// @Param        id path int true "Webhook ID"
// @Success      200 {object} apiModel.WebhookResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/webhooks/{id} [get]
// @Security     BearerAuth
func GetWebhook(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	webhookID, parseErr := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if parseErr != nil {
		ctx.JSON(http.StatusBadRequest, api.InvalidInputErrorWithDetail("webhook ID must be a valid unsigned integer"))
		return
	}

	if err := repos.Webhooks.CheckOwnership(uint(webhookID), userID); err != nil {
		if err == errors.ErrWebhookNotFound || err == errors.ErrWebhookNotOwner {
			ctx.JSON(http.StatusNotFound, api.Error(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	webhook, err := repos.Webhooks.GetWebhookByID(uint(webhookID))
	if err != nil {
		ctx.JSON(http.StatusNotFound, api.Error(errors.ErrWebhookNotFound))
		return
	}

	ctx.JSON(http.StatusOK, toWebhookResponse(&webhook))
}

// UpdateWebhook updates a webhook
// @Summary      Update a webhook
// @Description  Updates a webhook by ID
// @Tags         webhook
// @Accept       json
// @Produce      json
// @Param        id path int true "Webhook ID"
// @Param        request body apiModel.UpdateWebhookRequest true "Webhook update"
// @Success      200 {object} apiModel.WebhookResponse
// @Failure      400 {object} api.APIResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/webhooks/{id} [patch]
// @Security     BearerAuth
func UpdateWebhook(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	webhookID, parseErr := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if parseErr != nil {
		ctx.JSON(http.StatusBadRequest, api.InvalidInputErrorWithDetail("webhook ID must be a valid unsigned integer"))
		return
	}

	if err := repos.Webhooks.CheckOwnership(uint(webhookID), userID); err != nil {
		if err == errors.ErrWebhookNotFound || err == errors.ErrWebhookNotOwner {
			ctx.JSON(http.StatusNotFound, api.Error(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	webhook, err := repos.Webhooks.GetWebhookByID(uint(webhookID))
	if err != nil {
		ctx.JSON(http.StatusNotFound, api.Error(errors.ErrWebhookNotFound))
		return
	}

	var req apiModel.UpdateWebhookRequest
	if bindErr := ctx.ShouldBindJSON(&req); bindErr != nil {
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return
	}

	if req.URL != "" {
		webhook.URL = req.URL
	}
	if req.Secret != "" {
		webhook.Secret = req.Secret
	}
	if req.Events != nil {
		for _, event := range req.Events {
			if !isValidWebhookEvent(event) {
				ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrWebhookInvalidEvent))
				return
			}
		}
		eventsJSON, _ := json.Marshal(req.Events)
		webhook.Events = string(eventsJSON)
	}
	if req.Active != nil {
		webhook.Active = *req.Active
	}

	if err := repos.Webhooks.UpdateWebhook(&webhook); err != nil {
		logger.Error().Msgf("Error updating webhook: %v", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, toWebhookResponse(&webhook))
}

// DeleteWebhook deletes a webhook
// @Summary      Delete a webhook
// @Description  Deletes a webhook by ID
// @Tags         webhook
// @Produce      json
// @Param        id path int true "Webhook ID"
// @Success      200 {object} api.APIResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/webhooks/{id} [delete]
// @Security     BearerAuth
func DeleteWebhook(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	webhookID, parseErr := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if parseErr != nil {
		ctx.JSON(http.StatusBadRequest, api.InvalidInputErrorWithDetail("webhook ID must be a valid unsigned integer"))
		return
	}

	if err := repos.Webhooks.CheckOwnership(uint(webhookID), userID); err != nil {
		if err == errors.ErrWebhookNotFound || err == errors.ErrWebhookNotOwner {
			ctx.JSON(http.StatusNotFound, api.Error(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if err := repos.Webhooks.DeleteWebhook(uint(webhookID)); err != nil {
		logger.Error().Msgf("Error deleting webhook: %v", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Webhook deleted"})
}

// GetWebhookDeliveries returns delivery logs for a webhook
// @Summary      Get webhook delivery logs
// @Description  Returns delivery logs for a webhook
// @Tags         webhook
// @Produce      json
// @Param        id path int true "Webhook ID"
// @Success      200 {object} apiModel.DeliveryLogListResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/webhooks/{id}/deliveries [get]
// @Security     BearerAuth
func GetWebhookDeliveries(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	webhookID, parseErr := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if parseErr != nil {
		ctx.JSON(http.StatusBadRequest, api.InvalidInputErrorWithDetail("webhook ID must be a valid unsigned integer"))
		return
	}

	if err := repos.Webhooks.CheckOwnership(uint(webhookID), userID); err != nil {
		if err == errors.ErrWebhookNotFound || err == errors.ErrWebhookNotOwner {
			ctx.JSON(http.StatusNotFound, api.Error(err))
			return
		}
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	logs, err := repos.Webhooks.GetDeliveryLogs(uint(webhookID), 50)
	if err != nil {
		logger.Error().Msgf("Error getting delivery logs: %v", err)
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

func isValidWebhookEvent(event string) bool {
	for _, e := range apiModel.ValidWebhookEvents {
		if e == event {
			return true
		}
	}
	return false
}
