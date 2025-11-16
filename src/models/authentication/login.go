package authentication

import "gitlab.com/Isotop7/proviant/errors"

// Login is derived from User and primarily used for sign in
type Login struct {
	Username string `form:"username" json:"username" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

// IsValid checks if the given login instance is valid
func (login *Login) IsValid() error {
	if login.Username == "" {
		return errors.ErrUsernameEmpty
	}

	if len(login.Password) < 8 {
		return errors.ErrPasswordTooShort
	}

	return nil
}
