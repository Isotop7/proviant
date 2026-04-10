package authentication

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/errors"
)

type PasswordConfig struct {
	MinLength        int
	RequireUppercase bool
	RequireDigit     bool
	RequireSpecial   bool
	CheckBreached    bool
}

type PasswordValidator struct {
	config     PasswordConfig
	httpClient *http.Client
}

func NewPasswordValidator(cfg PasswordConfig) *PasswordValidator {
	return &PasswordValidator{
		config: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (pv *PasswordValidator) Validate(password string) error {
	if err := pv.validateLength(password); err != nil {
		return err
	}
	if pv.requireUppercase() {
		if err := pv.validateUppercase(password); err != nil {
			return err
		}
	}
	if pv.requireDigit() {
		if err := pv.validateDigit(password); err != nil {
			return err
		}
	}
	if pv.requireSpecial() {
		if err := pv.validateSpecial(password); err != nil {
			return err
		}
	}
	if pv.checkBreached() {
		if err := pv.validateBreached(password); err != nil {
			return err
		}
	}
	return nil
}

func (pv *PasswordValidator) ValidateAll(password string) []error {
	var errs []error
	if err := pv.validateLength(password); err != nil {
		errs = append(errs, err)
	}
	if pv.requireUppercase() {
		if err := pv.validateUppercase(password); err != nil {
			errs = append(errs, err)
		}
	}
	if pv.requireDigit() {
		if err := pv.validateDigit(password); err != nil {
			errs = append(errs, err)
		}
	}
	if pv.requireSpecial() {
		if err := pv.validateSpecial(password); err != nil {
			errs = append(errs, err)
		}
	}
	if pv.checkBreached() {
		if err := pv.validateBreached(password); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (pv *PasswordValidator) requireUppercase() bool {
	return pv.config.MinLength > 0 && pv.config.RequireUppercase
}

func (pv *PasswordValidator) requireDigit() bool {
	return pv.config.MinLength > 0 && pv.config.RequireDigit
}

func (pv *PasswordValidator) requireSpecial() bool {
	return pv.config.MinLength > 0 && pv.config.RequireSpecial
}

func (pv *PasswordValidator) checkBreached() bool {
	return pv.config.MinLength > 0 && pv.config.CheckBreached
}

func (pv *PasswordValidator) validateLength(password string) error {
	if len(password) < pv.config.MinLength {
		return errors.ErrPasswordTooShort
	}
	return nil
}

func (pv *PasswordValidator) validateUppercase(password string) error {
	for _, c := range password {
		if c >= 'A' && c <= 'Z' {
			return nil
		}
	}
	return errors.ErrPasswordUppercaseRequired
}

func (pv *PasswordValidator) validateDigit(password string) error {
	for _, c := range password {
		if c >= '0' && c <= '9' {
			return nil
		}
	}
	return errors.ErrPasswordDigitRequired
}

func (pv *PasswordValidator) validateSpecial(password string) error {
	specialChars := "!@#$%^&*()_+-=[]{}|;':\",./<>?"
	for _, c := range password {
		if strings.ContainsRune(specialChars, c) {
			return nil
		}
	}
	return errors.ErrPasswordSpecialRequired
}

func (pv *PasswordValidator) validateBreached(password string) error {
	hash := sha1.Sum([]byte(password)) //nolint:gosec // HIBP API requires SHA-1 for k-anonymity
	hashHex := strings.ToUpper(hex.EncodeToString(hash[:]))
	prefix := hashHex[:5]
	suffix := hashHex[5:]

	url := fmt.Sprintf("https://api.pwnedpasswords.com/range/%s", prefix)

	resp, err := pv.httpClient.Get(url)
	if err != nil {
		return nil
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(body), "\n")
	for _, line := range lines {
		parts := strings.Split(line, ":")
		if len(parts) >= 1 && strings.TrimSpace(parts[0]) == suffix {
			return errors.ErrPasswordBreached
		}
	}

	return nil
}
