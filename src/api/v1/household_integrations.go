package v1

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	apperrors "codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/services"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

func authorizeHouseholdIntegrationAdmin(ctx *gin.Context, appCtx *AppContext) (uint, bool) {
	user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if err != nil {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrInvalidUserID)
		return 0, false
	}
	if user.HouseholdID == 0 {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrUserHasNoHousehold)
		return 0, false
	}
	household, err := appCtx.Repos.Households.GetHouseholdByID(user.HouseholdID)
	if err != nil {
		appCtx.Logger.Error().Msgf(MsgErrFetchingHousehold, err)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return 0, false
	}
	if household.AdminID != appCtx.UserID {
		api.RespondError(ctx, http.StatusForbidden, apperrors.ErrNotHouseholdAdmin)
		return 0, false
	}
	return household.ID, true
}

func isKnownShoppingListProvider(provider string) bool {
	switch provider {
	case util.ShoppingListProviderBring, util.ShoppingListProviderTodoist, util.ShoppingListProviderNoop:
		return true
	default:
		return false
	}
}

func extractShoppingListSyncConfig(ctx *gin.Context) *configuration.ShoppingListSyncConfiguration {
	cfgVal, exists := ctx.Get(util.ContextKeyProviantConfig)
	if !exists {
		return nil
	}
	cfg, ok := cfgVal.(*configuration.ProviantConfiguration)
	if !ok || cfg == nil {
		return nil
	}
	if cfg.ShoppingListSync == (configuration.ShoppingListSyncConfiguration{}) {
		return nil
	}
	return &cfg.ShoppingListSync
}

func parseAndValidateIntegrationProvider(ctx *gin.Context) (string, bool) {
	provider := strings.ToLower(strings.TrimSpace(ctx.Param("provider")))
	if !isKnownShoppingListProvider(provider) {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrIntegrationInvalidProvider)
		return "", false
	}
	syncCfg := extractShoppingListSyncConfig(ctx)
	if syncCfg != nil && !syncCfg.IsProviderEnabled(provider) {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrIntegrationProviderDisabled)
		return "", false
	}
	return provider, true
}

func shoppingListSyncServiceForRequest(ctx *gin.Context, appCtx *AppContext) *services.ShoppingListSyncService {
	if appCtx.ShoppingListSync != nil {
		return appCtx.ShoppingListSync
	}
	return services.NewShoppingListSyncService(appCtx.Repos, appCtx.Logger, extractShoppingListSyncConfig(ctx))
}

func requestContext(ctx *gin.Context) context.Context {
	if ctx != nil && ctx.Request != nil {
		return ctx.Request.Context()
	}
	return context.Background()
}

func respondIntegrationProviderError(ctx *gin.Context, integration *dbModel.HouseholdIntegration, err error) {
	if errors.Is(err, apperrors.ErrIntegrationMissingCredentials) || errors.Is(err, apperrors.ErrIntegrationMissingListID) || errors.Is(err, apperrors.ErrIntegrationProviderDisabled) {
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}
	scrubbed := controllers.ScrubIntegrationError(err, integration)
	ctx.JSON(http.StatusBadGateway, api.APIResponse{
		Message: scrubbed,
	})
}

// ListHouseholdIntegrations lists all shopping list integrations configured for the caller's household.
// @Summary      Lists household shopping list integrations
// @Description  Returns all configured shopping list sync integrations for the caller's household (admin only). Secret tokens are never returned.
// @Tags         household
// @Produce      json
// @Success      200  {array}   apiModel.HouseholdIntegrationResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/integrations [get]
// @Security     BearerAuth
func ListHouseholdIntegrations(ctx *gin.Context, appCtx *AppContext) {
	householdID, ok := authorizeHouseholdIntegrationAdmin(ctx, appCtx)
	if !ok {
		return
	}

	integrations, err := appCtx.Repos.HouseholdIntegrations.ListByHousehold(householdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error listing household integrations: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewHouseholdIntegrationResponses(integrations))
}

// CreateHouseholdIntegration creates or replaces a shopping list integration for the caller's household.
// @Summary      Creates a household shopping list integration
// @Description  Configures a shopping list sync integration for the caller's household (admin only). Secret tokens are write-only and never returned.
// @Tags         household
// @Accept       json
// @Produce      json
// @Param        provider  path      string                                      true  "Provider identifier (bring or todoist)"
// @Param        request   body      apiModel.CreateHouseholdIntegrationRequest  true  "Integration configuration"
// @Success      201       {object}  apiModel.HouseholdIntegrationResponse
// @Failure      400       {object}  api.APIResponse
// @Failure      403       {object}  api.APIResponse
// @Failure      500       {object}  api.APIResponse
// @Router       /api/v1/household/integrations/{provider} [post]
// @Security     BearerAuth
func CreateHouseholdIntegration(ctx *gin.Context, appCtx *AppContext) {
	householdID, ok := authorizeHouseholdIntegrationAdmin(ctx, appCtx)
	if !ok {
		return
	}

	provider, ok := parseAndValidateIntegrationProvider(ctx)
	if !ok {
		return
	}

	var req apiModel.CreateHouseholdIntegrationRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	accessToken := strings.TrimSpace(req.AccessToken)
	refreshToken := strings.TrimSpace(req.RefreshToken)
	if existing, err := appCtx.Repos.HouseholdIntegrations.GetByHouseholdAndProvider(householdID, provider); err == nil && existing != nil {
		if accessToken == "" && !req.ClearAccessToken {
			accessToken = existing.AccessToken
		}
		if refreshToken == "" {
			refreshToken = existing.RefreshToken
		}
	}
	if req.ClearAccessToken {
		accessToken = ""
	}

	integration := &dbModel.HouseholdIntegration{
		HouseholdID:  householdID,
		Provider:     provider,
		Enabled:      req.Enabled,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		AccountEmail: strings.TrimSpace(req.AccountEmail),
		ListID:       strings.TrimSpace(req.ListID),
		ListName:     strings.TrimSpace(req.ListName),
	}

	if err := appCtx.Repos.HouseholdIntegrations.Upsert(integration); err != nil {
		appCtx.Logger.Error().Msgf("Error saving household integration: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	ctx.JSON(http.StatusCreated, apiModel.NewHouseholdIntegrationResponse(integration))
}

// UpdateHouseholdIntegration updates an existing shopping list integration for the caller's household.
// @Summary      Updates a household shopping list integration
// @Description  Updates settings or credentials for a shopping list sync integration (admin only). Empty token fields keep stored credentials unless clear flags are set.
// @Tags         household
// @Accept       json
// @Produce      json
// @Param        provider  path      string                                      true  "Provider identifier (bring or todoist)"
// @Param        request   body      apiModel.UpdateHouseholdIntegrationRequest  true  "Integration fields to update"
// @Success      200       {object}  apiModel.HouseholdIntegrationResponse
// @Failure      400       {object}  api.APIResponse
// @Failure      403       {object}  api.APIResponse
// @Failure      404       {object}  api.APIResponse
// @Failure      500       {object}  api.APIResponse
// @Router       /api/v1/household/integrations/{provider} [patch]
// @Security     BearerAuth
func UpdateHouseholdIntegration(ctx *gin.Context, appCtx *AppContext) {
	householdID, ok := authorizeHouseholdIntegrationAdmin(ctx, appCtx)
	if !ok {
		return
	}

	provider, ok := parseAndValidateIntegrationProvider(ctx)
	if !ok {
		return
	}

	var req apiModel.UpdateHouseholdIntegrationRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	integration, err := appCtx.Repos.HouseholdIntegrations.GetByHouseholdAndProvider(householdID, provider)
	if err != nil {
		if errors.Is(err, apperrors.ErrIntegrationNotFound) {
			api.RespondError(ctx, http.StatusNotFound, apperrors.ErrIntegrationNotFound)
			return
		}
		appCtx.Logger.Error().Msgf("Error loading household integration: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	if req.Enabled != nil {
		integration.Enabled = *req.Enabled
	}
	if req.AccountEmail != nil {
		integration.AccountEmail = strings.TrimSpace(*req.AccountEmail)
	}
	if req.ListID != nil {
		integration.ListID = strings.TrimSpace(*req.ListID)
	}
	if req.ListName != nil {
		integration.ListName = strings.TrimSpace(*req.ListName)
	}
	if req.ClearAccessToken {
		integration.AccessToken = ""
	} else if token := strings.TrimSpace(req.AccessToken); token != "" {
		integration.AccessToken = token
	}
	if req.ClearRefreshToken {
		integration.RefreshToken = ""
	} else if refresh := strings.TrimSpace(req.RefreshToken); refresh != "" {
		integration.RefreshToken = refresh
	}

	if err := appCtx.Repos.HouseholdIntegrations.Update(integration); err != nil {
		appCtx.Logger.Error().Msgf("Error updating household integration: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewHouseholdIntegrationResponse(integration))
}

// DeleteHouseholdIntegration removes a shopping list integration and its synced item references.
// @Summary      Deletes a household shopping list integration
// @Description  Deletes the specified shopping list sync integration and its item mappings for the caller's household (admin only).
// @Tags         household
// @Produce      json
// @Param        provider  path      string  true  "Provider identifier (bring or todoist)"
// @Success      200       {object}  api.APIResponse
// @Failure      400       {object}  api.APIResponse
// @Failure      403       {object}  api.APIResponse
// @Failure      404       {object}  api.APIResponse
// @Failure      500       {object}  api.APIResponse
// @Router       /api/v1/household/integrations/{provider} [delete]
// @Security     BearerAuth
func DeleteHouseholdIntegration(ctx *gin.Context, appCtx *AppContext) {
	householdID, ok := authorizeHouseholdIntegrationAdmin(ctx, appCtx)
	if !ok {
		return
	}

	provider := strings.ToLower(strings.TrimSpace(ctx.Param("provider")))
	if !isKnownShoppingListProvider(provider) {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrIntegrationInvalidProvider)
		return
	}

	_ = appCtx.Repos.ShoppingListSyncRefs.DeleteByHouseholdAndProvider(householdID, provider)
	if err := appCtx.Repos.HouseholdIntegrations.Delete(householdID, provider); err != nil {
		if errors.Is(err, apperrors.ErrIntegrationNotFound) {
			api.RespondError(ctx, http.StatusNotFound, apperrors.ErrIntegrationNotFound)
			return
		}
		appCtx.Logger.Error().Msgf("Error deleting household integration: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Household integration deleted"})
}

// TestHouseholdIntegration tests the connection for a configured household shopping list integration.
// @Summary      Tests a household shopping list integration
// @Description  Verifies credentials and connectivity for the specified household shopping list integration (admin only).
// @Tags         household
// @Produce      json
// @Param        provider  path      string  true  "Provider identifier (bring or todoist)"
// @Success      200       {object}  api.APIResponse
// @Failure      400       {object}  api.APIResponse
// @Failure      403       {object}  api.APIResponse
// @Failure      404       {object}  api.APIResponse
// @Failure      502       {object}  api.APIResponse
// @Router       /api/v1/household/integrations/{provider}/test [post]
// @Security     BearerAuth
func TestHouseholdIntegration(ctx *gin.Context, appCtx *AppContext) {
	householdID, ok := authorizeHouseholdIntegrationAdmin(ctx, appCtx)
	if !ok {
		return
	}

	provider, ok := parseAndValidateIntegrationProvider(ctx)
	if !ok {
		return
	}

	integration, err := appCtx.Repos.HouseholdIntegrations.GetByHouseholdAndProvider(householdID, provider)
	if err != nil {
		if errors.Is(err, apperrors.ErrIntegrationNotFound) {
			api.RespondError(ctx, http.StatusNotFound, apperrors.ErrIntegrationNotFound)
			return
		}
		appCtx.Logger.Error().Msgf("Error loading household integration: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	svc := shoppingListSyncServiceForRequest(ctx, appCtx)
	reqCtx := requestContext(ctx)
	if testErr := svc.TestIntegration(reqCtx, integration); testErr != nil {
		integration.LastError = controllers.ScrubIntegrationError(testErr, integration)
		_ = appCtx.Repos.HouseholdIntegrations.Update(integration)
		respondIntegrationProviderError(ctx, integration, testErr)
		return
	}

	integration.LastError = ""
	_ = appCtx.Repos.HouseholdIntegrations.Update(integration)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Connection test succeeded"})
}

// SyncHouseholdIntegrationNow triggers an immediate synchronization for a household shopping list integration.
// @Summary      Synchronizes a household shopping list integration now
// @Description  Pushes current household shopping list items to the external provider and pulls checked states back (admin only).
// @Tags         household
// @Produce      json
// @Param        provider  path      string  true  "Provider identifier (bring or todoist)"
// @Success      200       {object}  apiModel.HouseholdIntegrationResponse
// @Failure      400       {object}  api.APIResponse
// @Failure      403       {object}  api.APIResponse
// @Failure      404       {object}  api.APIResponse
// @Failure      502       {object}  api.APIResponse
// @Router       /api/v1/household/integrations/{provider}/sync [post]
// @Security     BearerAuth
func SyncHouseholdIntegrationNow(ctx *gin.Context, appCtx *AppContext) {
	householdID, ok := authorizeHouseholdIntegrationAdmin(ctx, appCtx)
	if !ok {
		return
	}

	provider, ok := parseAndValidateIntegrationProvider(ctx)
	if !ok {
		return
	}

	integration, err := appCtx.Repos.HouseholdIntegrations.GetByHouseholdAndProvider(householdID, provider)
	if err != nil {
		if errors.Is(err, apperrors.ErrIntegrationNotFound) {
			api.RespondError(ctx, http.StatusNotFound, apperrors.ErrIntegrationNotFound)
			return
		}
		appCtx.Logger.Error().Msgf("Error loading household integration: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	svc := shoppingListSyncServiceForRequest(ctx, appCtx)
	reqCtx := requestContext(ctx)
	if syncErr := svc.SyncProvider(reqCtx, householdID, provider); syncErr != nil {
		updated, reloadErr := appCtx.Repos.HouseholdIntegrations.GetByHouseholdAndProvider(householdID, provider)
		if reloadErr == nil && updated != nil {
			integration = updated
		}
		respondIntegrationProviderError(ctx, integration, syncErr)
		return
	}

	updated, err := appCtx.Repos.HouseholdIntegrations.GetByHouseholdAndProvider(householdID, provider)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error reloading household integration: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewHouseholdIntegrationResponse(updated))
}
