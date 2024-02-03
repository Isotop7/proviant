package authentication

import (
	"net/mail"

	"gitlab.com/Isotop7/expiro/errors"
	"gitlab.com/Isotop7/expiro/models/database"

	"gorm.io/gorm"
)

// User is the struct for the database definition and the JWT claims
// A single user can own many products
type User struct {
	gorm.Model
	ID          uint   `gorm:"primaryKey,unique"`
	Username    string `json:"username"`
	MailAddress string `json:"mailAddress"`
	Password    string `json:"-"`
	Products    []database.Product
}

// IsValid is a simple validator function to check for valid properties
func (user User) IsValid(skipPassword bool) error {
	if user.ID < 1 {
		return errors.ErrInvalidUserID
	}

	if len(user.Username) == 0 {
		return errors.ErrUsernameEmpty
	}

	if !skipPassword {
		if len(user.Password) < 8 {
			return errors.ErrPasswordTooShort
		}
	}

	_, mailParseErr := mail.ParseAddress(user.MailAddress)
	if mailParseErr != nil {
		return mailParseErr
	}

	return nil
}
