package controllers

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func testEncryptionKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestEncryptTwoFactorSecretRoundTrip(t *testing.T) {
	key := testEncryptionKey()

	encrypted, err := EncryptTwoFactorSecret("JBSWY3DPEHPK3PXP", key)
	if err != nil {
		t.Fatalf("EncryptTwoFactorSecret() error = %v", err)
	}
	if encrypted == "JBSWY3DPEHPK3PXP" {
		t.Fatal("the secret was stored in the clear")
	}

	decrypted, err := DecryptTwoFactorSecret(encrypted, key)
	if err != nil {
		t.Fatalf("DecryptTwoFactorSecret() error = %v", err)
	}
	if decrypted != "JBSWY3DPEHPK3PXP" {
		t.Errorf("decrypted = %q, want JBSWY3DPEHPK3PXP", decrypted)
	}
}

func TestEncryptTwoFactorSecretUsesFreshNonce(t *testing.T) {
	key := testEncryptionKey()

	first, err := EncryptTwoFactorSecret("secret", key)
	if err != nil {
		t.Fatalf("EncryptTwoFactorSecret() error = %v", err)
	}
	second, err := EncryptTwoFactorSecret("secret", key)
	if err != nil {
		t.Fatalf("EncryptTwoFactorSecret() error = %v", err)
	}
	if first == second {
		t.Error("two encryptions of the same secret produced the same ciphertext")
	}
}

func TestDecryptTwoFactorSecretRejectsWrongKey(t *testing.T) {
	encrypted, err := EncryptTwoFactorSecret("secret", testEncryptionKey())
	if err != nil {
		t.Fatalf("EncryptTwoFactorSecret() error = %v", err)
	}

	otherKey := make([]byte, 32)
	otherKey[0] = 255

	if _, err := DecryptTwoFactorSecret(encrypted, otherKey); err == nil {
		t.Error("DecryptTwoFactorSecret() with a wrong key returned no error")
	}
}

func TestDecryptTwoFactorSecretRejectsTamperedCiphertext(t *testing.T) {
	key := testEncryptionKey()

	encrypted, err := EncryptTwoFactorSecret("secret", key)
	if err != nil {
		t.Fatalf("EncryptTwoFactorSecret() error = %v", err)
	}

	raw, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	raw[len(raw)-1] ^= 0xff

	if _, err := DecryptTwoFactorSecret(base64.StdEncoding.EncodeToString(raw), key); err == nil {
		t.Error("DecryptTwoFactorSecret() with a tampered ciphertext returned no error")
	}
}

func TestDecodeTwoFactorEncryptionKey(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(make([]byte, 32))

	key, err := DecodeTwoFactorEncryptionKey(valid)
	if err != nil {
		t.Fatalf("DecodeTwoFactorEncryptionKey() error = %v", err)
	}
	if len(key) != 32 {
		t.Errorf("key length = %d, want 32", len(key))
	}

	if _, err := DecodeTwoFactorEncryptionKey(""); err == nil {
		t.Error("DecodeTwoFactorEncryptionKey() with an empty value returned no error")
	}
	if _, err := DecodeTwoFactorEncryptionKey("not base64!!"); err == nil {
		t.Error("DecodeTwoFactorEncryptionKey() with a malformed value returned no error")
	}
	if _, err := DecodeTwoFactorEncryptionKey(base64.StdEncoding.EncodeToString(make([]byte, 16))); err == nil {
		t.Error("DecodeTwoFactorEncryptionKey() with a short key returned no error")
	}
}

func TestGenerateTwoFactorSecret(t *testing.T) {
	secret, uri, err := GenerateTwoFactorSecret("Proviant", "alice")
	if err != nil {
		t.Fatalf("GenerateTwoFactorSecret() error = %v", err)
	}
	if secret == "" {
		t.Error("the generated secret is empty")
	}
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("uri = %q, want an otpauth://totp prefix", uri)
	}
	if !strings.Contains(uri, "Proviant") {
		t.Errorf("uri = %q, want it to carry the issuer", uri)
	}
}

func TestBuildTwoFactorQRDataURI(t *testing.T) {
	_, uri, err := GenerateTwoFactorSecret("Proviant", "alice")
	if err != nil {
		t.Fatalf("GenerateTwoFactorSecret() error = %v", err)
	}

	qr, err := BuildTwoFactorQRDataURI(uri)
	if err != nil {
		t.Fatalf("BuildTwoFactorQRDataURI() error = %v", err)
	}
	if !strings.HasPrefix(qr, "data:image/png;base64,") {
		t.Errorf("qr = %q, want a PNG data URI", qr)
	}
	if strings.Contains(qr, "otpauth://") {
		t.Error("the QR data URI carries the otpauth URI in plain text")
	}
}

func TestVerifyTwoFactorCode(t *testing.T) {
	secret, _, err := GenerateTwoFactorSecret("Proviant", "alice")
	if err != nil {
		t.Fatalf("GenerateTwoFactorSecret() error = %v", err)
	}

	now := time.Now()
	code, err := totp.GenerateCodeCustom(secret, now, totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	step, valid := VerifyTwoFactorCode(secret, code, now)
	if !valid {
		t.Fatal("VerifyTwoFactorCode() rejected a fresh code")
	}
	if step != now.Unix()/30 {
		t.Errorf("step = %d, want %d", step, now.Unix()/30)
	}
}

func TestVerifyTwoFactorCodeRejectsWrongCode(t *testing.T) {
	secret, _, err := GenerateTwoFactorSecret("Proviant", "alice")
	if err != nil {
		t.Fatalf("GenerateTwoFactorSecret() error = %v", err)
	}

	if _, valid := VerifyTwoFactorCode(secret, "000000", time.Now()); valid {
		t.Error("VerifyTwoFactorCode() accepted a wrong code")
	}
	if _, valid := VerifyTwoFactorCode(secret, "", time.Now()); valid {
		t.Error("VerifyTwoFactorCode() accepted an empty code")
	}
	if _, valid := VerifyTwoFactorCode("", "123456", time.Now()); valid {
		t.Error("VerifyTwoFactorCode() accepted a code without a secret")
	}
}

func TestVerifyTwoFactorCodeRejectsExpiredCode(t *testing.T) {
	secret, _, err := GenerateTwoFactorSecret("Proviant", "alice")
	if err != nil {
		t.Fatalf("GenerateTwoFactorSecret() error = %v", err)
	}

	expired := time.Now().Add(-10 * time.Minute)
	code, err := totp.GenerateCodeCustom(secret, expired, totp.ValidateOpts{Period: 30})
	if err != nil {
		t.Fatalf("GenerateCodeCustom() error = %v", err)
	}

	if _, valid := VerifyTwoFactorCode(secret, code, time.Now()); valid {
		t.Error("VerifyTwoFactorCode() accepted an expired code")
	}
}

func TestGenerateRecoveryCodes(t *testing.T) {
	codes, err := GenerateRecoveryCodes(10)
	if err != nil {
		t.Fatalf("GenerateRecoveryCodes() error = %v", err)
	}
	if len(codes) != 10 {
		t.Fatalf("len(codes) = %d, want 10", len(codes))
	}

	seen := map[string]bool{}
	for _, code := range codes {
		if len(code) != recoveryCodeLength {
			t.Errorf("code %q has length %d, want %d", code, len(code), recoveryCodeLength)
		}
		if seen[code] {
			t.Errorf("the code %q was issued twice", code)
		}
		seen[code] = true
		for _, r := range code {
			if !strings.ContainsRune(recoveryCodeAlphabet, r) {
				t.Errorf("the code %q carries the character %q", code, r)
			}
		}
	}
}

func TestGenerateRecoveryCodesRejectsEmptyCount(t *testing.T) {
	if _, err := GenerateRecoveryCodes(0); err == nil {
		t.Error("GenerateRecoveryCodes(0) returned no error")
	}
}

func TestHashRecoveryCodeIsStable(t *testing.T) {
	first := HashRecoveryCode("ABCDEFGHJK")
	second := HashRecoveryCode("ABCDEFGHJK")
	if first != second {
		t.Error("HashRecoveryCode() is not stable")
	}
	if first == HashRecoveryCode("ABCDEFGHJKL") {
		t.Error("HashRecoveryCode() collides on different codes")
	}
	if strings.Contains(first, "ABCDEFGHJK") {
		t.Error("HashRecoveryCode() returned the raw code")
	}
}
