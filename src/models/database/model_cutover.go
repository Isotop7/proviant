package database

import (
	"time"

	"gorm.io/gorm"
)

// ModelCutover marks the moment a change to the data model took effect. The
// records written before it belong to the old model and are not comparable with
// the ones written after it, so the aggregates leave them out.
type ModelCutover struct {
	gorm.Model
	Name      string    `gorm:"uniqueIndex;not null" json:"name"`
	CutoverAt time.Time `gorm:"not null"          json:"cutoverAt"`
}
