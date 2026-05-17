package v1

import (
	apperrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/database"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
		appCtx.Logger.Warn().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.InvalidInputError())
		return
	}

	user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Warn().Msgf(apperrors.ErrInvalidUserIDWrapperWithMessage, appCtx.UserID, err)
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrInvalidUserID)
		return
	}

	if user.HouseholdID == 0 {
		appCtx.Logger.Warn().Msgf("User %d has no household", appCtx.UserID)
		api.RespondError(ctx, http.StatusBadRequest, errors.New("user has no household"))
		return
	}

	var invitation database.HouseholdInvitation
	err = appCtx.DB.Transaction(func(tx *gorm.DB) error {
		createErr := error(nil)
		invitation, createErr = appCtx.Repos.Invitations.CreateInvitationTx(tx, user.HouseholdID, appCtx.UserID, req.Email)
		if createErr != nil {
			return createErr
		}

		proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
		notificationController, _ := ctx.MustGet(util.ContextKeyNotificationController).(*controllers.NotificationController)
		if notificationController != nil {
			inviterName := user.EffectiveName()
			householdName := fmt.Sprintf("Household #%d", user.HouseholdID)
			if household, householdErr := appCtx.Repos.Users.GetHouseholdByID(user.HouseholdID); householdErr == nil {
				householdName = household.Name
			}
			if emailErr := notificationController.SendInvitationEmail(&invitation, inviterName, householdName, proviantConfig.Server.BaseURL, tx); emailErr != nil {
				return emailErr
			}
		}
		return nil
	})
	if err != nil {
		switch err {
		case apperrors.ErrDuplicateInvitation:
			appCtx.Logger.Error().Msgf("Duplicate invitation for email %s: %s", req.Email, err)
			api.RespondError(ctx, http.StatusConflict, err)
			return
		case apperrors.ErrInvitationNotAuthorized:
			appCtx.Logger.Error().Msgf("User %d not authorized to invite to household %d: %s", appCtx.UserID, user.HouseholdID, err)
			api.RespondError(ctx, http.StatusForbidden, err)
			return
		default:
			appCtx.Logger.Error().Msgf("Error creating invitation: %s", err)
			api.RespondError(ctx, http.StatusInternalServerError, err)
			return
		}
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
		appCtx.Logger.Warn().Msgf(apperrors.ErrInvalidUserIDWrapperWithMessage, appCtx.UserID, err)
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrInvalidUserID)
		return
	}

	if user.HouseholdID == 0 {
		appCtx.Logger.Error().Msgf("User %d has no household", appCtx.UserID)
		api.RespondError(ctx, http.StatusNotFound, errors.New("user has no household"))
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
		case apperrors.ErrInvitationNotFound:
			appCtx.Logger.Error().Msgf("Invitation %d not found: %s", invitationID, err)
			api.RespondError(ctx, http.StatusNotFound, err)
			return
		case apperrors.ErrInvitationNotAuthorized:
			appCtx.Logger.Error().Msgf("User %d not authorized to cancel invitation %d: %s", appCtx.UserID, invitationID, err)
			api.RespondError(ctx, http.StatusForbidden, err)
			return
		default:
			appCtx.Logger.Error().Msgf("Error cancelling invitation: %s", err)
			api.RespondError(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Invitation cancelled successfully"})
}

type createInvitationRequest struct {
	Email string `json:"email" binding:"required,email"`
}
