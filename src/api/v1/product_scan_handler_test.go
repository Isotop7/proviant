package v1

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func setupScanTest(t *testing.T, maxUploadMB int) *handlerTestEnv {
	t.Helper()
	env := setupHandlerTest(t)
	config := &configuration.ProviantConfiguration{}
	config.Server.MaxUploadSizeMB = maxUploadMB
	env.Ctx.Set(util.ContextKeyProviantConfig, config)
	return env
}

func TestScanProduct(t *testing.T) {
	t.Run("missing file returns 400", func(t *testing.T) {
		env := setupScanTest(t, 10)
		env.Ctx.Request = newRawJSONRequest("{}")

		ScanProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("oversized file returns 400", func(t *testing.T) {
		env := setupScanTest(t, 0)
		env.Ctx.Request = multipartImageRequest(t, "image", "code.png", blankPNG(t))

		ScanProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("undecodable image returns 400", func(t *testing.T) {
		env := setupScanTest(t, 10)
		env.Ctx.Request = multipartImageRequest(t, "image", "code.png", []byte("not an image"))

		ScanProduct(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

}

// offTestServer returns a canned OpenFoodFacts product response.
func offTestServer(t *testing.T, barcode string, calls *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		if !strings.Contains(r.URL.Path, barcode) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": "` + barcode + `",
			"product": {
				"_id": "` + barcode + `",
				"product_name": "Test Milk",
				"categories": "Dairy",
				"countries": "Germany",
				"image_url": "",
				"ecoscore_data": {"agribalyse": {"co2_total": 1.5}},
				"conservation_conditions": "keep cold"
			}
		}`))
	}))
}

func setupOFFController(env *handlerTestEnv, url string, cacheEnabled bool) {
	logger := zerolog.Nop()
	env.Ctx.Set("offacntrl", &controllers.OpenFoodFactsAPIController{
		Logger: &logger,
		Configuration: configuration.OpenFoodFactsConfiguration{
			URL:          url,
			Timeout:      5,
			CacheEnabled: cacheEnabled,
		},
	})
}

func TestGetOpenFoodFactsData(t *testing.T) {
	barcode := "1234567890123"

	t.Run("missing barcode returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		setupOFFController(env, "http://127.0.0.1:1", false)

		GetOpenFoodFactsData(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("wrong-typed controller returns 500", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Set("offacntrl", "not-a-controller")
		env.Ctx.Params = []gin.Param{{Key: "barcode", Value: barcode}}

		GetOpenFoodFactsData(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", env.W.Code)
		}
	})

	t.Run("cache hit skips API", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.DB.Create(&dbModel.OpenFoodFactsCache{
			Barcode:     barcode,
			ProductName: "Cached Milk",
			Categories:  "Dairy",
		})
		calls := 0
		server := offTestServer(t, barcode, &calls)
		defer server.Close()
		setupOFFController(env, server.URL, true)
		env.Ctx.Params = []gin.Param{{Key: "barcode", Value: barcode}}

		GetOpenFoodFactsData(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var cached dbModel.OpenFoodFactsCache
		if err := json.Unmarshal(env.W.Body.Bytes(), &cached); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if cached.ProductName != "Cached Milk" {
			t.Errorf("productName = %q, want Cached Milk", cached.ProductName)
		}
		if calls != 0 {
			t.Errorf("API calls = %d, want 0 on cache hit", calls)
		}
	})

	t.Run("fetches from API and stores cache", func(t *testing.T) {
		env := setupHandlerTest(t)
		calls := 0
		server := offTestServer(t, barcode, &calls)
		defer server.Close()
		setupOFFController(env, server.URL, true)
		env.Ctx.Params = []gin.Param{{Key: "barcode", Value: barcode}}

		GetOpenFoodFactsData(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var entry dbModel.OpenFoodFactsCache
		if err := json.Unmarshal(env.W.Body.Bytes(), &entry); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if entry.ProductName != "Test Milk" || entry.Categories != "Dairy" || entry.Countries != "Germany" {
			t.Errorf("entry = %+v, want Test Milk/Dairy/Germany", entry)
		}
		if entry.CO2KgPerKg == nil || *entry.CO2KgPerKg != 1.5 {
			t.Errorf("co2 = %v, want 1.5", entry.CO2KgPerKg)
		}
		if entry.StorageHint != "keep cold" {
			t.Errorf("storageHint = %q, want 'keep cold'", entry.StorageHint)
		}
		if calls != 1 {
			t.Errorf("API calls = %d, want 1", calls)
		}
		var stored dbModel.OpenFoodFactsCache
		if err := env.DB.Where("barcode = ?", barcode).First(&stored).Error; err != nil {
			t.Fatalf("cache entry not stored: %v", err)
		}
		if stored.ProductName != "Test Milk" {
			t.Errorf("stored = %+v, want Test Milk", stored)
		}
	})

	t.Run("API timeout returns 502", func(t *testing.T) {
		env := setupHandlerTest(t)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()
		logger := zerolog.Nop()
		env.Ctx.Set("offacntrl", &controllers.OpenFoodFactsAPIController{
			Logger: &logger,
			Configuration: configuration.OpenFoodFactsConfiguration{
				URL:     server.URL,
				Timeout: 1,
			},
		})
		env.Ctx.Params = []gin.Param{{Key: "barcode", Value: barcode}}

		GetOpenFoodFactsData(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadGateway {
			t.Errorf("status = %d, want 502", env.W.Code)
		}
	})
}

func TestGetOpenFoodFactsDataImageCache(t *testing.T) {
	barcode := "5555555555555"
	env := setupHandlerTest(t)
	imageBytes := blankPNG(t)
	var imageCalls int
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/img.jpg") {
			imageCalls++
			_, _ = w.Write(imageBytes)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": "` + barcode + `",
			"product": {
				"product_name": "Imaged Yogurt",
				"categories": "Dairy",
				"image_url": "` + server.URL + `/img.jpg"
			}
		}`))
	}))
	defer server.Close()

	logger := zerolog.Nop()
	env.Ctx.Set("offacntrl", &controllers.OpenFoodFactsAPIController{
		Logger: &logger,
		Configuration: configuration.OpenFoodFactsConfiguration{
			URL:               server.URL,
			Timeout:           5,
			CacheEnabled:      true,
			ImageCacheEnabled: true,
			ImageCachePath:    t.TempDir(),
		},
	})
	env.Ctx.Params = []gin.Param{{Key: "barcode", Value: barcode}}

	GetOpenFoodFactsData(env.Ctx, env.AppCtx)

	if env.W.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
	}
	if imageCalls != 1 {
		t.Errorf("image calls = %d, want 1", imageCalls)
	}
	var stored dbModel.OpenFoodFactsCache
	if err := env.DB.Where("barcode = ?", barcode).First(&stored).Error; err != nil {
		t.Fatalf("cache entry not stored: %v", err)
	}
	if !strings.Contains(stored.ImageURL, "product-images") {
		t.Errorf("stored imageURL = %q, want local product-images path", stored.ImageURL)
	}
}
