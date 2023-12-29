package authentication

import (
	"gitlab.com/Isotop7/expiro/models/database"

	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	ID       uint   `gorm:"primaryKey,unique"`
	Username string `json:"username"`
	Password string `json:"-"`
	Products []database.Product
}

func (u User) IsValid() bool {
	return u.ID > 0 && u.Username != "" && u.Password != ""
}
