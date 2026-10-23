package v1

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/Isotop7/proviant/api"
	apperrors "github.com/Isotop7/proviant/errors"

	"github.com/gin-gonic/gin"
)

const (
	// dashboardLayoutVersion is the layout format the server understands. A
	// client that speaks a newer format must not have its layout silently
	// reshaped, so the version is checked instead of ignored.
	dashboardLayoutVersion = 1

	// dashboardLayoutMaxBytes caps the stored layout. A layout holds one small
	// entry per tile, so anything above this is not a dashboard layout.
	dashboardLayoutMaxBytes = 16 * 1024

	// dashboardLayoutMaxTiles caps the number of tiles a layout may describe.
	dashboardLayoutMaxTiles = 64

	// dashboardLayoutMaxOrder caps the position of a tile in the arrangement.
	dashboardLayoutMaxOrder = 256

	// dashboardLayoutMaxColumns and dashboardLayoutMaxRows bound the span of a
	// tile to what the bento grid can show.
	dashboardLayoutMaxColumns = 6
	dashboardLayoutMaxRows    = 12

	// dashboardLayoutMaxTileIDLength bounds a tile id.
	dashboardLayoutMaxTileIDLength = 64
)

// dashboardLayoutTileIDPattern restricts a tile id to the characters the
// frontend uses for its tile names.
var dashboardLayoutTileIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// dashboardLayoutTile is the arrangement of a single tile: its position in the
// grid and how many columns and rows it spans.
type dashboardLayoutTile struct {
	Order int `json:"order"`
	W     int `json:"w"`
	H     int `json:"h"`
}

// dashboardLayout is the arrangement of every tile on the dashboard.
type dashboardLayout struct {
	Version int                            `json:"version"`
	Tiles   map[string]dashboardLayoutTile `json:"tiles"`
}

// validate rejects a layout the frontend could not have produced.
func (l dashboardLayout) validate() error {
	if l.Version != dashboardLayoutVersion {
		return apperrors.ErrDashboardLayoutInvalid
	}
	if len(l.Tiles) > dashboardLayoutMaxTiles {
		return apperrors.ErrDashboardLayoutInvalid
	}
	for id, tile := range l.Tiles {
		if id == "" || len(id) > dashboardLayoutMaxTileIDLength || !dashboardLayoutTileIDPattern.MatchString(id) {
			return apperrors.ErrDashboardLayoutInvalid
		}
		if tile.Order < 0 || tile.Order >= dashboardLayoutMaxOrder {
			return apperrors.ErrDashboardLayoutInvalid
		}
		if tile.W < 1 || tile.W > dashboardLayoutMaxColumns {
			return apperrors.ErrDashboardLayoutInvalid
		}
		if tile.H < 1 || tile.H > dashboardLayoutMaxRows {
			return apperrors.ErrDashboardLayoutInvalid
		}
	}
	return nil
}

// emptyDashboardLayout is the layout a user without a stored arrangement gets:
// the default arrangement, spelled out as an empty tile map.
func emptyDashboardLayout() dashboardLayout {
	return dashboardLayout{Version: dashboardLayoutVersion, Tiles: map[string]dashboardLayoutTile{}}
}

// GetUserDashboardLayout returns the stored arrangement of the dashboard tiles.
// @Summary			Gets the current user's dashboard layout
// @Description		Returns the stored arrangement of the dashboard tiles: the position and the column and row span of every tile. An empty tile map means the default arrangement.
// @Tags				user
// @Accept			json
// @Produce			json
// @Success			200  {object}  dashboardLayout
// @Failure			500  {object}  api.APIResponse
// @Router			/api/v1/user/dashboard-layout [get]
func GetUserDashboardLayout(ctx *gin.Context, appCtx *AppContext) {
	user, getErr := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if getErr != nil {
		appCtx.Logger.Error().Msgf("Error getting user: %s", getErr)
		api.RespondError(ctx, http.StatusInternalServerError, getErr)
		return
	}

	layout := emptyDashboardLayout()
	if user.DashboardLayout != "" {
		if unmarshalErr := json.Unmarshal([]byte(user.DashboardLayout), &layout); unmarshalErr != nil {
			appCtx.Logger.Warn().Msgf("Stored dashboard layout is unreadable, falling back to the default: %s", unmarshalErr)
			layout = emptyDashboardLayout()
		}
	}
	if layout.Tiles == nil {
		layout.Tiles = map[string]dashboardLayoutTile{}
	}

	ctx.JSON(http.StatusOK, layout)
}

// UpdateUserDashboardLayout stores the arrangement of the dashboard tiles.
// @Summary			Updates the current user's dashboard layout
// @Description		Stores the arrangement of the dashboard tiles: the position and the column and row span of every tile. An empty tile map resets the dashboard to the default arrangement.
// @Tags				user
// @Accept			json
// @Produce			json
// @Param			layout  body      dashboardLayout  true  "Dashboard layout"
// @Success			200  {object}  api.APIResponse
// @Failure			400  {object}  api.APIResponse
// @Failure			500  {object}  api.APIResponse
// @Router			/api/v1/user/dashboard-layout [put]
func UpdateUserDashboardLayout(ctx *gin.Context, appCtx *AppContext) {
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, dashboardLayoutMaxBytes)

	var layout dashboardLayout
	if bindErr := ctx.ShouldBindJSON(&layout); bindErr != nil {
		appCtx.Logger.Warn().Msgf("%s: %s", apperrors.ErrDashboardLayoutInvalid.Error(), bindErr.Error())
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrDashboardLayoutInvalid)
		return
	}

	if layout.Tiles == nil {
		layout.Tiles = map[string]dashboardLayoutTile{}
	}
	if validateErr := layout.validate(); validateErr != nil {
		appCtx.Logger.Warn().Msgf("%s: %s", apperrors.ErrDashboardLayoutInvalid.Error(), validateErr.Error())
		api.RespondError(ctx, http.StatusBadRequest, validateErr)
		return
	}

	encoded, marshalErr := json.Marshal(layout)
	if marshalErr != nil {
		appCtx.Logger.Error().Msgf("Error encoding dashboard layout: %s", marshalErr)
		api.RespondError(ctx, http.StatusInternalServerError, marshalErr)
		return
	}

	if updateErr := appCtx.Repos.Users.UpdateUserDashboardLayout(appCtx.UserID, string(encoded)); updateErr != nil {
		appCtx.Logger.Error().Msgf("Error saving dashboard layout: %s", updateErr)
		api.RespondError(ctx, http.StatusInternalServerError, updateErr)
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Dashboard layout updated"})
}
