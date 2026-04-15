package controllers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"gorm.io/gorm"
)

const TokenPrefix = "proviant_pat_" // #nosec G101 -- this is a public token prefix, not a secret
const TokenLength = 40

func GeneratePAT() (string, error) {
	bytes := make([]byte, TokenLength/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return TokenPrefix + hex.EncodeToString(bytes), nil
}

func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func ValidateAndLookupPAT(token string, dbHandle *gorm.DB) (*authentication.PersonalAccessToken, error) {
	if !strings.HasPrefix(token, TokenPrefix) {
		return nil, errors.ErrPATInvalid
	}

	tokenHash := HashToken(token)
	patRepo := database.NewPATRepository(dbHandle)

	pat, err := patRepo.GetPATByTokenHash(tokenHash)
	if err != nil {
		return nil, errors.ErrPATNotFound
	}

	if pat.ExpiresAt != nil && pat.ExpiresAt.Before(time.Now()) {
		return nil, errors.ErrPATExpired
	}

	return pat, nil
}
