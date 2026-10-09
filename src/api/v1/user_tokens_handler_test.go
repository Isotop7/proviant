package v1

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/controllers"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"

	"github.com/gin-gonic/gin"
)

func TestCreateUserToken(t *testing.T) {
	t.Run("creates token", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, apiModel.CreateTokenRequest{
			Name:   "ci-token",
			Scopes: "products:read",
		})

		CreateUserToken(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.CreateTokenResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Token == "" || resp.Name != "ci-token" || resp.Scopes != "products:read" {
			t.Errorf("response = %+v, want token/ci-token/products:read", resp)
		}
		if resp.ExpiresAt != nil {
			t.Errorf("expiresAt = %v, want nil", *resp.ExpiresAt)
		}
		var count int64
		env.DB.Model(&authentication.PersonalAccessToken{}).
			Where("user_id = ? AND token_hash = ?", env.User.ID, controllers.HashToken(resp.Token)).
			Count(&count)
		if count != 1 {
			t.Errorf("stored PAT count = %d, want 1", count)
		}
	})

	t.Run("creates token with expiry", func(t *testing.T) {
		env := setupHandlerTest(t)
		expiresAt := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
		testutil.CreateTestRequest(env.Ctx, apiModel.CreateTokenRequest{
			Name:      "expiring",
			ExpiresAt: &expiresAt,
		})

		CreateUserToken(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", env.W.Code, env.W.Body.String())
		}
		var resp apiModel.CreateTokenResponse
		_ = json.Unmarshal(env.W.Body.Bytes(), &resp)
		if resp.ExpiresAt == nil || *resp.ExpiresAt == "" {
			t.Errorf("expiresAt = %v, want set", resp.ExpiresAt)
		}
	})

	t.Run("invalid expiry format returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		bad := "tomorrow"
		testutil.CreateTestRequest(env.Ctx, apiModel.CreateTokenRequest{
			Name:      "bad-expiry",
			ExpiresAt: &bad,
		})

		CreateUserToken(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("missing name returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		testutil.CreateTestRequest(env.Ctx, apiModel.CreateTokenRequest{Scopes: "products:read"})

		CreateUserToken(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.PATs.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaims(ctx, 1)
		testutil.CreateTestRequest(ctx, apiModel.CreateTokenRequest{Name: "ci-token"})
		appCtx := SetupTestAppContext(ctx, 1)

		CreateUserToken(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}

func TestListUserTokens(t *testing.T) {
	t.Run("lists tokens with formatted timestamps", func(t *testing.T) {
		env := setupHandlerTest(t)
		expiresAt := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
		lastUsed := time.Now().UTC().Truncate(time.Second)
		env.DB.Create(&authentication.PersonalAccessToken{
			UserID:    env.User.ID,
			Name:      "ci-token",
			TokenHash: "hash1",
			Scopes:    "products:read",
			ExpiresAt: &expiresAt,
		})
		env.DB.Create(&authentication.PersonalAccessToken{
			UserID:     env.User.ID,
			Name:       "other",
			TokenHash:  "hash2",
			LastUsedAt: &lastUsed,
		})

		ListUserTokens(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var tokens []apiModel.TokenResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &tokens); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(tokens) != 2 {
			t.Fatalf("tokens = %d, want 2", len(tokens))
		}
		var withExpiry *apiModel.TokenResponse
		for i := range tokens {
			if tokens[i].Name == "ci-token" {
				withExpiry = &tokens[i]
			}
		}
		if withExpiry == nil {
			t.Fatalf("ci-token not found in %+v", tokens)
		}
		if withExpiry.ExpiresAt == nil || *withExpiry.ExpiresAt != expiresAt.Format(time.RFC3339) {
			t.Errorf("expiresAt = %v, want %v", withExpiry.ExpiresAt, expiresAt.Format(time.RFC3339))
		}
		if withExpiry.CreatedAt == "" {
			t.Errorf("createdAt empty")
		}
	})

	t.Run("empty list", func(t *testing.T) {
		env := setupHandlerTest(t)

		ListUserTokens(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", env.W.Code)
		}
		var tokens []apiModel.TokenResponse
		if err := json.Unmarshal(env.W.Body.Bytes(), &tokens); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(tokens) != 0 {
			t.Errorf("tokens = %d, want 0", len(tokens))
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.PATs.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaims(ctx, 1)
		appCtx := SetupTestAppContext(ctx, 1)

		ListUserTokens(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}

func TestDeleteUserToken(t *testing.T) {
	t.Run("deletes own token", func(t *testing.T) {
		env := setupHandlerTest(t)
		pat := authentication.PersonalAccessToken{UserID: env.User.ID, Name: "ci", TokenHash: "hash1"}
		env.DB.Create(&pat)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(pat.ID)}}

		DeleteUserToken(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", env.W.Code, env.W.Body.String())
		}
		var count int64
		env.DB.Model(&authentication.PersonalAccessToken{}).Where("id = ?", pat.ID).Count(&count)
		if count != 0 {
			t.Errorf("token still present, count = %d", count)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		env := setupHandlerTest(t)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: "abc"}}

		DeleteUserToken(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", env.W.Code)
		}
	})

	t.Run("token of another user returns 404", func(t *testing.T) {
		env := setupHandlerTest(t)
		other := env.addMember(t, "other")
		pat := authentication.PersonalAccessToken{UserID: other.ID, Name: "ci", TokenHash: "hash1"}
		env.DB.Create(&pat)
		env.Ctx.Params = []gin.Param{{Key: "id", Value: itoa(pat.ID)}}

		DeleteUserToken(env.Ctx, env.AppCtx)

		if env.W.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", env.W.Code)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.PATs.Err = errors.New("db down")
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaims(ctx, 1)
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}
		appCtx := SetupTestAppContext(ctx, 1)

		DeleteUserToken(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", w.Code)
		}
	})
}
