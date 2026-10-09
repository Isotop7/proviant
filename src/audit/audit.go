// Package audit extracts plain audit-log values from a gin.Context while the
// request handler is still running. Goroutines spawned after the handler
// returned must not touch gin.Context (unsafe for post-handler use) — pass
// Values instead.
package audit

import (
	"context"
	"encoding/json"
	"time"

	"codeberg.org/isotop7/proviant/controllers/database"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// Values holds plain copies of everything an audit log entry needs.
type Values struct {
	Repos     *database.RepositoryContainer
	Logger    *zerolog.Logger
	IPAddress string
	RequestID string
}

// FromGin extracts Values synchronously from the request context. Call it only
// while the handler runs — before spawning goroutines. In a `go` statement the
// arguments (including the FromGin call) are evaluated in the calling
// goroutine, so `go recordX(audit.FromGin(ctx), ...)` is safe.
func FromGin(ctx *gin.Context) Values {
	var values Values
	reposVal, exists := ctx.Get(util.ContextKeyRepos)
	if exists {
		values.Repos, _ = reposVal.(*database.RepositoryContainer)
	}
	if ctx.Request != nil {
		values.IPAddress = ctx.Request.RemoteAddr
	}
	if requestID, ok := ctx.Get(util.ContextKeyRequestID); ok {
		values.RequestID, _ = requestID.(string)
	}
	if logger, ok := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger); ok {
		values.Logger = logger
	}
	return values
}

// Log writes one audit log entry. Details is JSON-marshalled (must be a
// JSON-serialisable value); a marshal or write failure is logged when a logger
// is available. No-op when repos are unavailable.
func (v Values) Log(userID uint, action string, details any) {
	if v.Repos == nil || v.Repos.AuditLogs == nil {
		return
	}
	detailsJSON, err := json.Marshal(details)
	if err != nil {
		detailsJSON = nil
		if v.Logger != nil {
			v.Logger.Warn().Err(err).Str("action", action).Msg("audit: failed to marshal details")
		}
	}
	if createErr := v.Repos.AuditLogs.Create(context.Background(), &dbModel.AuditLog{
		Timestamp: time.Now(),
		UserID:    &userID,
		Action:    action,
		IPAddress: v.IPAddress,
		RequestID: v.RequestID,
		Details:   string(detailsJSON),
	}); createErr != nil && v.Logger != nil {
		v.Logger.Warn().Err(createErr).Str("action", action).Msg("audit: failed to write entry")
	}
}
