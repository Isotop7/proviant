package migrations

import (
	controllerDB "github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// productCategoryRow is the projection the backfill scans: only the columns
// it needs are read, so a table with wide rows is not fully materialised per
// batch.
type productCategoryRow struct {
	ID         uint   `gorm:"column:id"`
	Categories string `gorm:"column:categories"`
}

// BackfillProductCategoryKeys fills the canonical category of the products
// that carry free-text categories.
//
// The free text itself is left alone: the recipe suggestions read it as
// Open Food Facts wrote it, and rewriting it would change the keywords they
// look for. The canonical key is what the household's shelf-life defaults are
// looked up by, and it is derived from the same rule the savings and the
// waste analytics use.
//
// A product whose text names no canonical category stays uncategorised —
// there is no key to give it. The backfill is idempotent: a row that already
// carries a category is left alone.
func BackfillProductCategoryKeys(logger *zerolog.Logger, db *gorm.DB) error {
	if !db.Migrator().HasColumn(&database.Product{}, "category_key") {
		return nil
	}

	resolver, err := controllerDB.LoadCategoryResolver(db)
	if err != nil {
		return err
	}
	if len(resolver.Keys()) == 0 {
		logger.Info().Msg("No canonical categories, skipping the product category backfill")
		return nil
	}

	rows := make([]productCategoryRow, 0)
	backfilled := 0
	err = db.Table("products").
		Select("id, categories").
		Where("category_key IS NULL").
		Where("categories IS NOT NULL AND categories != ''").
		FindInBatches(&rows, 500, func(tx *gorm.DB, _ int) error {
			for i := range rows {
				key, ok := resolver.ResolveKey(rows[i].Categories)
				if !ok {
					continue
				}
				updateErr := tx.Table("products").
					Where(util.QueryId, rows[i].ID).
					Update("category_key", key).Error
				if updateErr != nil {
					return updateErr
				}
				backfilled++
			}
			return nil
		}).Error
	if err != nil {
		return err
	}

	logger.Info().
		Int("products", backfilled).
		Msg("Filled the canonical category of the products that had none")
	return nil
}
