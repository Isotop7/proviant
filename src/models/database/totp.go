package database

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"gorm.io/gorm"
)

// TwoFactorRecoveryCode stores one single-use recovery code for a user.
//
// Like the password reset tokens, the raw code is never persisted: only its
// SHA-256 hex digest (CodeHash) is stored. The raw value is shown to the user
// exactly once, at enrolment, and is hashed on the server before the lookup.
// A database dump therefore never yields a usable recovery code.
type TwoFactorRecoveryCode struct {
	gorm.Model
	UserID   uint   `gorm:"index,not null"`
	CodeHash string `gorm:"uniqueIndex,not null"`
	UsedAt   *time.Time
}

// HashTwoFactorRecoveryCode returns the SHA-256 hex digest of a raw recovery
// code. Use this when persisting a new code or looking one up by its raw
// value.
func HashTwoFactorRecoveryCode(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
