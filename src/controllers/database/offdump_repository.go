package database

import (
	"context"
	"errors"

	"github.com/Isotop7/proviant/models/database"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OffDumpRepository owns access to the imported OpenFoodFacts bulk dump. The
// store is a dedicated SQLite file or MariaDB schema, separate from the main
// database, so a bulk import or re-import never touches household data.
type OffDumpRepository struct {
	DB *gorm.DB
}

// NewOffDumpRepository constructs a repository over an already-opened dump handle.
func NewOffDumpRepository(db *gorm.DB) *OffDumpRepository {
	return &OffDumpRepository{DB: db}
}

// Migrate creates the dump tables on the dump handle. It never creates a
// schema: a missing MariaDB schema is a configuration error the caller handles.
func (r *OffDumpRepository) Migrate() error {
	return r.DB.AutoMigrate(&database.OffProduct{}, &database.OffDumpState{})
}

// UpsertBatch inserts or replaces the given products in one statement.
func (r *OffDumpRepository) UpsertBatch(ctx context.Context, rows []database.OffProduct) error {
	if len(rows) == 0 {
		return nil
	}
	return r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "barcode"}},
		DoUpdates: clause.AssignmentColumns([]string{"data"}),
	}).Create(rows).Error
}

// Lookup returns the stored filtered JSON for a barcode, and whether it exists.
func (r *OffDumpRepository) Lookup(barcode string) (string, bool, error) {
	var product database.OffProduct
	err := r.DB.Where("barcode = ?", barcode).First(&product).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return product.Data, true, nil
}

// GetState returns a state value, or "" when it has never been set.
func (r *OffDumpRepository) GetState(key string) (string, error) {
	var state database.OffDumpState
	err := r.DB.Where("state_key = ?", key).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return state.Value, nil
}

// SetState writes a state value, replacing any previous value.
func (r *OffDumpRepository) SetState(key, value string) error {
	return r.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "state_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&database.OffDumpState{StateKey: key, Value: value}).Error
}

// Count returns the number of stored products.
func (r *OffDumpRepository) Count() (int64, error) {
	var count int64
	err := r.DB.Model(&database.OffProduct{}).Count(&count).Error
	return count, err
}

// Close releases the underlying connection pool.
func (r *OffDumpRepository) Close() error {
	sqlDB, err := r.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
