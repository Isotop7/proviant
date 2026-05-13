package auth

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	apperrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	"codeberg.org/isotop7/proviant/util"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// AcceptInvitation accepts a household invitation for the authenticated user.
// @Summary Accept invitation
// @Description Accepts a household invitation using a token
// @Tags Invitation
// @Accept json
// @Produce json
// @Param request body acceptInvitationRequest true "Accept invitation request"
// @Success 200 {object} api.APIResponse
// @Failure 400 {object} api.APIResponse
// @Failure 404 {object} api.APIResponse
// @Failure 409 {object} api.APIResponse
// @Failure 500 {object} api.APIResponse
// @Router /auth/invite/accept [post]
func AcceptInvitation(ctx *gin.Context) {
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrDatabaseContextNotFound)
		return
	}

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrUserIDFromToken)
		return
	}

	var req acceptInvitationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	user, err := repos.Users.GetUserByID(userID)
	if err != nil {
		logger.Error().Msgf(apperrors.ErrInvalidUserIDWrapper, userID, err)
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrInvalidUserID)
		return
	}

	if err := repos.Invitations.AcceptInvitation(req.Token, user.MailAddress, userID); err != nil {
		switch err {
		case apperrors.ErrInvitationNotFound:
			logger.Error().Msgf("Invitation not found: %s", err)
			api.RespondError(ctx, http.StatusNotFound, err)
		case apperrors.ErrInvitationExpired:
			logger.Error().Msgf("Invitation expired: %s", err)
			api.RespondError(ctx, http.StatusConflict, err)
		case apperrors.ErrInvitationAlreadyUsed:
			logger.Error().Msgf("Invitation already used: %s", err)
			api.RespondError(ctx, http.StatusConflict, err)
		case apperrors.ErrInvitationCancelled:
			logger.Error().Msgf("Invitation cancelled: %s", err)
			api.RespondError(ctx, http.StatusConflict, err)
		case apperrors.ErrInvitationEmailMismatch:
			logger.Error().Msgf("Email mismatch: %s", err)
			api.RespondError(ctx, http.StatusBadRequest, err)
		default:
			logger.Error().Msgf("Error accepting invitation: %s", err)
			api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrInternalServer)
		}
		return
	}

	go func() {
		if ws := controllers.GetWebhookService(); ws != nil {
			ws.FireEvent("household.member_joined", map[string]any{
				"userId":      userID,
				"username":    user.EffectiveName(),
				"householdId": user.HouseholdID,
			})
		}
	}()

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Invitation accepted successfully"})
}

type acceptInvitationRequest struct {
	Token string `json:"token" binding:"required"`
}
