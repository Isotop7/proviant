package controllers

import (
	"expiro/backend/models/auth"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type DatabaseController struct {
	DB *gorm.DB
}

func (db DatabaseController) FindUserByUsername(username string) (auth.User, error) {
	var user auth.User
	selectErr := db.DB.First(&user, "username = ?", username)
	return user, selectErr.Error
}

func (db DatabaseController) VerifyPassword(user *auth.User, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	return err == nil
}

func (db DatabaseController) Exists(user auth.User) bool {
	var dbUser auth.User
	selectErr := db.DB.First(&dbUser, "id = ? AND username = ?", user.ID, user.Username)
	return !(selectErr.Error == gorm.ErrRecordNotFound)
}

func (db DatabaseController) GetNextUserID() uint {
	var lastUser auth.User
	db.DB.Order("id").Limit(1).Find(&lastUser)
	return (lastUser.ID + 1)
}

func (db DatabaseController) Create(user *auth.User) error {
	hashedPassword, hashError := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if hashError != nil {
		return hashError
	}
	user.Password = string(hashedPassword)
	createResult := db.DB.Create(user)
	return createResult.Error
}
