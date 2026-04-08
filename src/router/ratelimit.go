package router

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var (
	loginLimiters  = &sync.Map{}
	signupLimiters = &sync.Map{}

	loginRate  = rate.Limit(5.0 / 60.0)
	signupRate = rate.Limit(3.0 / 60.0)
)

func getClientIP(ctx *gin.Context) string {
	ip := ctx.GetHeader("X-Forwarded-For")
	if ip == "" {
		ip = ctx.GetHeader("X-Real-IP")
	}
	if ip == "" {
		ip = ctx.ClientIP()
	}
	return ip
}

func getLimiter(store *sync.Map, key string, r rate.Limit) *rate.Limiter {
	now := time.Now()
	if v, ok := store.Load(key); ok {
		cl := v.(*clientLimiter)
		cl.lastSeen = now
		return cl.limiter
	}
	limiter := rate.NewLimiter(r, int(r*60)+1)
	store.Store(key, &clientLimiter{limiter: limiter, lastSeen: now})
	return limiter
}

func loginRateLimitMiddleware(ctx *gin.Context) {
	ip := getClientIP(ctx)
	limiter := getLimiter(loginLimiters, ip, loginRate)
	if !limiter.Allow() {
		retryAfter := strconv.Itoa(int(time.Until(time.Now().Add(time.Minute)).Seconds()))
		ctx.Header("Retry-After", retryAfter)
		ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"code":    "RATE_LIMIT_EXCEEDED",
			"message": "Too many login attempts. Please try again later.",
		})
		return
	}
	ctx.Next()
}

func signupRateLimitMiddleware(ctx *gin.Context) {
	ip := getClientIP(ctx)
	limiter := getLimiter(signupLimiters, ip, signupRate)
	if !limiter.Allow() {
		retryAfter := strconv.Itoa(int(time.Until(time.Now().Add(time.Minute)).Seconds()))
		ctx.Header("Retry-After", retryAfter)
		ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"code":    "RATE_LIMIT_EXCEEDED",
			"message": "Too many signup attempts. Please try again later.",
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
			cl := value.(*clientLimiter)
			if now.Sub(cl.lastSeen) > 10*time.Minute {
				loginLimiters.Delete(key)
			}
			return true
		})
		signupLimiters.Range(func(key, value interface{}) bool {
			cl := value.(*clientLimiter)
			if now.Sub(cl.lastSeen) > 10*time.Minute {
				signupLimiters.Delete(key)
			}
			return true
		})
	}
}

func init() {
	go cleanupLimiters()
}
