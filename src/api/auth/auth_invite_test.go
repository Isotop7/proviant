package auth

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/testutil"
	"github.com/gin-gonic/gin"
)

func TestAcceptInvitation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("invitation not found", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		reqBody := acceptInvitationRequest{
			Token: "non-existent-token",
		}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set("Content-Type", "application/json")

		AcceptInvitation(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})

	t.Run("missing token returns bad request", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)

		ctx, w := testutil.SetupGinContext(db)
		testutil.MockJWTClaimsWithKey(ctx, testUser.ID, testutil.TokenIdentityKey)

		ctx.Request = &http.Request{
			Header: make(http.Header),
		}

		AcceptInvitation(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

func TestVerifyEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)

	t.Run("successful email verification", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		verification := testutil.CreateTestEmailVerification(db, testUser.ID, "valid-token")

		ctx, w := testutil.SetupGinContext(db)
		ctx.Request = &http.Request{
			URL: &url.URL{RawQuery: "token=valid-token"},
		}
		_ = verification

		VerifyEmail(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("missing token returns bad request", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)
		_ = testUser

		ctx, w := testutil.SetupGinContext(db)
		ctx.Request = &http.Request{
			URL: &url.URL{RawQuery: "token="},
		}

		VerifyEmail(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid token returns bad request", func(t *testing.T) {
		testUser := testutil.CreateTestUser(db, 0)
		_ = testUser

		ctx, w := testutil.SetupGinContext(db)
		ctx.Request = &http.Request{
			URL: &url.URL{RawQuery: "token=invalid-token"},
		}

		VerifyEmail(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("expired token returns bad request", func(t *testing.T) {
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		verification := testutil.CreateTestEmailVerification(db, testUser.ID, "expired-token")
		verification.ExpiresAt = time.Now().Add(-1 * time.Hour)
		verification.Status = "pending"
		db.Save(&verification)

		ctx, w := testutil.SetupGinContext(db)
		ctx.Request = &http.Request{
			URL: &url.URL{RawQuery: "token=expired-token"},
		}

		VerifyEmail(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}