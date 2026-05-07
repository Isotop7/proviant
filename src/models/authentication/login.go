package authentication

import "codeberg.org/isotop7/proviant/errors"

var defaultPasswordConfig = PasswordConfig{
	MinLength:        12,
	RequireUppercase: true,
	RequireDigit:     true,
	RequireSpecial:   false,
	CheckBreached:    true,
}

var defaultPasswordValidator = NewPasswordValidator(defaultPasswordConfig)

func DefaultPasswordValidator() *PasswordValidator {
	return defaultPasswordValidator
}

func PasswordValidatorFromConfig(cfg PasswordConfig) *PasswordValidator {
	return NewPasswordValidator(cfg)
}

// Login is derived from User and primarily used for sign in
type Login struct {
	Username string `form:"username" json:"username" binding:"required"`
	Password string `form:"password" json:"password" binding:"required"`
}

// IsValid checks if the given login instance is valid
func (login *Login) IsValid() error {
	return login.IsValidWithValidator(defaultPasswordValidator)
}

// IsValidWithValidator checks if the given login instance is valid using a custom validator
func (login *Login) IsValidWithValidator(validator *PasswordValidator) error {
	if login.Username == "" {
		return errors.ErrUsernameEmpty
	}

	if err := validator.Validate(login.Password); err != nil {
		return err
	}

	return nil
}
