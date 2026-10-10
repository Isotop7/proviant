// common implements non-specifc handlers
package common

import (
	"context"
	"net/http"
	"sync"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

const readinessPingTimeout = 2 * time.Second

// readinessCacheTTL bounds how stale a readiness answer may be. It must stay
// well under the probe interval, or a database that dies between probes would
// not be noticed until the cache aged out.
const readinessCacheTTL = 5 * time.Second

// readinessCache memoizes the last database ping. The endpoint is
// unauthenticated, so probing it must stay cheap regardless of request volume:
// an overflow that answered 503 would let a flood of requests make the
// orchestrator's own probe fail and pull a healthy instance out of rotation.
type readinessCache struct {
	mutex     sync.Mutex
	ready     bool
	checkedAt time.Time
}

// fresh reports whether the cached answer may still be served. Callers must
// hold mutex.
func (c *readinessCache) fresh() bool {
	return !c.checkedAt.IsZero() && time.Since(c.checkedAt) < readinessCacheTTL
}

// readinessResults is shared by every request: concurrent probes read one
// answer instead of each opening a database connection.
var readinessResults readinessCache

// GetHealth returns the health status of the API
// @Summary      	Gets health
// @Description  	Gets health status of the API
// @Tags         	common
// @Accept			json
// @Produce      	json
// @Success      	200  {object}  api.APIResponse
// @Router       	/health [get]
func GetHealth(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "ok"})
}

// GetReadiness returns 200 when the database answers a ping, 503 otherwise.
// Kept separate from GetHealth because a failing liveness probe means "restart
// me" while a failing readiness probe means "stop routing traffic here".
// @Summary      	Readiness probe
// @Description  	Pings the database and returns 200 when it is reachable, 503 otherwise
// @Tags         	common
// @Accept			json
// @Produce      	json
// @Success      	200  {object}  api.APIResponse
// @Failure      	503  {object}  api.APIResponse
// @Router       	/health/ready [get]
func GetReadiness(ctx *gin.Context) {
	getReadiness(ctx, &readinessResults)
}

func getReadiness(ctx *gin.Context, cache *readinessCache) {
	logger := zerolog.Nop()
	if value, ok := ctx.Get(util.ContextKeyLogger); ok {
		if typed, ok := value.(*zerolog.Logger); ok && typed != nil {
			logger = *typed
		}
	}

	dbHandle, dbOk := ctx.Get(util.ContextKeyDBHandle)
	database, ok := dbHandle.(*gorm.DB)
	if !dbOk || !ok || database == nil {
		logger.Error().Msg(errors.ErrDatabaseContextNotFound.Error())
		api.RespondError(ctx, http.StatusServiceUnavailable, errors.ErrServiceNotReady)
		return
	}

	cache.mutex.Lock()
	defer cache.mutex.Unlock()

	// Serve from cache when fresh: this is what keeps a probe flood from
	// becoming a flood of database pings. The mutex serializes whoever misses,
	// so a burst costs one ping rather than one per request.
	if cache.fresh() {
		if cache.ready {
			ctx.JSON(http.StatusOK, api.APIResponse{Message: "ready"})
		} else {
			api.RespondError(ctx, http.StatusServiceUnavailable, errors.ErrServiceNotReady)
		}
		return
	}

	pingContext, cancel := context.WithTimeout(ctx.Request.Context(), readinessPingTimeout)
	defer cancel()

	sqlDB, err := database.DB()
	if err == nil {
		err = sqlDB.PingContext(pingContext)
	}
	cache.ready = err == nil
	cache.checkedAt = time.Now()

	if err != nil {
		// A client-side probe timeout cancels the request context: the caller
		// gave up, which is not the database failing. Logging it at Error would
		// turn an ordinary 1s kubelet timeout into a self-inflicted outage
		// signal from a process whose database is merely slow.
		if ctx.Request.Context().Err() != nil {
			logger.Warn().Err(err).Msg("readiness probe abandoned by the client")
		} else {
			logger.Error().Err(err).Msg("readiness probe database ping failed")
		}
		api.RespondError(ctx, http.StatusServiceUnavailable, errors.ErrServiceNotReady)
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "ready"})
}
