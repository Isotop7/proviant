package auth

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	proviantErrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestAcceptInvitation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("invitation not found", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.User = authentication.User{MailAddress: "test@example.com"}
		m.Invitations.Err = proviantErrors.ErrInvitationNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		reqBody := acceptInvitationRequest{Token: "non-existent-token"}
		body, _ := json.Marshal(reqBody)
		ctx.Request = &http.Request{
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			Header:        make(http.Header),
			ContentLength: int64(len(body)),
		}
		ctx.Request.Header.Set(util.RequestHeaderContentType, "application/json")

		AcceptInvitation(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})

	t.Run("missing token returns bad request", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)

		ctx.Request = &http.Request{Header: make(http.Header)}

		AcceptInvitation(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}

func TestVerifyEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("successful email verification", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.EmailVerification = dbModel.EmailVerification{
			Status:    dbModel.EmailVerificationStatusPending,
			ExpiresAt: time.Now().Add(1 * time.Hour),
			UserID:    1,
		}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		ctx.Request = &http.Request{
			URL: &url.URL{RawQuery: "token=valid-token"},
		}

		VerifyEmail(ctx)

		if w.Code != http.StatusOK {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
		}
	})

	t.Run("missing token returns bad request", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		ctx.Request = &http.Request{URL: &url.URL{RawQuery: "token="}}

		VerifyEmail(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid token returns not found", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.Err = gorm.ErrRecordNotFound
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		ctx.Request = &http.Request{URL: &url.URL{RawQuery: "token=invalid-token"}}

		VerifyEmail(ctx)

		if w.Code != http.StatusNotFound {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusNotFound)
		}
	})

	t.Run("expired token returns bad request", func(t *testing.T) {
		m := repomocks.NewMockRepositoryContainer()
		m.Users.EmailVerification = dbModel.EmailVerification{
			Status:    dbModel.EmailVerificationStatusPending,
			ExpiresAt: time.Now().Add(-1 * time.Hour),
			UserID:    1,
		}
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		ctx.Request = &http.Request{URL: &url.URL{RawQuery: "token=expired-token"}}

		VerifyEmail(ctx)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %v, want %v", w.Code, http.StatusBadRequest)
		}
	})
}
