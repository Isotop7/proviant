package v1

import (
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

// GetAuditLogs returns the audit log entries.
// @Summary      Get audit logs
// @Description  Returns paginated audit log entries for the caller's household. Household admins only; entries are scoped to the caller's household.
// @Tags         admin
// @Produce      json
// @Param        limit query int false "Maximum number of logs to return (default 100, max 1000)"
// @Param        date  query string false "Filter by date (YYYY-MM-DD format)"
// @Success      200  {array}  database.AuditLog
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/admin/audit-log [get]
func GetAuditLogs(ctx *gin.Context, appCtx *AppContext) {
	limit, ok := parseLimitParam(ctx, appCtx.Logger)
	if !ok {
		return
	}

	// Set by RequireHouseholdAdmin, which also rejects non-admins before this
	// handler runs. The repository filters on it, so no entry from another
	// household can be returned.
	householdIDValue, exists := ctx.Get(util.ContextKeyHouseholdID)
	if !exists {
		appCtx.Logger.Error().Msg("household id missing from context")
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to fetch audit logs"})
		return
	}
	householdID, ok := householdIDValue.(uint)
	if !ok || householdID == 0 {
		appCtx.Logger.Error().Msg("household id in context is not a valid uint")
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to fetch audit logs"})
		return
	}

	dateParam := ctx.Query("date")
	var logs []database.AuditLog
	var err error

	if dateParam != "" {
		// The repository parses this too, but a malformed date is client
		// input and must surface as 400, not as a parse-failure 500.
		if _, parseErr := time.Parse(util.DefaultDateFormatParseStr, dateParam); parseErr != nil {
			appCtx.Logger.Warn().Msgf("Invalid audit log date '%s'", dateParam)
			ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "date must be in YYYY-MM-DD format"})
			return
		}
		logs, err = appCtx.Repos.AuditLogs.GetAuditLogsByDate(ctx.Request.Context(), householdID, limit, dateParam)
	} else {
		logs, err = appCtx.Repos.AuditLogs.GetAuditLogs(ctx.Request.Context(), householdID, limit)
	}

	if err != nil {
		appCtx.Logger.Error().Err(err).Msg("failed to fetch audit logs")
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to fetch audit logs"})
		return
	}

	ctx.JSON(http.StatusOK, logs)
}
