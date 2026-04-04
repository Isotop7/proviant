package static

import (
	"strings"
	"testing"
)

func TestTokenConstants(t *testing.T) {
	t.Run("TokenRealm is set", func(t *testing.T) {
		if TokenRealm == "" {
			t.Errorf("TokenRealm is empty")
		}
		if TokenRealm != "proviant" {
			t.Errorf("TokenRealm = %v, want proviant", TokenRealm)
		}
	})

	t.Run("TokenIdentityKey is set", func(t *testing.T) {
		if TokenIdentityKey == "" {
			t.Errorf("TokenIdentityKey is empty")
		}
		if TokenIdentityKey != "id" {
			t.Errorf("TokenIdentityKey = %v, want id", TokenIdentityKey)
		}
	})

	t.Run("TokenUsernameKey is set", func(t *testing.T) {
		if TokenUsernameKey == "" {
			t.Errorf("TokenUsernameKey is empty")
		}
		if TokenUsernameKey != "username" {
			t.Errorf("TokenUsernameKey = %v, want username", TokenUsernameKey)
		}
	})

	t.Run("TokenHeadName is set", func(t *testing.T) {
		if TokenHeadName == "" {
			t.Errorf("TokenHeadName is empty")
		}
		if TokenHeadName != "Bearer" {
			t.Errorf("TokenHeadName = %v, want Bearer", TokenHeadName)
		}
	})

	t.Run("TokenLookup is set", func(t *testing.T) {
		if TokenLookup == "" {
			t.Errorf("TokenLookup is empty")
		}
		if !strings.Contains(TokenLookup, "header: Authorization") {
			t.Errorf("TokenLookup does not contain 'header: Authorization'")
		}
		if !strings.Contains(TokenLookup, "query: token") {
			t.Errorf("TokenLookup does not contain 'query: token'")
		}
		if !strings.Contains(TokenLookup, "cookie: jwt") {
			t.Errorf("TokenLookup does not contain 'cookie: jwt'")
		}
	})

	t.Run("BarcodeDecodingTimeout is set", func(t *testing.T) {
		if BarcodeDecodingTimeout == 0 {
			t.Errorf("BarcodeDecodingTimeout is zero")
		}
	})
}
