package controllers

import (
	"strings"
	"testing"
	"time"

	proviantErrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/testutil"
)

func TestGeneratePAT(t *testing.T) {
	token, err := GeneratePAT()
	if err != nil {
		t.Fatalf("GeneratePAT() = %v", err)
	}
	if !strings.HasPrefix(token, TokenPrefix) {
		t.Errorf("token = %q, want prefix %q", token, TokenPrefix)
	}
	if want := len(TokenPrefix) + TokenLength; len(token) != want {
		t.Errorf("token length = %d, want %d", len(token), want)
	}

	other, err := GeneratePAT()
	if err != nil {
		t.Fatalf("GeneratePAT() = %v", err)
	}
	if token == other {
		t.Error("GeneratePAT() returned the same token twice")
	}
}

func TestHashToken(t *testing.T) {
	// sha256("abc")
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := HashToken("abc"); got != want {
		t.Errorf("HashToken() = %q, want %q", got, want)
	}
	if got := HashToken("abc"); got != want {
		t.Error("HashToken() is not deterministic")
	}
}

func TestValidateAndLookupPAT(t *testing.T) {
	db := testutil.SetupTestDB(t)

	t.Run("missing prefix rejected", func(t *testing.T) {
		if _, err := ValidateAndLookupPAT("no-prefix-here", db); err != proviantErrors.ErrPATInvalid {
			t.Errorf("err = %v, want ErrPATInvalid", err)
		}
	})

	t.Run("unknown token not found", func(t *testing.T) {
		token, err := GeneratePAT()
		if err != nil {
			t.Fatalf("GeneratePAT() = %v", err)
		}
		if _, err := ValidateAndLookupPAT(token, db); err != proviantErrors.ErrPATNotFound {
			t.Errorf("err = %v, want ErrPATNotFound", err)
		}
	})

	t.Run("expired token rejected", func(t *testing.T) {
		token, err := GeneratePAT()
		if err != nil {
			t.Fatalf("GeneratePAT() = %v", err)
		}
		expired := time.Now().Add(-time.Hour)
		pat := authentication.PersonalAccessToken{UserID: 1, Name: "old", TokenHash: HashToken(token), ExpiresAt: &expired}
		if err := db.Create(&pat).Error; err != nil {
			t.Fatalf("seed PAT: %v", err)
		}
		if _, err := ValidateAndLookupPAT(token, db); err != proviantErrors.ErrPATExpired {
			t.Errorf("err = %v, want ErrPATExpired", err)
		}
	})

	t.Run("valid token returned", func(t *testing.T) {
		token, err := GeneratePAT()
		if err != nil {
			t.Fatalf("GeneratePAT() = %v", err)
		}
		future := time.Now().Add(time.Hour)
		pat := authentication.PersonalAccessToken{UserID: 2, Name: "ci", TokenHash: HashToken(token), ExpiresAt: &future, Scopes: "read"}
		if err := db.Create(&pat).Error; err != nil {
			t.Fatalf("seed PAT: %v", err)
		}
		found, err := ValidateAndLookupPAT(token, db)
		if err != nil {
			t.Fatalf("ValidateAndLookupPAT() = %v", err)
		}
		if found.UserID != 2 || found.Name != "ci" {
			t.Errorf("PAT = %+v, want user 2 named ci", found)
		}
	})

	t.Run("token without expiry is valid", func(t *testing.T) {
		token, err := GeneratePAT()
		if err != nil {
			t.Fatalf("GeneratePAT() = %v", err)
		}
		pat := authentication.PersonalAccessToken{UserID: 3, Name: "forever", TokenHash: HashToken(token)}
		if err := db.Create(&pat).Error; err != nil {
			t.Fatalf("seed PAT: %v", err)
		}
		if _, err := ValidateAndLookupPAT(token, db); err != nil {
			t.Errorf("ValidateAndLookupPAT() = %v, want nil", err)
		}
	})
}
