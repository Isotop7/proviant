package auth

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration/static"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
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
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	var req acceptInvitationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	invitationRepo := database.NewInvitationRepository(dbHandle)

	var user authentication.User
	if err := dbHandle.First(&user, userID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			logger.Error().Msgf("User with ID %d not found", userID)
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: errors.ErrInvalidUserID.Error()})
			return
		}
		logger.Error().Msgf("Error fetching user: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
		return
	}

	if err := invitationRepo.AcceptInvitation(req.Token, user.MailAddress, userID); err != nil {
		switch err {
		case errors.ErrInvitationNotFound:
			logger.Error().Msgf("Invitation not found: %s", err)
			ctx.JSON(http.StatusNotFound, api.Error(err))
		case errors.ErrInvitationExpired:
			logger.Error().Msgf("Invitation expired: %s", err)
			ctx.JSON(http.StatusConflict, api.Error(err))
		case errors.ErrInvitationAlreadyUsed:
			logger.Error().Msgf("Invitation already used: %s", err)
			ctx.JSON(http.StatusConflict, api.Error(err))
		case errors.ErrInvitationCancelled:
			logger.Error().Msgf("Invitation cancelled: %s", err)
			ctx.JSON(http.StatusConflict, api.Error(err))
		case errors.ErrInvitationEmailMismatch:
			logger.Error().Msgf("Email mismatch: %s", err)
			ctx.JSON(http.StatusBadRequest, api.Error(err))
		default:
			logger.Error().Msgf("Error accepting invitation: %s", err)
			ctx.JSON(http.StatusInternalServerError, api.Error(err))
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
