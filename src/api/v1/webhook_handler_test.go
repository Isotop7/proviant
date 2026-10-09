package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"

	"github.com/gin-gonic/gin"
)

// publicTestWebhookURL uses a public IP literal so validation needs no DNS.
const publicTestWebhookURL = "https://93.184.216.34/hook"

func TestCreateWebhookHandler(t *testing.T) {
	t.Run("creates webhook", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, apiModel.CreateWebhookRequest{
			URL:    publicTestWebhookURL,
			Secret: "secret-with-16-chars",
			Events: []string{"product.created", "product.expired"},
		})

		CreateWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.WebhookResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.ID == 0 || resp.URL != publicTestWebhookURL || !resp.Active {
			t.Errorf("response = %+v, want created active webhook", resp)
		}
		if len(resp.Events) != 2 {
			t.Errorf("events = %v, want 2", resp.Events)
		}
		var stored dbModel.Webhook
		if err := env.DB.First(&stored, resp.ID).Error; err != nil {
			t.Fatalf("load webhook: %v", err)
		}
		if stored.UserID != env.User.ID || stored.Secret != "secret-with-16-chars" {
			t.Errorf("stored = %+v, want user %d", stored, env.User.ID)
		}
	})

	t.Run("inactive flag is honored", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, apiModel.CreateWebhookRequest{
			URL:    publicTestWebhookURL,
			Secret: "secret-with-16-chars",
			Events: []string{"product.created"},
			Active: new(bool),
		})

		CreateWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.WebhookResponse
		_ = json.Unmarshal(env.W.Body.Bytes(), &resp)
		if resp.Active {
			t.Errorf("active = true, want false")
		}
		stored, err := env.AppCtx.Repos.Webhooks.GetWebhookByID(resp.ID)
		if err != nil {
			t.Fatalf("load webhook: %v", err)
		}
		if stored.Active {
			t.Errorf("stored active = true, want false")
		}
	})

	t.Run("missing fields return 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, apiModel.CreateWebhookRequest{URL: publicTestWebhookURL})

		CreateWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("short secret returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, apiModel.CreateWebhookRequest{
			URL:    publicTestWebhookURL,
			Secret: "short",
			Events: []string{"product.created"},
		})

		CreateWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("http URL returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, apiModel.CreateWebhookRequest{
			URL:    "http://93.184.216.34/hook",
			Secret: "secret-with-16-chars",
			Events: []string{"product.created"},
		})

		CreateWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("private IP URL returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, apiModel.CreateWebhookRequest{
			URL:    "https://192.168.1.1/hook",
			Secret: "secret-with-16-chars",
			Events: []string{"product.created"},
		})

		CreateWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("invalid event returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, apiModel.CreateWebhookRequest{
			URL:    publicTestWebhookURL,
			Secret: "secret-with-16-chars",
			Events: []string{"product.created", "not.an.event"},
		})

		CreateWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Webhooks.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaims(ctx, 1)
		testutil.CreateTestRequest(ctx, apiModel.CreateWebhookRequest{
			URL:    publicTestWebhookURL,
			Secret: "secret-with-16-chars",
			Events: []string{"product.created"},
		})
		appCtx := SetupTestAppContext(ctx, 1)

		CreateWebhook(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}

func TestGetWebhookHandler(t *testing.T) {
	t.Run("returns webhook", func(t *testing.T) {
		env := setupHandlerTest(t)
		webhook := testutil.CreateTestWebhook(env.DB, env.User.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(webhook.ID)}}

		GetWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.WebhookResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.ID != webhook.ID || resp.URL != webhook.URL {
			t.Errorf("response = %+v, want webhook %d", resp, webhook.ID)
		}
	})

	t.Run("webhook of another user returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		other := env.addMember(t, "other")
		webhook := testutil.CreateTestWebhook(env.DB, other.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(webhook.ID)}}

		GetWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("missing webhook returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "9999"}}

		GetWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "abc"}}

		GetWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestUpdateWebhookHandler(t *testing.T) {
	t.Run("updates fields", func(t *testing.T) {
		env := setupHandlerTest(t)
		webhook := testutil.CreateTestWebhook(env.DB, env.User.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(webhook.ID)}}
		active := false
		testutil.CreateTestRequest(env.Ctx, apiModel.UpdateWebhookRequest{
			URL:    "https://93.184.216.34/updated",
			Secret: "new-secret-16-chars",
			Events: []string{"product.wasted"},
			Active: &active,
		})

		UpdateWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.WebhookResponse
		_ = json.Unmarshal(env.W.Body.Bytes(), &resp)
		if resp.URL != "https://93.184.216.34/updated" || resp.Active {
			t.Errorf("response = %+v, want updated URL and inactive", resp)
		}
		if len(resp.Events) != 1 || resp.Events[0] != "product.wasted" {
			t.Errorf("events = %v, want [product.wasted]", resp.Events)
		}
	})

	t.Run("invalid event returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		webhook := testutil.CreateTestWebhook(env.DB, env.User.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(webhook.ID)}}
		testutil.CreateTestRequest(env.Ctx, apiModel.UpdateWebhookRequest{Events: []string{"bogus"}})

		UpdateWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("http URL returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		webhook := testutil.CreateTestWebhook(env.DB, env.User.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(webhook.ID)}}
		testutil.CreateTestRequest(env.Ctx, apiModel.UpdateWebhookRequest{URL: "http://93.184.216.34/hook"})

		UpdateWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("missing webhook returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "9999"}}
		testutil.CreateTestRequest(env.Ctx, apiModel.UpdateWebhookRequest{Active: new(bool)})

		UpdateWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("malformed body returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		webhook := testutil.CreateTestWebhook(env.DB, env.User.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(webhook.ID)}}
		env.Ctx.Request = newRawJSONRequest("{invalid")

		UpdateWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})
}

func TestDeleteWebhookHandler(t *testing.T) {
	t.Run("deletes webhook", func(t *testing.T) {
		env := setupHandlerTest(t)
		webhook := testutil.CreateTestWebhook(env.DB, env.User.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(webhook.ID)}}

		DeleteWebhook(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var count int64
		env.DB.Model(&dbModel.Webhook{}).Where("id = ?", webhook.ID).Count(&count)
		if count != 0 {
			t.Errorf("webhook still present, count = %d", count)
		}
	})

	t.Run("deleting again returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		webhook := testutil.CreateTestWebhook(env.DB, env.User.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(webhook.ID)}}

		DeleteWebhook(env.Ctx, env.AppCtx)
		if env.W.Code != http.StatusOK {
			t.Fatalf("first delete status = %d, want 200", env.W.Code)
		}

		ctx2, w2 := env.freshCtx(env.User.ID)
		ctx2.Params = []gin.Param{{Key: "id", Value: itoa(webhook.ID)}}
		appCtx2 := SetupTestAppContext(ctx2, env.User.ID)
		DeleteWebhook(ctx2, appCtx2)

		if w2.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", w2.Code)
		}
	})
}

func TestGetWebhookDeliveriesHandler(t *testing.T) {
	t.Run("returns delivery logs", func(t *testing.T) {
		env := setupHandlerTest(t)
		webhook := testutil.CreateTestWebhook(env.DB, env.User.ID)
		env.DB.Create(&dbModel.WebhookDeliveryLog{WebhookID: webhook.ID, StatusCode: 200, Attempt: 1, CreatedAt: time.Now()})
		env.DB.Create(&dbModel.WebhookDeliveryLog{WebhookID: webhook.ID, StatusCode: 500, Error: "boom", Attempt: 2, CreatedAt: time.Now()})
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(webhook.ID)}}

		GetWebhookDeliveries(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.DeliveryLogListResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(resp.Deliveries) != 2 {
			t.Fatalf("deliveries = %d, want 2", len(resp.Deliveries))
		}
		// repo orders created_at DESC — newest first
		if resp.Deliveries[0].StatusCode != 500 || resp.Deliveries[0].CreatedAt == "" {
			t.Errorf("delivery = %+v, want newest (500) with timestamp", resp.Deliveries[0])
		}
		if resp.Deliveries[1].StatusCode != 200 {
			t.Errorf("delivery[1] = %+v, want 200", resp.Deliveries[1])
		}
	})

	t.Run("webhook of another user returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		other := env.addMember(t, "other")
		webhook := testutil.CreateTestWebhook(env.DB, other.ID)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(webhook.ID)}}

		GetWebhookDeliveries(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})
}

func TestValidateWebhookEvents(t *testing.T) {
	if err := validateWebhookEvents([]string{"product.created", "product.expired"}); err != nil {
		t.Errorf("valid events returned error: %v", err)
	}
	if err := validateWebhookEvents([]string{"product.created", "bogus"}); err == nil {
		t.Errorf("invalid events returned nil, want error")
	}
	if err := validateWebhookEvents(nil); err != nil {
		t.Errorf("nil events returned error: %v", err)
	}
}

func TestIsValidWebhookEvent(t *testing.T) {
	for _, event := range apiModel.ValidWebhookEvents {
		if !isValidWebhookEvent(event) {
			t.Errorf("isValidWebhookEvent(%q) = false, want true", event)
		}
	}
	if isValidWebhookEvent("bogus.event") {
		t.Errorf("isValidWebhookEvent(bogus.event) = true, want false")
	}
	if isValidWebhookEvent("") {
		t.Errorf("isValidWebhookEvent(\"\") = true, want false")
	}
}
