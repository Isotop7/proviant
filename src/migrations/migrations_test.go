package migrations

import (
	"testing"
	"time"

	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRunBreakingDatabaseMigrations(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	user1 := authentication.User{Username: "user1", Password: "password1"}
	user2 := authentication.User{Username: "user2", Password: "password2"}
	db.Create(&user1)
	db.Create(&user2)

	var dbUser1, dbUser2 authentication.User
	db.First(&dbUser1, user1.ID)
	db.First(&dbUser2, user2.ID)
	if dbUser1.HouseholdID != 0 {
		t.Errorf("dbUser1.HouseholdID = %v, want 0", dbUser1.HouseholdID)
	}
	if dbUser2.HouseholdID != 0 {
		t.Errorf("dbUser2.HouseholdID = %v, want 0", dbUser2.HouseholdID)
	}

	logger := zerolog.Nop()

	err = RunBreakingDatabaseMigrations(&logger, db)
	if err != nil {
		t.Errorf("RunBreakingDatabaseMigrations() error = %v", err)
	}

	db.First(&dbUser1, user1.ID)
	db.First(&dbUser2, user2.ID)
	if dbUser1.HouseholdID == 0 {
		t.Errorf("dbUser1.HouseholdID = %v, want non-zero", dbUser1.HouseholdID)
	}
	if dbUser2.HouseholdID == 0 {
		t.Errorf("dbUser2.HouseholdID = %v, want non-zero", dbUser2.HouseholdID)
	}

	var household1, household2 database.Household
	db.First(&household1, dbUser1.HouseholdID)
	db.First(&household2, dbUser2.HouseholdID)
	if household1.AdminID != user1.ID {
		t.Errorf("household1.AdminID = %v, want %v", household1.AdminID, user1.ID)
	}
	if household2.AdminID != user2.ID {
		t.Errorf("household2.AdminID = %v, want %v", household2.AdminID, user2.ID)
	}
	if len(household1.Name) < len(user1.Username) {
		t.Errorf("household1.Name = %v, want to contain %v", household1.Name, user1.Username)
	}
	if len(household2.Name) < len(user2.Username) {
		t.Errorf("household2.Name = %v, want to contain %v", household2.Name, user2.Username)
	}
}

func TestRunBreakingDatabaseMigrations_EmptyDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	logger := zerolog.Nop()

	err = RunBreakingDatabaseMigrations(&logger, db)
	if err != nil {
		t.Errorf("RunBreakingDatabaseMigrations() error = %v", err)
	}

	var count int64
	db.Model(&database.Household{}).Count(&count)
	if count != 0 {
		t.Errorf("count = %v, want 0", count)
	}
}

func TestRunBreakingDatabaseMigrations_UsersWithHouseholds(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	household := database.Household{Name: "Existing Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "user1",
		Password:    "password1",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	originalHouseholdID := user.HouseholdID

	logger := zerolog.Nop()

	err = RunBreakingDatabaseMigrations(&logger, db)
	if err != nil {
		t.Errorf("RunBreakingDatabaseMigrations() error = %v", err)
	}

	var dbUser authentication.User
	db.First(&dbUser, user.ID)
	if dbUser.HouseholdID != originalHouseholdID {
		t.Errorf("dbUser.HouseholdID = %v, want %v", dbUser.HouseholdID, originalHouseholdID)
	}
}

// TestBackfillAuditLogHouseholdID covers the one-time attribution of audit
// entries written before household_id existed. Entries that cannot be
// attributed (unknown username, no user at all) must stay NULL.
func TestBackfillAuditLogHouseholdID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	household := database.Household{Name: "Household"}
	db.Create(&household)
	user := authentication.User{
		Username:    "user1",
		Password:    "password1",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	seed := func(userID *uint) database.AuditLog {
		row := database.AuditLog{
			Timestamp: time.Now(),
			UserID:    userID,
			Action:    database.AuditActionLoginSuccess,
			IPAddress: "10.0.0.1",
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("failed to seed audit log: %v", err)
		}
		return row
	}

	knownUser := seed(&user.ID)
	unknownUsername := uint(0)
	noUsernameMatch := seed(&unknownUsername)
	noUser := seed(nil)
	// Non-zero id whose row is gone: the EXISTS guard must leave it alone so
	// it is not re-scanned (and re-counted) on every startup.
	goneUserID := user.ID + 1000
	danglingUser := seed(&goneUserID)
	// Actor without a household: stamping household_id = 0 would take the row
	// out of the NULL set and freeze it out of any future attribution.
	householdless := authentication.User{
		Username:    "householdless",
		Password:    "password",
		HouseholdID: 0,
	}
	if err := db.Create(&householdless).Error; err != nil {
		t.Fatalf("failed to create householdless user: %v", err)
	}
	householdlessEntry := seed(&householdless.ID)

	logger := zerolog.Nop()
	if err := BackfillAuditLogHouseholdID(&logger, db); err != nil {
		t.Fatalf("BackfillAuditLogHouseholdID() error = %v", err)
	}

	reload := func(row database.AuditLog) database.AuditLog {
		var reloaded database.AuditLog
		if err := db.First(&reloaded, row.ID).Error; err != nil {
			t.Fatalf("failed to reload audit log: %v", err)
		}
		return reloaded
	}

	if got := reload(knownUser); got.HouseholdID == nil || *got.HouseholdID != household.ID {
		t.Errorf("known user entry HouseholdID = %v, want %d", got.HouseholdID, household.ID)
	}
	if got := reload(noUsernameMatch); got.HouseholdID != nil {
		t.Errorf("unknown username entry HouseholdID = %v, want nil", *got.HouseholdID)
	}
	if got := reload(noUser); got.HouseholdID != nil {
		t.Errorf("userless entry HouseholdID = %v, want nil", *got.HouseholdID)
	}
	if got := reload(danglingUser); got.HouseholdID != nil {
		t.Errorf("dangling user entry HouseholdID = %v, want nil", got.HouseholdID)
	}
	if got := reload(householdlessEntry); got.HouseholdID != nil {
		t.Errorf("householdless user entry HouseholdID = %v, want nil (0 would never be re-attributed)", got.HouseholdID)
	}

	// Re-running must be a no-op, not a re-attribution.
	if err := BackfillAuditLogHouseholdID(&logger, db); err != nil {
		t.Fatalf("second BackfillAuditLogHouseholdID() error = %v", err)
	}
	if got := reload(knownUser); *got.HouseholdID != household.ID {
		t.Errorf("after re-run HouseholdID = %v, want %d", *got.HouseholdID, household.ID)
	}

	// A re-run must touch nothing: attributed rows left the IS NULL set and
	// the unattributable ones — gone actor, or actor without a household —
	// are excluded by the EXISTS guard.
	var unattributable int64
	if err := db.Model(&database.AuditLog{}).
		Where("household_id IS NULL AND user_id IS NOT NULL AND user_id != 0 " +
			"AND EXISTS (SELECT 1 FROM users WHERE users.id = audit_logs.user_id AND users.household_id != 0)").
		Count(&unattributable).Error; err != nil {
		t.Fatalf("failed to count unattributable rows: %v", err)
	}
	if unattributable != 0 {
		t.Errorf("rows still matching the backfill predicate = %d, want 0", unattributable)
	}

	// The householdless entry stays NULL instead of being frozen at 0, so it
	// is attributed once the actor joins a household.
	householdless.HouseholdID = household.ID
	if err := db.Save(&householdless).Error; err != nil {
		t.Fatalf("failed to move user into a household: %v", err)
	}
	if err := BackfillAuditLogHouseholdID(&logger, db); err != nil {
		t.Fatalf("third BackfillAuditLogHouseholdID() error = %v", err)
	}
	if got := reload(householdlessEntry); got.HouseholdID == nil || *got.HouseholdID != household.ID {
		t.Errorf("after joining a household HouseholdID = %v, want %d", got.HouseholdID, household.ID)
	}
}

// TestBackfillClampDaysAfterOpening pins the data-side half of the
// opened-shelf-life invariant: rows written before the create path validated
// days_after_opening get clamped to the bound the SQL prunes assume, rows at
// or below the bound are untouched, and a re-run changes nothing.
func TestBackfillClampDaysAfterOpening(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	over := 1000
	atBound := database.MaxDaysAfterOpening
	below := 14
	seed := []database.Product{
		{ProductName: "over bound", Barcode: "1000000000001", DaysAfterOpening: &over},
		{ProductName: "at bound", Barcode: "1000000000002", DaysAfterOpening: &atBound},
		{ProductName: "below bound", Barcode: "1000000000003", DaysAfterOpening: &below},
		{ProductName: "unset", Barcode: "1000000000004"},
	}
	for i := range seed {
		if err := db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("failed to seed product: %v", err)
		}
	}

	logger := zerolog.Nop()
	if err := BackfillClampDaysAfterOpening(&logger, db); err != nil {
		t.Fatalf("BackfillClampDaysAfterOpening() error = %v", err)
	}

	reload := func(barcode string) database.Product {
		t.Helper()
		var p database.Product
		if err := db.Where("barcode = ?", barcode).First(&p).Error; err != nil {
			t.Fatalf("failed to reload product %s: %v", barcode, err)
		}
		return p
	}

	if got := reload("1000000000001").DaysAfterOpening; got == nil || *got != database.MaxDaysAfterOpening {
		t.Errorf("over-bound DaysAfterOpening = %v, want %d", got, database.MaxDaysAfterOpening)
	}
	if got := reload("1000000000002").DaysAfterOpening; got == nil || *got != database.MaxDaysAfterOpening {
		t.Errorf("at-bound DaysAfterOpening = %v, want %d (must not change)", got, database.MaxDaysAfterOpening)
	}
	if got := reload("1000000000003").DaysAfterOpening; got == nil || *got != below {
		t.Errorf("below-bound DaysAfterOpening = %v, want %d (must not change)", got, below)
	}
	if got := reload("1000000000004").DaysAfterOpening; got != nil {
		t.Errorf("unset DaysAfterOpening = %v, want nil", *got)
	}

	// Re-run must be a no-op.
	if err := BackfillClampDaysAfterOpening(&logger, db); err != nil {
		t.Fatalf("second BackfillClampDaysAfterOpening() error = %v", err)
	}
	if got := reload("1000000000001").DaysAfterOpening; got == nil || *got != database.MaxDaysAfterOpening {
		t.Errorf("after re-run over-bound DaysAfterOpening = %v, want %d", got, database.MaxDaysAfterOpening)
	}
}

// TestConvertProductAmountsToQuantities pins the conversion of the stored net
// contents into pack counts: a measured product becomes one pack whose size is
// its content, a counted product keeps its count, and a minimum stock that was
// a net content becomes the number of packs it is worth.
func TestConvertProductAmountsToQuantities(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}
	// The columns the count model replaced are recreated so the conversion
	// has something to read.
	for _, column := range []string{"amount INTEGER", "unit TEXT"} {
		if execErr := db.Exec("ALTER TABLE products ADD COLUMN " + column).Error; execErr != nil {
			t.Fatalf("Failed to add the legacy column %s: %v", column, execErr)
		}
	}

	seed := []struct {
		barcode      string
		unit         string
		amount       int
		minStock     int
		wantCount    int
		wantPackSize string
		wantMinStock int
	}{
		{barcode: "2000000000001", unit: "g", amount: 1000, minStock: 1500, wantCount: 1, wantPackSize: "1000 g", wantMinStock: 2},
		{barcode: "2000000000002", unit: "ml", amount: 1000, minStock: 1000, wantCount: 1, wantPackSize: "1000 ml", wantMinStock: 1},
		{barcode: "2000000000003", unit: "pcs", amount: 6, minStock: 4, wantCount: 6, wantPackSize: "", wantMinStock: 4},
		{barcode: "2000000000004", unit: "", amount: 3, minStock: 0, wantCount: 3, wantPackSize: "", wantMinStock: 0},
		{barcode: "2000000000005", unit: "g", amount: 900, minStock: 0, wantCount: 1, wantPackSize: "900 g", wantMinStock: 0},
		{barcode: "2000000000006", unit: "kg", amount: 2, minStock: 6, wantCount: 1, wantPackSize: "2 kg", wantMinStock: 3},
		// The free-text units the old demo data used count items too, so they
		// keep their count and their minimum stock.
		{barcode: "2000000000007", unit: "bags", amount: 50, minStock: 100, wantCount: 50, wantPackSize: "", wantMinStock: 100},
		{barcode: "2000000000008", unit: "loaf", amount: 1, minStock: 2, wantCount: 1, wantPackSize: "", wantMinStock: 2},
	}
	now := time.Now()
	for _, row := range seed {
		insert := db.Exec(
			"INSERT INTO products (created_at, updated_at, barcode, product_name, household_id, user_id, amount, unit, min_stock_amount, quantity) VALUES (?, ?, ?, ?, 1, 1, ?, ?, ?, 0)",
			now, now, row.barcode, "legacy", row.amount, row.unit, row.minStock)
		if insert.Error != nil {
			t.Fatalf("Failed to seed product %s: %v", row.barcode, insert.Error)
		}
	}

	logger := zerolog.Nop()
	if convertErr := ConvertProductAmountsToQuantities(&logger, db); convertErr != nil {
		t.Fatalf("ConvertProductAmountsToQuantities() error = %v", convertErr)
	}

	for _, row := range seed {
		var count int
		if readErr := db.Raw("SELECT quantity FROM products WHERE barcode = ?", row.barcode).Scan(&count).Error; readErr != nil {
			t.Fatalf("Failed to read the count of %s: %v", row.barcode, readErr)
		}
		if count != row.wantCount {
			t.Errorf("count of %s = %d, want %d", row.barcode, count, row.wantCount)
		}
		var packSize string
		if readErr := db.Raw("SELECT pack_size FROM products WHERE barcode = ?", row.barcode).Scan(&packSize).Error; readErr != nil {
			t.Fatalf("Failed to read the pack size of %s: %v", row.barcode, readErr)
		}
		if packSize != row.wantPackSize {
			t.Errorf("pack size of %s = %q, want %q", row.barcode, packSize, row.wantPackSize)
		}
		var minStock int
		if readErr := db.Raw("SELECT min_stock_amount FROM products WHERE barcode = ?", row.barcode).Scan(&minStock).Error; readErr != nil {
			t.Fatalf("Failed to read the minimum stock of %s: %v", row.barcode, readErr)
		}
		if minStock != row.wantMinStock {
			t.Errorf("minimum stock of %s = %d, want %d", row.barcode, minStock, row.wantMinStock)
		}
	}

	// A second conversion must not change anything: a row that already carries
	// a pack size is left alone.
	if convertErr := ConvertProductAmountsToQuantities(&logger, db); convertErr != nil {
		t.Fatalf("ConvertProductAmountsToQuantities() second run error = %v", convertErr)
	}
	for _, row := range seed {
		var count int
		if readErr := db.Raw("SELECT quantity FROM products WHERE barcode = ?", row.barcode).Scan(&count).Error; readErr != nil {
			t.Fatalf("Failed to read the count of %s: %v", row.barcode, readErr)
		}
		if count != row.wantCount {
			t.Errorf("count of %s after the second run = %d, want %d", row.barcode, count, row.wantCount)
		}
		var packSize string
		if readErr := db.Raw("SELECT pack_size FROM products WHERE barcode = ?", row.barcode).Scan(&packSize).Error; readErr != nil {
			t.Fatalf("Failed to read the pack size of %s: %v", row.barcode, readErr)
		}
		if packSize != row.wantPackSize {
			t.Errorf("pack size of %s after the second run = %q, want %q", row.barcode, packSize, row.wantPackSize)
		}
	}

	// The columns the count model replaced are dropped afterwards, and the
	// drop is idempotent: a database that never had them is left alone.
	if dropErr := DropUnitColumns(&logger, db); dropErr != nil {
		t.Fatalf("DropUnitColumns() error = %v", dropErr)
	}
	for _, column := range []string{"amount", "unit"} {
		if db.Migrator().HasColumn(&database.Product{}, column) {
			t.Errorf("products.%s was not dropped", column)
		}
	}
	if !db.Migrator().HasColumn(&database.Product{}, "pack_size") {
		t.Error("products.pack_size was dropped but carries the pack size")
	}
	if dropErr := DropUnitColumns(&logger, db); dropErr != nil {
		t.Fatalf("DropUnitColumns() second run error = %v", dropErr)
	}
}

// TestRunBreakingDatabaseMigrationsOnALegacyDatabase runs the whole chain on a
// database that still carries the columns the count model replaced, and pins
// the state it leaves behind: the product holds a pack count and a pack size,
// the recipe ingredient holds a portion, and the old columns are gone.
func TestRunBreakingDatabaseMigrationsOnALegacyDatabase(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}
	for _, column := range []string{"amount INTEGER", "unit TEXT"} {
		if execErr := db.Exec("ALTER TABLE products ADD COLUMN " + column).Error; execErr != nil {
			t.Fatalf("Failed to add the legacy product column %s: %v", column, execErr)
		}
		if execErr := db.Exec("ALTER TABLE recipe_ingredients ADD COLUMN " + column).Error; execErr != nil {
			t.Fatalf("Failed to add the legacy ingredient column %s: %v", column, execErr)
		}
	}

	now := time.Now()
	seed := db.Exec(
		"INSERT INTO products (created_at, updated_at, barcode, product_name, household_id, user_id, amount, unit, min_stock_amount, quantity) VALUES (?, ?, '3000000000001', 'Flour', 1, 1, 1000, 'g', 0, 0)",
		now, now)
	if seed.Error != nil {
		t.Fatalf("Failed to seed the legacy product: %v", seed.Error)
	}
	ingredient := db.Exec(
		"INSERT INTO recipe_ingredients (created_at, updated_at, recipe_id, name, amount, unit, portion) VALUES (?, ?, 1, 'Flour', 250, 'g', '')",
		now, now)
	if ingredient.Error != nil {
		t.Fatalf("Failed to seed the legacy ingredient: %v", ingredient.Error)
	}

	logger := zerolog.Nop()
	if migrateErr := RunBreakingDatabaseMigrations(&logger, db); migrateErr != nil {
		t.Fatalf("RunBreakingDatabaseMigrations() error = %v", migrateErr)
	}

	var quantity int
	var packSize string
	if readErr := db.Raw("SELECT quantity, pack_size FROM products WHERE barcode = ?", "3000000000001").Row().Scan(&quantity, &packSize); readErr != nil {
		t.Fatalf("Failed to read the converted product: %v", readErr)
	}
	if quantity != 1 {
		t.Errorf("count = %d, want 1", quantity)
	}
	if packSize != "1000 g" {
		t.Errorf("pack size = %q, want %q", packSize, "1000 g")
	}

	var portion string
	if readErr := db.Raw("SELECT portion FROM recipe_ingredients WHERE name = ?", "Flour").Row().Scan(&portion); readErr != nil {
		t.Fatalf("Failed to read the converted ingredient: %v", readErr)
	}
	if portion != "250 g" {
		t.Errorf("portion = %q, want %q", portion, "250 g")
	}

	for _, column := range []string{"amount", "unit"} {
		if db.Migrator().HasColumn(&database.Product{}, column) {
			t.Errorf("products.%s was not dropped", column)
		}
		if db.Migrator().HasColumn(&database.RecipeIngredient{}, column) {
			t.Errorf("recipe_ingredients.%s was not dropped", column)
		}
	}
}

// TestConvertRecipeIngredientsToPortions pins the conversion of the amount and
// the unit of a recipe ingredient into a single free-text portion.
func TestConvertRecipeIngredientsToPortions(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}
	// The columns the portion replaced are recreated so the conversion has
	// something to read.
	for _, column := range []string{"amount INTEGER", "unit TEXT"} {
		if execErr := db.Exec("ALTER TABLE recipe_ingredients ADD COLUMN " + column).Error; execErr != nil {
			t.Fatalf("Failed to add the legacy column %s: %v", column, execErr)
		}
	}

	seed := []struct {
		name        string
		amount      int
		unit        string
		wantPortion string
	}{
		{name: "Flour", amount: 250, unit: "g", wantPortion: "250 g"},
		{name: "Eggs", amount: 2, unit: "pcs", wantPortion: "2 pcs"},
		{name: "Salt", amount: 1, unit: "", wantPortion: "1"},
		{name: "Pepper", amount: 0, unit: "pinch", wantPortion: "pinch"},
	}
	now := time.Now()
	for i, row := range seed {
		insert := db.Exec(
			"INSERT INTO recipe_ingredients (created_at, updated_at, recipe_id, name, amount, unit, portion) VALUES (?, ?, 1, ?, ?, ?, '')",
			now, now, row.name, row.amount, row.unit)
		if insert.Error != nil {
			t.Fatalf("Failed to seed ingredient %s: %v", row.name, insert.Error)
		}
		_ = i
	}

	logger := zerolog.Nop()
	if convertErr := ConvertRecipeIngredientsToPortions(&logger, db); convertErr != nil {
		t.Fatalf("ConvertRecipeIngredientsToPortions() error = %v", convertErr)
	}

	for _, row := range seed {
		var portion string
		if readErr := db.Raw("SELECT portion FROM recipe_ingredients WHERE name = ?", row.name).Scan(&portion).Error; readErr != nil {
			t.Fatalf("Failed to read the portion of %s: %v", row.name, readErr)
		}
		if portion != row.wantPortion {
			t.Errorf("portion of %s = %q, want %q", row.name, portion, row.wantPortion)
		}
	}

	// A second conversion must not change anything: a row that already carries
	// a portion is left alone.
	if convertErr := ConvertRecipeIngredientsToPortions(&logger, db); convertErr != nil {
		t.Fatalf("ConvertRecipeIngredientsToPortions() second run error = %v", convertErr)
	}
	for _, row := range seed {
		var portion string
		if readErr := db.Raw("SELECT portion FROM recipe_ingredients WHERE name = ?", row.name).Scan(&portion).Error; readErr != nil {
			t.Fatalf("Failed to read the portion of %s: %v", row.name, readErr)
		}
		if portion != row.wantPortion {
			t.Errorf("portion of %s after the second run = %q, want %q", row.name, portion, row.wantPortion)
		}
	}

	// The columns the portion replaced are dropped afterwards, and the drop is
	// idempotent.
	if dropErr := DropUnitColumns(&logger, db); dropErr != nil {
		t.Fatalf("DropUnitColumns() error = %v", dropErr)
	}
	for _, column := range []string{"amount", "unit"} {
		if db.Migrator().HasColumn(&database.RecipeIngredient{}, column) {
			t.Errorf("recipe_ingredients.%s was not dropped", column)
		}
	}
	if !db.Migrator().HasColumn(&database.RecipeIngredient{}, "portion") {
		t.Error("recipe_ingredients.portion was dropped but carries the portion")
	}
	if dropErr := DropUnitColumns(&logger, db); dropErr != nil {
		t.Fatalf("DropUnitColumns() second run error = %v", dropErr)
	}
}

// A row that predates the amount column holds NULL there. The migration that
// preceded the count model gave such a row the default of one, so the
// conversion has to read it as one too: a measured row keeps its unit as the
// size of its pack instead of losing it.
func TestConvertProductAmountsToQuantitiesReadsANullAmountAsOne(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	if err := testutil.MigrateAllModels(db); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}
	for _, column := range []string{"amount INTEGER", "unit TEXT"} {
		if execErr := db.Exec("ALTER TABLE products ADD COLUMN " + column).Error; execErr != nil {
			t.Fatalf("Failed to add the legacy column %s: %v", column, execErr)
		}
	}

	now := time.Now()
	insert := db.Exec(
		"INSERT INTO products (created_at, updated_at, barcode, product_name, household_id, user_id, amount, unit, min_stock_amount, quantity) VALUES (?, ?, ?, ?, 1, 1, NULL, ?, 0, 0)",
		now, now, "2000000000009", "legacy", "g")
	if insert.Error != nil {
		t.Fatalf("Failed to seed product: %v", insert.Error)
	}

	logger := zerolog.Nop()
	if convertErr := ConvertProductAmountsToQuantities(&logger, db); convertErr != nil {
		t.Fatalf("ConvertProductAmountsToQuantities() error = %v", convertErr)
	}

	var count int
	if readErr := db.Raw("SELECT quantity FROM products WHERE barcode = ?", "2000000000009").Scan(&count).Error; readErr != nil {
		t.Fatalf("Failed to read the count: %v", readErr)
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
	var packSize string
	if readErr := db.Raw("SELECT pack_size FROM products WHERE barcode = ?", "2000000000009").Scan(&packSize).Error; readErr != nil {
		t.Fatalf("Failed to read the pack size: %v", readErr)
	}
	if packSize != "1 g" {
		t.Errorf("pack size = %q, want 1 g (the unit is kept)", packSize)
	}
}
