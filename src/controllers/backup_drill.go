package controllers

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/migrations"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// BackupDrillEntity names one thing the drill compares between a backup
// document and the database it was restored into.
type BackupDrillEntity string

const (
	// BackupDrillProducts are the household's active products.
	BackupDrillProducts BackupDrillEntity = "products"
	// BackupDrillArchivedProducts are the household's archived products.
	BackupDrillArchivedProducts BackupDrillEntity = "archivedProducts"
	// BackupDrillStorageLocations are the household's storage locations.
	BackupDrillStorageLocations BackupDrillEntity = "storageLocations"
	// BackupDrillRecipes are the household's saved recipes.
	BackupDrillRecipes BackupDrillEntity = "recipes"
	// BackupDrillMealPlanEntries are the household's meal plan entries.
	BackupDrillMealPlanEntries BackupDrillEntity = "mealPlanEntries"
	// BackupDrillShoppingListItems are the household's shopping list items.
	BackupDrillShoppingListItems BackupDrillEntity = "shoppingListItems"
)

// backupDrillEntities is the order the drill reports its counts in. The
// entities the document carries come first, the ones it does not carry last.
var backupDrillEntities = []BackupDrillEntity{
	BackupDrillProducts,
	BackupDrillArchivedProducts,
	BackupDrillStorageLocations,
	BackupDrillRecipes,
	BackupDrillMealPlanEntries,
	BackupDrillShoppingListItems,
}

// The exit codes the drill command returns. A broken backup and a moved
// schema are different problems, and a scheduled job has to be able to tell
// them apart: the first needs the backup fixed, the second needs the
// migration that reads the new document written.
const (
	// BackupDrillExitOK means the newest backup restored and the counts match.
	BackupDrillExitOK = 0
	// BackupDrillExitFailed means the backup is broken: it could not be read,
	// it could not be restored, or the restored rows do not match it.
	BackupDrillExitFailed = 1
	// BackupDrillExitSchemaMismatch means the document carries a schema
	// version this release does not write.
	BackupDrillExitSchemaMismatch = 2
	// BackupDrillExitUnavailable means there is nothing to drill: no backup
	// document, or a backup directory that cannot be used.
	BackupDrillExitUnavailable = 3
)

// BackupDrillCount is one row of the drill's comparison table.
type BackupDrillCount struct {
	Entity BackupDrillEntity
	// Expected is the count the document says the entity has.
	Expected int
	// Restored is the count the scratch database holds after the restore.
	Restored int
	// Covered is false when the document does not carry the entity at all.
	// The drill still reports what the scratch database holds, but there is
	// nothing to compare it against.
	Covered bool
}

// BackupDrillResult is what a successful drill found.
type BackupDrillResult struct {
	// BackupPath is the document that was drilled.
	BackupPath string
	// BackupAge is how long ago the document was written.
	BackupAge time.Duration
	// SchemaVersion is the schema version the document carries.
	SchemaVersion int
	// Counts is the comparison table.
	Counts []BackupDrillCount
	// Mismatches names the fields that came back different from the
	// document. The counts say how many rows came back; these say whether
	// they are the right ones.
	Mismatches []BackupDrillMismatch
}

// BackupDrill restores a scheduled backup into a throwaway database and
// compares the restored rows with the rows the document says it carries.
//
// The drill never opens the live database. It restores into a scratch file of
// its own and deletes that file afterwards, on success and on failure: a
// drill that wrote into the production database would be worse than no drill.
//
// The comparison is against the document, not against the live household. A
// backup is as old as the backup interval, and the household buys products in
// between, so live counts would disagree with a perfectly good backup. The
// document is the recorded state of the household at the moment it was
// written, and a restore that reproduces it is a restore that works.
type BackupDrill struct {
	Logger *zerolog.Logger
	// BackupDirectory holds the backup documents. It is the configured
	// backup.directory.
	BackupDirectory string
	// BackupFile drills one specific document instead of the newest one. It
	// is empty for the everyday drill.
	BackupFile string
	// ScratchRoot is the directory the scratch database is created in. It is
	// empty for the operating system's temporary directory.
	ScratchRoot string
	// LiveDatabasePath is the path of the live SQLite database. The drill
	// asserts that its scratch database is a different file, so a
	// misconfigured drill can never touch the production data.
	LiveDatabasePath string
}

// Run drills the backup and returns what it found. The scratch database is
// removed before it returns, whatever the outcome.
func (d *BackupDrill) Run(ctx context.Context) (BackupDrillResult, error) {
	if err := ctx.Err(); err != nil {
		return BackupDrillResult{}, err
	}

	path, err := d.documentToDrill()
	if err != nil {
		return BackupDrillResult{}, err
	}

	document, err := readBackupDocument(path)
	if err != nil {
		return BackupDrillResult{}, err
	}
	if document.Version == 0 {
		return BackupDrillResult{}, fmt.Errorf("%w: the document names no schema version",
			errors.ErrBackupInvalidDocument)
	}
	if document.Version != apiModel.BackupSchemaVersion {
		return BackupDrillResult{}, fmt.Errorf("%w: the document is version %d, this release writes version %d",
			errors.ErrBackupUnsupportedVersion, document.Version, apiModel.BackupSchemaVersion)
	}
	if err := checkDocumentStats(&document); err != nil {
		return BackupDrillResult{}, err
	}

	scratchDB, closeScratch, err := d.openScratchDatabase()
	if err != nil {
		return BackupDrillResult{}, err
	}
	defer closeScratch()

	scoped := scratchDB.WithContext(ctx)

	household, err := seedScratchHousehold(scoped, &document)
	if err != nil {
		return BackupDrillResult{}, fmt.Errorf("%w: %s", errors.ErrBackupRestoreFailed, err)
	}

	if err := ctx.Err(); err != nil {
		return BackupDrillResult{}, err
	}

	if _, err := RestoreHouseholdBackup(scoped, household.ID, household.AdminUserID, &document); err != nil {
		return BackupDrillResult{}, fmt.Errorf("%w: %w", errors.ErrBackupRestoreFailed, err)
	}

	restored, err := countScratchRows(scoped, household.ID)
	if err != nil {
		return BackupDrillResult{}, fmt.Errorf("%w: %w", errors.ErrBackupRestoreFailed, err)
	}

	rowMismatches, err := compareProductRows(&document, scoped, household)
	if err != nil {
		return BackupDrillResult{}, fmt.Errorf("%w: %w", errors.ErrBackupRestoreFailed, err)
	}
	locationMismatches, err := compareStorageLocations(&document, scoped, household.ID)
	if err != nil {
		return BackupDrillResult{}, fmt.Errorf("%w: %w", errors.ErrBackupRestoreFailed, err)
	}
	householdMismatches, err := compareHouseholdMetadata(&document, scoped, household.ID)
	if err != nil {
		return BackupDrillResult{}, fmt.Errorf("%w: %w", errors.ErrBackupRestoreFailed, err)
	}

	result := BackupDrillResult{
		BackupPath:    path,
		SchemaVersion: document.Version,
		Counts:        compareBackupCounts(&document, restored),
		Mismatches:    append(rowMismatches, locationMismatches...),
	}
	result.Mismatches = append(result.Mismatches, householdMismatches...)
	result.BackupAge = backupFileAge(path)

	mismatches := backupDrillMismatches(result.Counts)
	if len(mismatches) > 0 {
		details := make([]string, 0, len(mismatches))
		for _, mismatch := range mismatches {
			details = append(details, fmt.Sprintf("%s: the document has %d, the restore has %d",
				mismatch.Entity, mismatch.Expected, mismatch.Restored))
		}
		return result, fmt.Errorf("%w: %s", errors.ErrBackupDrillMismatch, strings.Join(details, "; "))
	}
	if len(result.Mismatches) > 0 {
		details := make([]string, 0, len(result.Mismatches))
		for _, mismatch := range result.Mismatches {
			details = append(details, fmt.Sprintf("%s %s: %s is %q in the document and %q in the restore",
				mismatch.Entity, mismatch.Key, mismatch.Field, mismatch.Document, mismatch.Restored))
		}
		return result, fmt.Errorf("%w: %s", errors.ErrBackupDrillMismatch, strings.Join(details, "; "))
	}

	if d.Logger != nil {
		d.Logger.Info().Msgf("Backup drill: %s restored %d products, %d archived products and %d storage locations",
			path, len(document.Products.Active), len(document.Products.Archived), len(backupLocationDrafts(&document)))
	}
	return result, nil
}

// documentToDrill returns the document the drill works on: the one the caller
// named, or the newest one in the backup directory.
func (d *BackupDrill) documentToDrill() (string, error) {
	if named := strings.TrimSpace(d.BackupFile); named != "" {
		if _, err := os.Stat(named); err != nil {
			return "", fmt.Errorf("%w: %s", errors.ErrBackupNotFound, err)
		}
		return named, nil
	}

	directory := strings.TrimSpace(d.BackupDirectory)
	if directory == "" {
		return "", fmt.Errorf("%w: no backup directory configured", errors.ErrBackupInvalidDirectory)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: the backup directory %s does not exist", errors.ErrBackupNotFound, directory)
		}
		return "", fmt.Errorf("%w: %s", errors.ErrBackupNotFound, err)
	}

	names := make([]string, 0, len(entries))
	for i := range entries {
		if entries[i].IsDir() {
			continue
		}
		name := entries[i].Name()
		if !strings.HasPrefix(name, util.BackupFilePrefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "", fmt.Errorf("%w: %s holds no backup document", errors.ErrBackupNotFound, directory)
	}
	// The timestamp in the filename is UTC and sorts lexicographically, so a
	// reverse sort leaves the newest document at the front.
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return filepath.Join(directory, names[0]), nil
}

// readBackupDocument reads a backup document from disk. A file that cannot
// be read or decoded is a broken backup, not a missing one.
func readBackupDocument(path string) (apiModel.FullExportResponse, error) {
	var document apiModel.FullExportResponse
	payload, err := os.ReadFile(path) // #nosec G304 - the path names a backup document the operator configured
	if err != nil {
		return document, fmt.Errorf("%w: %s", errors.ErrBackupInvalidDocument, err)
	}
	if err := json.Unmarshal(payload, &document); err != nil {
		return document, fmt.Errorf("%w: %s", errors.ErrBackupInvalidDocument, err)
	}
	return document, nil
}

// checkDocumentStats compares the document's own statistics with its rows. The
// statistics are derived from the rows, so a document whose two halves
// disagree was written by something else — or was cut short.
func checkDocumentStats(document *apiModel.FullExportResponse) error {
	if document.Stats.TotalActive != len(document.Products.Active) {
		return fmt.Errorf("%w: the document reports %d active products but carries %d",
			errors.ErrBackupDrillMismatch, document.Stats.TotalActive, len(document.Products.Active))
	}
	if document.Stats.TotalArchived != len(document.Products.Archived) {
		return fmt.Errorf("%w: the document reports %d archived products but carries %d",
			errors.ErrBackupDrillMismatch, document.Stats.TotalArchived, len(document.Products.Archived))
	}
	return nil
}

// openScratchDatabase creates the throwaway database the restore runs in. The
// returned function closes the database and removes it, so the scratch data
// never outlives the drill.
func (d *BackupDrill) openScratchDatabase() (*gorm.DB, func(), error) {
	root := strings.TrimSpace(d.ScratchRoot)
	if root == "" {
		root = os.TempDir()
	}
	directory, err := os.MkdirTemp(root, util.BackupDrillScratchPrefix)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %s", errors.ErrBackupInvalidDirectory, err)
	}
	cleanup := func() { _ = os.RemoveAll(directory) }

	path := filepath.Join(directory, util.BackupDrillScratchFilename)
	if err := d.assertScratchIsNotLive(path); err != nil {
		cleanup()
		return nil, nil, err
	}

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("%w: %s", errors.ErrBackupRestoreFailed, err)
	}
	closeDB := func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		cleanup()
	}

	if err := migrations.AutoMigrateAll(db); err != nil {
		closeDB()
		return nil, nil, fmt.Errorf("%w: %s", errors.ErrBackupRestoreFailed, err)
	}
	// The canonical categories are reference data every instance holds, and
	// the restore resolves a product's category against them. The scratch
	// database needs them too, or the drill would restore into a household
	// that production never has. The seed logs through a silent logger: the
	// operator reads the drill's findings, not the scratch setup.
	silent := zerolog.Nop()
	if err := migrations.SeedProductCategoryPrices(&silent, db); err != nil {
		closeDB()
		return nil, nil, fmt.Errorf("%w: %s", errors.ErrBackupRestoreFailed, err)
	}
	return db, closeDB, nil
}

// assertScratchIsNotLive refuses to restore into the live database. The drill
// opens exactly one database handle — the scratch one — so this can only fire
// when the scratch root and the live database resolve to the same file. The
// live path is resolved through its symlinks, so a symlinked live database
// cannot smuggle it past the comparison.
func (d *BackupDrill) assertScratchIsNotLive(scratchPath string) error {
	live := strings.TrimSpace(d.LiveDatabasePath)
	if live == "" {
		return nil
	}
	scratchAbs, scratchErr := filepath.Abs(scratchPath)
	liveAbs, liveErr := filepath.Abs(live)
	if scratchErr != nil || liveErr != nil {
		return nil
	}
	if resolved, resolveErr := filepath.EvalSymlinks(liveAbs); resolveErr == nil {
		liveAbs = resolved
	}
	if scratchAbs == liveAbs {
		return fmt.Errorf("%w: the scratch database is the live database", errors.ErrBackupInvalidDirectory)
	}
	return nil
}

// scratchHousehold is the household the drill created in the scratch
// database.
type scratchHousehold struct {
	// ID is the id of the household in the scratch database.
	ID uint
	// AdminUserID is the account the restore falls back to for a product the
	// document does not name an owner for.
	AdminUserID uint
	// Usernames maps the id of a seeded account to its name, so the drill can
	// compare the owner of a restored product by name.
	Usernames map[uint]string
}

// seedScratchHousehold creates the household the document belongs to in the
// scratch database, with one account per member. The restore maps product
// ownership onto members by name and falls back to the restoring admin, so
// the members have to exist before it runs.
func seedScratchHousehold(
	db *gorm.DB,
	document *apiModel.FullExportResponse,
) (scratchHousehold, error) {
	// The scratch household carries the drill's own name, never the
	// document's: the restore has to write the document's name, or the
	// comparison would only ever find the name the seed put there.
	household := dbModel.Household{Name: util.BackupDrillHouseholdName}
	if err := db.Create(&household).Error; err != nil {
		return scratchHousehold{}, err
	}

	seeded := scratchHousehold{ID: household.ID, Usernames: map[uint]string{}}
	for i := range document.Members {
		member := &document.Members[i]
		user := authentication.User{
			Username:    member.Username,
			MailAddress: member.MailAddress,
			HouseholdID: household.ID,
			Role:        member.Role,
		}
		if err := db.Create(&user).Error; err != nil {
			return scratchHousehold{}, err
		}
		seeded.Usernames[user.ID] = user.Username
		if seeded.AdminUserID == 0 && (member.Role == authentication.RoleAdmin || i == 0) {
			seeded.AdminUserID = user.ID
		}
	}
	if seeded.AdminUserID == 0 {
		// A document without members still restores: every product falls back
		// to the restoring admin.
		admin := authentication.User{
			Username:    util.BackupDrillAdminName,
			MailAddress: util.BackupDrillAdminMail,
			HouseholdID: household.ID,
			Role:        authentication.RoleAdmin,
		}
		if err := db.Create(&admin).Error; err != nil {
			return scratchHousehold{}, err
		}
		seeded.AdminUserID = admin.ID
		seeded.Usernames[admin.ID] = admin.Username
	}
	return seeded, nil
}

// countScratchRows counts what the restored household holds.
func countScratchRows(db *gorm.DB, householdID uint) (map[BackupDrillEntity]int, error) {
	counts := map[BackupDrillEntity]int{}

	var active int64
	if err := db.Model(&dbModel.Product{}).
		Where("household_id = ? AND deleted_at IS NULL", householdID).
		Count(&active).Error; err != nil {
		return nil, err
	}
	counts[BackupDrillProducts] = int(active)

	var archived int64
	if err := db.Unscoped().Model(&dbModel.Product{}).
		Where("household_id = ? AND deleted_at IS NOT NULL", householdID).
		Count(&archived).Error; err != nil {
		return nil, err
	}
	counts[BackupDrillArchivedProducts] = int(archived)

	var locations int64
	if err := db.Unscoped().Model(&dbModel.StorageLocation{}).
		Where("household_id = ?", householdID).
		Count(&locations).Error; err != nil {
		return nil, err
	}
	counts[BackupDrillStorageLocations] = int(locations)

	var recipes int64
	if err := db.Unscoped().Model(&dbModel.Recipe{}).
		Where("household_id = ?", householdID).
		Count(&recipes).Error; err != nil {
		return nil, err
	}
	counts[BackupDrillRecipes] = int(recipes)

	var entries int64
	if err := db.Unscoped().Model(&dbModel.MealPlanEntry{}).
		Where("household_id = ?", householdID).
		Count(&entries).Error; err != nil {
		return nil, err
	}
	counts[BackupDrillMealPlanEntries] = int(entries)

	var items int64
	if err := db.Unscoped().Model(&dbModel.ShoppingListItem{}).
		Where("household_id = ?", householdID).
		Count(&items).Error; err != nil {
		return nil, err
	}
	counts[BackupDrillShoppingListItems] = int(items)

	return counts, nil
}

// compareBackupCounts builds the comparison table and reports the entities
// whose counts differ. The entities the document does not carry are listed
// without an expectation: the backup schema does not hold them, so there is
// nothing to compare them against.
func compareBackupCounts(
	document *apiModel.FullExportResponse,
	restored map[BackupDrillEntity]int,
) []BackupDrillCount {
	expected := map[BackupDrillEntity]int{
		BackupDrillProducts:         len(document.Products.Active),
		BackupDrillArchivedProducts: len(document.Products.Archived),
		BackupDrillStorageLocations: len(backupLocationDrafts(document)),
	}

	counts := make([]BackupDrillCount, 0, len(backupDrillEntities))
	for _, entity := range backupDrillEntities {
		expectedCount, covered := expected[entity]
		counts = append(counts, BackupDrillCount{
			Entity:   entity,
			Expected: expectedCount,
			Restored: restored[entity],
			Covered:  covered,
		})
	}
	return counts
}

// backupDrillMismatches returns the entities whose restored count differs
// from the count the document claims.
func backupDrillMismatches(counts []BackupDrillCount) []BackupDrillCount {
	mismatches := make([]BackupDrillCount, 0)
	for _, count := range counts {
		if count.Covered && count.Expected != count.Restored {
			mismatches = append(mismatches, count)
		}
	}
	return mismatches
}

// BackupDrillExitCode maps a drill failure to the exit code of the drill
// command.
func BackupDrillExitCode(err error) int {
	if err == nil {
		return BackupDrillExitOK
	}
	switch {
	case stderrors.Is(err, errors.ErrBackupUnsupportedVersion):
		return BackupDrillExitSchemaMismatch
	case stderrors.Is(err, errors.ErrBackupNotFound), stderrors.Is(err, errors.ErrBackupInvalidDirectory):
		return BackupDrillExitUnavailable
	default:
		return BackupDrillExitFailed
	}
}

// backupFileAge returns how long ago the document was written.
func backupFileAge(path string) time.Duration {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	age := time.Since(info.ModTime())
	if age < 0 {
		return 0
	}
	return age
}
