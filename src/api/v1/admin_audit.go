package v1

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// GetAuditLogs returns the audit log entries.
// @Summary      Get audit logs
// @Description  Returns paginated audit log entries (admin only)
// @Tags         admin
// @Produce      json
// @Param        limit query int false "Maximum number of logs to return (default 100, max 1000)"
// @Param        date  query string false "Filter by date (YYYY-MM-DD format)"
// @Success      200  {array}  database.AuditLog
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/admin/audit-log [get]
func GetAuditLogs(ctx *gin.Context) {
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	limit, ok := parseLimitParam(ctx, logger)
	if !ok {
		return
	}

	dateParam := ctx.Query("date")
	var logs []database.AuditLog
	var err error

	if dateParam != "" {
		logs, err = repos.AuditLogs.GetAuditLogsByDate(ctx.Request.Context(), limit, dateParam)
	} else {
		logs, err = repos.AuditLogs.GetAuditLogs(ctx.Request.Context(), limit)
	}

	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch audit logs")
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Failed to fetch audit logs"})
		return
	}

	ctx.JSON(http.StatusOK, logs)
}
