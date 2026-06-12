package router

import (
	"errors"
	"net/http"
	"strconv"
	"sync"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const (
	codeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
	headerRetryAfter      = "Retry-After"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	loginLimiters    = &sync.Map{}
	signupLimiters   = &sync.Map{}
	exportLimiters   = &sync.Map{}
	passwordLimiters = &sync.Map{}
	scanLimiters     = &sync.Map{}

	loginRate    = rate.Limit(5.0 / 60.0)
	signupRate   = rate.Limit(3.0 / 60.0)
	exportRate   = rate.Limit(1.0 / 60.0)
	passwordRate = rate.Limit(3.0 / 60.0)
	scanRate     = rate.Limit(10.0 / 60.0)
)

func InitRateLimits(cfg configuration.RateLimitConfiguration) {
	loginRate = rate.Limit(float64(cfg.LoginPerMinute) / 60.0)
	signupRate = rate.Limit(float64(cfg.SignupPerMinute) / 60.0)
	exportRate = rate.Limit(float64(cfg.ExportPerMinute) / 60.0)
	passwordRate = rate.Limit(float64(cfg.PasswordPerMinute) / 60.0)
	scanRate = rate.Limit(float64(cfg.ScanPerMinute) / 60.0)
}

func getLimiter(store *sync.Map, key string, limit rate.Limit) *rate.Limiter {
	now := time.Now()
	if storedValue, ok := store.Load(key); ok {
		storedLimiter := storedValue.(*clientLimiter)
		storedLimiter.lastSeen = now
		return storedLimiter.limiter
	}
	limiter := rate.NewLimiter(limit, int(limit*60)+1)
	store.Store(key, &clientLimiter{limiter: limiter, lastSeen: now})
	return limiter
}

func loginRateLimitMiddleware(ctx *gin.Context) {
	clientIP := ctx.ClientIP()
	limiter := getLimiter(loginLimiters, clientIP, loginRate)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many login attempts, please try again later"))
		return
	}
	ctx.Next()
}

func signupRateLimitMiddleware(ctx *gin.Context) {
	clientIP := ctx.ClientIP()
	limiter := getLimiter(signupLimiters, clientIP, signupRate)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many signup attempts, please try again later"))
		return
	}
	ctx.Next()
}

func exportRateLimitMiddleware(ctx *gin.Context) {
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		api.RespondError(ctx, http.StatusUnauthorized, errors.New("Unauthorized"))
		return
	}

	key := strconv.FormatUint(uint64(userID), 10)
	limiter := getLimiter(exportLimiters, key, exportRate)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many export requests, please try again later"))
		return
	}
	ctx.Next()
}

// passwordRateLimitMiddleware rate-limits password-change traffic. When a
// JWT identity is present (authenticated user changing their own password)
// it keys on user ID; otherwise it falls back to client IP for the public
// forgot-password / reset-password routes.
func passwordRateLimitMiddleware(ctx *gin.Context) {
	key := ctx.ClientIP()
	claims := jwt.ExtractClaims(ctx)
	if claims != nil {
		if raw, ok := claims[static.TokenIdentityKey].(float64); ok && raw > 0 {
			key = strconv.FormatUint(uint64(uint(raw)), 10)
		}
	}
	limiter := getLimiter(passwordLimiters, key, passwordRate)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many password change attempts, please try again later"))
		return
	}
	ctx.Next()
}

// publicPasswordRateLimitMiddleware is the IP-only variant used on the
// unauthenticated POST /auth/forgot-password and /auth/reset-password
// routes. It is separate from passwordRateLimitMiddleware so that an
// attacker spraying public reset attempts from a shared egress IP cannot
// lock out legitimate authenticated users on POST /api/v1/user/password.
func publicPasswordRateLimitMiddleware(ctx *gin.Context) {
	key := ctx.ClientIP()
	limiter := getLimiter(passwordLimiters, key, passwordRate)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many password reset attempts, please try again later"))
		return
	}
	ctx.Next()
}

func scanRateLimitMiddleware(ctx *gin.Context) {
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		api.RespondError(ctx, http.StatusUnauthorized, errors.New("Unauthorized"))
		return
	}

	key := strconv.FormatUint(uint64(userID), 10)
	limiter := getLimiter(scanLimiters, key, scanRate)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many scan requests, please try again later"))
		return
	}
	ctx.Next()
}

func cleanupLimiters() {
	for {
		time.Sleep(time.Minute)
		now := time.Now()
		loginLimiters.Range(func(key, value interface{}) bool {
			storedLimiter := value.(*clientLimiter)
			if now.Sub(storedLimiter.lastSeen) > 10*time.Minute {
				loginLimiters.Delete(key)
			}
			return true
		})
		signupLimiters.Range(func(key, value interface{}) bool {
			storedLimiter := value.(*clientLimiter)
			if now.Sub(storedLimiter.lastSeen) > 10*time.Minute {
				signupLimiters.Delete(key)
			}
			return true
		})
		exportLimiters.Range(func(key, value interface{}) bool {
			storedLimiter := value.(*clientLimiter)
			if now.Sub(storedLimiter.lastSeen) > 10*time.Minute {
				exportLimiters.Delete(key)
			}
			return true
		})
		passwordLimiters.Range(func(key, value any) bool {
			storedLimiter := value.(*clientLimiter)
			if now.Sub(storedLimiter.lastSeen) > 10*time.Minute {
				passwordLimiters.Delete(key)
			}
			return true
		})
		scanLimiters.Range(func(key, value any) bool {
			storedLimiter := value.(*clientLimiter)
			if now.Sub(storedLimiter.lastSeen) > 10*time.Minute {
				scanLimiters.Delete(key)
			}
			return true
		})
	}
}

func init() {
	go cleanupLimiters()
}
