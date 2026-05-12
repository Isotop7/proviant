package v1

import (
	"fmt"
	"net/http"
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

// CreateInvitation creates a new household invitation and sends an email to the recipient.
// @Summary Create invitation
// @Description Creates a new household invitation and sends an email to the recipient
// @Tags Invitation
// @Accept json
// @Produce json
// @Param request body createInvitationRequest true "Invitation request"
// @Success 201 {object} api.APIResponse
// @Failure 400 {object} api.APIResponse
// @Failure 404 {object} api.APIResponse
// @Failure 409 {object} api.APIResponse
// @Failure 500 {object} api.APIResponse
// @Router /api/v1/household/invitations [post]
func CreateInvitation(ctx *gin.Context, appCtx *AppContext) {
	var req createInvitationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		appCtx.Logger.Warn().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return
	}

	user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Warn().Msgf(errors.ErrInvalidUserIDWrapperWithMessage, appCtx.UserID, err)
		ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrInvalidUserID))
		return
	}

	if user.HouseholdID == 0 {
		appCtx.Logger.Warn().Msgf("User %d has no household", appCtx.UserID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "user has no household"})
		return
	}

	invitation, err := appCtx.Repos.Invitations.CreateInvitation(user.HouseholdID, appCtx.UserID, req.Email)
	if err != nil {
		switch err {
		case errors.ErrDuplicateInvitation:
			appCtx.Logger.Error().Msgf("Duplicate invitation for email %s: %s", req.Email, err)
			ctx.JSON(http.StatusConflict, api.Error(err))
			return
		case errors.ErrInvitationNotAuthorized:
			appCtx.Logger.Error().Msgf("User %d not authorized to invite to household %d: %s", appCtx.UserID, user.HouseholdID, err)
			ctx.JSON(http.StatusForbidden, api.Error(err))
			return
		default:
			appCtx.Logger.Error().Msgf("Error creating invitation: %s", err)
			ctx.JSON(http.StatusInternalServerError, api.Error(err))
			return
		}
	}

	proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
	notificationController, _ := ctx.MustGet(util.ContextKeyNotificationController).(*controllers.NotificationController)
	if notificationController != nil {
		inviterName := user.EffectiveName()
		household, householdErr := appCtx.Repos.Users.GetHouseholdByID(user.HouseholdID)
		householdName := fmt.Sprintf("Household #%d", user.HouseholdID)
		if householdErr == nil {
			householdName = household.Name
		}

		if err := notificationController.SendInvitationEmail(&invitation, inviterName, householdName, proviantConfig.Server.BaseURL); err != nil {
			appCtx.Logger.Error().Msgf("Failed to send invitation email to %s: %s", req.Email, err)
		} else {
			appCtx.Logger.Info().Msgf("Invitation email sent to %s", req.Email)
		}
	} else {
		appCtx.Logger.Warn().Msg("InvitationController not available, email will be sent by retry dispatcher")
	}

	ctx.JSON(http.StatusCreated, api.APIResponse{Message: "Invitation created successfully"})
}

// GetInvitations returns all invitations for the calling user's household.
// @Summary Get household invitations
// @Description Returns all invitations for the calling user's household
// @Tags Invitation
// @Produce json
// @Success 200 {array} database.HouseholdInvitation
// @Failure 400 {object} api.APIResponse
// @Failure 404 {object} api.APIResponse
// @Failure 500 {object} api.APIResponse
// @Router /api/v1/household/invitations [get]
func GetInvitations(ctx *gin.Context, appCtx *AppContext) {
	user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Warn().Msgf(errors.ErrInvalidUserIDWrapperWithMessage, appCtx.UserID, err)
		ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrInvalidUserID))
		return
	}

	if user.HouseholdID == 0 {
		appCtx.Logger.Error().Msgf("User %d has no household", appCtx.UserID)
		ctx.JSON(http.StatusNotFound, api.APIResponse{Message: "user has no household"})
		return
	}

	invitations, err := appCtx.Repos.Invitations.GetInvitationsForHousehold(user.HouseholdID, appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error fetching invitations: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, invitations)
}

// CancelInvitation cancels a pending invitation.
// @Summary Cancel invitation
// @Description Cancels a pending invitation by ID
// @Tags Invitation
// @Produce json
// @Param id path int true "Invitation ID"
// @Success 200 {object} api.APIResponse
// @Failure 400 {object} api.APIResponse
// @Failure 403 {object} api.APIResponse
// @Failure 404 {object} api.APIResponse
// @Failure 500 {object} api.APIResponse
// @Router /api/v1/household/invitations/{id} [delete]
func CancelInvitation(ctx *gin.Context, appCtx *AppContext) {
	invitationIDStr := ctx.Param("id")
	invitationID, err := strconv.ParseUint(invitationIDStr, 10, 64)
	if err != nil {
		appCtx.Logger.Warn().Msgf("Invalid invitation ID '%s': %s", invitationIDStr, err)
		ctx.JSON(http.StatusBadRequest, api.InvalidInputErrorWithDetail("invitation ID must be a valid unsigned integer"))
		return
	}

	if err := appCtx.Repos.Invitations.CancelInvitation(uint(invitationID), appCtx.UserID); err != nil {
		switch err {
		case errors.ErrInvitationNotFound:
			appCtx.Logger.Error().Msgf("Invitation %d not found: %s", invitationID, err)
			ctx.JSON(http.StatusNotFound, api.Error(err))
			return
		case errors.ErrInvitationNotAuthorized:
			appCtx.Logger.Error().Msgf("User %d not authorized to cancel invitation %d: %s", appCtx.UserID, invitationID, err)
			ctx.JSON(http.StatusForbidden, api.Error(err))
			return
		default:
			appCtx.Logger.Error().Msgf("Error cancelling invitation: %s", err)
			ctx.JSON(http.StatusInternalServerError, api.InternalError())
			return
		}
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Invitation cancelled successfully"})
}

type createInvitationRequest struct {
	Email string `json:"email" binding:"required,email"`
}
