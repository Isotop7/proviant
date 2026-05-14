// onboarding contains handlers for the post-signup onboarding flow
package onboarding

import (
	"errors"
	"net/http"
	"strings"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	apperrors "codeberg.org/isotop7/proviant/errors"
	modelsAPI "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/util"

	"codeberg.org/isotop7/proviant/models/configuration/static"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	MsgFailedToExtract       = "Failed to extract user ID from JWT claims"
	MsgFailedToMarkHousehold = "Failed to mark household step done: %s"
)

func mustGetOnboardingContext(ctx *gin.Context) (*zerolog.Logger, *database.RepositoryContainer, uint, bool) {
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID == 0 {
		logger.Error().Msg(MsgFailedToExtract)
		api.RespondError(ctx, http.StatusUnauthorized, errors.New("Unauthorized"))
		return nil, nil, 0, false
	}

	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrDatabaseContextNotFound)
		return nil, nil, 0, false
	}

	return logger, repos, userID, true
}

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
	logger, repos, userID, ok := mustGetOnboardingContext(ctx)
	if !ok {
		return
	}

	onboardingState, err := repos.Users.GetOnboardingState(userID)
	if err != nil {
		// No onboarding state exists — this is an existing user, treat as completed
		logger.Debug().Msgf("No onboarding state for user %d, treating as completed", userID)
		ctx.JSON(http.StatusOK, modelsAPI.OnboardingStateResponse{
			ProfileStepDone:     true,
			NotificationsSetup:  true,
			HouseholdStepDone:   true,
			OnboardingCompleted: true,
		})
		return
	}

	response := modelsAPI.OnboardingStateResponse{
		ProfileStepDone:     onboardingState.ProfileStepDone,
		NotificationsSetup:  onboardingState.NotificationsSetup,
		HouseholdStepDone:   onboardingState.HouseholdStepDone,
		OnboardingCompleted: onboardingState.OnboardingCompleted,
	}

	ctx.JSON(http.StatusOK, response)
}

// UpdateOnboardingProfile updates the user's display name during onboarding
// @Summary      	Update profile
// @Description  	Updates the user's display name and marks the profile step as done
// @Tags         	onboarding
// @Accept			json
// @Produce      	json
// @Param			body	body		object	true	"Display name"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	401  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/onboarding/profile [patch]
func UpdateOnboardingProfile(ctx *gin.Context) {
	logger, repos, userID, ok := mustGetOnboardingContext(ctx)
	if !ok {
		return
	}

	var req struct {
		DisplayName string `json:"displayName" binding:"max=100"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(apperrors.ErrParseBodyWrapper, err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName != "" {
		if err := repos.Users.UpdateDisplayName(userID, displayName); err != nil {
			logger.Error().Msgf("Failed to update display name: %s", err.Error())
			api.RespondError(ctx, http.StatusInternalServerError, errors.New("failed to update display name"))
			return
		}
	}

	if markErr := repos.Users.MarkProfileStepDone(userID); markErr != nil {
		logger.Warn().Msgf("Failed to mark profile step done: %s", markErr.Error())
	}

	logger.Info().Msgf("User %d updated profile during onboarding", userID)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Profile updated"})
}

// CreateOnboardingHousehold creates a new household for the user during onboarding
// @Summary      	Create household
// @Description  	Creates a new household and assigns the user to it
// @Tags         	onboarding
// @Accept			json
// @Produce      	json
// @Param			body	body		object	true	"Household name"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	401  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/onboarding/create-household [post]
func CreateOnboardingHousehold(ctx *gin.Context) {
	logger, repos, userID, ok := mustGetOnboardingContext(ctx)
	if !ok {
		return
	}

	var req struct {
		Name string `json:"name" binding:"required,min=1,max=100"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Warn().Msgf(apperrors.ErrParseBodyWrapper, err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrHouseholdNameEmpty)
		return
	}

	if err := repos.Households.CreateAndSwitchHousehold(userID, req.Name); err != nil {
		logger.Error().Msgf("Failed to create household: %s", err.Error())
		api.RespondError(ctx, http.StatusInternalServerError, errors.New("failed to create household"))
		return
	}

	if markErr := repos.Users.MarkHouseholdStepDone(userID); markErr != nil {
		logger.Warn().Msgf(MsgFailedToMarkHousehold, markErr.Error())
	}

	logger.Info().Msgf("User %d created household during onboarding", userID)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Household created"})
}

// JoinOnboardingByInvite accepts a household invitation by token during onboarding
// @Summary      	Join by invite
// @Description  	Accepts a household invitation using an invite token
// @Tags         	onboarding
// @Accept			json
// @Produce      	json
// @Param			body	body		object	true	"Invite token"
// @Success      	200  {object}  api.APIResponse
// @Failure      	400  {object}  api.APIResponse
// @Failure      	401  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/onboarding/join-invite [post]
func JoinOnboardingByInvite(ctx *gin.Context) {
	logger, repos, userID, ok := mustGetOnboardingContext(ctx)
	if !ok {
		return
	}

	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Warn().Msgf(apperrors.ErrParseBodyWrapper, err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	user, err := repos.Users.GetUserByID(userID)
	if err != nil {
		logger.Error().Msgf("Failed to get user: %s", err.Error())
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		return
	}

	if err := repos.Invitations.AcceptInvitation(req.Token, user.MailAddress, userID); err != nil {
		logger.Error().Msgf("Failed to accept invitation: %s", err.Error())
		switch err {
		case apperrors.ErrInvitationNotFound:
			api.RespondError(ctx, http.StatusNotFound, err)
		case apperrors.ErrInvitationExpired, apperrors.ErrInvitationAlreadyUsed, apperrors.ErrInvitationCancelled:
			api.RespondError(ctx, http.StatusConflict, err)
		case apperrors.ErrInvitationEmailMismatch:
			api.RespondError(ctx, http.StatusBadRequest, err)
		default:
			api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		}
		return
	}

	if markErr := repos.Users.MarkHouseholdStepDone(userID); markErr != nil {
		logger.Warn().Msgf(MsgFailedToMarkHousehold, markErr.Error())
	}

	logger.Info().Msgf("User %d joined household via invite during onboarding", userID)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Joined household"})
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
	logger, repos, userID, ok := mustGetOnboardingContext(ctx)
	if !ok {
		return
	}

	user, userErr := repos.Users.GetUserByID(userID)
	if userErr != nil {
		logger.Error().Msgf("Failed to get user: %s", userErr.Error())
		api.RespondError(ctx, http.StatusInternalServerError, errors.New("failed to get user"))
		return
	}

	households, err := repos.Households.GetPublicHouseholds(user.HouseholdID)
	if err != nil {
		logger.Error().Msgf("Failed to get households: %s", err.Error())
		api.RespondError(ctx, http.StatusInternalServerError, errors.New("failed to get households"))
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
	logger, repos, userID, ok := mustGetOnboardingContext(ctx)
	if !ok {
		return
	}

	var req struct {
		HouseholdID uint `json:"householdId" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(apperrors.ErrParseBodyWrapper, err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	if req.HouseholdID == 0 {
		logger.Error().Msg("Household ID is required")
		api.RespondError(ctx, http.StatusBadRequest, errors.New("household ID is required"))
		return
	}

	err := repos.Households.ApplyForHousehold(userID, req.HouseholdID)
	if err != nil {
		logger.Error().Msgf("Failed to apply for household: %s", err.Error())
		switch err {
		case apperrors.ErrHouseholdNotFound:
			api.RespondError(ctx, http.StatusNotFound, err)
		case apperrors.ErrApplicationAlreadyPending:
			api.RespondError(ctx, http.StatusConflict, err)
		default:
			api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		}
		return
	}

	if markErr := repos.Users.MarkHouseholdStepDone(userID); markErr != nil {
		logger.Warn().Msgf(MsgFailedToMarkHousehold, markErr.Error())
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
	logger, repos, userID, ok := mustGetOnboardingContext(ctx)
	if !ok {
		return
	}

	_ = repos.Users.EnsureOnboardingState(userID)
	err := repos.Users.MarkOnboardingComplete(userID)
	if err != nil {
		logger.Error().Msgf("Failed to complete onboarding: %s", err.Error())
		api.RespondError(ctx, http.StatusInternalServerError, errors.New("failed to complete onboarding"))
		return
	}

	logger.Info().Msgf("User %d completed onboarding", userID)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Onboarding completed"})
}
