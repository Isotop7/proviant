package v1

import (
	"net/http"
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
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
func LeaveHousehold(ctx *gin.Context) {
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
	if err := dbController.LeaveHousehold(userID); err != nil {
		logger.Error().Msgf("Error leaving household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
		return
	}

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
func CreateHousehold(ctx *gin.Context) {
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

	var req createHouseholdRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf(errors.FormatGenericError, errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(err))
		return
	}
	if req.Name == "" {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "household name cannot be empty"})
		return
	}

	dbController := database.DatabaseController{DBHandle: dbHandle}
	if err := dbController.CreateAndSwitchHousehold(userID, req.Name); err != nil {
		logger.Error().Msgf("Error creating household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
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
func ApplyForHousehold(ctx *gin.Context) {
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

	idParam := ctx.Param("id")
	householdID, convErr := strconv.ParseUint(idParam, 10, 64)
	if convErr != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "invalid household id"})
		return
	}

	dbController := database.DatabaseController{DBHandle: dbHandle}
	applyErr := dbController.ApplyForHousehold(userID, uint(householdID))
	switch applyErr {
	case nil:
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Application submitted"})
	case errors.ErrHouseholdNotFound:
		ctx.JSON(http.StatusNotFound, api.Error(applyErr))
	case errors.ErrApplicationAlreadyPending:
		ctx.JSON(http.StatusConflict, api.Error(applyErr))
	default:
		logger.Error().Msgf("Error applying for household: %s", applyErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(applyErr))
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
func GetHouseholdApplications(ctx *gin.Context) {
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
	applications, err := dbController.GetPendingApplicationsForAdmin(userID)
	switch err {
	case nil:
		ctx.JSON(http.StatusOK, applications)
	case errors.ErrNotHouseholdAdmin:
		ctx.JSON(http.StatusForbidden, api.Error(err))
	default:
		logger.Error().Msgf("Error fetching applications: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
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
func ApproveHouseholdApplication(ctx *gin.Context) {
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

	idParam := ctx.Param("id")
	applicationID, convErr := strconv.ParseUint(idParam, 10, 64)
	if convErr != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "invalid application id"})
		return
	}

	dbController := database.DatabaseController{DBHandle: dbHandle}
	approveErr := dbController.ApproveApplication(uint(applicationID), userID)
	switch approveErr {
	case nil:
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Application approved"})
	case errors.ErrApplicationNotFound:
		ctx.JSON(http.StatusNotFound, api.Error(approveErr))
	case errors.ErrNotHouseholdAdmin:
		ctx.JSON(http.StatusForbidden, api.Error(approveErr))
	default:
		logger.Error().Msgf("Error approving application: %s", approveErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(approveErr))
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
func RejectHouseholdApplication(ctx *gin.Context) {
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

	idParam := ctx.Param("id")
	applicationID, convErr := strconv.ParseUint(idParam, 10, 64)
	if convErr != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "invalid application id"})
		return
	}

	dbController := database.DatabaseController{DBHandle: dbHandle}
	rejectErr := dbController.RejectApplication(uint(applicationID), userID)
	switch rejectErr {
	case nil:
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "Application rejected"})
	case errors.ErrApplicationNotFound:
		ctx.JSON(http.StatusNotFound, api.Error(rejectErr))
	case errors.ErrNotHouseholdAdmin:
		ctx.JSON(http.StatusForbidden, api.Error(rejectErr))
	default:
		logger.Error().Msgf("Error rejecting application: %s", rejectErr)
		ctx.JSON(http.StatusInternalServerError, api.Error(rejectErr))
	}
}

type createHouseholdRequest struct {
	Name string `json:"name"`
}
