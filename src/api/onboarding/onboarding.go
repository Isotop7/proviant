// onboarding contains handlers for the post-signup onboarding flow
package onboarding

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	modelsAPI "codeberg.org/isotop7/proviant/models/api"

	"codeberg.org/isotop7/proviant/models/configuration/static"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// GetOnboardingState returns the current onboarding progress for the authenticated user
// @Summary      	Get onboarding state
// @Description  	Returns the current onboarding progress for the user
// @Tags         	onboarding
// @Produce      	json
// @Success      	200  {object}  modelsAPI.OnboardingStateResponse
// @Failure      	401  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/onboarding/state [get]
func GetOnboardingState(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID == 0 {
		logger.Error().Msg("Failed to extract user ID from JWT claims")
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Unauthorized"})
		return
	}

	dbHandle, ok := ctx.Get("dbHandle")
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	userRepo := database.NewUserRepository(dbHandle.(*gorm.DB))

	onboardingState, err := userRepo.GetOnboardingState(userID)
	if err != nil {
		// No onboarding state exists — this is an existing user, treat as completed
		logger.Debug().Msgf("No onboarding state for user %d, treating as completed", userID)
		ctx.JSON(http.StatusOK, modelsAPI.OnboardingStateResponse{
			NotificationsSetup:  true,
			HouseholdStepDone:   true,
			OnboardingCompleted: true,
		})
		return
	}

	response := modelsAPI.OnboardingStateResponse{
		NotificationsSetup:  onboardingState.NotificationsSetup,
		HouseholdStepDone:   onboardingState.HouseholdStepDone,
		OnboardingCompleted: onboardingState.OnboardingCompleted,
	}

	ctx.JSON(http.StatusOK, response)
}

// GetAvailableHouseholds returns a list of households the user can apply to
// @Summary      	Get available households
// @Description  	Returns a list of households that the user can apply to join
// @Tags         	onboarding
// @Produce      	json
// @Success      	200  {array}   modelsAPI.HouseholdListItem
// @Failure      	401  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/onboarding/households [get]
func GetAvailableHouseholds(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID == 0 {
		logger.Error().Msg("Failed to extract user ID from JWT claims")
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Unauthorized"})
		return
	}

	dbHandle, ok := ctx.Get("dbHandle")
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	userRepo := database.NewUserRepository(dbHandle.(*gorm.DB))
	householdRepo := database.NewHouseholdRepository(dbHandle.(*gorm.DB))

	user, userErr := userRepo.GetUserByID(userID)
	if userErr != nil {
		logger.Error().Msgf("Failed to get user: %s", userErr.Error())
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to get user"})
		return
	}

	households, err := householdRepo.GetPublicHouseholds(user.HouseholdID)
	if err != nil {
		logger.Error().Msgf("Failed to get households: %s", err.Error())
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to get households"})
		return
	}

	items := make([]modelsAPI.HouseholdListItem, len(households))
	for i := range households {
		items[i] = modelsAPI.HouseholdListItem{
			ID:          households[i].ID,
			Name:        households[i].Name,
			Description: households[i].Description,
			MemberCount: households[i].MemberCount,
		}
	}
	ctx.JSON(http.StatusOK, items)
}

// ApplyForHousehold submits an application to join a household during onboarding
// @Summary      	Apply for household
// @Description  	Submits an application to join a household during onboarding
// @Tags         	onboarding
// @Accept			json
// @Produce      	json
// @Param			householdId	body		object	true	"Household ID"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	401  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/onboarding/apply-household [post]
func ApplyForHousehold(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID == 0 {
		logger.Error().Msg("Failed to extract user ID from JWT claims")
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Unauthorized"})
		return
	}

	dbHandle, ok := ctx.Get("dbHandle")
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	householdRepo := database.NewHouseholdRepository(dbHandle.(*gorm.DB))
	userRepo := database.NewUserRepository(dbHandle.(*gorm.DB))

	var req struct {
		HouseholdID uint `json:"householdId" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf("Error parsing body: %s", err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	if req.HouseholdID == 0 {
		logger.Error().Msg("Household ID is required")
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Household ID is required"})
		return
	}

	err := householdRepo.ApplyForHousehold(userID, req.HouseholdID)
	if err != nil {
		logger.Error().Msgf("Failed to apply for household: %s", err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	if markErr := userRepo.MarkHouseholdStepDone(userID); markErr != nil {
		logger.Warn().Msgf("Failed to mark household step done: %s", markErr.Error())
	}

	logger.Info().Msgf("User %d applied for household %d", userID, req.HouseholdID)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Application submitted successfully"})
}

// CompleteOnboarding marks the onboarding as complete
// @Summary      	Complete onboarding
// @Description  	Marks the onboarding process as complete
// @Tags         	onboarding
// @Produce      	json
// @Success      	200  {object}  api.APIResponse
// @Failure      	401  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/onboarding/complete [post]
func CompleteOnboarding(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID == 0 {
		logger.Error().Msg("Failed to extract user ID from JWT claims")
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Unauthorized"})
		return
	}

	dbHandle, ok := ctx.Get("dbHandle")
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	userRepo := database.NewUserRepository(dbHandle.(*gorm.DB))

	err := userRepo.MarkOnboardingComplete(userID)
	if err != nil {
		logger.Error().Msgf("Failed to complete onboarding: %s", err.Error())
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to complete onboarding"})
		return
	}

	logger.Info().Msgf("User %d completed onboarding", userID)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Onboarding completed"})
}
