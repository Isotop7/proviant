package authentication

import "gorm.io/gorm"

type CalendarToken struct {
	gorm.Model
	UserID uint   `gorm:"index, not null"`
	Token  string `gorm:"uniqueIndex, not null"`
}
