package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/testutil"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// setSSRFGuardTransport swaps the package-level SSRF transport for the
// duration of the test and restores it via cleanup. All access goes through
// ssrfGuardTransportMu so the swap stays race-free if tests in this package
// ever run in parallel.
func setSSRFGuardTransport(t *testing.T, transport http.RoundTripper) {
	t.Helper()
	ssrfGuardTransportMu.Lock()
	previous := ssrfGuardTransport
	ssrfGuardTransport = transport
	ssrfGuardTransportMu.Unlock()
	t.Cleanup(func() {
		ssrfGuardTransportMu.Lock()
		ssrfGuardTransport = previous
		ssrfGuardTransportMu.Unlock()
	})
}

func TestSSRFGuardDialContext(t *testing.T) {
	t.Run("loopback IPv4 blocked", func(t *testing.T) {
		_, err := ssrfGuardDialContext(context.Background(), "tcp", "127.0.0.1:443")
		if err == nil {
			t.Fatal("expected error for loopback IP")
		}
	})

	t.Run("private 10.x blocked", func(t *testing.T) {
		_, err := ssrfGuardDialContext(context.Background(), "tcp", "10.0.0.1:443")
		if err == nil {
			t.Fatal("expected error for private IP")
		}
	})

	t.Run("private 192.168.x blocked", func(t *testing.T) {
		_, err := ssrfGuardDialContext(context.Background(), "tcp", "192.168.1.1:443")
		if err == nil {
			t.Fatal("expected error for private IP")
		}
	})

	t.Run("IPv6 loopback blocked", func(t *testing.T) {
		_, err := ssrfGuardDialContext(context.Background(), "tcp", "[::1]:443")
		if err == nil {
			t.Fatal("expected error for IPv6 loopback")
		}
	})

	t.Run("link-local 169.254.x blocked", func(t *testing.T) {
		_, err := ssrfGuardDialContext(context.Background(), "tcp", "169.254.169.254:80")
		if err == nil {
			t.Fatal("expected error for link-local IP")
		}
	})

	t.Run("CG-NAT 100.64.x blocked", func(t *testing.T) {
		_, err := ssrfGuardDialContext(context.Background(), "tcp", "100.64.0.1:443")
		if err == nil {
			t.Fatal("expected error for CG-NAT IP")
		}
	})

	t.Run("IPv4-mapped IPv6 loopback blocked", func(t *testing.T) {
		_, err := ssrfGuardDialContext(context.Background(), "tcp", "[::ffff:127.0.0.1]:443")
		if err == nil {
			t.Fatal("expected error for IPv4-mapped IPv6 loopback")
		}
	})
}

func TestReceiptScanSSRFProtection(t *testing.T) {
	transport, ok := ssrfGuardTransportFor().(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatal("receipt scan transport must not use an environment proxy")
	}

	got := redactReceiptEndpoint("http://user:secret@example.com/v1?key=secret#token")
	if got != "http://example.com/v1" {
		t.Errorf("redactReceiptEndpoint() = %q, want %q", got, "http://example.com/v1")
	}
}

func TestResolveReceiptScanConfiguration(t *testing.T) {
	app := configuration.ReceiptOCRConfiguration{
		Enabled:  true,
		Provider: "openai",
		APIKey:   "app-key",
		Endpoint: "https://app.example.com/v1",
		Model:    "app-model",
		Timeout:  120,
	}

	t.Run("override off returns app config unchanged", func(t *testing.T) {
		prefs := authentication.ReceiptScanPreferences{
			OverrideEnabled: false,
			Endpoint:        "https://user.example.com/v1",
			APIKey:          "user-key",
			Model:           "user-model",
			Timeout:         300,
		}
		got := ResolveReceiptScanConfiguration(&app, prefs)
		if got != app {
			t.Errorf("got %+v, want app config %+v", got, app)
		}
	})

	t.Run("override on with all fields set", func(t *testing.T) {
		prefs := authentication.ReceiptScanPreferences{
			OverrideEnabled: true,
			Endpoint:        "https://user.example.com/v1",
			APIKey:          "user-key",
			Model:           "user-model",
			Timeout:         300,
		}
		got := ResolveReceiptScanConfiguration(&app, prefs)
		if got.Endpoint != "https://user.example.com/v1" {
			t.Errorf("Endpoint = %q", got.Endpoint)
		}
		if got.APIKey != "user-key" {
			t.Errorf("APIKey = %q", got.APIKey)
		}
		if got.Model != "user-model" {
			t.Errorf("Model = %q", got.Model)
		}
		if got.Timeout != 300 {
			t.Errorf("Timeout = %d", got.Timeout)
		}
		if got.Provider != "openai" {
			t.Errorf("Provider = %q, want app value", got.Provider)
		}
		if !got.Enabled {
			t.Error("Enabled = false, want app value true")
		}
	})

	t.Run("partial override falls back per field except the app key", func(t *testing.T) {
		prefs := authentication.ReceiptScanPreferences{
			OverrideEnabled: true,
			Endpoint:        "https://user.example.com/v1",
		}
		got := ResolveReceiptScanConfiguration(&app, prefs)
		if got.Endpoint != "https://user.example.com/v1" {
			t.Errorf("Endpoint = %q, want user value", got.Endpoint)
		}
		if got.APIKey != "" {
			t.Errorf("APIKey = %q, want empty: the app key must never be sent to a user-controlled endpoint", got.APIKey)
		}
		if got.Model != "app-model" {
			t.Errorf("Model = %q, want app fallback", got.Model)
		}
		if got.Timeout != 120 {
			t.Errorf("Timeout = %d, want app fallback 120", got.Timeout)
		}
	})

	t.Run("app key kept when user endpoint equals app endpoint", func(t *testing.T) {
		prefs := authentication.ReceiptScanPreferences{
			OverrideEnabled: true,
			Endpoint:        app.Endpoint,
			Model:           "user-model",
		}
		got := ResolveReceiptScanConfiguration(&app, prefs)
		if got.APIKey != "app-key" {
			t.Errorf("APIKey = %q, want app key: endpoint is not user-controlled", got.APIKey)
		}
		if got.Model != "user-model" {
			t.Errorf("Model = %q, want user value", got.Model)
		}
	})

	t.Run("whitespace-only fields fall back", func(t *testing.T) {
		prefs := authentication.ReceiptScanPreferences{
			OverrideEnabled: true,
			Endpoint:        "  ",
			APIKey:          " ",
			Model:           " ",
			Timeout:         0,
		}
		got := ResolveReceiptScanConfiguration(&app, prefs)
		if got.Endpoint != app.Endpoint {
			t.Errorf("Endpoint = %q, want app fallback", got.Endpoint)
		}
		if got.APIKey != app.APIKey {
			t.Errorf("APIKey = %q, want app fallback", got.APIKey)
		}
		if got.Model != app.Model {
			t.Errorf("Model = %q, want app fallback", got.Model)
		}
		if got.Timeout != app.Timeout {
			t.Errorf("Timeout = %d, want app fallback", got.Timeout)
		}
	})
}

func setupReceiptScanTestContext(t *testing.T, userID uint) (*gin.Context, *httptest.ResponseRecorder, *AppContext) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	ctx, w := testutil.SetupGinContext(db)
	testutil.MockJWTClaims(ctx, userID)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))

	cfg := &configuration.ProviantConfiguration{}
	cfg.OCR.Receipt.Enabled = true
	cfg.OCR.Receipt.Provider = util.ReceiptOCRProviderOpenAI
	cfg.OCR.Receipt.APIKey = "app-key"
	cfg.OCR.Receipt.Endpoint = "https://app.example.com/v1"
	cfg.OCR.Receipt.Model = "app-model"
	cfg.OCR.Receipt.Timeout = 120
	ctx.Set(util.ContextKeyProviantConfig, cfg)

	appCtx := SetupTestAppContext(ctx, userID)
	return ctx, w, appCtx
}

func TestGetUserReceiptScanSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("returns defaults and no raw keys", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		user.ReceiptScanPreferences = authentication.ReceiptScanPreferences{
			OverrideEnabled: true,
			Endpoint:        "https://user.example.com/v1",
			APIKey:          "user-secret-key",
			Model:           "user-model",
			Timeout:         300,
		}
		if err := db.Save(user).Error; err != nil {
			t.Fatalf("save user: %v", err)
		}

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))

		cfg := &configuration.ProviantConfiguration{}
		cfg.OCR.Receipt.Enabled = true
		cfg.OCR.Receipt.Provider = util.ReceiptOCRProviderOpenAI
		cfg.OCR.Receipt.APIKey = "app-secret-key"
		cfg.OCR.Receipt.Endpoint = "https://app.example.com/v1"
		cfg.OCR.Receipt.Model = "app-model"
		cfg.OCR.Receipt.Timeout = 120
		ctx.Set(util.ContextKeyProviantConfig, cfg)

		appCtx := SetupTestAppContext(ctx, user.ID)

		GetUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}

		var resp receiptScanSettingsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if !resp.OverrideEnabled {
			t.Error("OverrideEnabled = false, want true")
		}
		if resp.Endpoint != "https://user.example.com/v1" {
			t.Errorf("Endpoint = %q", resp.Endpoint)
		}
		if resp.Model != "user-model" {
			t.Errorf("Model = %q", resp.Model)
		}
		if resp.Timeout != 300 {
			t.Errorf("Timeout = %d", resp.Timeout)
		}
		if !resp.APIKeyConfigured {
			t.Error("APIKeyConfigured = false, want true")
		}
		if resp.Defaults.Endpoint != "https://app.example.com/v1" {
			t.Errorf("Defaults.Endpoint = %q", resp.Defaults.Endpoint)
		}
		if resp.Defaults.Model != "app-model" {
			t.Errorf("Defaults.Model = %q", resp.Defaults.Model)
		}
		if resp.Defaults.Timeout != 120 {
			t.Errorf("Defaults.Timeout = %d", resp.Defaults.Timeout)
		}
		if !resp.Defaults.APIKeyConfigured {
			t.Error("Defaults.APIKeyConfigured = false, want true")
		}

		body := w.Body.String()
		if bytes.Contains([]byte(body), []byte("user-secret-key")) {
			t.Error("response body contains user API key")
		}
		if bytes.Contains([]byte(body), []byte("app-secret-key")) {
			t.Error("response body contains app API key")
		}
	})

	t.Run("unset app timeout defaults to ReceiptScanDefaultTimeout", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))

		cfg := &configuration.ProviantConfiguration{}
		cfg.OCR.Receipt.Enabled = true
		ctx.Set(util.ContextKeyProviantConfig, cfg)

		appCtx := SetupTestAppContext(ctx, user.ID)

		GetUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}

		var resp receiptScanSettingsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Defaults.Timeout != util.ReceiptScanDefaultTimeout {
			t.Errorf("Defaults.Timeout = %d, want %d", resp.Defaults.Timeout, util.ReceiptScanDefaultTimeout)
		}
	})
}

func TestReceiptScanEffectiveTimeout(t *testing.T) {
	if got := ReceiptScanEffectiveTimeout(0); got != util.ReceiptScanDefaultTimeout {
		t.Errorf("ReceiptScanEffectiveTimeout(0) = %d, want %d", got, util.ReceiptScanDefaultTimeout)
	}
	if got := ReceiptScanEffectiveTimeout(-5); got != util.ReceiptScanDefaultTimeout {
		t.Errorf("ReceiptScanEffectiveTimeout(-5) = %d, want %d", got, util.ReceiptScanDefaultTimeout)
	}
	if got := ReceiptScanEffectiveTimeout(300); got != 300 {
		t.Errorf("ReceiptScanEffectiveTimeout(300) = %d, want 300", got)
	}
}

func TestUpdateUserReceiptScanSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("valid settings persist", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))

		cfg := &configuration.ProviantConfiguration{}
		cfg.OCR.Receipt.Enabled = true
		ctx.Set(util.ContextKeyProviantConfig, cfg)

		appCtx := SetupTestAppContext(ctx, user.ID)

		reqBody := receiptScanSettingsRequest{
			OverrideEnabled: true,
			Endpoint:        "https://example.com/v1",
			APIKey:          "user-key",
			Model:           "user-model",
			Timeout:         300,
		}
		testutil.CreateTestRequest(ctx, reqBody)
		UpdateUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}

		var reloaded authentication.User
		if err := db.First(&reloaded, user.ID).Error; err != nil {
			t.Fatalf("reload: %v", err)
		}
		if !reloaded.ReceiptScanPreferences.OverrideEnabled {
			t.Error("OverrideEnabled not persisted")
		}
		if reloaded.ReceiptScanPreferences.Endpoint != "https://example.com/v1" {
			t.Errorf("Endpoint = %q", reloaded.ReceiptScanPreferences.Endpoint)
		}
		if reloaded.ReceiptScanPreferences.APIKey != "user-key" {
			t.Errorf("APIKey = %q", reloaded.ReceiptScanPreferences.APIKey)
		}
		if reloaded.ReceiptScanPreferences.Model != "user-model" {
			t.Errorf("Model = %q", reloaded.ReceiptScanPreferences.Model)
		}
		if reloaded.ReceiptScanPreferences.Timeout != 300 {
			t.Errorf("Timeout = %d", reloaded.ReceiptScanPreferences.Timeout)
		}
	})

	t.Run("empty apiKey keeps stored key", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		user.ReceiptScanPreferences = authentication.ReceiptScanPreferences{
			OverrideEnabled: true,
			APIKey:          "existing-key",
		}
		if err := db.Save(user).Error; err != nil {
			t.Fatalf("save: %v", err)
		}

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
		cfg := &configuration.ProviantConfiguration{}
		ctx.Set(util.ContextKeyProviantConfig, cfg)
		appCtx := SetupTestAppContext(ctx, user.ID)

		reqBody := receiptScanSettingsRequest{
			OverrideEnabled: true,
			APIKey:          "",
		}
		testutil.CreateTestRequest(ctx, reqBody)
		UpdateUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}

		var reloaded authentication.User
		if err := db.First(&reloaded, user.ID).Error; err != nil {
			t.Fatalf("reload: %v", err)
		}
		if reloaded.ReceiptScanPreferences.APIKey != "existing-key" {
			t.Errorf("APIKey = %q, want %q (kept)", reloaded.ReceiptScanPreferences.APIKey, "existing-key")
		}
	})

	t.Run("clearApiKey wipes stored key", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		user.ReceiptScanPreferences = authentication.ReceiptScanPreferences{
			OverrideEnabled: true,
			APIKey:          "existing-key",
		}
		if err := db.Save(user).Error; err != nil {
			t.Fatalf("save: %v", err)
		}

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
		cfg := &configuration.ProviantConfiguration{}
		ctx.Set(util.ContextKeyProviantConfig, cfg)
		appCtx := SetupTestAppContext(ctx, user.ID)

		reqBody := receiptScanSettingsRequest{
			OverrideEnabled: true,
			ClearAPIKey:     true,
		}
		testutil.CreateTestRequest(ctx, reqBody)
		UpdateUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}

		var reloaded authentication.User
		if err := db.First(&reloaded, user.ID).Error; err != nil {
			t.Fatalf("reload: %v", err)
		}
		if reloaded.ReceiptScanPreferences.APIKey != "" {
			t.Errorf("APIKey = %q, want empty (cleared)", reloaded.ReceiptScanPreferences.APIKey)
		}
	})

	t.Run("invalid endpoint scheme rejected", func(t *testing.T) {
		ctx, w, appCtx := setupReceiptScanTestContext(t, 1)

		reqBody := receiptScanSettingsRequest{
			OverrideEnabled: true,
			Endpoint:        "ftp://example.com",
		}
		testutil.CreateTestRequest(ctx, reqBody)
		UpdateUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("endpoint without host rejected", func(t *testing.T) {
		ctx, w, appCtx := setupReceiptScanTestContext(t, 1)

		reqBody := receiptScanSettingsRequest{
			OverrideEnabled: true,
			Endpoint:        "https://",
		}
		testutil.CreateTestRequest(ctx, reqBody)
		UpdateUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("loopback IP rejected", func(t *testing.T) {
		ctx, w, appCtx := setupReceiptScanTestContext(t, 1)

		reqBody := receiptScanSettingsRequest{
			OverrideEnabled: true,
			Endpoint:        "https://127.0.0.1/v1",
		}
		testutil.CreateTestRequest(ctx, reqBody)
		UpdateUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("private 10.x IP rejected", func(t *testing.T) {
		ctx, w, appCtx := setupReceiptScanTestContext(t, 1)

		reqBody := receiptScanSettingsRequest{
			OverrideEnabled: true,
			Endpoint:        "https://10.0.0.1/v1",
		}
		testutil.CreateTestRequest(ctx, reqBody)
		UpdateUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("private 192.168.x IP rejected", func(t *testing.T) {
		ctx, w, appCtx := setupReceiptScanTestContext(t, 1)

		reqBody := receiptScanSettingsRequest{
			OverrideEnabled: true,
			Endpoint:        "https://192.168.1.1/v1",
		}
		testutil.CreateTestRequest(ctx, reqBody)
		UpdateUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("IPv6 loopback rejected", func(t *testing.T) {
		ctx, w, appCtx := setupReceiptScanTestContext(t, 1)

		reqBody := receiptScanSettingsRequest{
			OverrideEnabled: true,
			Endpoint:        "https://[::1]/v1",
		}
		testutil.CreateTestRequest(ctx, reqBody)
		UpdateUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("link-local 169.254.x rejected", func(t *testing.T) {
		ctx, w, appCtx := setupReceiptScanTestContext(t, 1)

		reqBody := receiptScanSettingsRequest{
			OverrideEnabled: true,
			Endpoint:        "https://169.254.169.254/latest/meta-data",
		}
		testutil.CreateTestRequest(ctx, reqBody)
		UpdateUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("timeout out of range rejected", func(t *testing.T) {
		ctx, w, appCtx := setupReceiptScanTestContext(t, 1)

		reqBody := receiptScanSettingsRequest{
			OverrideEnabled: true,
			Timeout:         901,
		}
		testutil.CreateTestRequest(ctx, reqBody)
		UpdateUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("negative timeout rejected", func(t *testing.T) {
		ctx, w, appCtx := setupReceiptScanTestContext(t, 1)

		reqBody := receiptScanSettingsRequest{
			OverrideEnabled: true,
			Timeout:         -1,
		}
		testutil.CreateTestRequest(ctx, reqBody)
		UpdateUserReceiptScanSettings(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
	})
}

func TestReceiptScanControllerForUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("non-endpoint override keeps app transport", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		user.ReceiptScanPreferences = authentication.ReceiptScanPreferences{
			OverrideEnabled: true,
			Model:           "user-model",
		}
		if err := db.Save(user).Error; err != nil {
			t.Fatalf("save: %v", err)
		}

		ctx, _ := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))

		logger := zerolog.Nop()
		cfg := &configuration.ProviantConfiguration{}
		cfg.OCR.Receipt.Enabled = true
		cfg.OCR.Receipt.Provider = util.ReceiptOCRProviderOpenAI
		cfg.OCR.Receipt.Endpoint = "http://127.0.0.1:11434/v1"
		cfg.OCR.Receipt.Model = "app-model"
		ctx.Set(util.ContextKeyProviantConfig, cfg)

		base := controllers.NewReceiptScanController(&logger, &cfg.OCR.Receipt)
		appCtx := SetupTestAppContext(ctx, user.ID)

		got, err := receiptScanControllerForUser(ctx, appCtx, base)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		impl := got.(*controllers.ReceiptScanControllerImpl)
		if impl.Config.Model != "user-model" {
			t.Errorf("Model = %q, want user-model", impl.Config.Model)
		}
		if impl.Client.Transport != base.Client.Transport {
			t.Error("non-endpoint override must preserve the app-configured transport")
		}
	})

	t.Run("user without override gets base controller unchanged", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		ctx, _ := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))

		logger := zerolog.Nop()
		cfg := &configuration.ProviantConfiguration{}
		cfg.OCR.Receipt.Enabled = true
		cfg.OCR.Receipt.Provider = util.ReceiptOCRProviderOpenAI
		cfg.OCR.Receipt.APIKey = "app-key"
		cfg.OCR.Receipt.Endpoint = "https://app.example.com/v1"
		cfg.OCR.Receipt.Model = "app-model"
		cfg.OCR.Receipt.Timeout = 120
		ctx.Set(util.ContextKeyProviantConfig, cfg)

		base := controllers.NewReceiptScanController(&logger, &cfg.OCR.Receipt)
		ctx.Set(util.ContextKeyReceiptCtrl, base)

		appCtx := SetupTestAppContext(ctx, user.ID)

		got, gotErr := receiptScanControllerForUser(ctx, appCtx, base)
		if gotErr != nil {
			t.Fatalf("unexpected error: %v", gotErr)
		}
		if got != controllers.ReceiptScanController(base) {
			t.Error("expected base controller to be returned unchanged")
		}
	})

	t.Run("user with override gets cloned controller with effective config", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		user.ReceiptScanPreferences = authentication.ReceiptScanPreferences{
			OverrideEnabled: true,
			Endpoint:        "https://user.example.com/v1",
			APIKey:          "user-key",
			Model:           "user-model",
			Timeout:         300,
		}
		if err := db.Save(user).Error; err != nil {
			t.Fatalf("save: %v", err)
		}

		ctx, _ := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))

		logger := zerolog.Nop()
		cfg := &configuration.ProviantConfiguration{}
		cfg.OCR.Receipt.Enabled = true
		cfg.OCR.Receipt.Provider = util.ReceiptOCRProviderOpenAI
		cfg.OCR.Receipt.APIKey = "app-key"
		cfg.OCR.Receipt.Endpoint = "https://app.example.com/v1"
		cfg.OCR.Receipt.Model = "app-model"
		cfg.OCR.Receipt.Timeout = 120
		ctx.Set(util.ContextKeyProviantConfig, cfg)

		base := controllers.NewReceiptScanController(&logger, &cfg.OCR.Receipt)
		ctx.Set(util.ContextKeyReceiptCtrl, base)

		appCtx := SetupTestAppContext(ctx, user.ID)

		got, gotErr := receiptScanControllerForUser(ctx, appCtx, base)
		if gotErr != nil {
			t.Fatalf("unexpected error: %v", gotErr)
		}
		impl, ok := got.(*controllers.ReceiptScanControllerImpl)
		if !ok {
			t.Fatalf("expected *ReceiptScanControllerImpl, got %T", got)
		}
		if impl.Config.Endpoint != "https://user.example.com/v1" {
			t.Errorf("Endpoint = %q", impl.Config.Endpoint)
		}
		if impl.Config.APIKey != "user-key" {
			t.Errorf("APIKey = %q", impl.Config.APIKey)
		}
		if impl.Config.Model != "user-model" {
			t.Errorf("Model = %q", impl.Config.Model)
		}
		if impl.Config.Timeout != 300 {
			t.Errorf("Timeout = %d", impl.Config.Timeout)
		}
		if impl.Config.Provider != "openai" {
			t.Errorf("Provider = %q, want app value", impl.Config.Provider)
		}
		if impl.Client.Transport != ssrfGuardTransportFor() {
			t.Error("expected ssrfGuardTransport on per-user client")
		}
	})

	t.Run("non-impl controller passes through unchanged", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		user.ReceiptScanPreferences = authentication.ReceiptScanPreferences{
			OverrideEnabled: true,
			Endpoint:        "https://user.example.com/v1",
		}
		if err := db.Save(user).Error; err != nil {
			t.Fatalf("save: %v", err)
		}

		ctx, _ := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))

		cfg := &configuration.ProviantConfiguration{}
		cfg.OCR.Receipt.Enabled = true
		cfg.OCR.Receipt.Provider = util.ReceiptOCRProviderOpenAI
		cfg.OCR.Receipt.APIKey = "app-key"
		cfg.OCR.Receipt.Endpoint = "https://app.example.com/v1"
		cfg.OCR.Receipt.Model = "app-model"
		cfg.OCR.Receipt.Timeout = 120
		ctx.Set(util.ContextKeyProviantConfig, cfg)

		mock := &mockReceiptScanController{}
		ctx.Set(util.ContextKeyReceiptCtrl, mock)

		appCtx := SetupTestAppContext(ctx, user.ID)

		got, gotErr := receiptScanControllerForUser(ctx, appCtx, mock)
		if gotErr != nil {
			t.Fatalf("unexpected error: %v", gotErr)
		}
		if got != controllers.ReceiptScanController(mock) {
			t.Error("expected mock controller to pass through unchanged")
		}
	})

	t.Run("user lookup failure fails closed instead of using app config", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		_ = household

		ctx, _ := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, 9999)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))

		cfg := &configuration.ProviantConfiguration{}
		cfg.OCR.Receipt.Enabled = true
		ctx.Set(util.ContextKeyProviantConfig, cfg)

		logger := zerolog.Nop()
		base := controllers.NewReceiptScanController(&logger, &cfg.OCR.Receipt)
		ctx.Set(util.ContextKeyReceiptCtrl, base)

		appCtx := SetupTestAppContext(ctx, 9999)

		got, gotErr := receiptScanControllerForUser(ctx, appCtx, base)
		if gotErr == nil {
			t.Fatal("expected error for missing user, got nil")
		}
		if got != nil {
			t.Error("expected nil controller on lookup failure")
		}
	})
}

type mockReceiptScanController struct{}

func (m *mockReceiptScanController) ScanReceipt(_ context.Context, _ []byte) (*apiModel.ReceiptScanResponse, error) {
	return &apiModel.ReceiptScanResponse{}, nil
}
