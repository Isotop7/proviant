package auth

import "gorm.io/gorm"

type User struct {
	gorm.Model
	ID       uint   `gorm:"primaryKey,unique"`
	Username string `json:"username"`
	Password string `json:"-"`
}

func (u User) IsValid() bool {
	return u.ID > 0 && u.Username != "" && u.Password != ""
}
