package static

import (
	"strings"
	"testing"
)

func TestTokenConstants(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"TokenRealm", TokenRealm, "proviant"},
		{"TokenIdentityKey", TokenIdentityKey, "id"},
		{"TokenUsernameKey", TokenUsernameKey, "username"},
		{"TokenHeadName", TokenHeadName, "Bearer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got == "" {
				t.Errorf("%s is empty", tc.name)
				return
			}
			if tc.got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
			}
		})
	}
}

func TestTokenLookup(t *testing.T) {
	if TokenLookup == "" {
		t.Fatal("TokenLookup is empty")
	}
	for _, sub := range []string{"header: Authorization", "query: token", "cookie: jwt"} {
		if !strings.Contains(TokenLookup, sub) {
			t.Errorf("TokenLookup does not contain %q", sub)
		}
	}
}

func TestBarcodeDecodingTimeout(t *testing.T) {
	if BarcodeDecodingTimeout == 0 {
		t.Error("BarcodeDecodingTimeout is zero")
	}
}
