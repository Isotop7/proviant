package migrations

import (
	"fmt"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

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

// SetDefaultProductAmounts sets amount=1 for all existing products that have amount=0 or NULL.
// When the amount column is first added via AutoMigrate, existing rows receive NULL (not 0),
// so both cases must be handled.
func SetDefaultProductAmounts(logger *zerolog.Logger, db *gorm.DB) error {
	return db.Exec("UPDATE products SET amount = 1 WHERE amount IS NULL OR amount = 0").Error
}

// DropLegacyStorageLocationColumn removes the old free-text storage_location column from products.
// GORM AutoMigrate never drops columns, so this must be done explicitly.
// SQLite does not support IF EXISTS on DROP COLUMN, so we attempt the drop and swallow
// any error that indicates the column is already absent.
func DropLegacyStorageLocationColumn(logger *zerolog.Logger, db *gorm.DB) error {
	err := db.Exec(`ALTER TABLE products DROP COLUMN storage_location`).Error
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "no such column") ||
			strings.Contains(errStr, "no such table") ||
			strings.Contains(errStr, "Unknown column") ||
			strings.Contains(errStr, "Can't DROP") ||
			strings.Contains(errStr, "syntax error") {
			logger.Debug().Msg("storage_location column not present or not droppable, skipping")
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
