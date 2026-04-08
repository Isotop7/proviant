package v1

import (
	"fmt"
	"net/http"
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/models/configuration/static"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
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
func CreateInvitation(ctx *gin.Context) {
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

	var req createInvitationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}

	dbController := database.DatabaseController{DBHandle: dbHandle}

	// Get user's household
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

	if user.HouseholdID == 0 {
		logger.Error().Msgf("User %d has no household", userID)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "user has no household"})
		return
	}

	invitation, err := dbController.CreateInvitation(user.HouseholdID, userID, req.Email)
	if err != nil {
		switch err {
		case errors.ErrDuplicateInvitation:
			logger.Error().Msgf("Duplicate invitation for email %s: %s", req.Email, err)
			ctx.JSON(http.StatusConflict, api.Error(err))
			return
		case errors.ErrInvitationNotAuthorized:
			logger.Error().Msgf("User %d not authorized to invite to household %d: %s", userID, user.HouseholdID, err)
			ctx.JSON(http.StatusForbidden, api.Error(err))
			return
		default:
			logger.Error().Msgf("Error creating invitation: %s", err)
			ctx.JSON(http.StatusInternalServerError, api.Error(err))
			return
		}
	}

	// Send invitation email via NotificationController
	proviantConfig, _ := ctx.MustGet("proviantConfig").(*configuration.ProviantConfiguration)
	notificationController, _ := ctx.MustGet("notificationController").(*controllers.NotificationController)
	if notificationController != nil {
		inviterName := user.Username
		household, householdErr := dbController.GetHouseholdByID(user.HouseholdID)
		householdName := fmt.Sprintf("Household #%d", user.HouseholdID)
		if householdErr == nil {
			householdName = household.Name
		}

		if err := notificationController.SendInvitationEmail(&invitation, inviterName, householdName, proviantConfig.Server.BaseURL); err != nil {
			logger.Error().Msgf("Failed to send invitation email to %s: %s", req.Email, err)
			// Don't fail the request, invitation is still created; retry handled by dispatcher
		} else {
			logger.Info().Msgf("Invitation email sent to %s", req.Email)
		}
	} else {
		logger.Warn().Msg("InvitationController not available, email will be sent by retry dispatcher")
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
func GetInvitations(ctx *gin.Context) {
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

	dbController := database.DatabaseController{DBHandle: dbHandle}

	// Get user's household
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

	if user.HouseholdID == 0 {
		logger.Error().Msgf("User %d has no household", userID)
		ctx.JSON(http.StatusNotFound, api.APIResponse{Message: "user has no household"})
		return
	}

	invitations, err := dbController.GetInvitationsForHousehold(user.HouseholdID, userID)
	if err != nil {
		logger.Error().Msgf("Error fetching invitations: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
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
func CancelInvitation(ctx *gin.Context) {
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

	invitationIDStr := ctx.Param("id")
	invitationID, err := strconv.ParseUint(invitationIDStr, 10, 64)
	if err != nil {
		logger.Error().Msgf("Invalid invitation ID '%s': %s", invitationIDStr, err)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "invalid invitation ID"})
		return
	}

	dbController := database.DatabaseController{DBHandle: dbHandle}
	if err := dbController.CancelInvitation(uint(invitationID), userID); err != nil {
		switch err {
		case errors.ErrInvitationNotFound:
			logger.Error().Msgf("Invitation %d not found: %s", invitationID, err)
			ctx.JSON(http.StatusNotFound, api.Error(err))
			return
		case errors.ErrInvitationNotAuthorized:
			logger.Error().Msgf("User %d not authorized to cancel invitation %d: %s", userID, invitationID, err)
			ctx.JSON(http.StatusForbidden, api.Error(err))
			return
		default:
			logger.Error().Msgf("Error cancelling invitation: %s", err)
			ctx.JSON(http.StatusInternalServerError, api.Error(err))
			return
		}
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Invitation cancelled successfully"})
}

type createInvitationRequest struct {
	Email string `json:"email" binding:"required,email"`
}
