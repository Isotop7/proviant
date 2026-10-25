package migrations

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// AssignHouseholdsToUsers ensures every user belongs to a household
// This fixes breaking changes of version 0.2.0
func assignHouseholdsToUsers(db *gorm.DB) error {
	// Fetch all users who are not assigned to a household
	var usersWithoutHouseholds []*authentication.User
	result := db.Where("household_id IS NULL or household_id = 0").Find(&usersWithoutHouseholds)
	if result.Error != nil {
		return fmt.Errorf("error fetching users without households: %v", result.Error)
	}

	// Assign each user a new household
	for _, user := range usersWithoutHouseholds {
		newHousehold := database.Household{
			Name:    fmt.Sprintf("%s's Household", user.Username),
			AdminID: user.ID, // Automatically set the user as the admin
		}

		// Create the household
		if err := db.Create(&newHousehold).Error; err != nil {
			return fmt.Errorf("error creating household for user '%s': %v", user.Username, err)
		}

		// Update the user's HouseholdID
		user.HouseholdID = newHousehold.ID
		if err := db.Save(&user).Error; err != nil {
			return fmt.Errorf("error updating user '%s' with household ID: %v", user.Username, err)
		}
	}

	return nil
}

// AddNotificationPreferencesMigration adds notification preference columns to users table
func AddNotificationPreferencesMigration(db *gorm.DB) error {
	// Try to add the columns - if they already exist, this will fail but that's okay
	err := db.Exec(`
		ALTER TABLE users
		ADD COLUMN email_enabled BOOLEAN DEFAULT TRUE,
		ADD COLUMN ntfy_enabled BOOLEAN DEFAULT FALSE,
		ADD COLUMN ntfy_url TEXT,
		ADD COLUMN ntfy_topic TEXT,
		ADD COLUMN ntfy_token TEXT
	`).Error

	// If the error is about duplicate columns, we can ignore it
	if err != nil && (strings.Contains(err.Error(), "duplicate column") ||
		strings.Contains(err.Error(), "already exists") ||
		strings.Contains(err.Error(), "already has column")) {
		return nil
	}

	return err
}

// SetDefaultProductQuantities sets quantity=1 for all existing products that have
// quantity=0 or NULL. When the quantity column is first added via AutoMigrate,
// existing rows receive NULL (not 0), so both cases must be handled.
func SetDefaultProductQuantities(logger *zerolog.Logger, db *gorm.DB) error {
	return db.Exec("UPDATE products SET quantity = 1 WHERE quantity IS NULL OR quantity = 0").Error
}

// legacyAmountRow is the projection the conversion scans: only the columns
// the conversion needs are read, so a table with wide rows is not fully
// materialised per batch. The column names are pinned because they do not
// follow from the field names. The amount is nullable because a row that
// predates the column holds NULL there, and a NULL amount must not be read
// as a net content of zero.
type legacyAmountRow struct {
	ID       uint          `gorm:"column:id"`
	Amount   sql.NullInt64 `gorm:"column:amount"`
	Unit     string        `gorm:"column:unit"`
	MinStock int           `gorm:"column:min_stock_amount"`
	PackSize string        `gorm:"column:pack_size"`
}

// netContent reads the stored net content of a row. A row that holds no
// amount — NULL or zero — was given the default of one by the migration that
// preceded the count model, so it reads as one here too: a measured row keeps
// its unit as the size of its pack instead of losing it.
func (row *legacyAmountRow) netContent() int {
	if !row.Amount.Valid || row.Amount.Int64 < 1 {
		return 1
	}
	return int(row.Amount.Int64)
}

// legacyIngredientRow is the projection the recipe ingredient conversion
// scans: only the columns the conversion needs are read, so a table with wide
// rows is not fully materialised per batch. The column names are pinned
// because they do not follow from the field names. The amount is nullable
// because a row that predates the column holds NULL there.
type legacyIngredientRow struct {
	ID      uint          `gorm:"column:id"`
	Amount  sql.NullInt64 `gorm:"column:amount"`
	Unit    string        `gorm:"column:unit"`
	Portion string        `gorm:"column:portion"`
}

// ConvertRecipeIngredientsToPortions rewrites the amount and the unit of every
// recipe ingredient into a single free-text portion. The two fields were the
// last amount-plus-unit pair in the codebase, and the stock is counted in
// packs, so a portion is a label the recipe states and never a number the
// system computes with. Soft-deleted rows are included so a restored recipe
// comes back with the same portion. The conversion is idempotent: a row that
// already carries a portion is left alone, and a database that no longer
// carries the old columns is skipped entirely.
func ConvertRecipeIngredientsToPortions(logger *zerolog.Logger, db *gorm.DB) error {
	if !db.Migrator().HasColumn(&database.RecipeIngredient{}, "amount") {
		return nil
	}
	rows := make([]legacyIngredientRow, 0)
	converted := 0
	columns := "id, amount, unit"
	if db.Migrator().HasColumn(&database.RecipeIngredient{}, "portion") {
		columns += ", portion"
	}
	err := db.Table("recipe_ingredients").
		Select(columns).
		FindInBatches(&rows, 500, func(tx *gorm.DB, _ int) error {
			rewrites := make(map[uint]string, len(rows))
			for i := range rows {
				row := &rows[i]
				if row.Portion != "" {
					continue
				}
				rewrites[row.ID] = legacyIngredientPortion(row)
			}
			for id, portion := range rewrites {
				if updateErr := tx.Table("recipe_ingredients").
					Where(util.QueryId, id).
					Update("portion", portion).Error; updateErr != nil {
					return updateErr
				}
				converted++
			}
			return nil
		}).Error
	if err != nil {
		return err
	}
	logger.Info().
		Int("ingredients", converted).
		Msg("Converted recipe ingredient amounts into portions")
	return nil
}

// legacyIngredientPortion composes the portion of a recipe ingredient from the
// amount and the unit it was stored as. A row without an amount keeps the unit
// alone, and a row without a unit keeps the amount alone. A zero amount means
// the ingredient carried no amount, so the unit alone is the portion.
func legacyIngredientPortion(row *legacyIngredientRow) string {
	amount := ""
	if row.Amount.Valid && row.Amount.Int64 > 0 {
		amount = strconv.FormatInt(row.Amount.Int64, 10)
	}
	unit := strings.TrimSpace(row.Unit)
	switch {
	case amount == "":
		return unit
	case unit == "":
		return amount
	}
	return amount + " " + unit
}

// ConvertProductAmountsToQuantities rewrites the stored net content of every
// product into a count of packs. A product measured in grams or millilitres
// held the content of its pack, so it becomes one pack whose size is that
// content; a product counted in items already held a count and keeps it. The
// minimum stock moves with it: it was a net content next to the amount, so it
// becomes the number of packs that content is worth. Soft-deleted rows are
// included so an archived product comes back with the same count. The
// conversion is idempotent: a row that already carries a pack size is left
// alone, and a database that no longer carries the amount column is skipped
// entirely.
func ConvertProductAmountsToQuantities(logger *zerolog.Logger, db *gorm.DB) error {
	if !db.Migrator().HasColumn(&database.Product{}, "amount") {
		return nil
	}
	rows := make([]legacyAmountRow, 0)
	converted := 0
	columns := "id, amount, unit, min_stock_amount"
	if db.Migrator().HasColumn(&database.Product{}, "pack_size") {
		columns += ", pack_size"
	}
	err := db.Table("products").
		Select(columns).
		FindInBatches(&rows, 500, func(tx *gorm.DB, _ int) error {
			// The rewrites are collected and applied after the batch scan
			// finishes: updating the scanned table while its rows are still
			// open errors on MySQL ("command out of sync") and can deadlock
			// on PostgreSQL.
			rewrites := make(map[uint]map[string]any, len(rows))
			for i := range rows {
				row := &rows[i]
				if row.PackSize != "" {
					continue
				}
				netContent := row.netContent()
				quantity, packSize := util.PackSizeFromLegacyAmount(netContent, row.Unit)
				rewrites[row.ID] = map[string]any{
					"quantity":         quantity,
					"pack_size":        packSize,
					"min_stock_amount": legacyMinStockPacks(netContent, row.MinStock, row.Unit),
				}
			}
			for id, values := range rewrites {
				if updateErr := tx.Table("products").
					Where(util.QueryId, id).
					Updates(values).Error; updateErr != nil {
					return updateErr
				}
				converted++
			}
			return nil
		}).Error
	if err != nil {
		return err
	}
	logger.Info().
		Int("products", converted).
		Msg("Converted product amounts into pack counts")
	return nil
}

// legacyMinStockPacks converts a minimum stock that was stored next to a net
// content into a count of packs. A counted product already stored its minimum
// as a count and keeps it; a measured one stored it as a net content in the
// same unit as the amount, so it becomes the number of packs that content is
// worth.
func legacyMinStockPacks(amount, minStock int, unit string) int {
	if minStock < 1 {
		return 0
	}
	trimmed := strings.TrimSpace(unit)
	if trimmed == "" || util.IsCountUnit(trimmed) {
		return minStock
	}
	if amount < 1 {
		return 1
	}
	packs := int(math.Round(float64(minStock) / float64(amount)))
	if packs < 1 {
		packs = 1
	}
	return min(packs, util.MaxProductQuantity)
}

// MarkLegacySavingsRecords flags the savings records that were written before
// the count model. Their money and CO2 were scaled by the net content of the
// product, so they are not comparable with the records after the cutover and
// are kept out of the aggregates.
//
// The cutover row is the marker that makes this a one-time step: it is written
// when the records are flagged and read on every later boot, so the records
// written after the cutover are never caught. Only the records that existed
// when the cutover ran are flagged.
func MarkLegacySavingsRecords(logger *zerolog.Logger, db *gorm.DB) error {
	var cutover database.ModelCutover
	err := db.Where("name = ?", util.CountModelCutover).First(&cutover).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	now := time.Now()
	result := db.Model(&database.SavingsRecord{}).
		Where("legacy IS NULL OR legacy = ?", false).
		Where("created_at <= ?", now).
		Update("legacy", true)
	if result.Error != nil {
		return result.Error
	}

	cutover = database.ModelCutover{Name: util.CountModelCutover, CutoverAt: now}
	if err := db.Create(&cutover).Error; err != nil {
		return err
	}

	logger.Info().
		Int64("records", result.RowsAffected).
		Msg("Marked the savings records written before the count model")
	return nil
}

// DropUnitColumns removes the columns that carried units and net contents.
// GORM AutoMigrate never drops columns, so this must be done explicitly. A
// database that never had them is left alone, which makes the step
// idempotent.
func DropUnitColumns(logger *zerolog.Logger, db *gorm.DB) error {
	columns := []struct {
		table  string
		column string
	}{
		{table: "products", column: "amount"},
		{table: "products", column: "unit"},
		{table: "shopping_list_items", column: "unit"},
		{table: "households", column: "unit_system"},
		{table: "recipe_ingredients", column: "amount"},
		{table: "recipe_ingredients", column: "unit"},
	}
	for _, column := range columns {
		if err := dropColumn(logger, db, column.table, column.column); err != nil {
			return err
		}
	}
	return nil
}

// dropColumn removes one column, tolerating a database that does not have it.
// SQLite does not support IF EXISTS on DROP COLUMN, so the drop is attempted
// and any error that reports the column as absent is swallowed. A database
// that cannot drop a column at all fails the drop, and that failure is
// reported: the column stays, but the operator has to know.
func dropColumn(logger *zerolog.Logger, db *gorm.DB, table, column string) error {
	err := db.Exec(fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", table, column)).Error
	if err == nil {
		return nil
	}
	if isMissingColumnError(db.Name(), err) {
		logger.Debug().Msgf("%s.%s column not present, skipping", table, column)
		return nil
	}
	return err
}

// isMissingColumnError reports whether the driver complained that the column
// (or the table) does not exist. A broken statement must not be mistaken for
// an absent column, so the match is limited to the wording each driver uses
// for a missing object.
func isMissingColumnError(dialectorName string, err error) bool {
	errStr := err.Error()
	switch dialectorName {
	case "sqlite":
		return strings.Contains(errStr, "no such column") || strings.Contains(errStr, "no such table")
	case "mysql":
		return strings.Contains(errStr, "Unknown column") || strings.Contains(errStr, "Can't DROP")
	case "postgres":
		return strings.Contains(errStr, "does not exist") || strings.Contains(errStr, "undefined column")
	}
	return strings.Contains(errStr, "no such column") ||
		strings.Contains(errStr, "no such table") ||
		strings.Contains(errStr, "Unknown column") ||
		strings.Contains(errStr, "Can't DROP") ||
		strings.Contains(errStr, "does not exist") ||
		strings.Contains(errStr, "undefined column")
}

// DropLegacyStorageLocationColumn removes the old free-text storage_location column from products.
// GORM AutoMigrate never drops columns, so this must be done explicitly.
// SQLite does not support IF EXISTS on DROP COLUMN, so we attempt the drop and swallow
// any error that indicates the column is already absent.
func DropLegacyStorageLocationColumn(logger *zerolog.Logger, db *gorm.DB) error {
	err := db.Exec(`ALTER TABLE products DROP COLUMN storage_location`).Error
	if err != nil {
		if isMissingColumnError(db.Name(), err) {
			logger.Debug().Msg("storage_location column not present, skipping")
			return nil
		}
		return err
	}
	return nil
}

// SeedDefaultStorageLocations creates Fridge, Freezer and Pantry for every
// existing household that has no storage locations yet.
func SeedDefaultStorageLocations(logger *zerolog.Logger, db *gorm.DB) error {
	type defaultLocation struct {
		Name      string
		Icon      string
		SortOrder int
	}
	defaults := []defaultLocation{
		{"Fridge", "🧊", 0},
		{"Freezer", "❄️", 1},
		{"Pantry", "🗄️", 2},
	}

	var householdIDs []uint
	if err := db.Model(&database.Household{}).Pluck("id", &householdIDs).Error; err != nil {
		return fmt.Errorf("fetching household IDs: %w", err)
	}

	for _, hhID := range householdIDs {
		var count int64
		db.Model(&database.StorageLocation{}).Where(util.QueryHouseholdId, hhID).Count(&count)
		if count > 0 {
			continue
		}
		for _, d := range defaults {
			loc := database.StorageLocation{
				HouseholdID: hhID,
				Name:        d.Name,
				Icon:        d.Icon,
				SortOrder:   d.SortOrder,
			}
			if err := db.Create(&loc).Error; err != nil {
				return fmt.Errorf("seeding location '%s' for household %d: %w", d.Name, hhID, err)
			}
		}
	}
	return nil
}

// AddPerformanceIndexes creates missing indexes for product and user tables
// to improve query performance for common access patterns.
func AddPerformanceIndexes(logger *zerolog.Logger, db *gorm.DB) error {
	// On MariaDB/MySQL, hint online DDL for new composite indexes so the
	// migration does not take a long table lock or stall replication on
	// large `products` tables. SQLite ignores ALGORITHM/LOCK clauses, so
	// the bare CREATE INDEX works there.
	isMariaDB := db.Name() == "mysql"

	statements := []string{
		"CREATE INDEX IF NOT EXISTS idx_products_household_deleted ON products(household_id, deleted_at)",
		"CREATE INDEX IF NOT EXISTS idx_products_expire_at ON products(expire_at)",
		"CREATE INDEX IF NOT EXISTS idx_products_barcode_household ON products(barcode, household_id)",
		"CREATE INDEX IF NOT EXISTS idx_products_household_name_deleted ON products(household_id, product_name, deleted_at)",
		"CREATE INDEX IF NOT EXISTS idx_users_mail_address ON users(mail_address)",
		"CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)",
		// Composite indexes supporting the effectiveExpiryCandidateScope
		// OR-across-columns predicate introduced by the "secondary expiry after opening" feature.
		// On SQLite the OR across columns cannot use a single-column index; the
		// composite (household_id, opened_at) / (household_id, expire_at)
		// indexes let each branch of the OR use an index. MariaDB can use
		// index_merge(union) on top of these; InnoDB will fall back to one
		// index or a scan.
		"CREATE INDEX IF NOT EXISTS idx_products_household_opened_at ON products(household_id, opened_at)",
		"CREATE INDEX IF NOT EXISTS idx_products_household_expire_at ON products(household_id, expire_at)",
	}
	for _, sql := range statements {
		if isMariaDB && (strings.Contains(sql, "products(household_id, product_name, deleted_at)") ||
			strings.Contains(sql, "products(household_id, opened_at)") ||
			strings.Contains(sql, "products(household_id, expire_at)")) {
			sql += " ALGORITHM=INPLACE, LOCK=NONE"
		}
		if err := db.Exec(sql).Error; err != nil {
			return fmt.Errorf("creating index: %w (sql: %s)", err, sql)
		}
	}
	return nil
}

// BackfillProductUserID sets user_id for existing products that are marked private but have no owner.
// For private products without an owner, assigns the household admin as the owner so they remain visible.
func BackfillProductUserID(logger *zerolog.Logger, db *gorm.DB) error {
	result := db.Exec(`
		UPDATE products
		SET user_id = (
			SELECT users.id FROM users
			WHERE users.household_id = products.household_id
			AND users.role = 'admin'
			LIMIT 1
		)
		WHERE is_private = 1 AND user_id = 0
	`)
	if result.Error != nil {
		return result.Error
	}
	logger.Info().Int64("count", result.RowsAffected).Msg("Backfilled product owners for private products")
	return nil
}

func RunBreakingDatabaseMigrations(logger *zerolog.Logger, db *gorm.DB) error {
	logger.Info().Msg("Running database migrations")

	logger.Debug().Msg("Running database migrations to assign households to users")
	if err := assignHouseholdsToUsers(db); err != nil {
		return err
	}

	// Migrations for notification preferences
	logger.Debug().Msg("Running database migrations for notification preferences")
	if err := AddNotificationPreferencesMigration(db); err != nil {
		return err
	}

	// Migrations for push notifications
	logger.Debug().Msg("Running database migrations for web push notifications")
	if err := AddWebPushNotificationMigration(db); err != nil {
		return err
	}

	logger.Debug().Msg("Renaming legacy push columns to web_push columns")
	if err := RenamePushNotificationColumns(logger, db); err != nil {
		return err
	}

	// Backfill email verification for existing users
	logger.Debug().Msg("Running database migrations for email verification backfill")
	if err := BackfillEmailVerification(logger, db); err != nil {
		return err
	}

	// Drop legacy free-text storage_location column from products
	logger.Debug().Msg("Drop legacy storage location columns")
	if err := DropLegacyStorageLocationColumn(logger, db); err != nil {
		return err
	}

	// Seed default storage locations for existing households
	logger.Debug().Msg("Backfill database with default storage locations")
	if err := SeedDefaultStorageLocations(logger, db); err != nil {
		return err
	}

	// Add performance indexes
	logger.Debug().Msg("Add performance indexes")
	if err := AddPerformanceIndexes(logger, db); err != nil {
		return err
	}

	// Backfill removal_reason for existing consumed products
	logger.Debug().Msg("Backfill removal_reason for consumed products")
	if err := BackfillRemovalReason(logger, db); err != nil {
		return err
	}

	// Backfill household roles for existing users
	logger.Debug().Msg("Backfill household roles")
	if err := BackfillHouseholdRoles(logger, db); err != nil {
		return err
	}

	// Backfill product owners for private products
	logger.Debug().Msg("Backfill product owners for private products")
	if err := BackfillProductUserID(logger, db); err != nil {
		return err
	}

	// Attribute pre-existing audit log entries to a household
	logger.Debug().Msg("Backfill audit log household attribution")
	if err := BackfillAuditLogHouseholdID(logger, db); err != nil {
		return err
	}

	// Restore the opened-shelf-life bound that the SQL prunes assume
	logger.Debug().Msg("Backfill clamp days_after_opening to the validated maximum")
	if err := BackfillClampDaysAfterOpening(logger, db); err != nil {
		return err
	}

	// Turn the stored net contents into pack counts before the columns that
	// carried them are dropped
	logger.Debug().Msg("Convert product amounts into pack counts")
	if err := ConvertProductAmountsToQuantities(logger, db); err != nil {
		return err
	}

	logger.Debug().Msg("Mark the savings records written before the count model")
	if err := MarkLegacySavingsRecords(logger, db); err != nil {
		return err
	}

	logger.Debug().Msg("Convert recipe ingredient amounts into portions")
	if err := ConvertRecipeIngredientsToPortions(logger, db); err != nil {
		return err
	}

	logger.Debug().Msg("Fill the canonical category of the products")
	if err := BackfillProductCategoryKeys(logger, db); err != nil {
		return err
	}

	logger.Debug().Msg("Drop unit and net content columns")
	if err := DropUnitColumns(logger, db); err != nil {
		return err
	}
	return nil
}

// BackfillAuditLogHouseholdID attributes existing audit log entries to the
// household their actor belonged to at the time of the run. It is idempotent
// rather than one-shot: RunBreakingDatabaseMigrations runs it on every
// startup, and the household_id IS NULL predicate means already-attributed
// rows are never touched again. The EXISTS guard excludes rows whose actor is
// gone (or was never known, as with a failed login for an unknown username):
// they can never be attributed, keep household_id NULL, and stay invisible to
// every household — which is the safe direction to err in — without being
// re-scanned and re-counted on every boot.
//
// Attribution uses the household the user is in *now*: a member who changed
// households before this release has their older entries stamped into the
// household they currently belong to. That is a one-time, best-effort
// compromise — there is no membership history table to attribute those rows
// any more precisely.
//
// The subqueries deliberately ignore users.deleted_at: account_deleted is
// written after the target user is soft-deleted, and that entry still belongs
// to the household. The EXISTS guard also requires a non-zero household:
// stamping 0 would take the row out of the NULL set for good, so an actor
// without a household keeps household_id NULL and is attributed once they
// join one, instead of being frozen at 0 forever.
func BackfillAuditLogHouseholdID(logger *zerolog.Logger, db *gorm.DB) error {
	result := db.Exec(`
		UPDATE audit_logs
		SET household_id = (
			SELECT users.household_id FROM users
			WHERE users.id = audit_logs.user_id
		)
		WHERE household_id IS NULL
		AND user_id IS NOT NULL
		AND user_id != 0
		AND EXISTS (
			SELECT 1 FROM users
			WHERE users.id = audit_logs.user_id
			AND users.household_id != 0
		)
	`)
	if result.Error != nil {
		return result.Error
	}
	logger.Info().Int64("count", result.RowsAffected).Msg("Backfilled audit log household attribution")
	return nil
}

// BackfillClampDaysAfterOpening clamps stored days_after_opening values to
// database.MaxDaysAfterOpening (365). The create path (POST /api/v1/products
// binds the raw model) accepted unvalidated values until the repository-level
// check was added, and effectiveExpiryCandidateScope prunes opened_at rows
// using that same bound: a row with days_after_opening = 1000 and opened_at
// 995 days ago has an effective expiry inside a 7-day window but sits below
// the prune floor, so it would silently miss the expiring-soon list and its
// notification. The clamp is idempotent, runs on every startup, and only
// touches rows above the bound. Clamping (rather than deleting) moves such
// rows to "expired", which is the safe direction: visible, not hidden.
func BackfillClampDaysAfterOpening(logger *zerolog.Logger, db *gorm.DB) error {
	result := db.Exec(
		"UPDATE products SET days_after_opening = ? WHERE days_after_opening > ?",
		database.MaxDaysAfterOpening, database.MaxDaysAfterOpening,
	)
	if result.Error != nil {
		return result.Error
	}
	logger.Info().Int64("count", result.RowsAffected).Msg("Clamped days_after_opening to the validated maximum")
	return nil
}

// BackfillRemovalReason sets removal_reason = "consumed" for all existing soft-deleted products
// that have no removal reason set. These predate the RemovalReason field and were all consumed
// (wasted products were hard-deleted at the time and therefore absent from the table).
func BackfillRemovalReason(logger *zerolog.Logger, db *gorm.DB) error {
	result := db.Unscoped().Exec(
		"UPDATE products SET removal_reason = ? WHERE deleted_at IS NOT NULL AND (removal_reason IS NULL OR removal_reason = '')",
		database.RemovalReasonConsumed,
	)
	if result.Error != nil {
		return result.Error
	}
	logger.Info().Int64("count", result.RowsAffected).Msg("Backfilled removal_reason for consumed products")
	return nil
}

// BackfillEmailVerification sets EmailVerifiedAt for all existing users that don't have it set.
// This is a one-time migration to ensure existing users aren't locked out after email verification is introduced.
func BackfillEmailVerification(logger *zerolog.Logger, db *gorm.DB) error {
	result := db.Exec("UPDATE users SET email_verified_at = ? WHERE email_verified_at IS NULL", time.Now())
	if result.Error != nil {
		return result.Error
	}

	logger.Info().Int64("count", result.RowsAffected).Msg("Backfilled email verification")
	return nil
}

// AddWebPushNotificationMigration adds web push notification columns to users table
func AddWebPushNotificationMigration(db *gorm.DB) error {
	err := db.Exec(`
		ALTER TABLE users
		ADD COLUMN web_push_enabled BOOLEAN DEFAULT FALSE,
		ADD COLUMN web_push_subscription_json TEXT
	`).Error

	if err != nil && (strings.Contains(err.Error(), "duplicate column") ||
		strings.Contains(err.Error(), "already exists") ||
		strings.Contains(err.Error(), "already has column")) {
		return nil
	}

	return err
}

// RenamePushNotificationColumns renames legacy push columns to web_push prefix
func RenamePushNotificationColumns(logger *zerolog.Logger, db *gorm.DB) error {
	var columnCount int64
	db.Raw("SELECT COUNT(*) FROM pragma_table_info('users') WHERE name = 'push_enabled'").Scan(&columnCount)
	if columnCount == 0 {
		logger.Debug().Msg("push_enabled column already renamed, skipping")
		return nil
	}

	stmts := []string{
		"ALTER TABLE users RENAME COLUMN push_enabled TO web_push_enabled",
		"ALTER TABLE users RENAME COLUMN push_subscription_json TO web_push_subscription_json",
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			logger.Warn().Err(err).Msgf("Failed to rename column: %s", sql)
			return err
		}
	}
	logger.Info().Msg("Renamed push columns to web_push columns")
	return nil
}

// BackfillHouseholdRoles sets role='admin' for household admins and role='member' for all others.
// This migration ensures existing users get appropriate roles after the role field is added.
func BackfillHouseholdRoles(logger *zerolog.Logger, db *gorm.DB) error {
	result := db.Exec(`
		UPDATE users
		SET role = 'admin'
		WHERE EXISTS (
			SELECT 1 FROM households WHERE households.id = users.household_id AND households.admin_id = users.id
		)
	`)
	if result.Error != nil {
		return result.Error
	}
	logger.Info().Int64("admins", result.RowsAffected).Msg("Backfilled admin roles")

	result = db.Exec(`
		UPDATE users
		SET role = 'member'
		WHERE (role IS NULL OR role = '')
		AND household_id IS NOT NULL AND household_id != 0
	`)
	if result.Error != nil {
		return result.Error
	}
	logger.Info().Int64("members", result.RowsAffected).Msg("Backfilled member roles")

	return nil
}
