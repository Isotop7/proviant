package web

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/templates"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func fmtUint(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

// handlerEnv bundles a test database with a Frontend backed by the shared
// template cache.
type handlerEnv struct {
	t        *testing.T
	db       *gorm.DB
	frontend *Frontend
}

func newHandlerEnv(t *testing.T) *handlerEnv {
	t.Helper()
	cache, err := templates.NewTemplateCache()
	if err != nil {
		t.Fatalf("NewTemplateCache() error = %v", err)
	}
	return &handlerEnv{t: t, db: testutil.SetupTestDB(t), frontend: &Frontend{TemplateCache: cache}}
}

// run invokes handler with a fully prepared context. userID 0 means "zero
// user id from the token". cfg may be nil to omit the config from the context.
func (h *handlerEnv) run(userID uint, target string, cfg *configuration.ProviantConfiguration, handler func(*gin.Context)) *httptest.ResponseRecorder {
	h.t.Helper()
	ctx, w := repomocks.SetupGinContextWithDB(h.db)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	testutil.MockJWTClaimsWithKey(ctx, userID, static.TokenIdentityKey)
	if cfg != nil {
		ctx.Set(util.ContextKeyProviantConfig, cfg)
	}
	handler(ctx)
	return w
}

// runWrongRepoType sets the repos context key to a value of the wrong type,
// for exercising the "database context not found" assertion paths.
func (h *handlerEnv) runWrongRepoType(userID uint, target string, handler func(*gin.Context)) *httptest.ResponseRecorder {
	h.t.Helper()
	ctx, w := testutil.SetupGinContext(h.db)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	testutil.MockJWTClaimsWithKey(ctx, userID, static.TokenIdentityKey)
	ctx.Set(util.ContextKeyRepos, "not-a-repository-container")
	handler(ctx)
	return w
}

// runWithParams mirrors run but injects path parameters.
func (h *handlerEnv) runWithParams(userID uint, cfg *configuration.ProviantConfiguration, params gin.Params, handler func(*gin.Context)) *httptest.ResponseRecorder {
	h.t.Helper()
	ctx, w := repomocks.SetupGinContextWithDB(h.db)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx.Params = params
	testutil.MockJWTClaimsWithKey(ctx, userID, static.TokenIdentityKey)
	if cfg != nil {
		ctx.Set(util.ContextKeyProviantConfig, cfg)
	}
	handler(ctx)
	return w
}

func requireStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status = %d, want %d; body: %s", w.Code, want, w.Body.String())
	}
}

func requireContains(t *testing.T, w *httptest.ResponseRecorder, substrings ...string) {
	t.Helper()
	body := w.Body.String()
	for _, s := range substrings {
		if !strings.Contains(body, s) {
			t.Errorf("body missing %q; body: %s", s, body)
		}
	}
}

// seedHouseholdUser creates a household with the user as its admin.
func seedHouseholdUser(t *testing.T, db *gorm.DB) (*dbModel.Household, *authentication.User) {
	t.Helper()
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	household.AdminID = user.ID
	if err := db.Save(household).Error; err != nil {
		t.Fatalf("failed to save household: %v", err)
	}
	return household, user
}

func TestRootHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("renders home page for user with household", func(t *testing.T) {
		_, user := seedHouseholdUser(t, env.db)
		w := env.run(user.ID, "/web", nil, env.frontend.Root)
		requireStatus(t, w, http.StatusOK)
	})

	t.Run("renders home page for user without household", func(t *testing.T) {
		user := testutil.CreateTestUser(env.db, 0)
		w := env.run(user.ID, "/web", nil, env.frontend.Root)
		requireStatus(t, w, http.StatusOK)
	})

	t.Run("renders home page in demo mode", func(t *testing.T) {
		_, user := seedHouseholdUser(t, env.db)
		cfg := &configuration.ProviantConfiguration{}
		cfg.Server.DemoMode = true
		w := env.run(user.ID, "/web", cfg, env.frontend.Root)
		requireStatus(t, w, http.StatusOK)
	})

	t.Run("zero user id renders error page", func(t *testing.T) {
		w := env.run(0, "/web", nil, env.frontend.Root)
		requireStatus(t, w, http.StatusBadRequest)
		requireContains(t, w, "Error 400")
	})

	t.Run("wrong repository type renders error page", func(t *testing.T) {
		w := env.runWrongRepoType(1, "/web", env.frontend.Root)
		requireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestAuthHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("renders auth page without config", func(t *testing.T) {
		w := env.run(0, "/web/auth", nil, env.frontend.Auth)
		requireStatus(t, w, http.StatusOK)
	})

	t.Run("renders auth page with password policy config", func(t *testing.T) {
		cfg := &configuration.ProviantConfiguration{}
		cfg.Server.Authentication.PasswordMinLength = 16
		cfg.Server.Authentication.PasswordRequireUppercase = true
		cfg.Server.Authentication.PasswordRequireDigit = true
		cfg.Server.Authentication.PasswordRequireSpecial = true
		w := env.run(0, "/web/auth", cfg, env.frontend.Auth)
		requireStatus(t, w, http.StatusOK)
		requireContains(t, w, "Authentication")
	})
}

func TestUserHandler(t *testing.T) {
	env := newHandlerEnv(t)
	w := env.run(0, "/web/user", nil, env.frontend.User)
	requireStatus(t, w, http.StatusOK)
}

func TestUserSettingsHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("renders settings for household admin", func(t *testing.T) {
		household, user := seedHouseholdUser(t, env.db)
		testutil.CreateTestStorageLocation(env.db, household.ID)
		w := env.run(user.ID, "/web/user/settings", nil, env.frontend.UserSettings)
		requireStatus(t, w, http.StatusOK)
	})

	t.Run("renders settings for non-admin member", func(t *testing.T) {
		household, _ := seedHouseholdUser(t, env.db)
		member := testutil.CreateTestUser(env.db, household.ID)
		w := env.run(member.ID, "/web/user/settings", nil, env.frontend.UserSettings)
		requireStatus(t, w, http.StatusOK)
	})

	t.Run("missing user renders error page", func(t *testing.T) {
		w := env.run(9999, "/web/user/settings", nil, env.frontend.UserSettings)
		requireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("user without household renders error page", func(t *testing.T) {
		user := testutil.CreateTestUser(env.db, 0)
		w := env.run(user.ID, "/web/user/settings", nil, env.frontend.UserSettings)
		requireStatus(t, w, http.StatusBadRequest)
	})
}

func TestRecipesHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("renders with household", func(t *testing.T) {
		_, user := seedHouseholdUser(t, env.db)
		w := env.run(user.ID, "/web/recipes", nil, env.frontend.Recipes)
		requireStatus(t, w, http.StatusOK)
	})

	t.Run("renders without household", func(t *testing.T) {
		user := testutil.CreateTestUser(env.db, 0)
		w := env.run(user.ID, "/web/recipes", nil, env.frontend.Recipes)
		requireStatus(t, w, http.StatusOK)
	})
}

func TestWasteAnalyticsHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("renders with household", func(t *testing.T) {
		_, user := seedHouseholdUser(t, env.db)
		w := env.run(user.ID, "/web/waste-analytics", nil, env.frontend.WasteAnalytics)
		requireStatus(t, w, http.StatusOK)
	})

	t.Run("renders without household", func(t *testing.T) {
		user := testutil.CreateTestUser(env.db, 0)
		w := env.run(user.ID, "/web/waste-analytics", nil, env.frontend.WasteAnalytics)
		requireStatus(t, w, http.StatusOK)
	})
}

func TestShoppingListHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("renders empty list", func(t *testing.T) {
		_, user := seedHouseholdUser(t, env.db)
		w := env.run(user.ID, "/web/shopping-list", nil, env.frontend.ShoppingList)
		requireStatus(t, w, http.StatusOK)
	})

	t.Run("renders sorted checked and unchecked items", func(t *testing.T) {
		household, user := seedHouseholdUser(t, env.db)
		items := []dbModel.ShoppingListItem{
			{HouseholdID: household.ID, Name: "Bananas", Checked: false, CreatedBy: user.ID},
			{HouseholdID: household.ID, Name: "Apples", Checked: false, CreatedBy: user.ID},
			{HouseholdID: household.ID, Name: "Milk", Checked: true, CreatedBy: user.ID},
		}
		for i := range items {
			if err := env.db.Create(&items[i]).Error; err != nil {
				t.Fatalf("failed to create shopping list item: %v", err)
			}
		}
		w := env.run(user.ID, "/web/shopping-list", nil, env.frontend.ShoppingList)
		requireStatus(t, w, http.StatusOK)
	})

	t.Run("missing user renders error page", func(t *testing.T) {
		w := env.run(9999, "/web/shopping-list", nil, env.frontend.ShoppingList)
		requireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestProductsScanHandler(t *testing.T) {
	env := newHandlerEnv(t)
	w := env.run(0, "/web/products/scan", nil, env.frontend.ProductsScan)
	requireStatus(t, w, http.StatusOK)
}

func TestProductsScanReceiptHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("feature disabled redirects to products page", func(t *testing.T) {
		_, user := seedHouseholdUser(t, env.db)
		cfg := &configuration.ProviantConfiguration{}
		w := env.run(user.ID, "/web/products/scan-receipt", cfg, env.frontend.ProductsScanReceipt)
		requireStatus(t, w, http.StatusFound)
		if loc := w.Header().Get("Location"); loc != "/web/products" {
			t.Errorf("Location = %q, want /web/products", loc)
		}
	})

	t.Run("enabled without model redirects to products page", func(t *testing.T) {
		_, user := seedHouseholdUser(t, env.db)
		cfg := &configuration.ProviantConfiguration{}
		cfg.OCR.Receipt.Enabled = true
		w := env.run(user.ID, "/web/products/scan-receipt", cfg, env.frontend.ProductsScanReceipt)
		requireStatus(t, w, http.StatusFound)
	})

	t.Run("enabled with model renders scan page", func(t *testing.T) {
		_, user := seedHouseholdUser(t, env.db)
		testutil.CreateTestStorageLocation(env.db, user.HouseholdID)
		cfg := &configuration.ProviantConfiguration{}
		cfg.OCR.Receipt.Enabled = true
		cfg.OCR.Receipt.Model = "vision-model"
		w := env.run(user.ID, "/web/products/scan-receipt", cfg, env.frontend.ProductsScanReceipt)
		requireStatus(t, w, http.StatusOK)
		requireContains(t, w, "Scan Receipt")
	})

	t.Run("enabled with missing user renders error page", func(t *testing.T) {
		cfg := &configuration.ProviantConfiguration{}
		cfg.OCR.Receipt.Enabled = true
		cfg.OCR.Receipt.Model = "vision-model"
		w := env.run(9999, "/web/products/scan-receipt", cfg, env.frontend.ProductsScanReceipt)
		requireStatus(t, w, http.StatusInternalServerError)
	})
}

func TestProductsViewHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("invalid id renders error page", func(t *testing.T) {
		w := env.runWithParams(1, nil, gin.Params{{Key: "id", Value: "abc"}}, env.frontend.ProductsView)
		requireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("missing product renders error page", func(t *testing.T) {
		w := env.runWithParams(1, nil, gin.Params{{Key: "id", Value: "123"}}, env.frontend.ProductsView)
		requireStatus(t, w, http.StatusNotFound)
	})

	t.Run("renders product page", func(t *testing.T) {
		household, user := seedHouseholdUser(t, env.db)
		product := testutil.CreateTestProduct(env.db, household.ID, user.ID)
		w := env.runWithParams(user.ID, nil, gin.Params{{Key: "id", Value: fmtUint(product.ID)}}, env.frontend.ProductsView)
		requireStatus(t, w, http.StatusOK)
		requireContains(t, w, "Test Product")
	})
}

func TestProductsEditHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("invalid id renders error page", func(t *testing.T) {
		w := env.runWithParams(1, nil, gin.Params{{Key: "id", Value: "abc"}}, env.frontend.ProductsEdit)
		requireStatus(t, w, http.StatusBadRequest)
	})

	t.Run("missing product renders error page", func(t *testing.T) {
		w := env.runWithParams(1, nil, gin.Params{{Key: "id", Value: "123"}}, env.frontend.ProductsEdit)
		requireStatus(t, w, http.StatusNotFound)
	})

	t.Run("renders edit page", func(t *testing.T) {
		household, user := seedHouseholdUser(t, env.db)
		product := testutil.CreateTestProduct(env.db, household.ID, user.ID)
		w := env.runWithParams(user.ID, nil, gin.Params{{Key: "id", Value: fmtUint(product.ID)}}, env.frontend.ProductsEdit)
		requireStatus(t, w, http.StatusOK)
	})
}

func TestAcceptInviteHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("no token renders error form", func(t *testing.T) {
		w := env.run(0, "/web/invite/accept", nil, env.frontend.AcceptInvite)
		requireStatus(t, w, http.StatusBadRequest)
		requireContains(t, w, "No invitation token provided.")
	})

	t.Run("missing repositories renders error form", func(t *testing.T) {
		w := env.runWrongRepoType(0, "/web/invite/accept?token=abc", env.frontend.AcceptInvite)
		requireStatus(t, w, http.StatusInternalServerError)
	})

	t.Run("unknown token renders not-found", func(t *testing.T) {
		w := env.run(0, "/web/invite/accept?token=does-not-exist", nil, env.frontend.AcceptInvite)
		requireStatus(t, w, http.StatusNotFound)
	})

	t.Run("cancelled invitation renders gone", func(t *testing.T) {
		household := testutil.CreateTestHousehold(env.db, 0)
		invitation := testutil.CreateTestInvitation(env.db, household.ID, 0, "cancelled@example.com")
		invitation.Status = dbModel.InvitationStatusCancelled
		if err := env.db.Save(invitation).Error; err != nil {
			t.Fatalf("failed to save invitation: %v", err)
		}
		w := env.run(0, "/web/invite/accept?token="+invitation.Token, nil, env.frontend.AcceptInvite)
		requireStatus(t, w, http.StatusGone)
	})

	t.Run("expired invitation renders gone and marks invitation expired", func(t *testing.T) {
		household := testutil.CreateTestHousehold(env.db, 0)
		invitation := testutil.CreateTestInvitation(env.db, household.ID, 0, "expired@example.com")
		invitation.ExpiresAt = time.Now().Add(-time.Hour)
		if err := env.db.Save(invitation).Error; err != nil {
			t.Fatalf("failed to save invitation: %v", err)
		}
		w := env.run(0, "/web/invite/accept?token="+invitation.Token, nil, env.frontend.AcceptInvite)
		requireStatus(t, w, http.StatusGone)

		var updated dbModel.HouseholdInvitation
		if err := env.db.First(&updated, invitation.ID).Error; err != nil {
			t.Fatalf("failed to reload invitation: %v", err)
		}
		if updated.Status != dbModel.InvitationStatusExpired {
			t.Errorf("status = %q, want %q", updated.Status, dbModel.InvitationStatusExpired)
		}
	})

	t.Run("pending invitation for anonymous user shows auth hint", func(t *testing.T) {
		household := testutil.CreateTestHousehold(env.db, 0)
		invitation := testutil.CreateTestInvitation(env.db, household.ID, 0, "anon@example.com")
		invitation.ExpiresAt = time.Now().Add(24 * time.Hour)
		if err := env.db.Save(invitation).Error; err != nil {
			t.Fatalf("failed to save invitation: %v", err)
		}
		w := env.run(0, "/web/invite/accept?token="+invitation.Token, nil, env.frontend.AcceptInvite)
		requireStatus(t, w, http.StatusOK)
	})

	t.Run("pending invitation for logged-in user shows confirmation", func(t *testing.T) {
		household, user := seedHouseholdUser(t, env.db)
		invitation := testutil.CreateTestInvitation(env.db, household.ID, user.ID, "user@example.com")
		invitation.ExpiresAt = time.Now().Add(24 * time.Hour)
		if err := env.db.Save(invitation).Error; err != nil {
			t.Fatalf("failed to save invitation: %v", err)
		}
		w := env.run(user.ID, "/web/invite/accept?token="+invitation.Token, nil, env.frontend.AcceptInvite)
		requireStatus(t, w, http.StatusOK)
		requireContains(t, w, household.Name)
	})
}

func TestVerifyEmailHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("no token renders error page", func(t *testing.T) {
		w := env.run(0, "/web/verify-email", nil, env.frontend.VerifyEmail)
		requireStatus(t, w, http.StatusBadRequest)
		requireContains(t, w, "No verification token provided.")
	})

	t.Run("with token renders verifying page", func(t *testing.T) {
		w := env.run(0, "/web/verify-email?token=abc", nil, env.frontend.VerifyEmail)
		requireStatus(t, w, http.StatusOK)
	})
}

func TestOnboardingHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("renders onboarding page", func(t *testing.T) {
		user := testutil.CreateTestUser(env.db, 0)
		w := env.run(user.ID, "/web/onboarding", nil, env.frontend.Onboarding)
		requireStatus(t, w, http.StatusOK)
		requireContains(t, w, user.Username)
	})

	t.Run("missing user renders error page", func(t *testing.T) {
		w := env.run(9999, "/web/onboarding", nil, env.frontend.Onboarding)
		requireStatus(t, w, http.StatusBadRequest)
	})
}

func TestUnsubscribeHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("no token renders error page", func(t *testing.T) {
		w := env.run(0, "/web/unsubscribe", nil, env.frontend.Unsubscribe)
		requireStatus(t, w, http.StatusBadRequest)
		requireContains(t, w, "No unsubscribe token provided.")
	})

	t.Run("unknown token renders not-found", func(t *testing.T) {
		w := env.run(0, "/web/unsubscribe?token=unknown", nil, env.frontend.Unsubscribe)
		requireStatus(t, w, http.StatusNotFound)
	})

	t.Run("valid token disables digest and deletes token", func(t *testing.T) {
		user := testutil.CreateTestUser(env.db, 0)
		token := dbModel.MailDigestUnsubscribeToken{
			MailDigestToken: "test-unsubscribe-token",
			UserID:          user.ID,
			CreatedAt:       time.Now(),
		}
		if err := env.db.Create(&token).Error; err != nil {
			t.Fatalf("failed to create token: %v", err)
		}
		w := env.run(0, "/web/unsubscribe?token=test-unsubscribe-token", nil, env.frontend.Unsubscribe)
		requireStatus(t, w, http.StatusOK)

		var updated authentication.User
		if err := env.db.First(&updated, user.ID).Error; err != nil {
			t.Fatalf("failed to reload user: %v", err)
		}
		if updated.NotificationPreferences.MailDigestFrequency != authentication.MailDigestFrequencyDisabled {
			t.Errorf("MailDigestFrequency = %q, want %q", updated.NotificationPreferences.MailDigestFrequency, authentication.MailDigestFrequencyDisabled)
		}
		var count int64
		if err := env.db.Model(&dbModel.MailDigestUnsubscribeToken{}).Count(&count).Error; err != nil {
			t.Fatalf("failed to count unsubscribe tokens: %v", err)
		}
		if count != 0 {
			t.Errorf("token rows = %d, want 0", count)
		}
	})
}

func TestForgotPasswordHandler(t *testing.T) {
	env := newHandlerEnv(t)
	w := env.run(0, "/web/forgot-password", nil, env.frontend.ForgotPassword)
	requireStatus(t, w, http.StatusOK)
	requireContains(t, w, "Forgot Password")
}

func TestResetPasswordHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("without token shows error message", func(t *testing.T) {
		w := env.run(0, "/web/reset-password", nil, env.frontend.ResetPassword)
		requireStatus(t, w, http.StatusOK)
		requireContains(t, w, "Please request a new reset link")
	})

	t.Run("with token renders reset form", func(t *testing.T) {
		w := env.run(0, "/web/reset-password?token=abc", nil, env.frontend.ResetPassword)
		requireStatus(t, w, http.StatusOK)
		requireContains(t, w, "Reset Password")
	})
}

func TestToJSON(t *testing.T) {
	got := toJSON(map[string]string{"a": "<b>"})
	if !strings.Contains(got, "\\u003cb\\u003e") {
		t.Errorf("toJSON() = %q, want HTML-escaped output", got)
	}
	if strings.HasSuffix(got, "\n") {
		t.Errorf("toJSON() = %q, want trailing newline trimmed", got)
	}
}

func TestProductExpiryStatus(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		expireAt time.Time
		want     string
	}{
		{"no expiry date", time.Time{}, "nodate"},
		{"already expired", now.Add(-24 * time.Hour), "expired"},
		{"critical window", now.Add(2 * 24 * time.Hour), "critical"},
		{"soon window", now.Add(5 * 24 * time.Hour), "soon"},
		{"fresh", now.Add(30 * 24 * time.Hour), "fresh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &dbModel.Product{ExpireAt: tt.expireAt}
			if got := productExpiryStatus(p, now, 3, 7); got != tt.want {
				t.Errorf("status = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterProductsByStatus(t *testing.T) {
	now := time.Now()
	products := []dbModel.Product{
		{ProductName: "expired", ExpireAt: now.Add(-24 * time.Hour)},
		{ProductName: "soon", ExpireAt: now.Add(5 * 24 * time.Hour)},
		{ProductName: "fresh", ExpireAt: now.Add(30 * 24 * time.Hour)},
	}

	filtered := filterProductsByStatus(products, "soon", now, 3, 7)
	if len(filtered) != 1 || filtered[0].ProductName != "soon" {
		t.Errorf("filterProductsByStatus(soon) = %v, want only the soon product", filtered)
	}
}
