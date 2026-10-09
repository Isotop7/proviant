package v1

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/controllers"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	"codeberg.org/isotop7/proviant/util"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/rs/zerolog"
)

// setupOCRTest returns an env whose context carries a proviant config with the
// given upload limit and, unless skipOCR is true, a real OCR controller.
func setupOCRTest(t *testing.T, maxUploadMB int, withOCRController bool) *handlerTestEnv {
	t.Helper()
	env := setupHandlerTest(t)
	config := &configuration.ProviantConfiguration{}
	config.Server.MaxUploadSizeMB = maxUploadMB
	env.Ctx.Set(util.ContextKeyProviantConfig, config)
	if withOCRController {
		logger := zerolog.Nop()
		env.Ctx.Set("ocrController", controllers.NewOCRController(&logger, &configuration.OCRConfiguration{
			Timeout:   30,
			Languages: "eng",
		}))
	}
	return env
}

func TestScanExpiryDate(t *testing.T) {
	t.Run("missing file returns 400", func(t *testing.T) {
		env := setupOCRTest(t, 10, true)
		env.Ctx.Request = newRawJSONRequest("{}")

		ScanExpiryDate(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("oversized file returns 400", func(t *testing.T) {
		env := setupOCRTest(t, 0, true)
		env.Ctx.Request = multipartImageRequest(t, "image", "pack.png", pngWithText(t, "MHD 31.12.2027"))

		ScanExpiryDate(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("wrong-typed OCR controller returns 500", func(t *testing.T) {
		env := setupOCRTest(t, 10, false)
		env.Ctx.Set("ocrController", "not-a-controller")
		env.Ctx.Request = multipartImageRequest(t, "image", "pack.png", pngWithText(t, "MHD 31.12.2027"))

		ScanExpiryDate(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", env.W.Code)
		}
	})

	t.Run("undecodable image returns 500", func(t *testing.T) {
		env := setupOCRTest(t, 10, true)
		env.Ctx.Request = multipartImageRequest(t, "image", "pack.png", []byte("this is not an image"))

		ScanExpiryDate(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", env.W.Code)
		}
	})

	t.Run("scans expiry date from image", func(t *testing.T) {
		if _, err := exec.LookPath("tesseract"); err != nil {
			t.Skip("tesseract not installed")
		}
		env := setupOCRTest(t, 10, true)
		env.Ctx.Request = multipartImageRequest(t, "image", "pack.png", pngWithText(t, "MHD 31.12.2027"))

		ScanExpiryDate(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.ExpiryScanResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		// Tesseract must have run, recognized the text, and the parser must
		// have turned "MHD 31.12.2027" into an ISO date.
		if resp.RawText == "" {
			t.Errorf("rawText empty, want OCR output")
		}
		if resp.DetectedDate != "2027-12-31" {
			t.Errorf("detectedDate = %q, want 2027-12-31 (raw text %q)", resp.DetectedDate, resp.RawText)
		}
	})
}

func TestLogExpiryScan(t *testing.T) {
	t.Run("stores scan record", func(t *testing.T) {
		env := setupHandlerTest(t)
		logger := zerolog.Nop()
		resp := &apiModel.ExpiryScanResponse{DetectedDate: "2027-12-31", Confidence: 0.9, RawText: "MHD 31.12.2027"}

		logExpiryScan(env.Ctx, &logger, env.User.ID, resp, []byte("fake-image"))

		var scans []dbModel.ExpiryScan
		if err := env.DB.Where("user_id = ?", env.User.ID).Find(&scans).Error; err != nil {
			t.Fatalf("load scans: %v", err)
		}
		if len(scans) != 1 {
			t.Fatalf("scans = %d, want 1", len(scans))
		}
		scan := scans[0]
		if scan.DetectedDate.Format("2006-01-02") != "2027-12-31" {
			t.Errorf("detectedDate = %v, want 2027-12-31", scan.DetectedDate)
		}
		if scan.Confidence != 0.9 || scan.RawText != "MHD 31.12.2027" {
			t.Errorf("scan = %+v, want confidence 0.9 and raw text", scan)
		}
		if scan.ImageHash != hashImage([]byte("fake-image")) {
			t.Errorf("imageHash = %q, want sha256 of image", scan.ImageHash)
		}
	})

	t.Run("no repos in context is a no-op", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Set(util.ContextKeyRepos, "not-repos")
		logger := zerolog.Nop()
		resp := &apiModel.ExpiryScanResponse{DetectedDate: "2027-12-31"}

		logExpiryScan(env.Ctx, &logger, env.User.ID, resp, nil)

		var count int64
		env.DB.Model(&dbModel.ExpiryScan{}).Count(&count)
		if count != 0 {
			t.Errorf("scans = %d, want 0", count)
		}
	})
}

func TestGetCurrentUserID(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("PAT context value wins", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Set(util.ContextKeyUserID, uint(7))
		id, ok := getCurrentUserID(env.Ctx, &logger)
		if !ok || id != 7 {
			t.Errorf("id = %d, ok = %v, want 7, true", id, ok)
		}
	})

	t.Run("JWT claim is used", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.MockJWTClaims(env.Ctx, 42)
		id, ok := getCurrentUserID(env.Ctx, &logger)
		if !ok || id != 42 {
			t.Errorf("id = %d, ok = %v, want 42, true", id, ok)
		}
	})

	t.Run("missing claim fails", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Keys = map[any]any{}
		if _, ok := getCurrentUserID(env.Ctx, &logger); ok {
			t.Error("ok = true, want false")
		}
	})

	t.Run("non-numeric claim fails", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Set("JWT_PAYLOAD", jwt.MapClaims{"id": "nope"})
		if _, ok := getCurrentUserID(env.Ctx, &logger); ok {
			t.Error("ok = true, want false")
		}
	})

	t.Run("zero user id fails", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.MockJWTClaims(env.Ctx, 0)
		if _, ok := getCurrentUserID(env.Ctx, &logger); ok {
			t.Error("ok = true, want false")
		}
	})
}

func TestHashImage(t *testing.T) {
	hash := hashImage([]byte("hello"))
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64 hex chars", len(hash))
	}
	if hash != hashImage([]byte("hello")) {
		t.Errorf("hash not deterministic")
	}
	if hash == hashImage([]byte("world")) {
		t.Errorf("different inputs must hash differently")
	}
}

func TestMustParseDate(t *testing.T) {
	if got := mustParseDate("2027-12-31"); got.Format("2006-01-02") != "2027-12-31" {
		t.Errorf("mustParseDate = %v, want 2027-12-31", got)
	}
	if got := mustParseDate("garbage"); !got.Equal(time.Time{}) {
		t.Errorf("mustParseDate(garbage) = %v, want zero time", got)
	}
}
