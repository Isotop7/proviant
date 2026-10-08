package v1

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// newReceiptTestSetup wires a gin context, app context and receipt controller
// with the VLM endpoint pointing at the given test server URL. A real user is
// created because the scan path loads the user's receipt scan preferences
// (and fails closed when the user cannot be resolved).
func newReceiptTestSetup(t *testing.T, receiptEnabled bool, endpointURL string, userID uint) (*gin.Context, *httptest.ResponseRecorder, *AppContext) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	_ = userID
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	ctx, w := testutil.SetupGinContext(db)

	testutil.MockJWTClaims(ctx, user.ID)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db))

	cfg := &configuration.ProviantConfiguration{}
	cfg.Server.MaxUploadSizeMB = 5
	cfg.OCR.Receipt.Enabled = receiptEnabled
	cfg.OCR.Receipt.Provider = util.ReceiptOCRProviderOpenAI
	cfg.OCR.Receipt.APIKey = "test-key"
	cfg.OCR.Receipt.Endpoint = endpointURL
	cfg.OCR.Receipt.Model = "test-model"
	cfg.OCR.Receipt.Timeout = 5
	ctx.Set(util.ContextKeyProviantConfig, cfg)

	logger := zerolog.Nop()
	receiptCtrl := controllers.NewReceiptScanController(&logger, &cfg.OCR.Receipt)
	ctx.Set(util.ContextKeyReceiptCtrl, receiptCtrl)

	appCtx := SetupTestAppContext(ctx, user.ID)
	return ctx, w, appCtx
}

func newMultipartImageRequest(t *testing.T, fieldName, filename string) *http.Request {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	var imgBuf bytes.Buffer
	if err := png.Encode(&imgBuf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return newMultipartBytesRequest(t, fieldName, filename, imgBuf.Bytes())
}

func newMultipartBytesRequest(t *testing.T, fieldName, filename string, payload []byte) *http.Request {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(payload); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, "/api/v1/products/scan-receipt", body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// mockVLM returns an httptest server answering like an OpenAI-compatible
// chat completions endpoint with the given content.
func mockVLM(t *testing.T, status int, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization header = %q, want bearer token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": content}},
			},
		})
	}))
}

func TestScanReceipt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("disabled feature is rejected with 403", func(t *testing.T) {
		ctx, w, appCtx := newReceiptTestSetup(t, false, "", 1)
		ctx.Request = newMultipartImageRequest(t, "image", "receipt.png")

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want 403; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("missing image part is rejected with 400", func(t *testing.T) {
		ctx, w, appCtx := newReceiptTestSetup(t, true, "http://127.0.0.1:1", 1)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/products/scan-receipt", bytes.NewBufferString(""))

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("non-image upload is rejected with 400", func(t *testing.T) {
		ctx, w, appCtx := newReceiptTestSetup(t, true, "http://127.0.0.1:1", 1)
		ctx.Request = newMultipartBytesRequest(t, "image", "receipt.pdf", []byte("%PDF-1.4 not an image at all"))

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("scan timeout answers 504 even when the client timeout races the deadline", func(t *testing.T) {
		slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(3 * time.Second)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{}})
		}))
		defer slow.Close()

		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db))
		cfg := &configuration.ProviantConfiguration{}
		cfg.Server.MaxUploadSizeMB = 5
		cfg.OCR.Receipt.Enabled = true
		cfg.OCR.Receipt.Provider = util.ReceiptOCRProviderOpenAI
		cfg.OCR.Receipt.APIKey = "test-key"
		cfg.OCR.Receipt.Endpoint = slow.URL
		cfg.OCR.Receipt.Model = "test-model"
		// Client timeout and request deadline expire at the same instant:
		// the error arrives via errChan wrapped as a url.Error, not via the
		// ctxTimeout.Done() grace path.
		cfg.OCR.Receipt.Timeout = 1
		ctx.Set(util.ContextKeyProviantConfig, cfg)
		logger := zerolog.Nop()
		ctx.Set(util.ContextKeyReceiptCtrl, controllers.NewReceiptScanController(&logger, &cfg.OCR.Receipt))
		appCtx := SetupTestAppContext(ctx, user.ID)

		ctx.Request = newMultipartImageRequest(t, "image", "receipt.png")

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusGatewayTimeout {
			t.Fatalf("status = %d, want 504; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("image with lying content type is still accepted", func(t *testing.T) {
		server := mockVLM(t, http.StatusOK, `{"items":[]}`)
		defer server.Close()

		ctx, w, appCtx := newReceiptTestSetup(t, true, server.URL, 1)
		ctx.Request = newMultipartImageRequest(t, "image", "receipt.txt")

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("successful scan returns validated items", func(t *testing.T) {
		server := mockVLM(t, http.StatusOK, `{"items":[{"name":"Milk","amount":0,"unit":"l","price":2.49},{"name":"","amount":3}]}`)
		defer server.Close()

		ctx, w, appCtx := newReceiptTestSetup(t, true, server.URL, 1)
		ctx.Request = newMultipartImageRequest(t, "image", "receipt.png")

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var resp apiModel.ReceiptScanResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.Items) != 1 {
			t.Fatalf("items = %d, want 1 (empty name dropped)", len(resp.Items))
		}
		if resp.Items[0].Name != "Milk" || resp.Items[0].Amount != 1 || resp.Items[0].Unit != "l" {
			t.Errorf("item = %+v, want clamped Milk/1/l", resp.Items[0])
		}
		if resp.Items[0].Price == nil || *resp.Items[0].Price != 2.49 {
			t.Errorf("price = %v, want 2.49", resp.Items[0].Price)
		}
	})

	t.Run("nothing recognized is a successful empty scan", func(t *testing.T) {
		server := mockVLM(t, http.StatusOK, `{"items":[]}`)
		defer server.Close()

		ctx, w, appCtx := newReceiptTestSetup(t, true, server.URL, 1)
		ctx.Request = newMultipartImageRequest(t, "image", "receipt.png")

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var resp apiModel.ReceiptScanResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if len(resp.Items) != 0 {
			t.Errorf("items = %d, want 0", len(resp.Items))
		}
	})

	t.Run("endpoint failure returns 502", func(t *testing.T) {
		server := mockVLM(t, http.StatusInternalServerError, "{}")
		defer server.Close()

		ctx, w, appCtx := newReceiptTestSetup(t, true, server.URL, 1)
		ctx.Request = newMultipartImageRequest(t, "image", "receipt.png")

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("endpoint auth failure returns 502", func(t *testing.T) {
		server := mockVLM(t, http.StatusUnauthorized, `{"error":{"message":"invalid api key"}}`)
		defer server.Close()

		ctx, w, appCtx := newReceiptTestSetup(t, true, server.URL, 1)
		ctx.Request = newMultipartImageRequest(t, "image", "receipt.png")

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusBadGateway {
			t.Fatalf("status = %d, want 502; body = %s", w.Code, w.Body.String())
		}
	})

	t.Run("unparseable model output returns 500", func(t *testing.T) {
		server := mockVLM(t, http.StatusOK, "I could not read this receipt, sorry!")
		defer server.Close()

		ctx, w, appCtx := newReceiptTestSetup(t, true, server.URL, 1)
		ctx.Request = newMultipartImageRequest(t, "image", "receipt.png")

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500; body = %s", w.Code, w.Body.String())
		}
	})
}

// mockVLMStream returns an httptest server answering with an OpenAI-style
// SSE stream whose deltas assemble into the given JSON content.
func mockVLMStream(t *testing.T, content string) *httptest.Server {
	t.Helper()
	deltas := []string{`{"items":[`, `{"name":"Milk"`, `,"amount":2`, `,"unit":"l"`, `}]}`}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Stream bool `json:"stream"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &req)
		if !req.Stream {
			t.Errorf("request must ask for streaming")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			// t.Fatalf would Goexit this server goroutine, not the test
			// goroutine, hanging the test confusingly on regression.
			t.Errorf("response writer does not support flushing")
			return
		}
		for _, d := range deltas {
			payload, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{{"delta": map[string]any{"content": d}}},
			})
			_, _ = w.Write([]byte("data: " + string(payload) + "\n\n"))
			flusher.Flush()
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
}

func TestScanReceiptStreamed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("SSE stream deltas are assembled into items", func(t *testing.T) {
		server := mockVLMStream(t, `{"items":[{"name":"Milk","amount":2,"unit":"l"}]}`)
		defer server.Close()

		ctx, w, appCtx := newReceiptTestSetup(t, true, server.URL, 1)
		ctx.Request = newMultipartImageRequest(t, "image", "receipt.png")

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var resp apiModel.ReceiptScanResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.Items) != 1 || resp.Items[0].Name != "Milk" || resp.Items[0].Amount != 2 || resp.Items[0].Unit != "l" {
			t.Errorf("items = %+v, want Milk/2/l", resp.Items)
		}
	})
}

func TestParseReceiptItems(t *testing.T) {
	t.Run("strips markdown fences", func(t *testing.T) {
		items, err := controllers.ParseReceiptItems("```json\n{\"items\":[{\"name\":\"Bread\",\"amount\":1,\"unit\":\"pcs\"}]}\n```")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 || items[0].Name != "Bread" {
			t.Errorf("items = %+v, want one Bread", items)
		}
	})

	t.Run("extracts JSON embedded in prose", func(t *testing.T) {
		items, err := controllers.ParseReceiptItems(`Here you go: {"items":[{"name":"Butter","amount":2}]} hope this helps`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 || items[0].Name != "Butter" || items[0].Amount != 2 {
			t.Errorf("items = %+v, want one Butter/2", items)
		}
	})

	t.Run("clamps out-of-range values", func(t *testing.T) {
		items, err := controllers.ParseReceiptItems(`{"items":[{"name":"` + strings.Repeat("X", 300) + `","amount":5000,"unit":"kilogramm","price":-5}]}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 {
			t.Fatalf("items = %d, want 1", len(items))
		}
		if len(items[0].Name) != util.ReceiptItemMaxNameLength {
			t.Errorf("name length = %d, want %d", len(items[0].Name), util.ReceiptItemMaxNameLength)
		}
		if items[0].Amount != util.ReceiptItemMaxAmount {
			t.Errorf("amount = %d, want %d", items[0].Amount, util.ReceiptItemMaxAmount)
		}
		if items[0].Price != nil {
			t.Errorf("negative price must be dropped, got %v", items[0].Price)
		}
	})

	t.Run("no JSON object is an error", func(t *testing.T) {
		if _, err := controllers.ParseReceiptItems("no json here"); err == nil {
			t.Errorf("expected error for non-JSON output")
		}
	})

	// Exact shape glm-ocr produced in a live scan: missing closing braces and
	// commas between items, non-integer amounts, and a trailing total block.
	t.Run("repairs malformed glm-ocr output", func(t *testing.T) {
		broken := "```json\n{\"items\": [\n" +
			"    {\"name\": \"Milk 1.2 oz\", \"amount\": 1.2,\n      \"unit\": \"oz\",\n      \"price\": 1.2}\n" +
			"    {\"name\": \"Bread 1 pack 2.79\",\n      \"amount\": 2.79,\n      \"unit\": \"pack\"\n" +
			"    {\"name\": \"Butter 200g 2.98\",\n      \"amount\": 2.98,\n      \"unit\": \"g\"\n" +
			"  ],\n  \"total\": 6.97}\n```"
		items, err := controllers.ParseReceiptItems(broken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 3 {
			t.Fatalf("items = %d, want 3: %+v", len(items), items)
		}
		if items[0].Name != "Milk 1.2 oz" || items[0].Amount != 1 || items[0].Price == nil || *items[0].Price != 1.2 {
			t.Errorf("item 0 = %+v, want Milk amount=1 price=1.2", items[0])
		}
		if items[1].Name != "Bread 1 pack 2.79" || items[1].Unit != "pack" {
			t.Errorf("item 1 = %+v", items[1])
		}
	})

	t.Run("salvages complete items from truncated output", func(t *testing.T) {
		truncated := `{"items": [{"name": "Milk", "amount": 1, "unit": "l"}, {"name": "Bread", "amount": 1, "unit": "pcs"}, {"name": "Butte`
		items, err := controllers.ParseReceiptItems(truncated)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("items = %d (%+v), want 2 salvaged", len(items), items)
		}
		if items[0].Name != "Milk" || items[1].Name != "Bread" {
			t.Errorf("items = %+v", items)
		}
	})

	// A pattern like `"\s*\{` also occurs inside a string literal (name with
	// an escaped quote before a brace); repair must not corrupt it.
	t.Run("repair does not touch patterns inside string literals", func(t *testing.T) {
		broken := `{"items": [{"name": "2\" {inch hose", "amount": 1} {"name": "Bread", "amount": 1}]}`
		items, err := controllers.ParseReceiptItems(broken)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 2 {
			t.Fatalf("items = %d (%+v), want 2", len(items), items)
		}
		if items[0].Name != `2" {inch hose` {
			t.Errorf("item 0 name = %q, want %q", items[0].Name, `2" {inch hose`)
		}
		if items[1].Name != "Bread" {
			t.Errorf("item 1 name = %q, want Bread", items[1].Name)
		}
	})
}

func TestScanReceiptPerUserOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("user with override hits user endpoint with user key and model", func(t *testing.T) {
		setSSRFGuardTransport(t, http.DefaultTransport)

		var appHits, userHits int
		var userAuthHeader, userModel string

		appServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			appHits++
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{}})
		}))
		defer appServer.Close()

		userServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userHits++
			userAuthHeader = r.Header.Get("Authorization")
			var req struct {
				Model string `json:"model"`
			}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &req)
			userModel = req.Model
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]any{"content": `{"items":[{"name":"Milk","amount":1}]}`}},
				},
			})
		}))
		defer userServer.Close()

		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)
		user.ReceiptScanPreferences = authentication.ReceiptScanPreferences{
			OverrideEnabled: true,
			Endpoint:        userServer.URL,
			APIKey:          "user-key",
			Model:           "user-model",
			Timeout:         5,
		}
		if err := db.Save(user).Error; err != nil {
			t.Fatalf("save user: %v", err)
		}

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db))

		cfg := &configuration.ProviantConfiguration{}
		cfg.Server.MaxUploadSizeMB = 5
		cfg.OCR.Receipt.Enabled = true
		cfg.OCR.Receipt.Provider = util.ReceiptOCRProviderOpenAI
		cfg.OCR.Receipt.APIKey = "app-key"
		cfg.OCR.Receipt.Endpoint = appServer.URL
		cfg.OCR.Receipt.Model = "app-model"
		cfg.OCR.Receipt.Timeout = 5
		ctx.Set(util.ContextKeyProviantConfig, cfg)

		logger := zerolog.Nop()
		receiptCtrl := controllers.NewReceiptScanController(&logger, &cfg.OCR.Receipt)
		ctx.Set(util.ContextKeyReceiptCtrl, receiptCtrl)

		appCtx := SetupTestAppContext(ctx, user.ID)
		ctx.Request = newMultipartImageRequest(t, "image", "receipt.png")

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		if appHits != 0 {
			t.Errorf("app endpoint hit %d times, want 0", appHits)
		}
		if userHits != 1 {
			t.Errorf("user endpoint hit %d times, want 1", userHits)
		}
		if userAuthHeader != "Bearer user-key" {
			t.Errorf("Authorization = %q, want %q", userAuthHeader, "Bearer user-key")
		}
		if userModel != "user-model" {
			t.Errorf("model = %q, want %q", userModel, "user-model")
		}
	})

	t.Run("user without override hits app endpoint", func(t *testing.T) {
		var appHits int
		var appAuthHeader, appModel string

		appServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			appHits++
			appAuthHeader = r.Header.Get("Authorization")
			var req struct {
				Model string `json:"model"`
			}
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &req)
			appModel = req.Model
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"message": map[string]any{"content": `{"items":[]}`}},
				},
			})
		}))
		defer appServer.Close()

		db := testutil.SetupTestDB(t)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaims(ctx, user.ID)
		ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db))

		cfg := &configuration.ProviantConfiguration{}
		cfg.Server.MaxUploadSizeMB = 5
		cfg.OCR.Receipt.Enabled = true
		cfg.OCR.Receipt.Provider = util.ReceiptOCRProviderOpenAI
		cfg.OCR.Receipt.APIKey = "app-key"
		cfg.OCR.Receipt.Endpoint = appServer.URL
		cfg.OCR.Receipt.Model = "app-model"
		cfg.OCR.Receipt.Timeout = 5
		ctx.Set(util.ContextKeyProviantConfig, cfg)

		logger := zerolog.Nop()
		receiptCtrl := controllers.NewReceiptScanController(&logger, &cfg.OCR.Receipt)
		ctx.Set(util.ContextKeyReceiptCtrl, receiptCtrl)

		appCtx := SetupTestAppContext(ctx, user.ID)
		ctx.Request = newMultipartImageRequest(t, "image", "receipt.png")

		ScanReceipt(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		if appHits != 1 {
			t.Errorf("app endpoint hit %d times, want 1", appHits)
		}
		if appAuthHeader != "Bearer app-key" {
			t.Errorf("Authorization = %q, want %q", appAuthHeader, "Bearer app-key")
		}
		if appModel != "app-model" {
			t.Errorf("model = %q, want %q", appModel, "app-model")
		}
	})
}
