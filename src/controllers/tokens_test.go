package controllers

import (
	"encoding/hex"
	"testing"
	"time"
)

func TestGeneratePasswordResetToken(t *testing.T) {
	before := time.Now()
	token, expiresAt, err := GeneratePasswordResetToken()
	if err != nil {
		t.Fatalf("GeneratePasswordResetToken() = %v", err)
	}
	if _, err := hex.DecodeString(token); err != nil {
		t.Errorf("token %q is not hex: %v", token, err)
	}
	if want := PasswordResetTokenLength * 2; len(token) != want {
		t.Errorf("token length = %d, want %d", len(token), want)
	}
	if expiresAt.Before(before.Add(PasswordResetTokenDuration)) || expiresAt.After(time.Now().Add(PasswordResetTokenDuration)) {
		t.Errorf("expiresAt = %v, want ~%v from now", expiresAt, PasswordResetTokenDuration)
	}

	other, _, err := GeneratePasswordResetToken()
	if err != nil {
		t.Fatalf("GeneratePasswordResetToken() = %v", err)
	}
	if token == other {
		t.Error("GeneratePasswordResetToken() returned the same token twice")
	}
}

func TestGenerateEmailVerificationToken(t *testing.T) {
	before := time.Now()
	token, expiresAt, err := GenerateEmailVerificationToken()
	if err != nil {
		t.Fatalf("GenerateEmailVerificationToken() = %v", err)
	}
	if _, err := hex.DecodeString(token); err != nil {
		t.Errorf("token %q is not hex: %v", token, err)
	}
	if want := EmailVerificationTokenLength * 2; len(token) != want {
		t.Errorf("token length = %d, want %d", len(token), want)
	}
	if expiresAt.Before(before.Add(EmailVerificationTokenDuration)) || expiresAt.After(time.Now().Add(EmailVerificationTokenDuration)) {
		t.Errorf("expiresAt = %v, want ~%v from now", expiresAt, EmailVerificationTokenDuration)
	}

	other, _, err := GenerateEmailVerificationToken()
	if err != nil {
		t.Fatalf("GenerateEmailVerificationToken() = %v", err)
	}
	if token == other {
		t.Error("GenerateEmailVerificationToken() returned the same token twice")
	}
}
