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
	"codeberg.org/isotop7/proviant/util"
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
	mu       sync.Mutex
	lastSeen time.Time
}

// touch records that the limiter was just used. lastSeen is written by every
// request goroutine and read by cleanupLimiters, so all access is guarded.
func (cl *clientLimiter) touch(now time.Time) {
	cl.mu.Lock()
	cl.lastSeen = now
	cl.mu.Unlock()
}

// idleFor reports how long the limiter has gone unused.
func (cl *clientLimiter) idleFor(now time.Time) time.Duration {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return now.Sub(cl.lastSeen)
}

var (
	loginLimiters    = &sync.Map{}
	signupLimiters   = &sync.Map{}
	exportLimiters   = &sync.Map{}
	passwordLimiters = &sync.Map{}
	scanLimiters     = &sync.Map{}
	recipesLimiters  = &sync.Map{}
	importLimiters   = &sync.Map{}
	bulkLimiters     = &sync.Map{}
	receiptLimiters  = &sync.Map{}

	loginRate    = rate.Limit(5.0 / 60.0)
	signupRate   = rate.Limit(3.0 / 60.0)
	exportRate   = rate.Limit(1.0 / 60.0)
	passwordRate = rate.Limit(3.0 / 60.0)
	scanRate     = rate.Limit(10.0 / 60.0)
	recipesRate  = rate.Limit(6.0 / 60.0)
	importRate   = rate.Limit(5.0 / 60.0)
	bulkRate     = rate.Limit(5.0 / 60.0)
	receiptRate  = rate.Limit(5.0 / 60.0)

	// importBurstCap bounds how many CSV imports one user can start at once.
	// importRate is deliberately not config-driven: a config key that defaults
	// to zero would make ValidateServerConfiguration reject every existing
	// config.yaml that predates the key. Five imports a minute with a burst of
	// one matches how often the feature is actually used and keeps the per-call
	// cost — up to util.CsvImportMaxRows inserts plus the bounded Open Food
	// Facts lookups — from being repeatable back to back.
	importBurstCap = 1

	// bulkBurstCap mirrors importBurstCap for POST /api/v1/products/bulk: one
	// call inserts up to util.ReceiptBulkMaxItems products, so back-to-back
	// bulk creates are not a pattern the review UI produces and the limiter
	// exists to stop them. bulkRate stays a package constant, same reasoning
	// as importRate — a config key defaulting to zero would break startup
	// validation for existing deployments.
	bulkBurstCap = 1

	// receiptBurstCap mirrors bulkBurstCap for POST /api/v1/products/scan-receipt.
	// Each call sends the photo to an external vision endpoint (multi-second,
	// potentially paid), so it gets its own limiter instead of sharing the
	// scanRate budget with the cheap local expiry-date scans. receiptRate
	// stays a package constant, same reasoning as importRate — a config key
	// defaulting to zero would break startup validation for existing
	// deployments.
	receiptBurstCap = 1

	// recipesBurstCap stays far below the minute's worth of tokens the default
	// burst would hand out: one suggestions request fans out into up to 10
	// provider searches plus 20 detail lookups, so the limiter exists to bound
	// that fan-out rather than to police one request at a time.
	//
	// It also has to cover the request pattern the recipes page generates on
	// its own. The page fires one request on load, and cookMatchedProducts
	// reloads suggestions after every successful cook — and that reload passes
	// refresh=1, so it is a guaranteed cache miss and a full fan-out rather
	// than a cheap read. With a burst of 2 the tokens were gone after the
	// first cook and the second one 429'd. The cap of 4 covers load plus three
	// cooks.
	recipesBurstCap = 4

	// recipesBurst is min(recipesBurstCap, RecipesPerMinute), resolved in
	// InitRateLimits. It is not the cap alone, because a fixed cap leaves
	// recipes_per_minute unable to govern the endpoint in either direction:
	// `recipes_per_minute: 1` would still admit 4 back-to-back fan-outs, i.e.
	// 4x its configured quota in the first instant, while a large value would
	// stay pinned at 4 and 429 on the fifth rapid request. ValidateServerConfiguration
	// already rejects a non-positive RecipesPerMinute, so the floor of 1 here is
	// only a guard against the default value preceding that validation.
	recipesBurst = recipesBurstCap
)

func InitRateLimits(cfg configuration.RateLimitConfiguration) {
	loginRate = rate.Limit(float64(cfg.LoginPerMinute) / 60.0)
	signupRate = rate.Limit(float64(cfg.SignupPerMinute) / 60.0)
	exportRate = rate.Limit(float64(cfg.ExportPerMinute) / 60.0)
	passwordRate = rate.Limit(float64(cfg.PasswordPerMinute) / 60.0)
	scanRate = rate.Limit(float64(cfg.ScanPerMinute) / 60.0)
	recipesRate = rate.Limit(float64(cfg.RecipesPerMinute) / 60.0)
	recipesBurst = min(max(cfg.RecipesPerMinute, 1), recipesBurstCap)
}

func getLimiter(store *sync.Map, key string, limit rate.Limit) *rate.Limiter {
	return getLimiterBurst(store, key, limit, int(limit*60)+1)
}

// getLimiterBurst returns the limiter for key, creating it with the given burst
// on first use. Endpoints whose single request fans out into many outbound
// calls must pass an explicit small burst: the default int(limit*60)+1 hands
// out a full minute of tokens at once, which for a fan-out endpoint means that
// many simultaneous upstream bursts.
func getLimiterBurst(store *sync.Map, key string, limit rate.Limit, burst int) *rate.Limiter {
	now := time.Now()
	if storedValue, ok := store.Load(key); ok {
		storedLimiter := storedValue.(*clientLimiter)
		storedLimiter.touch(now)
		return storedLimiter.limiter
	}
	// LoadOrStore so concurrent first requests for the same key cannot
	// create duplicate limiters: only one goroutine's limiter is stored,
	// and the losers discard theirs and reuse the winner's.
	newLimiter := &clientLimiter{limiter: rate.NewLimiter(limit, burst), lastSeen: now}
	storedValue, loaded := store.LoadOrStore(key, newLimiter)
	if !loaded {
		return newLimiter.limiter
	}
	storedLimiter := storedValue.(*clientLimiter)
	storedLimiter.touch(now)
	return storedLimiter.limiter
}

func loginRateLimitMiddleware(ctx *gin.Context) {
	clientIP := ctx.ClientIP()
	limiter := getLimiter(loginLimiters, clientIP, loginRate)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many login attempts, please try again later"))
		ctx.Abort()
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
		ctx.Abort()
		return
	}
	ctx.Next()
}

func exportRateLimitMiddleware(ctx *gin.Context) {
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		api.RespondError(ctx, http.StatusUnauthorized, errors.New("Unauthorized"))
		ctx.Abort()
		return
	}

	key := strconv.FormatUint(uint64(userID), 10)
	limiter := getLimiter(exportLimiters, key, exportRate)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many export requests, please try again later"))
		ctx.Abort()
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
		ctx.Abort()
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
		ctx.Abort()
		return
	}
	ctx.Next()
}

func scanRateLimitMiddleware(ctx *gin.Context) {
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		api.RespondError(ctx, http.StatusUnauthorized, errors.New("Unauthorized"))
		ctx.Abort()
		return
	}

	key := strconv.FormatUint(uint64(userID), 10)
	limiter := getLimiter(scanLimiters, key, scanRate)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many scan requests, please try again later"))
		ctx.Abort()
		return
	}
	ctx.Next()
}

// recipesRateLimitMiddleware rate-limits recipe suggestion traffic per user.
// A single suggestions request can fan out to many outbound provider calls
// (especially with refresh=1), so this protects the configured recipe backend.
// Both the JWT and the PAT auth path set util.ContextKeyUserID for
// authenticated requests, so one key source covers both; requests without a
// resolvable user fall back to the client IP.
func recipesRateLimitMiddleware(ctx *gin.Context) {
	key := ctx.ClientIP()
	if raw, ok := ctx.Get(util.ContextKeyUserID); ok {
		if userID, ok := raw.(uint); ok && userID > 0 {
			key = strconv.FormatUint(uint64(userID), 10)
		}
	}

	limiter := getLimiterBurst(recipesLimiters, key, recipesRate, recipesBurst)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many recipe requests, please try again later"))
		ctx.Abort()
		return
	}
	ctx.Next()
}

// importRateLimitMiddleware rate-limits CSV product imports per user. One import
// can insert up to util.CsvImportMaxRows products and — for rows that carry only
// a barcode — make live Open Food Facts requests, so it gets its own limiter
// rather than sharing the scan budget. importRate stays a package constant so a
// missing config key cannot fail startup validation for existing deployments.
func importRateLimitMiddleware(ctx *gin.Context) {
	key := ctx.ClientIP()
	if raw, ok := ctx.Get(util.ContextKeyUserID); ok {
		if userID, ok := raw.(uint); ok && userID > 0 {
			key = strconv.FormatUint(uint64(userID), 10)
		}
	}

	limiter := getLimiterBurst(importLimiters, key, importRate, importBurstCap)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many import requests, please try again later"))
		ctx.Abort()
		return
	}
	ctx.Next()
}

// bulkRateLimitMiddleware rate-limits bulk product creates per user. One call
// inserts up to util.ReceiptBulkMaxItems products; unlike the CSV import path
// there are no outbound lookups, but 100 inserts per request still warrant a
// dedicated limiter rather than sharing the scan budget.
func bulkRateLimitMiddleware(ctx *gin.Context) {
	key := ctx.ClientIP()
	if raw, ok := ctx.Get(util.ContextKeyUserID); ok {
		if userID, ok := raw.(uint); ok && userID > 0 {
			key = strconv.FormatUint(uint64(userID), 10)
		}
	}

	limiter := getLimiterBurst(bulkLimiters, key, bulkRate, bulkBurstCap)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many bulk create requests, please try again later"))
		ctx.Abort()
		return
	}
	ctx.Next()
}

// receiptRateLimitMiddleware rate-limits receipt photo scans per user. Every
// call forwards the upload to an external OpenAI-compatible vision endpoint,
// so rapid repeats burn the configured provider's quota — hence a dedicated
// budget instead of sharing scanRate with local expiry-date scans.
func receiptRateLimitMiddleware(ctx *gin.Context) {
	key := ctx.ClientIP()
	if raw, ok := ctx.Get(util.ContextKeyUserID); ok {
		if userID, ok := raw.(uint); ok && userID > 0 {
			key = strconv.FormatUint(uint64(userID), 10)
		}
	}

	limiter := getLimiterBurst(receiptLimiters, key, receiptRate, receiptBurstCap)
	reservation := limiter.Reserve()
	if delay := reservation.Delay(); delay > 0 {
		reservation.Cancel()
		ctx.Header(headerRetryAfter, strconv.Itoa(int(delay.Seconds())))
		api.RespondError(ctx, http.StatusTooManyRequests, errors.New("too many receipt scan requests, please try again later"))
		ctx.Abort()
		return
	}
	ctx.Next()
}

func cleanupLimiters() {
	for {
		time.Sleep(time.Minute)
		now := time.Now()
		for _, store := range []*sync.Map{
			loginLimiters, signupLimiters, exportLimiters, passwordLimiters,
			scanLimiters, recipesLimiters, importLimiters, bulkLimiters, receiptLimiters,
		} {
			cleanupLimiterStore(store, now)
		}
	}
}

// cleanupLimiterStore drops the limiters in one store that have gone unused
// for 10 minutes.
func cleanupLimiterStore(store *sync.Map, now time.Time) {
	store.Range(func(key, value any) bool {
		storedLimiter := value.(*clientLimiter)
		if storedLimiter.idleFor(now) > 10*time.Minute {
			store.Delete(key)
		}
		return true
	})
}

func init() {
	go cleanupLimiters()
}
