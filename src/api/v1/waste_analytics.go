// v1 implements version 1 of the proviant API
package v1

import (
	"net/http"
	"strconv"
	"time"

	"codeberg.org/isotop7/proviant/api"
	apiModel "codeberg.org/isotop7/proviant/models/api"

	"github.com/gin-gonic/gin"
)

// GetWasteAnalytics returns consumed vs. wasted aggregations, monthly breakdown,
// most-wasted categories, and a trend for the authenticated user's household.
// @Summary      Get waste analytics
// @Description  Returns per-month consumed vs. wasted metrics, top wasted categories
// @Description  with monetary and CO2 impact, and a 6-month trend. EUR prices come from
// @Description  per-product overrides (if set) or category averages; CO2 is sourced
// @Description  from the Agribalyse LCA database via Open Food Facts ecoscore_data.
// @Tags         stats
// @Produce      json
// @Param        period  query     string  false  "Period window: month | 3months | 6months (default 6months)"
// @Success      200  {object}  apiModel.WasteAnalyticsResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/stats/waste [get]
func GetWasteAnalytics(ctx *gin.Context, appCtx *AppContext) {
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil || householdID == 0 {
		// No household: return zeroed response, do not error.
		ctx.JSON(http.StatusOK, apiModel.WasteAnalyticsResponse{
			Period:    "6months",
			Monthly:   []apiModel.WasteMonthly{},
			Trend:     []apiModel.StatsMonthlyCount{},
			CO2Source: "Agribalyse LCA database via Open Food Facts ecoscore_data",
		})
		return
	}

	period := ctx.DefaultQuery("period", "6months")
	months, ok := periodToMonths(period)
	if !ok {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "invalid period (allowed: month, 3months, 6months)"})
		return
	}

	now := time.Now()
	since := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -months+1, 0)

	row, err := appCtx.Repos.WasteAnalytics.GetConsumedVsWasted(householdID, since)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetConsumedVsWasted: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing waste analytics"})
		return
	}

	monthly, err := appCtx.Repos.WasteAnalytics.GetMonthlyBreakdown(householdID, since, months)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetMonthlyBreakdown: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing monthly breakdown"})
		return
	}

	categories, err := appCtx.Repos.WasteAnalytics.GetMostWastedCategories(householdID, since, 5)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetMostWastedCategories: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing most-wasted categories"})
		return
	}

	trend, err := appCtx.Repos.WasteAnalytics.GetTrendMonths(householdID, 6)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetTrendMonths: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing trend"})
		return
	}

	totalRemoved := row.ConsumedCount + row.WastedCount
	var wastedPercent float64
	if totalRemoved > 0 {
		wastedPercent = float64(row.WastedCount) / float64(totalRemoved) * 100
	}

	if monthly == nil {
		monthly = []apiModel.WasteMonthly{}
	}
	if categories == nil {
		categories = []apiModel.WasteCategoryStat{}
	}
	if trend == nil {
		trend = []apiModel.StatsMonthlyCount{}
	}

	ctx.JSON(http.StatusOK, apiModel.WasteAnalyticsResponse{
		Period:               period,
		ConsumedCount:        row.ConsumedCount,
		WastedCount:          row.WastedCount,
		TotalRemoved:         totalRemoved,
		WastedPercent:        wastedPercent,
		WastedEUR:            row.WastedEUR,
		WastedCO2Kg:          row.WastedCO2Kg,
		Monthly:              monthly,
		MostWastedCategories: categories,
		Trend:                trend,
		CO2Source:            "Agribalyse LCA database via Open Food Facts ecoscore_data",
	})
}

func periodToMonths(p string) (int, bool) {
	switch p {
	case "", "6months":
		return 6, true
	case "month":
		return 1, true
	case "3months":
		return 3, true
	}
	if n, err := strconv.Atoi(p); err == nil && n > 0 && n <= 24 {
		return n, true
	}
	return 0, false
}
