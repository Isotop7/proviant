// v1 implements version 1 of the proviant API
package v1

import (
	"net/http"
	"strconv"
	"time"

	"codeberg.org/isotop7/proviant/api"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

// GetWasteAnalytics returns consumed vs. wasted aggregations, monthly breakdown,
// most-wasted categories, and a trend for the authenticated user's household.
// @Summary      Get waste analytics
// @Description  Returns per-month consumed vs. wasted metrics, top wasted categories
// @Description  with monetary and CO2 impact (each category nests its top wasted products),
// @Description  and a 6-month trend. EUR prices come from per-product overrides (if set)
// @Description  or category averages; CO2 is sourced from the Agribalyse LCA database via
// @Description  Open Food Facts ecoscore_data.
// @Tags         stats
// @Produce      json
// @Param        period  query     string  false  "Period window: month | 3months | 6months | 12months (default 6months). The window is floored to the first of the month for monthly-breakdown contiguity."
// @Param        sort    query     string  false  "Sort mostWastedCategories by: count | cost (default count)"
// @Param        limit   query     int     false  "Max number of categories to return (default 5, max 50)"
// @Success      200  {object}  apiModel.WasteAnalyticsResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/stats/waste [get]
func GetWasteAnalytics(ctx *gin.Context, appCtx *AppContext) {
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil || householdID == 0 {
		// No household: return zeroed response, do not error.
		ctx.JSON(http.StatusOK, apiModel.WasteAnalyticsResponse{
			Period:    "6months",
			Sort:      "count",
			Monthly:   []apiModel.WasteMonthly{},
			Trend:     []apiModel.StatsMonthlyCount{},
			CO2Source: "Agribalyse LCA database via Open Food Facts ecoscore_data",
		})
		return
	}

	period := ctx.DefaultQuery("period", "6months")
	months, ok := periodToMonths(period)
	if !ok {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "invalid period (allowed: month, 3months, 6months, 12months)"})
		return
	}

	sortBy := ctx.DefaultQuery("sort", "count")
	if sortBy != "count" && sortBy != "cost" {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "invalid sort (allowed: count, cost)"})
		return
	}

	limit := 5
	if v := ctx.Query("limit"); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed <= 0 {
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "invalid limit (must be a positive integer)"})
			return
		}
		if parsed > 50 {
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "limit must be 1..50"})
			return
		}
		limit = parsed
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

	categories, err := appCtx.Repos.WasteAnalytics.GetMostWastedCategories(householdID, since, limit, sortBy)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetMostWastedCategories: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing most-wasted categories"})
		return
	}

	products, err := appCtx.Repos.WasteAnalytics.GetMostWastedProducts(householdID, since, 3)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetMostWastedProducts: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing most-wasted products"})
		return
	}
	for i := range categories {
		if p, ok := products[categories[i].CategoryKey]; ok {
			categories[i].Products = p
		}
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
		Sort:                 sortBy,
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
	case "", util.PeriodValue6Months:
		return 6, true
	case util.PeriodValueMonth:
		return 1, true
	case util.PeriodValue3Months:
		return 3, true
	case util.PeriodValue12Months:
		return 12, true
	}
	return 0, false
}
