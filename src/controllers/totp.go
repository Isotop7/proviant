package controllers

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/Isotop7/proviant/util"
)

const (
	// TOTPStepSeconds is the length of one TOTP time step. A code is valid
	// for the step it was generated in.
	TOTPStepSeconds = 30

	// TOTPWindowSteps is the number of steps before and after the current one
	// that are accepted, to tolerate clock drift between the server and the
	// authenticator app.
	TOTPWindowSteps = 1

	// TOTPSecretBytes is the size of a generated TOTP secret. RFC 4226
	// recommends at least 128 bits.
	TOTPSecretBytes = 20

	// TOTPSecretAAD binds a ciphertext to its purpose, so a secret encrypted
	// for the TOTP purpose cannot be replayed into another field.
	TOTPSecretAAD = "proviant-totp-secret-v1" //nolint:gosec // G101: additional authenticated data, not a credential

	// TOTPQRSize is the edge length of the generated enrolment QR code.
	TOTPQRSize = 256

	// recoveryCodeLength is the number of characters in a recovery code.
	recoveryCodeLength = 10

	// recoveryCodeAlphabet omits the characters that are easy to confuse when
	// a recovery code is copied off the screen by hand.
	recoveryCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

// ErrTwoFactorEncryptionKey is returned when the configured encryption key is
// not a usable AES-256 key.
var ErrTwoFactorEncryptionKey = errors.New("the two-factor encryption key is invalid: keep server.twoFactor.encryptionKey configured while accounts have an enrolled second factor")

// DecodeTwoFactorEncryptionKey decodes the base64 encoded configuration value
// into an AES-256 key.
func DecodeTwoFactorEncryptionKey(encoded string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return nil, ErrTwoFactorEncryptionKey
	}
	if len(decoded) != util.TwoFactorEncryptionKeyBytes {
		return nil, ErrTwoFactorEncryptionKey
	}
	return decoded, nil
}

// EncryptTwoFactorSecret encrypts a TOTP secret with AES-256-GCM. The random
// nonce is stored in front of the ciphertext, so the stored value is
// base64(nonce || ciphertext || tag).
func EncryptTwoFactorSecret(plaintext string, key []byte) (string, error) {
	gcm, err := newTwoFactorGCM(key)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	sealed := gcm.Seal(nil, nonce, []byte(plaintext), []byte(TOTPSecretAAD))
	return base64.StdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

// DecryptTwoFactorSecret decrypts a value produced by
// EncryptTwoFactorSecret. A tampered ciphertext fails the GCM tag check.
func DecryptTwoFactorSecret(stored string, key []byte) (string, error) {
	gcm, err := newTwoFactorGCM(key)
	if err != nil {
		return "", err
	}

	raw, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		return "", ErrTwoFactorEncryptionKey
	}
	if len(raw) < gcm.NonceSize() {
		return "", ErrTwoFactorEncryptionKey
	}

	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, []byte(TOTPSecretAAD))
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func newTwoFactorGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrTwoFactorEncryptionKey
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrTwoFactorEncryptionKey
	}
	return gcm, nil
}

// GenerateTwoFactorSecret creates a new TOTP secret together with the
// otpauth:// URI an authenticator app scans.
func GenerateTwoFactorSecret(issuer, accountName string) (secret string, uri string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Period:      TOTPStepSeconds,
		SecretSize:  TOTPSecretBytes,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// TwoFactorURIFromSecret rebuilds the otpauth:// URI of an already generated
// secret, so a pending enrolment can be handed out again without replacing the
// secret the user may already have scanned.
func TwoFactorURIFromSecret(secret, issuer, accountName string) (string, error) {
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.TrimSpace(secret))
	if err != nil {
		return "", err
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Period:      TOTPStepSeconds,
		Secret:      raw,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return "", err
	}
	return key.URL(), nil
}

// BuildTwoFactorQRDataURI renders the otpauth:// URI as a PNG QR code
// encoded as a data URI. The TOTP secret therefore only ever reaches the
// browser as pixels, never as text in a response body, a log line or the HTML.
func BuildTwoFactorQRDataURI(uri string) (string, error) {
	key, err := otp.NewKeyFromURL(uri)
	if err != nil {
		return "", err
	}

	image, err := key.Image(TOTPQRSize, TOTPQRSize)
	if err != nil {
		return "", err
	}

	var buffer strings.Builder
	encoder := base64.NewEncoder(base64.StdEncoding, &buffer)
	if err := png.Encode(encoder, image); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}

	return "data:image/png;base64," + buffer.String(), nil
}

// VerifyTwoFactorCode checks a TOTP code against a secret. It returns the time
// step the code belongs to, which the caller compares against the last
// accepted step to reject a replayed code.
func VerifyTwoFactorCode(secret, code string, now time.Time) (int64, bool) {
	trimmed := strings.TrimSpace(code)
	if trimmed == "" || secret == "" {
		return 0, false
	}

	currentStep := now.Unix() / TOTPStepSeconds
	for offset := int64(-TOTPWindowSteps); offset <= TOTPWindowSteps; offset++ {
		step := currentStep + offset
		candidate, err := totp.GenerateCodeCustom(secret, time.Unix(step*TOTPStepSeconds, 0), totp.ValidateOpts{
			Period:    TOTPStepSeconds,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		})
		if err != nil {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(trimmed)) == 1 {
			return step, true
		}
	}

	return 0, false
}

// GenerateRecoveryCodes creates single-use recovery codes for an account.
func GenerateRecoveryCodes(count int) ([]string, error) {
	if count < 1 {
		return nil, fmt.Errorf("the recovery code count must be positive")
	}

	codes := make([]string, 0, count)
	buffer := make([]byte, recoveryCodeLength)
	for range count {
		if _, err := rand.Read(buffer); err != nil {
			return nil, err
		}
		var builder strings.Builder
		builder.Grow(recoveryCodeLength)
		for _, b := range buffer {
			builder.WriteByte(recoveryCodeAlphabet[int(b)%len(recoveryCodeAlphabet)])
		}
		codes = append(codes, builder.String())
	}

	return codes, nil
}

// HashRecoveryCode returns the SHA-256 hex digest of a raw recovery code.
func HashRecoveryCode(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}
