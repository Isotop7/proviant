// v1 implements version 1 of the proviant API
package v1

import (
	"net/http"
	"strconv"
	"time"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/i18n"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

// GetWasteAnalytics returns consumed vs. wasted aggregations, monthly breakdown,
// most-wasted categories, and a trend for the authenticated user's household.
// @Summary      Get waste analytics
// @Description  Returns per-month consumed vs. wasted metrics, top wasted categories
// @Description  with monetary and CO2 impact (each category nests its top wasted products),
// @Description  the waste broken down by the reason it was wasted, the category the household
// @Description  most over-buys and a one-sentence insight about it, and a 6-month trend.
// @Description  EUR prices come from per-product overrides (if set)
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
	lang := appCtx.analyticsLanguage(ctx)
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil || householdID == 0 {
		// No household: return zeroed response, do not error.
		ctx.JSON(http.StatusOK, apiModel.WasteAnalyticsResponse{
			Period:    "6months",
			Sort:      "count",
			Monthly:   []apiModel.WasteMonthly{},
			Trend:     []apiModel.StatsMonthlyCount{},
			Causes:    []apiModel.WasteCauseStat{},
			CO2Source: "Agribalyse LCA database via Open Food Facts ecoscore_data",
			Insight:   i18n.T(lang, "waste.insight.insufficientData"),
		})
		return
	}

	period := ctx.DefaultQuery("period", "6months")
	months, ok := periodToMonths(period)
	if !ok {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrWasteInvalidPeriod)
		return
	}

	sortBy := ctx.DefaultQuery("sort", "count")
	if sortBy != "count" && sortBy != "cost" {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrWasteInvalidSort)
		return
	}

	limit := 5
	if v := ctx.Query("limit"); v != "" {
		parsed, parseErr := strconv.Atoi(v)
		if parseErr != nil || parsed <= 0 {
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrWasteInvalidLimit)
			return
		}
		if parsed > 50 {
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrWasteLimitOutOfRange)
			return
		}
		limit = parsed
	}

	now := time.Now()
	since := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -months+1, 0)

	row, err := appCtx.Repos.WasteAnalytics.GetConsumedVsWasted(householdID, since)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetConsumedVsWasted: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	monthly, err := appCtx.Repos.WasteAnalytics.GetMonthlyBreakdown(householdID, since, months)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetMonthlyBreakdown: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	categories, err := appCtx.Repos.WasteAnalytics.GetMostWastedCategories(householdID, since, limit, sortBy)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetMostWastedCategories: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	products, err := appCtx.Repos.WasteAnalytics.GetMostWastedProducts(householdID, since, 3)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetMostWastedProducts: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
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
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	breakdown, err := appCtx.Repos.WasteAnalytics.GetWasteCauseBreakdown(householdID, since)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetWasteCauseBreakdown: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
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
	if breakdown.Causes == nil {
		breakdown.Causes = []apiModel.WasteCauseStat{}
	}

	insight, insightAvailable := controllers.WasteInsight(lang, breakdown.Overbuy, breakdown.Causes, breakdown.WastedEvents)

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
		Causes:               breakdown.Causes,
		Overbuy:              breakdown.Overbuy,
		Insight:              insight,
		InsightAvailable:     insightAvailable,
	})
}

// analyticsLanguage resolves the language the insight is written in: the
// user's preference, then the household's, then the Accept-Language header. A
// failed preference lookup still leaves the header as a fallback, so the
// insight never falls back to the default language while the browser asks for
// another one.
func (a *AppContext) analyticsLanguage(ctx *gin.Context) string {
	userLanguage, householdLanguage, err := a.Repos.Users.GetUserDisplayPreferencesByID(a.UserID)
	if err != nil {
		a.Logger.Warn().Msgf("Waste analytics: could not read the language preferences: %s", err)
		userLanguage, householdLanguage = "", ""
	}
	acceptLanguage := ""
	if ctx.Request != nil {
		acceptLanguage = ctx.GetHeader("Accept-Language")
	}
	return i18n.ResolveLanguage(userLanguage, householdLanguage, acceptLanguage)
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
