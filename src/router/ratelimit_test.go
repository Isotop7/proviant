package router

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// The recipes limiter must not hand out a full minute of tokens at once: one
// suggestions request fans out into up to 10 provider searches plus 20 detail
// lookups, so the default int(limit*60)+1 burst would let a single client open
// that many simultaneous upstream bursts.
//
// It must still cover the request pattern the recipes page produces on its own:
// one request on load, then cookMatchedProducts reloads suggestions after every
// successful cook. Each of those reloads passes refresh=1, so it is a guaranteed
// cache miss and a full fan-out rather than a cheap read. The test below pins
// the flow that regressed when the burst was 2: load, then cook twice.
func TestRecipesRateLimitMiddleware_CoversThePagesOwnRequestPattern(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recipesLimiters = &sync.Map{}

	engine := gin.New()
	engine.GET("/recipes", recipesRateLimitMiddleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	request := func() int {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/recipes", nil))
		return recorder.Code
	}

	// Page load, then the post-cook reload of the first and second recipe.
	for _, step := range []string{"page load", "post-cook reload after the first recipe", "post-cook reload after the second recipe"} {
		if code := request(); code != http.StatusOK {
			t.Fatalf("%s got HTTP %d, want 200 — the burst self-limits the page's own flow", step, code)
		}
	}
}

// The burst must stay well below the full minute of tokens the default
// int(limit*60)+1 would hand out, so it stays a fan-out bound rather than a
// per-minute quota. It also has to leave headroom over the three-request flow
// above so ordinary page use is never rate limited.
func TestRecipesRateLimitMiddleware_BurstBoundsFanOutWithoutSelfLimiting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recipesLimiters = &sync.Map{}

	engine := gin.New()
	engine.GET("/recipes", recipesRateLimitMiddleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	allowed := 0
	for range recipesBurst + 3 {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/recipes", nil)
		engine.ServeHTTP(recorder, request)
		if recorder.Code == http.StatusOK {
			allowed++
		}
	}

	if allowed != recipesBurst {
		t.Errorf("allowed %d back-to-back requests, want exactly %d", allowed, recipesBurst)
	}
	if recipesBurst < 3 {
		t.Errorf("recipesBurst = %d, want at least 3 so page load plus post-cook reloads are not self-limited", recipesBurst)
	}
	// At the default 6 recipes_per_minute the minute's worth of tokens would be
	// 7; the burst must stay clearly below that.
	if fullMinute := int(recipesRate*60) + 1; recipesBurst >= fullMinute {
		t.Errorf("recipesBurst = %d, want below the full minute of tokens (%d) so the limiter still bounds the fan-out",
			recipesBurst, fullMinute)
	}
}

// A rejected request must abort the chain, not fall through to the handler.
// The first requests legitimately succeed (fresh bucket), so exactly the ones
// beyond the burst may reach the handler.
func TestRecipesRateLimitMiddleware_AbortsOnRejection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recipesLimiters = &sync.Map{}

	handlerHits := 0
	engine := gin.New()
	engine.GET("/recipes", recipesRateLimitMiddleware, func(c *gin.Context) {
		handlerHits++
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	limited := 0
	for range recipesBurst + 2 {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/recipes", nil)
		engine.ServeHTTP(recorder, request)
		if recorder.Code == http.StatusTooManyRequests {
			limited++
		}
	}

	if limited != 2 {
		t.Errorf("got %d rate-limited responses, want 2", limited)
	}
	if handlerHits != recipesBurst {
		t.Errorf("handler ran %d times, want %d (a rejected request must abort the chain)", handlerHits, recipesBurst)
	}
}

func TestGetLimiterBurst_ReusesStoredLimiter(t *testing.T) {
	store := &sync.Map{}

	first := getLimiterBurst(store, "client", rate.Every(time.Second), 1)
	second := getLimiterBurst(store, "client", rate.Every(time.Second), 1)
	if first != second {
		t.Error("getLimiterBurst() returned a new limiter for a known key, want the stored one")
	}

	other := getLimiterBurst(store, "other", rate.Every(time.Second), 1)
	if other == first {
		t.Error("getLimiterBurst() shared a limiter across keys")
	}
}
