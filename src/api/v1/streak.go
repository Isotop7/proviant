// v1 implements version 1 of the proviant API
package v1

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	apiModel "codeberg.org/isotop7/proviant/models/api"

	"github.com/gin-gonic/gin"
)

// GetStreak returns the current waste-free streak for the user's household
// @Summary      Get waste-free streak
// @Description  Returns the current and longest waste-free streak for the caller's household
// @Tags         streak
// @Produce      json
// @Success      200  {object}  apiModel.StreakResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/streak [get]
func GetStreak(ctx *gin.Context, appCtx *AppContext) {
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil || householdID == 0 {
		ctx.JSON(http.StatusOK, apiModel.StreakResponse{CurrentStreak: 0, LongestStreak: 0})
		return
	}

	streak, err := appCtx.Repos.Streaks.GetOrCreateStreakForHousehold(householdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetStreak: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, apiModel.StreakResponse{
		CurrentStreak: streak.CurrentStreak,
		LongestStreak: streak.LongestStreak,
	})
}
