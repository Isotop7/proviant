// v1 implements version 1 of the proviant API
package v1

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	apiModel "codeberg.org/isotop7/proviant/models/api"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// GetStreak returns the current waste-free streak for the user's household
// @Summary      Get waste-free streak
// @Description  Returns the current and longest waste-free streak for the caller's household
// @Tags         streak
// @Produce      json
// @Success      200  {object}  apiModel.StreakResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/streak [get]
func GetStreak(ctx *gin.Context) {
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

	userRepo := database.NewUserRepository(dbHandle)
	householdID, err := userRepo.GetUserHouseholdByID(userID)
	if err != nil || householdID == 0 {
		ctx.JSON(http.StatusOK, apiModel.StreakResponse{CurrentStreak: 0, LongestStreak: 0})
		return
	}

	streakRepo := database.NewStreakRepository(dbHandle)
	streak, err := streakRepo.GetOrCreateStreakForHousehold(householdID)
	if err != nil {
		logger.Error().Msgf("GetStreak: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, apiModel.StreakResponse{
		CurrentStreak: streak.CurrentStreak,
		LongestStreak: streak.LongestStreak,
	})
}
