package router

import (
	"net/http"
	"strconv"
	"sync"
	"time"

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
	loginLimiters  = &sync.Map{}
	signupLimiters = &sync.Map{}
	exportLimiters = &sync.Map{}

	loginRate  = rate.Limit(5.0 / 60.0)
	signupRate = rate.Limit(3.0 / 60.0)
	exportRate = rate.Limit(1.0 / 60.0)
)

func InitRateLimits(cfg configuration.RateLimitConfiguration) {
	loginRate = rate.Limit(float64(cfg.LoginPerMinute) / 60.0)
	signupRate = rate.Limit(float64(cfg.SignupPerMinute) / 60.0)
	exportRate = rate.Limit(float64(cfg.ExportPerMinute) / 60.0)
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
		ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"code":    codeRateLimitExceeded,
			"message": "Too many login attempts. Please try again later.",
		})
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
		ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"code":    codeRateLimitExceeded,
			"message": "Too many signup attempts. Please try again later.",
		})
		return
	}
	ctx.Next()
}

func exportRateLimitMiddleware(ctx *gin.Context) {
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"code":    "UNAUTHORIZED",
			"message": "Unauthorized",
		})
		return
	}

	key := strconv.FormatUint(uint64(userID), 10)
	limiter := getLimiter(exportLimiters, key, exportRate)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"code":    codeRateLimitExceeded,
			"message": "Too many export requests. Please try again later.",
		})
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
	}
}

func init() {
	go cleanupLimiters()
}
