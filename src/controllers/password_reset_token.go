package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const PasswordResetTokenLength = 32
const PasswordResetTokenDuration = 1 * time.Hour

func GeneratePasswordResetToken() (string, time.Time, error) {
	bytes := make([]byte, PasswordResetTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(bytes)
	expiresAt := time.Now().Add(PasswordResetTokenDuration)
	return token, expiresAt, nil
}
