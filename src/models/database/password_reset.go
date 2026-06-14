package database

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"gorm.io/gorm"
)

// PasswordReset stores a one-time password-reset token for a user.
//
// Unlike the older EmailVerification flow, the raw token is NEVER persisted —
// only its SHA-256 hex digest (TokenHash) is stored. The raw value is sent to
// the user via email and posted back on the reset form, and is hashed on the
// server before lookup. This way a DB dump or backup never yields valid
// reset tokens.
type PasswordReset struct {
	gorm.Model
	UserID    uint      `gorm:"index,not null"`
	TokenHash string    `gorm:"uniqueIndex,not null"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedAt    *time.Time
	IPAddress string `gorm:"size:64"`
}

// HashPasswordResetToken returns the SHA-256 hex digest of a raw reset token.
// Use this when persisting a new PasswordReset or looking one up by token.
func HashPasswordResetToken(raw string) string {
	h := sha256.New()
	h.Write([]byte(raw))
	return hex.EncodeToString(h.Sum(nil))
}
