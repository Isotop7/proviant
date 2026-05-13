package router

import (
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"

	"codeberg.org/isotop7/proviant/api"
	apperrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"
)

const (
	csrfCookieName    = "csrf_token"
	csrfHeaderName    = "X-CSRF-Token"
	csrfTokenBytes    = 32
	csrfDefaultMaxAge = 86400 // 24 hours
)

func generateCSRFToken() (string, error) {
	b := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func isStateMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}

// CSRFMiddleware implements the double-submit cookie CSRF protection pattern.
// It sets a csrf_token cookie on all responses and validates the X-CSRF-Token
// request header against that cookie on state-mutating requests. Requests that
// include an Authorization header are exempt because non-browser programmatic
// clients (PAT, Bearer token in header) are not subject to CSRF.
func CSRFMiddleware(cfg *configuration.ProviantConfiguration) gin.HandlerFunc {
	maxAge := cfg.Server.SecurityHeaders.CSRFTokenMaxAge
	if maxAge <= 0 {
		maxAge = csrfDefaultMaxAge
	}

	return func(ctx *gin.Context) {
		// Programmatic clients that send an explicit Authorization header are not
		// vulnerable to CSRF — browsers do not auto-send custom auth headers.
		if ctx.GetHeader("Authorization") != "" {
			ctx.Next()
			return
		}

		cookieToken, cookieErr := ctx.Cookie(csrfCookieName)

		if isStateMutatingMethod(ctx.Request.Method) {
			headerToken := ctx.GetHeader(csrfHeaderName)
			if cookieErr != nil || headerToken == "" || !hmac.Equal([]byte(headerToken), []byte(cookieToken)) {
				api.RespondError(ctx, http.StatusForbidden, apperrors.ErrCSRFTokenInvalid)
				return
			}
		}

		// Issue a new CSRF token if none exists; otherwise reuse the existing one.
		if cookieErr != nil {
			token, err := generateCSRFToken()
			if err != nil {
				ctx.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			// Not HttpOnly — JavaScript must read this value to inject it into request headers.
			ctx.SetSameSite(http.SameSiteStrictMode)
			ctx.SetCookie(csrfCookieName, token, maxAge, "/", "", false, false)
			ctx.Set(util.ContextKeyCSRFToken, token)
		} else {
			ctx.Set(util.ContextKeyCSRFToken, cookieToken)
		}

		ctx.Next()
	}
}
