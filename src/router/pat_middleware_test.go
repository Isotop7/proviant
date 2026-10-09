package router

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/testutil"
	"codeberg.org/isotop7/proviant/util"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func itoa(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

func patTestJWTMiddleware(t *testing.T, db *gorm.DB) *jwt.GinJWTMiddleware {
	t.Helper()
	cfg := &configuration.ProviantConfiguration{}
	cfg.Server.Authentication.TokenPassword = "test-token-password"
	cfg.Server.Authentication.TokenLifetime = 1

	middleware, err := JWTMiddleware(cfg, db, func(data any, ctx *gin.Context) bool { return true }, patUnauthorized)
	if err != nil {
		t.Fatalf("JWTMiddleware() error = %v", err)
	}
	if err := middleware.MiddlewareInit(); err != nil {
		t.Fatalf("MiddlewareInit() error = %v", err)
	}
	return middleware
}

func patTestRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := zerolog.Nop()
	engine := gin.New()
	engine.Use(func(ctx *gin.Context) {
		ctx.Set(util.ContextKeyDBHandle, db)
		ctx.Set(util.ContextKeyLogger, &logger)
		ctx.Next()
	})
	engine.Use(PATMiddleware(patTestJWTMiddleware(t, db)))
	engine.GET("/", func(ctx *gin.Context) {
		userID, _ := ctx.Get(util.ContextKeyUserID)
		ctx.String(http.StatusOK, "user %v", userID)
	})
	return engine
}

// patUnauthorized aborts the request like the production unauthorized handler.
func patUnauthorized(ctx *gin.Context, code int, message string) {
	ctx.AbortWithStatus(code)
}

func seedPAT(t *testing.T, db *gorm.DB, userID uint) string {
	t.Helper()
	token, err := controllers.GeneratePAT()
	if err != nil {
		t.Fatalf("GeneratePAT() error = %v", err)
	}
	pat := authentication.PersonalAccessToken{
		UserID:    userID,
		Name:      "test-pat",
		TokenHash: controllers.HashToken(token),
	}
	if err := db.Create(&pat).Error; err != nil {
		t.Fatalf("failed to create PAT: %v", err)
	}
	return token
}

func TestPATMiddleware(t *testing.T) {
	t.Run("valid PAT authenticates and sets user id", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		user := testutil.CreateTestUser(db, 0)
		token := seedPAT(t, db, user.ID)
		engine := patTestRouter(t, db)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", token)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
		}
		if got := w.Body.String(); got != "user "+itoa(user.ID) {
			t.Errorf("body = %q, want %q", got, "user "+itoa(user.ID))
		}
	})

	t.Run("expired PAT falls through to JWT middleware", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		user := testutil.CreateTestUser(db, 0)
		token := seedPAT(t, db, user.ID)
		expiry := time.Now().Add(-time.Hour)
		if err := db.Model(&authentication.PersonalAccessToken{}).Where("user_id = ?", user.ID).Update("expires_at", expiry).Error; err != nil {
			t.Fatalf("failed to expire PAT: %v", err)
		}
		engine := patTestRouter(t, db)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", token)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			t.Errorf("status = %d, want non-200 for expired PAT", w.Code)
		}
	})

	t.Run("non-PAT Authorization header falls through to JWT middleware", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		engine := patTestRouter(t, db)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer not-a-pat")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code == http.StatusOK {
			t.Errorf("status = %d, want non-200 for plain bearer token", w.Code)
		}
	})

	t.Run("no Authorization header falls through to JWT middleware", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		engine := patTestRouter(t, db)

		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

		if w.Code == http.StatusOK {
			t.Errorf("status = %d, want non-200 without credentials", w.Code)
		}
	})

	t.Run("wrong db handle type aborts with 401", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		db := testutil.SetupTestDB(t)
		user := testutil.CreateTestUser(db, 0)
		token := seedPAT(t, db, user.ID)
		logger := zerolog.Nop()

		engine := gin.New()
		engine.Use(func(ctx *gin.Context) {
			ctx.Set(util.ContextKeyDBHandle, "not-a-db")
			ctx.Set(util.ContextKeyLogger, &logger)
			ctx.Next()
		})
		engine.Use(PATMiddleware(patTestJWTMiddleware(t, db)))
		engine.GET("/", func(ctx *gin.Context) { ctx.String(http.StatusOK, "ok") })

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", token)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
	})
}
