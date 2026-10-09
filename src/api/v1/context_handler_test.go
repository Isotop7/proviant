package v1

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	apperrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// runWithMiddleware executes AppContextMiddleware + WrapHandler through a real
// gin engine, with a leading middleware that injects the given context values.
func runWithMiddleware(t *testing.T, inject func(ctx *gin.Context), handler APIHandler) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(ctx *gin.Context) {
		if inject != nil {
			inject(ctx)
		}
		ctx.Next()
	})
	router.Use(AppContextMiddleware())
	router.GET("/test", WrapHandler(handler))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	router.ServeHTTP(w, req)
	return w
}

func okHandler(ctx *gin.Context, appCtx *AppContext) {
	ctx.JSON(http.StatusOK, gin.H{"userID": appCtx.UserID})
}

func TestAppContextMiddleware(t *testing.T) {
	db := testutil.SetupTestDB(t)
	logger := zerolog.Nop()
	repos := database.NewRepositoryContainer(db, nil)

	t.Run("missing logger aborts with 500", func(t *testing.T) {
		w := runWithMiddleware(t, nil, okHandler)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})

	t.Run("wrong db handle type returns 500", func(t *testing.T) {
		w := runWithMiddleware(t, func(ctx *gin.Context) {
			ctx.Set(util.ContextKeyLogger, &logger)
			ctx.Set(util.ContextKeyDBHandle, "not-a-db")
		}, okHandler)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})

	t.Run("missing repos returns 500", func(t *testing.T) {
		w := runWithMiddleware(t, func(ctx *gin.Context) {
			ctx.Set(util.ContextKeyLogger, &logger)
			ctx.Set(util.ContextKeyDBHandle, db)
		}, okHandler)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})

	t.Run("missing JWT claims returns 400", func(t *testing.T) {
		w := runWithMiddleware(t, func(ctx *gin.Context) {
			ctx.Set(util.ContextKeyLogger, &logger)
			ctx.Set(util.ContextKeyDBHandle, db)
			ctx.Set(util.ContextKeyRepos, repos)
		}, okHandler)
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("valid context reaches handler", func(t *testing.T) {
		w := runWithMiddleware(t, func(ctx *gin.Context) {
			ctx.Set(util.ContextKeyLogger, &logger)
			ctx.Set(util.ContextKeyDBHandle, db)
			ctx.Set(util.ContextKeyRepos, repos)
			testutil.MockJWTClaims(ctx, 42)
		}, okHandler)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		if w.Body.String() != `{"userID":42}` {
			t.Errorf("body = %s, want userID 42", w.Body.String())
		}
	})

	t.Run("PAT user id in context is preferred", func(t *testing.T) {
		w := runWithMiddleware(t, func(ctx *gin.Context) {
			ctx.Set(util.ContextKeyLogger, &logger)
			ctx.Set(util.ContextKeyDBHandle, db)
			ctx.Set(util.ContextKeyRepos, repos)
			ctx.Set(util.ContextKeyUserID, uint(7))
			testutil.MockJWTClaims(ctx, 42)
		}, okHandler)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		if w.Body.String() != `{"userID":7}` {
			t.Errorf("body = %s, want userID 7", w.Body.String())
		}
	})
}

func TestWrapHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing appContext returns 500", func(t *testing.T) {
		ctx, w := testutil.SetupGinContext(testutil.SetupTestDB(t))
		called := false
		WrapHandler(func(ctx *gin.Context, appCtx *AppContext) { called = true })(ctx)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
		if called {
			t.Error("handler must not be called without appContext")
		}
	})

	t.Run("wrong appContext type returns 500", func(t *testing.T) {
		ctx, w := testutil.SetupGinContext(testutil.SetupTestDB(t))
		ctx.Set("appContext", "not-an-app-context")
		WrapHandler(func(ctx *gin.Context, appCtx *AppContext) {})(ctx)
		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})

	t.Run("valid appContext invokes handler", func(t *testing.T) {
		ctx, w := testutil.SetupGinContext(testutil.SetupTestDB(t))
		called := false
		SetupTestAppContext(ctx, 1)
		WrapHandler(func(ctx *gin.Context, appCtx *AppContext) {
			called = true
			ctx.JSON(http.StatusOK, gin.H{"ok": true})
		})(ctx)
		if !called {
			t.Error("handler was not called")
		}
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})
}

func TestMustGetUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zerolog.Nop()

	newCtx := func() *gin.Context {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Set(util.ContextKeyLogger, &logger)
		return ctx
	}

	t.Run("PAT context value wins", func(t *testing.T) {
		ctx := newCtx()
		ctx.Set(util.ContextKeyUserID, uint(7))
		testutil.MockJWTClaims(ctx, 42)
		id, ok := mustGetUserID(ctx, &logger)
		if !ok || id != 7 {
			t.Errorf("id = %d, ok = %v, want 7, true", id, ok)
		}
	})

	t.Run("JWT claim is used", func(t *testing.T) {
		ctx := newCtx()
		testutil.MockJWTClaims(ctx, 42)
		id, ok := mustGetUserID(ctx, &logger)
		if !ok || id != 42 {
			t.Errorf("id = %d, ok = %v, want 42, true", id, ok)
		}
	})

	t.Run("missing claim returns 400", func(t *testing.T) {
		ctx := newCtx()
		_, ok := mustGetUserID(ctx, &logger)
		if ok || ctx.Writer.Status() != http.StatusBadRequest {
			t.Errorf("ok = %v, status = %d, want false/400", ok, ctx.Writer.Status())
		}
	})

	t.Run("non-numeric claim returns 400", func(t *testing.T) {
		ctx := newCtx()
		ctx.Set("JWT_PAYLOAD", jwt.MapClaims{"id": "not-a-number"})
		_, ok := mustGetUserID(ctx, &logger)
		if ok || ctx.Writer.Status() != http.StatusBadRequest {
			t.Errorf("ok = %v, status = %d, want false/400", ok, ctx.Writer.Status())
		}
	})

	t.Run("zero user id returns 400", func(t *testing.T) {
		ctx := newCtx()
		testutil.MockJWTClaims(ctx, 0)
		_, ok := mustGetUserID(ctx, &logger)
		if ok || ctx.Writer.Status() != http.StatusBadRequest {
			t.Errorf("ok = %v, status = %d, want false/400", ok, ctx.Writer.Status())
		}
	})
}

func TestParseLimitParam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zerolog.Nop()

	newCtx := func(target string) (*gin.Context, *httptest.ResponseRecorder) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Set(util.ContextKeyLogger, &logger)
		ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
		return ctx, w
	}

	tests := []struct {
		name    string
		target  string
		want    int
		wantOK  bool
		wantCod int
	}{
		{"empty defaults to 100", "/x", 100, true, 0},
		{"valid limit", "/x?limit=25", 25, true, 0},
		{"above max clamps to 1000", "/x?limit=5000", 1000, true, 0},
		{"invalid limit", "/x?limit=abc", 0, false, http.StatusBadRequest},
		{"zero limit rejected", "/x?limit=0", 0, false, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, w := newCtx(tt.target)
			got, ok := parseLimitParam(ctx, &logger)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("limit = %d, want %d", got, tt.want)
			}
			if !ok && w.Code != tt.wantCod {
				t.Errorf("status = %d, want %d", w.Code, tt.wantCod)
			}
		})
	}
}

func TestMustGetOwnedWebhookID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zerolog.Nop()

	newCtx := func(m *repomocks.MockRepositoryContainer, idParam string) (*gin.Context, *httptest.ResponseRecorder) {
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		ctx.Params = []gin.Param{{Key: "id", Value: idParam}}
		return ctx, w
	}

	t.Run("invalid id returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := newCtx(m, "abc")
		if _, ok := mustGetOwnedWebhookID(ctx, m.ToRepositoryContainer(), &logger, 1); ok {
			t.Error("ok = true, want false")
		}
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("zero id returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := newCtx(m, "0")
		if _, ok := mustGetOwnedWebhookID(ctx, m.ToRepositoryContainer(), &logger, 1); ok {
			t.Error("ok = true, want false")
		}
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("webhook not found returns 404", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Webhooks.Err = apperrors.ErrWebhookNotFound
		ctx, w := newCtx(m, "1")
		if _, ok := mustGetOwnedWebhookID(ctx, m.ToRepositoryContainer(), &logger, 1); ok {
			t.Error("ok = true, want false")
		}
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("webhook owned by another user returns 404", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Webhooks.Err = apperrors.ErrWebhookNotOwner
		ctx, w := newCtx(m, "1")
		if _, ok := mustGetOwnedWebhookID(ctx, m.ToRepositoryContainer(), &logger, 1); ok {
			t.Error("ok = true, want false")
		}
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Webhooks.Err = errors.New("db down")
		ctx, w := newCtx(m, "1")
		if _, ok := mustGetOwnedWebhookID(ctx, m.ToRepositoryContainer(), &logger, 1); ok {
			t.Error("ok = true, want false")
		}
		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})

	t.Run("owned webhook returns id", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, _ := newCtx(m, "9")
		id, ok := mustGetOwnedWebhookID(ctx, m.ToRepositoryContainer(), &logger, 1)
		if !ok || id != 9 {
			t.Errorf("id = %d, ok = %v, want 9, true", id, ok)
		}
	})
}

func TestAuthorizeHouseholdAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zerolog.Nop()

	t.Run("unknown user returns 400", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.Err = gorm.ErrRecordNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		if _, ok := authorizeHouseholdAdmin(ctx, m.ToRepositoryContainer(), &logger, 1); ok {
			t.Error("ok = true, want false")
		}
		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", w.Code)
		}
	})

	t.Run("household fetch error returns 500", func(t *testing.T) {
		// Real DB: the user exists but points at a household that does not.
		db := testutil.SetupTestDB(t)
		user := testutil.CreateTestUser(db, 0)
		user.HouseholdID = 9999
		db.Save(user)
		ctx, w := testutil.SetupGinContext(db)
		repos := database.NewRepositoryContainer(db, nil)
		_, ok := authorizeHouseholdAdmin(ctx, repos, &logger, user.ID)
		if ok {
			t.Error("ok = true, want false")
		}
		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})

	t.Run("non-admin returns 403", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.User = authentication.User{HouseholdID: 2}
		m.Users.Household = dbModel.Household{AdminID: 99}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		if _, ok := authorizeHouseholdAdmin(ctx, m.ToRepositoryContainer(), &logger, 1); ok {
			t.Error("ok = true, want false")
		}
		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", w.Code)
		}
	})

	t.Run("admin gets household id", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.User = authentication.User{HouseholdID: 2}
		household := dbModel.Household{AdminID: 1}
		household.ID = 2
		m.Users.Household = household
		ctx, _ := repomocks.SetupGinContextWithMocks(m)
		id, ok := authorizeHouseholdAdmin(ctx, m.ToRepositoryContainer(), &logger, 1)
		if !ok || id != 2 {
			t.Errorf("id = %d, ok = %v, want 2, true", id, ok)
		}
	})
}

func TestFetchHouseholdMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := zerolog.Nop()

	t.Run("missing user returns 404", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.Err = gorm.ErrRecordNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		if _, ok := fetchHouseholdMember(ctx, m.ToRepositoryContainer(), &logger, 5, 2); ok {
			t.Error("ok = true, want false")
		}
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w.Code)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		if _, ok := fetchHouseholdMember(ctx, m.ToRepositoryContainer(), &logger, 5, 2); ok {
			t.Error("ok = true, want false")
		}
		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})

	t.Run("user in other household returns 403", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.User = authentication.User{HouseholdID: 9}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		if _, ok := fetchHouseholdMember(ctx, m.ToRepositoryContainer(), &logger, 5, 2); ok {
			t.Error("ok = true, want false")
		}
		if w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", w.Code)
		}
	})

	t.Run("member of household is returned", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.User = authentication.User{HouseholdID: 2, Username: "member"}
		ctx, _ := repomocks.SetupGinContextWithMocks(m)
		user, ok := fetchHouseholdMember(ctx, m.ToRepositoryContainer(), &logger, 5, 2)
		if !ok || user.Username != "member" {
			t.Errorf("user = %+v, ok = %v, want member, true", user, ok)
		}
	})
}

func TestGetNotificationController(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("wrong type returns false", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Set(util.ContextKeyNotificationController, "not-a-controller")
		if _, ok := getNotificationController(ctx); ok {
			t.Error("ok = true, want false")
		}
	})

	t.Run("controller is returned", func(t *testing.T) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Set(util.ContextKeyNotificationController, &controllers.NotificationController{})
		nc, ok := getNotificationController(ctx)
		if !ok || nc == nil {
			t.Errorf("controller = %v, ok = %v, want non-nil, true", nc, ok)
		}
	})
}
