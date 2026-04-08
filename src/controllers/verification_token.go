package controllers

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

const EmailVerificationTokenLength = 32
const EmailVerificationTokenDuration = 24 * time.Hour

func GenerateEmailVerificationToken() (string, time.Time, error) {
	bytes := make([]byte, EmailVerificationTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(bytes)
	expiresAt := time.Now().Add(EmailVerificationTokenDuration)
	return token, expiresAt, nil
}
