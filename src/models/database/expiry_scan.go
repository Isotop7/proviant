package database

import (
	"time"

	"gorm.io/gorm"
)

// ExpiryScan records a single OCR scan for expiry date detection
type ExpiryScan struct {
	gorm.Model
	ProductID     *uint      `gorm:"index" json:"productId,omitempty"`
	UserID        uint       `gorm:"index, not null" json:"userId"`
	ScannedAt     time.Time  `json:"scannedAt"`
	DetectedDate  time.Time  `json:"detectedDate"`
	Confidence    float64    `json:"confidence"`
	RawText       string     `json:"rawText"`                 // full OCR output
	ImageHash     string     `gorm:"index" json:"imageHash"`  // SHA256 for dedup
	CorrectedDate *time.Time `json:"correctedDate,omitempty"` // if user modified
}
