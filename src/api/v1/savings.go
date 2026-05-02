// v1 implements version 1 of the proviant API
package v1

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// GetSavingsStats returns money and CO2 savings for the authenticated user's household
// @Summary      Get savings statistics
// @Description  Returns EUR saved/wasted and kg CO2 avoided/emitted for the current month and lifetime.
// @Description  CO2 coefficients sourced from Agribalyse LCA database via Open Food Facts ecoscore_data.
// @Tags         savings
// @Produce      json
// @Success      200  {object}  apiModel.SavingsStatsResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/savings/stats [get]
func GetSavingsStats(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	householdID, err := repos.Users.GetUserHouseholdByID(userID)
	if err != nil || householdID == 0 {
		ctx.JSON(http.StatusOK, apiModel.SavingsStatsResponse{
			CO2Source: "Agribalyse LCA database via Open Food Facts ecoscore_data",
		})
		return
	}

	stats, err := repos.Savings.GetSavingsStats(householdID)
	if err != nil {
		logger.Error().Msgf("GetSavingsStats: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing savings statistics"})
		return
	}

	ctx.JSON(http.StatusOK, stats)
}
