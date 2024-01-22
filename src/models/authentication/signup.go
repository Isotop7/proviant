package authentication

import (
	"errors"
	"net/mail"
)

var (
	// ErrUsernameTooShort is thrown when the given username is too short
	ErrUsernameTooShort = errors.New("username must at least be 6 characters long")

	// ErrPasswordTooShort is thrown when the given password is too short
	ErrPasswordTooShort = errors.New("password must at least be 8 characters long")
)

// Signup is derived from User and Login and primarily used for registration
type Signup struct {
	Username    string `form:"username" json:"username" binding:"required"`
	Password    string `form:"password" json:"password" binding:"required"`
	MailAddress string `form:"mailAddress" json:"mailAddress" binding:"required"`
}

// IsValid checks if the given signup instance is valid
func (signup *Signup) IsValid() error {
	if len(signup.Username) < 6 {
		return ErrUsernameTooShort
	}

	if len(signup.Password) < 8 {
		return ErrPasswordTooShort
	}

	_, mailParseErr := mail.ParseAddress(signup.MailAddress)
	if mailParseErr != nil {
		return mailParseErr
	}

	return nil
}
