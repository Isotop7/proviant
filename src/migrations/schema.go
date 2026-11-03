package migrations

import (
	"github.com/Isotop7/proviant/models"

	"gorm.io/gorm"
)

// AutoMigrateAll creates or updates the schema of every stored model.
func AutoMigrateAll(db *gorm.DB) error {
	return db.AutoMigrate(models.AllModels()...)
}
