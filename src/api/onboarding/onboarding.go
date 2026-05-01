// onboarding contains handlers for the post-signup onboarding flow
package onboarding

import (
	"net/http"
	"strings"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	modelsAPI "codeberg.org/isotop7/proviant/models/api"

	"codeberg.org/isotop7/proviant/models/configuration/static"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	MsgFailedToExtract       = "Failed to extract user ID from JWT claims"
	MsgFailedToMarkHousehold = "Failed to mark household step done: %s"
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

	repos, ok := ctx.MustGet("repos").(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
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
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID == 0 {
		logger.Error().Msg(MsgFailedToExtract)
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Unauthorized"})
		return
	}

	repos, ok := ctx.MustGet("repos").(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	var req struct {
		DisplayName string `json:"displayName"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(errors.ErrParseBodyWrapper, err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	displayName := strings.TrimSpace(req.DisplayName)
	if displayName != "" {
		if err := repos.Users.UpdateDisplayName(userID, displayName); err != nil {
			logger.Error().Msgf("Failed to update display name: %s", err.Error())
			ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to update display name"})
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
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID == 0 {
		logger.Error().Msg(MsgFailedToExtract)
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Unauthorized"})
		return
	}

	repos, ok := ctx.MustGet("repos").(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(errors.ErrParseBodyWrapper, err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Household name cannot be empty"})
		return
	}

	if err := repos.Households.CreateAndSwitchHousehold(userID, name); err != nil {
		logger.Error().Msgf("Failed to create household: %s", err.Error())
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to create household"})
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
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID == 0 {
		logger.Error().Msg(MsgFailedToExtract)
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Unauthorized"})
		return
	}

	repos, ok := ctx.MustGet("repos").(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(errors.ErrParseBodyWrapper, err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	user, err := repos.Users.GetUserByID(userID)
	if err != nil {
		logger.Error().Msgf("Failed to get user: %s", err.Error())
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if err := repos.Invitations.AcceptInvitation(req.Token, user.MailAddress, userID); err != nil {
		logger.Error().Msgf("Failed to accept invitation: %s", err.Error())
		switch err {
		case errors.ErrInvitationNotFound:
			ctx.JSON(http.StatusNotFound, api.Error(err))
		case errors.ErrInvitationExpired, errors.ErrInvitationAlreadyUsed, errors.ErrInvitationCancelled:
			ctx.JSON(http.StatusConflict, api.Error(err))
		case errors.ErrInvitationEmailMismatch:
			ctx.JSON(http.StatusBadRequest, api.Error(err))
		default:
			ctx.JSON(http.StatusInternalServerError, api.InternalError())
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
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID == 0 {
		logger.Error().Msg(MsgFailedToExtract)
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Unauthorized"})
		return
	}

	repos, ok := ctx.MustGet("repos").(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	user, userErr := repos.Users.GetUserByID(userID)
	if userErr != nil {
		logger.Error().Msgf("Failed to get user: %s", userErr.Error())
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to get user"})
		return
	}

	households, err := repos.Households.GetPublicHouseholds(user.HouseholdID)
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
		logger.Error().Msg(MsgFailedToExtract)
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Unauthorized"})
		return
	}

	repos, ok := ctx.MustGet("repos").(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	var req struct {
		HouseholdID uint `json:"householdId" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(errors.ErrParseBodyWrapper, err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	if req.HouseholdID == 0 {
		logger.Error().Msg("Household ID is required")
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Household ID is required"})
		return
	}

	err := repos.Households.ApplyForHousehold(userID, req.HouseholdID)
	if err != nil {
		logger.Error().Msgf("Failed to apply for household: %s", err.Error())
		switch err {
		case errors.ErrHouseholdNotFound:
			ctx.JSON(http.StatusNotFound, api.Error(err))
		case errors.ErrApplicationAlreadyPending:
			ctx.JSON(http.StatusConflict, api.Error(err))
		default:
			ctx.JSON(http.StatusInternalServerError, api.InternalError())
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
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID == 0 {
		logger.Error().Msg(MsgFailedToExtract)
		ctx.JSON(http.StatusUnauthorized, api.APIResponse{Message: "Unauthorized"})
		return
	}

	repos, ok := ctx.MustGet("repos").(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	_ = repos.Users.EnsureOnboardingState(userID)
	err := repos.Users.MarkOnboardingComplete(userID)
	if err != nil {
		logger.Error().Msgf("Failed to complete onboarding: %s", err.Error())
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to complete onboarding"})
		return
	}

	logger.Info().Msgf("User %d completed onboarding", userID)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Onboarding completed"})
}
