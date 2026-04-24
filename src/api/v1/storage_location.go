package v1

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

type storageLocationRequest struct {
	Name      string `json:"name"      binding:"required"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sortOrder"`
}

// ListStorageLocations returns all storage locations for the calling user's household.
// @Summary      List storage locations
// @Description  Returns all storage locations belonging to the user's household, ordered by sort_order.
// @Tags         household
// @Produce      json
// @Success      200  {array}   database.StorageLocation
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/storage-locations [get]
func ListStorageLocations(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	storageLocationRepo := database.NewStorageLocationRepository(dbHandle)
	locs, err := storageLocationRepo.GetByHousehold(userID)
	if err != nil {
		logger.Error().Msgf("Error listing storage locations: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
		return
	}

	ctx.JSON(http.StatusOK, locs)
}

// CreateStorageLocation adds a new storage location to the calling user's household.
// @Summary      Create a storage location
// @Description  Creates a named storage location for the household.
// @Tags         household
// @Accept       json
// @Produce      json
// @Param        body  body      storageLocationRequest  true  "Location data"
// @Success      201   {object}  database.StorageLocation
// @Failure      400   {object}  api.APIResponse
// @Failure      500   {object}  api.APIResponse
// @Router       /api/v1/household/storage-locations [post]
func CreateStorageLocation(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	var req storageLocationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf("%s: %s", errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	icon := req.Icon
	if icon == "" {
		icon = "📦"
	}

	storageLocationRepo := database.NewStorageLocationRepository(dbHandle)
	loc, err := storageLocationRepo.Create(userID, req.Name, icon, req.SortOrder)
	if err != nil {
		logger.Error().Msgf("Error creating storage location: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
		return
	}

	ctx.JSON(http.StatusCreated, loc)
}

// UpdateStorageLocation renames or re-icons a storage location.
// @Summary      Update a storage location
// @Description  Updates the name, icon, and sort order of an existing storage location.
// @Tags         household
// @Accept       json
// @Produce      json
// @Param        id    path      int                     true  "Location ID"
// @Param        body  body      storageLocationRequest  true  "Location data"
// @Success      200   {object}  database.StorageLocation
// @Failure      400   {object}  api.APIResponse
// @Failure      404   {object}  api.APIResponse
// @Failure      500   {object}  api.APIResponse
// @Router       /api/v1/household/storage-locations/:id [patch]
func UpdateStorageLocation(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	locationID, ok := parseUintParam(ctx, logger, "id", "location ID")
	if !ok {
		return
	}

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	var req storageLocationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		logger.Error().Msgf("%s: %s", errors.ErrParseBody.Error(), err.Error())
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: err.Error()})
		return
	}

	icon := req.Icon
	if icon == "" {
		icon = "📦"
	}

	storageLocationRepo := database.NewStorageLocationRepository(dbHandle)
	loc, err := storageLocationRepo.Update(locationID, userID, req.Name, icon, req.SortOrder)
	if err != nil {
		if err == errors.ErrStorageLocationNotFound {
			ctx.JSON(http.StatusNotFound, api.Error(err))
			return
		}
		if err == errors.ErrStorageLocationNotOwned {
			ctx.JSON(http.StatusForbidden, api.Error(err))
			return
		}
		logger.Error().Msgf("Error updating storage location: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
		return
	}

	ctx.JSON(http.StatusOK, loc)
}

// DeleteStorageLocation removes a storage location. Assigned products become unassigned.
// @Summary      Delete a storage location
// @Description  Deletes a storage location and unassigns all products from it.
// @Tags         household
// @Produce      json
// @Param        id   path      int  true  "Location ID"
// @Success      200  {object}  api.APIResponse
// @Failure      404  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/household/storage-locations/:id [delete]
func DeleteStorageLocation(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	locationID, ok := parseUintParam(ctx, logger, "id", "location ID")
	if !ok {
		return
	}

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	storageLocationRepo := database.NewStorageLocationRepository(dbHandle)
	if err := storageLocationRepo.Delete(locationID, userID); err != nil {
		if err == errors.ErrStorageLocationNotFound {
			ctx.JSON(http.StatusNotFound, api.Error(err))
			return
		}
		if err == errors.ErrStorageLocationNotOwned {
			ctx.JSON(http.StatusForbidden, api.Error(err))
			return
		}
		logger.Error().Msgf("Error deleting storage location: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Storage location deleted"})
}
