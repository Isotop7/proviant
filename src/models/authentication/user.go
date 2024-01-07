package authentication

import (
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
func (u User) IsValid() bool {
	return u.ID > 0 && u.Username != "" && u.Password != ""
}
