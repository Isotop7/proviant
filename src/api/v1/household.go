package v1

import (
	"context"
	"errors"
	apperrors "codeberg.org/isotop7/proviant/errors"
	"net/http"
	"strconv"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	MsgInvalidApplicationId = "invalid application id"
	MsgHouseholdNameEmpty   = "household name cannot be empty"
)

// LeaveHousehold removes the calling user from their current household and assigns them a new personal one.
// @Summary      Leave current household
// @Description  Creates a new personal household for the user. Products are moved if they were the sole member.
// @Tags         household
// @Produce      json
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/user/household/leave [post]
func LeaveHousehold(ctx *gin.Context, appCtx *AppContext) {
	user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting user: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, err)
		return
	}
	oldHouseholdID := user.HouseholdID

	if err := appCtx.Repos.Households.LeaveHousehold(appCtx.UserID); err != nil {
		appCtx.Logger.Error().Msgf("Error leaving household: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, err)
		return
	}

	go recordMemberLeft(ctx, appCtx.UserID, oldHouseholdID)
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Successfully left household"})
}

// CreateHousehold creates a new named household and switches the calling user to it.
// @Summary      Create and switch to a new household
// @Description  Creates a new household with the given name and assigns the user to it.
// @Tags         household
// @Accept       json
// @Produce      json
// @Param        household  body      createHouseholdRequest  true  "Household"
// @Success      200        {object}  api.APIResponse
// @Failure      400        {object}  api.APIResponse
// @Failure      500        {object}  api.APIResponse
// @Router       /api/v1/user/household/create [post]
func CreateHousehold(ctx *gin.Context, appCtx *AppContext) {
	var req createHouseholdRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		appCtx.Logger.Error().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		api.RespondError(ctx, http.StatusBadRequest, errors.New(MsgHouseholdNameEmpty))
		return
	}

	if err := appCtx.Repos.Households.CreateAndSwitchHousehold(appCtx.UserID, req.Name); err != nil {
		appCtx.Logger.Error().Msgf("Error creating household: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Household created and activated"})
}

// ApplyForHousehold submits a join application for an existing household.
// @Summary      Apply to join a household
// @Description  Creates a pending application for the calling user to join the specified household.
// @Tags         household
// @Produce      json
// @Param        id   path      int  true  "Household ID"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      409  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/{id}/apply [post]
func ApplyForHousehold(ctx *gin.Context, appCtx *AppContext) {
	householdID, ok := parseUintParam(ctx, appCtx.Logger, "id", "invalid household id")
	if !ok {
		return
	}

	applyErr := appCtx.Repos.Households.ApplyForHousehold(appCtx.UserID, householdID)
	switch applyErr {
	case nil:
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Application submitted"})
	case apperrors.ErrHouseholdNotFound:
		api.RespondError(ctx, http.StatusNotFound, applyErr)
	case apperrors.ErrApplicationAlreadyPending:
		api.RespondError(ctx, http.StatusConflict, applyErr)
	default:
		appCtx.Logger.Error().Msgf("Error applying for household: %s", applyErr)
		api.RespondError(ctx, http.StatusInternalServerError, applyErr)
	}
}

// GetHouseholdApplications returns all pending applications for the household the caller administrates.
// @Summary      List pending household applications
// @Description  Returns pending join applications for the household the calling user is admin of.
// @Tags         household
// @Produce      json
// @Success      200  {array}   database.HouseholdApplication
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/applications [get]
func GetHouseholdApplications(ctx *gin.Context, appCtx *AppContext) {
	applications, err := appCtx.Repos.Households.GetPendingApplicationsForAdmin(appCtx.UserID)
	switch err {
	case nil:
		ctx.JSON(http.StatusOK, applications)
	case apperrors.ErrNotHouseholdAdmin:
		api.RespondError(ctx, http.StatusForbidden, err)
	default:
		appCtx.Logger.Error().Msgf("Error fetching applications: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, err)
	}
}

// ApproveHouseholdApplication approves a pending join application.
// @Summary      Approve a household application
// @Description  Moves the applicant into the household. Caller must be the household admin.
// @Tags         household
// @Produce      json
// @Param        id   path      int  true  "Application ID"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/applications/{id}/approve [post]
func ApproveHouseholdApplication(ctx *gin.Context, appCtx *AppContext) {
	applicationID, ok := parseUintParam(ctx, appCtx.Logger, "id", MsgInvalidApplicationId)
	if !ok {
		return
	}

	approveErr := appCtx.Repos.Households.ApproveApplication(applicationID, appCtx.UserID)
	switch approveErr {
	case nil:
		go recordMemberAdded(ctx, appCtx.UserID, applicationID)
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Application approved"})
	case apperrors.ErrApplicationNotFound:
		api.RespondError(ctx, http.StatusNotFound, approveErr)
	case apperrors.ErrNotHouseholdAdmin:
		api.RespondError(ctx, http.StatusForbidden, approveErr)
	default:
		appCtx.Logger.Error().Msgf("Error approving application: %s", approveErr)
		api.RespondError(ctx, http.StatusInternalServerError, approveErr)
	}
}

// RejectHouseholdApplication rejects a pending join application.
// @Summary      Reject a household application
// @Description  Marks the application as rejected. Caller must be the household admin.
// @Tags         household
// @Produce      json
// @Param        id   path      int  true  "Application ID"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/applications/{id}/reject [post]
func RejectHouseholdApplication(ctx *gin.Context, appCtx *AppContext) {
	applicationID, ok := parseUintParam(ctx, appCtx.Logger, "id", MsgInvalidApplicationId)
	if !ok {
		return
	}

	rejectErr := appCtx.Repos.Households.RejectApplication(applicationID, appCtx.UserID)
	switch rejectErr {
	case nil:
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Application rejected"})
	case apperrors.ErrApplicationNotFound:
		api.RespondError(ctx, http.StatusNotFound, rejectErr)
	case apperrors.ErrNotHouseholdAdmin:
		api.RespondError(ctx, http.StatusForbidden, rejectErr)
	default:
		appCtx.Logger.Error().Msgf("Error rejecting application: %s", rejectErr)
		api.RespondError(ctx, http.StatusInternalServerError, rejectErr)
	}
}

// UpdateHouseholdName renames the caller's household. Caller must be the household admin.
// @Summary      Rename household
// @Tags         household
// @Accept       json
// @Produce      json
// @Param        household  body      updateHouseholdNameRequest  true  "Name"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/name [patch]
func UpdateHouseholdName(ctx *gin.Context, appCtx *AppContext) {
	var req updateHouseholdNameRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		appCtx.Logger.Error().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		api.RespondError(ctx, http.StatusBadRequest, errors.New(MsgHouseholdNameEmpty))
		return
	}

	user, userErr := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if userErr != nil {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrInvalidUserData)
		return
	}

	updateErr := appCtx.Repos.Households.UpdateHouseholdName(user.HouseholdID, appCtx.UserID, req.Name)
	switch updateErr {
	case nil:
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Household name updated"})
	case apperrors.ErrHouseholdNotFound:
		api.RespondError(ctx, http.StatusNotFound, updateErr)
	case apperrors.ErrNotHouseholdAdmin:
		api.RespondError(ctx, http.StatusForbidden, updateErr)
	default:
		appCtx.Logger.Error().Msgf("Error updating household name: %s", updateErr)
		api.RespondError(ctx, http.StatusInternalServerError, updateErr)
	}
}

// CancelHouseholdApplication cancels a pending application submitted by the caller.
// @Summary      Cancel own household application
// @Tags         household
// @Produce      json
// @Param        id   path      int  true  "Application ID"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/applications/{id} [delete]
func CancelHouseholdApplication(ctx *gin.Context, appCtx *AppContext) {
	applicationID, ok := parseUintParam(ctx, appCtx.Logger, "id", MsgInvalidApplicationId)
	if !ok {
		return
	}

	cancelErr := appCtx.Repos.Households.CancelApplication(applicationID, appCtx.UserID)
	switch cancelErr {
	case nil:
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Application cancelled"})
	case apperrors.ErrApplicationNotFound:
		api.RespondError(ctx, http.StatusNotFound, cancelErr)
	case apperrors.ErrNotApplicationApplicant:
		api.RespondError(ctx, http.StatusForbidden, cancelErr)
	default:
		appCtx.Logger.Error().Msgf("Error cancelling application: %s", cancelErr)
		api.RespondError(ctx, http.StatusInternalServerError, cancelErr)
	}
}

// RemoveHouseholdMember removes a member from the caller's household. Caller must be the admin.
// @Summary      Remove a household member
// @Tags         household
// @Produce      json
// @Param        userId  path      int  true  "User ID to remove"
// @Success      200  {object}  api.APIResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      403  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/members/{userId} [delete]
func RemoveHouseholdMember(ctx *gin.Context, appCtx *AppContext) {
	memberID, ok := parseUintParam(ctx, appCtx.Logger, "userId", apperrors.ErrInvalidUserID.Error())
	if !ok {
		return
	}

removeErr := appCtx.Repos.Households.RemoveMemberFromHousehold(memberID, appCtx.UserID)
	switch removeErr {
	case nil:
		go recordMemberRemoved(ctx, appCtx.UserID, memberID)
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Member removed from household"})
	case apperrors.ErrNotHouseholdAdmin:
		api.RespondError(ctx, http.StatusForbidden, removeErr)
	case apperrors.ErrMemberNotInHousehold:
		api.RespondError(ctx, http.StatusNotFound, removeErr)
	case apperrors.ErrCannotRemoveAdmin:
		api.RespondError(ctx, http.StatusBadRequest, removeErr)
	default:
		appCtx.Logger.Error().Msgf("Error removing member: %s", removeErr)
		api.RespondError(ctx, http.StatusInternalServerError, removeErr)
	}
}

func parseUintParam(ctx *gin.Context, logger *zerolog.Logger, paramName, invalidMsg string) (uint, bool) {
	param := ctx.Param(paramName)
	val, err := strconv.ParseUint(param, 10, 64)
	if err != nil {
		logger.Warn().Msgf(apperrors.FormatInvalidRequestId, param)
		api.RespondError(ctx, http.StatusBadRequest, errors.New(invalidMsg))
		return 0, false
	}
	return uint(val), true
}

type createHouseholdRequest struct {
	Name string `json:"name"`
}

type updateHouseholdNameRequest struct {
	Name string `json:"name"`
}

func recordMemberLeft(ctx *gin.Context, userID uint, householdID uint) {
	reposVal, exists := ctx.Get(util.ContextKeyRepos)
	if !exists {
		return
	}
	repos, ok := reposVal.(*database.RepositoryContainer)
	if !ok {
		return
	}
	ipAddress := ""
	if ctx.Request != nil {
		ipAddress = ctx.Request.RemoteAddr
	}
	var requestIDStr string
	if requestID, ok := ctx.Get(util.ContextKeyRequestID); ok {
		requestIDStr, _ = requestID.(string)
	}
	auditLog := &dbModel.AuditLog{
		Timestamp: time.Now(),
		UserID:    &userID,
		Action:    dbModel.AuditActionMemberLeft,
		IPAddress: ipAddress,
		RequestID: requestIDStr,
		Details:   `{"household_id": ` + strconv.FormatUint(uint64(householdID), 10) + `}`,
	}
	if repos.AuditLogs == nil {
		return
	}
	_ = repos.AuditLogs.Create(context.Background(), auditLog)
}

func recordMemberAdded(ctx *gin.Context, adminID uint, applicationID uint) {
	reposVal, exists := ctx.Get(util.ContextKeyRepos)
	if !exists {
		return
	}
	repos, ok := reposVal.(*database.RepositoryContainer)
	if !ok {
		return
	}
	ipAddress := ""
	if ctx.Request != nil {
		ipAddress = ctx.Request.RemoteAddr
	}
	var requestIDStr string
	if requestID, ok := ctx.Get(util.ContextKeyRequestID); ok {
		requestIDStr, _ = requestID.(string)
	}
	auditLog := &dbModel.AuditLog{
		Timestamp: time.Now(),
		UserID:    &adminID,
		Action:    dbModel.AuditActionMemberAdded,
		IPAddress: ipAddress,
		RequestID: requestIDStr,
		Details:   `{"application_id": ` + strconv.FormatUint(uint64(applicationID), 10) + `}`,
	}
	if repos.AuditLogs == nil {
		return
	}
	_ = repos.AuditLogs.Create(context.Background(), auditLog)
}

func recordMemberRemoved(ctx *gin.Context, adminID uint, removedUserID uint) {
	reposVal, exists := ctx.Get(util.ContextKeyRepos)
	if !exists {
		return
	}
	repos, ok := reposVal.(*database.RepositoryContainer)
	if !ok {
		return
	}
	ipAddress := ""
	if ctx.Request != nil {
		ipAddress = ctx.Request.RemoteAddr
	}
	var requestIDStr string
	if requestID, ok := ctx.Get(util.ContextKeyRequestID); ok {
		requestIDStr, _ = requestID.(string)
	}
	auditLog := &dbModel.AuditLog{
		Timestamp: time.Now(),
		UserID:    &adminID,
		Action:    dbModel.AuditActionMemberRemoved,
		IPAddress: ipAddress,
		RequestID: requestIDStr,
		Details:   `{"removed_user_id": ` + strconv.FormatUint(uint64(removedUserID), 10) + `}`,
	}
	if repos.AuditLogs == nil {
		return
	}
	_ = repos.AuditLogs.Create(context.Background(), auditLog)
}
