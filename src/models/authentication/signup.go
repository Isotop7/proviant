package authentication

import (
	"errors"
	"net/mail"
)

var (
	// ErrUsernameEmpty is thrown when the given username is too short
	ErrUsernameEmpty = errors.New("username can't be empty")

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
	if len(signup.Username) == 0 {
		return ErrUsernameEmpty
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
