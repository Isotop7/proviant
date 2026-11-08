package controllers

import (
	"cmp"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
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

// backupDrillCarriedEntities are the entities the backup document carries.
// The comparison has an expectation for them; the others are reported
// without one.
var backupDrillCarriedEntities = map[BackupDrillEntity]bool{
	BackupDrillProducts:         true,
	BackupDrillArchivedProducts: true,
	BackupDrillStorageLocations: true,
}

// BackupDrillUncoveredEntities returns the entities the backup document does
// not carry. The drill reports them whether or not a document could be read:
// the gap is a property of the backup schema, not of one household.
func BackupDrillUncoveredEntities() []BackupDrillEntity {
	uncovered := make([]BackupDrillEntity, 0, len(backupDrillEntities))
	for _, entity := range backupDrillEntities {
		if !backupDrillCarriedEntities[entity] {
			uncovered = append(uncovered, entity)
		}
	}
	return uncovered
}

// The exit codes the drill command returns. A broken backup and a moved
// schema are different problems, and a scheduled job has to be able to tell
// them apart: the first needs the backup fixed, the second needs the
// migration that reads the new document written.
const (
	// BackupDrillExitOK means every newest backup restored and the counts
	// match.
	BackupDrillExitOK = 0
	// BackupDrillExitFailed means a backup is broken: it could not be read,
	// it could not be restored, or the restored rows do not match it.
	BackupDrillExitFailed = 1
	// BackupDrillExitSchemaMismatch means the document carries a schema
	// version this release does not write.
	BackupDrillExitSchemaMismatch = 2
	// BackupDrillExitUnavailable means there is nothing to drill: no backup
	// document, or a backup directory that cannot be used.
	BackupDrillExitUnavailable = 3
	// BackupDrillExitStale means the backup restores but is older than the
	// bound the drill applies: the job that writes the backups stopped.
	BackupDrillExitStale = 4
	// BackupDrillExitTimeout means the drill ran out of time before it
	// could restore a household. The backup was not shown to be broken.
	BackupDrillExitTimeout = 5
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

// BackupDrillResult is what the drill found for one document.
type BackupDrillResult struct {
	// HouseholdID is the household the document's file name carries. It is
	// zero for a document whose name names no household.
	HouseholdID uint
	// HouseholdName is the name of the household the document carries. It
	// is the name the household knows itself by; the id in the file name is
	// the bookkeeping of the instance that wrote it.
	HouseholdName string
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
	// Stale reports that the document is older than the bound the drill
	// applies. The document may still restore: what stopped is the job
	// that writes the backups.
	Stale bool
	// Err is the failure the drill found in this document. It is nil when
	// the document restored and its rows match.
	Err error
}

// BackupDrillRun is what one drill over the backup directory found.
type BackupDrillRun struct {
	// Results holds one entry per drilled document, one per household.
	Results []BackupDrillResult
	// Err is the failure of the run itself: the drill ran out of time before
	// it could drill every household. It belongs to no household.
	Err error
}

// BackupDrill restores the newest backup of every household into a throwaway
// database and compares the restored rows with the rows the document says it
// carries.
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
	// MaxAge is the age a backup may reach before the drill calls it stale.
	// Zero disables the check.
	MaxAge time.Duration
	// DocumentTimeout bounds the restore of one household. It is per
	// document, not per run: an instance with many households needs the
	// time, and a restore that hangs is what the bound has to catch. Zero
	// means the default bound.
	DocumentTimeout time.Duration
}

// Run drills the newest backup of every household and returns what it found.
// The scratch database is removed before it returns, whatever the outcome.
//
// One broken household does not stop the drill: every household is restored
// and compared, and the failures are reported together. A household that
// cannot be restored must not hide the ones that can.
func (d *BackupDrill) Run(ctx context.Context) (BackupDrillRun, error) {
	run := BackupDrillRun{}
	if err := ctx.Err(); err != nil {
		run.Err = fmt.Errorf("%w: %w", errors.ErrBackupDrillTimeout, err)
		return run, run.Err
	}

	documents, err := d.documentsToDrill()
	if err != nil {
		run.Err = err
		return run, err
	}

	runCtx, cancelRun := d.overallBound(ctx, len(documents))
	defer cancelRun()

	scratchDB, closeScratch, err := d.openScratchDatabase(runCtx)
	if err != nil {
		run.Err = err
		return run, err
	}
	defer closeScratch()

	failures := make([]error, 0, len(documents))
	for i := range documents {
		if err := runCtx.Err(); err != nil {
			run.Err = fmt.Errorf("%w: %w", errors.ErrBackupDrillTimeout, err)
			failures = append(failures, run.Err)
			return run, stderrors.Join(failures...)
		}
		documentCtx, cancel := d.bound(runCtx)
		result := d.drillDocument(documentCtx, scratchDB, documents[i])
		cancel()
		run.Results = append(run.Results, result)
		if result.Err != nil {
			failures = append(failures, result.Err)
		}
	}
	return run, stderrors.Join(failures...)
}

// effectiveTimeout is the bound one document's restore gets.
func (d *BackupDrill) effectiveTimeout() time.Duration {
	if d.DocumentTimeout > 0 {
		return d.DocumentTimeout
	}
	return time.Duration(util.DefaultBackupDrillTimeoutMinutes) * time.Minute
}

// bound limits one document's restore. The bound is per document, not per
// run: an instance with many households needs the time, and a restore that
// hangs is what the bound has to catch.
func (d *BackupDrill) bound(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d.effectiveTimeout())
}

// overallBound limits the whole run. The per-document bound catches a restore
// that hangs; the overall bound adds one document's worth for the setup and
// the gaps between the documents, so a drill of many households cannot
// outgrow the sum of its per-document bounds.
func (d *BackupDrill) overallBound(ctx context.Context, documents int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d.effectiveTimeout()*time.Duration(documents+1))
}

// drillDocument restores one document into the scratch database and compares
// the restored rows with the rows the document says it carries.
func (d *BackupDrill) drillDocument(ctx context.Context, db *gorm.DB, document *backupDocument) BackupDrillResult {
	result := d.compareDocument(ctx, db.WithContext(ctx), document)
	if !result.Stale {
		return result
	}
	stale := fmt.Errorf("%w: %s is %s old, the drill allows %s",
		errors.ErrBackupDrillStale, result.BackupPath, result.BackupAge.Round(time.Second), d.MaxAge)
	if result.Err == nil {
		result.Err = stale
		return result
	}
	result.Err = fmt.Errorf("%w: %w", result.Err, stale)
	return result
}

// drillFailure names the error the drill found. A restore that ran out of
// time is not a broken backup: the operator has to know which of the two it
// was.
func drillFailure(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%w: %w", errors.ErrBackupDrillTimeout, ctx.Err())
	}
	return err
}

// compareDocument restores one document and compares the restored rows with
// the rows the document says it carries.
func (d *BackupDrill) compareDocument(ctx context.Context, db *gorm.DB, document *backupDocument) BackupDrillResult {
	result := BackupDrillResult{
		HouseholdID: document.HouseholdID,
		BackupPath:  document.Path,
		BackupAge:   backupAge(document.WrittenAt),
	}
	if document.Err != nil {
		result.Err = document.Err
		return result
	}
	if document.FromDirectory && d.MaxAge > 0 && result.BackupAge > d.MaxAge {
		result.Stale = true
	}

	backup, err := readBackupDocument(document.Path)
	if err != nil {
		result.Err = drillFailure(ctx, err)
		return result
	}
	if backup.Household != nil {
		result.HouseholdName = backup.Household.Name
	}
	if backup.Version == 0 {
		result.Err = fmt.Errorf("%w: the document names no schema version",
			errors.ErrBackupInvalidDocument)
		return result
	}
	if backup.Version != apiModel.BackupSchemaVersion {
		result.Err = fmt.Errorf("%w: the document is version %d, this release writes version %d",
			errors.ErrBackupUnsupportedVersion, backup.Version, apiModel.BackupSchemaVersion)
		return result
	}
	if err := checkDocumentStats(&backup); err != nil {
		result.Err = err
		return result
	}

	household, err := seedScratchHousehold(db, &backup)
	if err != nil {
		result.Err = drillFailure(ctx, fmt.Errorf("%w: %s", errors.ErrBackupRestoreFailed, err))
		return result
	}

	if _, err := RestoreHouseholdBackup(db, household.ID, household.AdminUserID, &backup); err != nil {
		result.Err = drillFailure(ctx, fmt.Errorf("%w: %w", errors.ErrBackupRestoreFailed, err))
		return result
	}

	restored, err := countScratchRows(db, household.ID)
	if err != nil {
		result.Err = drillFailure(ctx, fmt.Errorf("%w: %w", errors.ErrBackupRestoreFailed, err))
		return result
	}

	rowMismatches, err := compareProductRows(&backup, db, household)
	if err != nil {
		result.Err = drillFailure(ctx, fmt.Errorf("%w: %w", errors.ErrBackupRestoreFailed, err))
		return result
	}
	locationMismatches, err := compareStorageLocations(&backup, db, household.ID)
	if err != nil {
		result.Err = drillFailure(ctx, fmt.Errorf("%w: %w", errors.ErrBackupRestoreFailed, err))
		return result
	}
	householdMismatches, err := compareHouseholdMetadata(&backup, db, household.ID)
	if err != nil {
		result.Err = drillFailure(ctx, fmt.Errorf("%w: %w", errors.ErrBackupRestoreFailed, err))
		return result
	}

	result.SchemaVersion = backup.Version
	result.Counts = compareBackupCounts(&backup, restored)
	result.Mismatches = slices.Concat(rowMismatches, locationMismatches, householdMismatches)

	mismatches := backupDrillMismatches(result.Counts)
	if len(mismatches) > 0 {
		details := make([]string, 0, len(mismatches))
		for _, mismatch := range mismatches {
			details = append(details, fmt.Sprintf("%s: the document has %d, the restore has %d",
				mismatch.Entity, mismatch.Expected, mismatch.Restored))
		}
		result.Err = fmt.Errorf("%w: %s", errors.ErrBackupDrillMismatch, strings.Join(details, "; "))
		return result
	}
	if len(result.Mismatches) > 0 {
		details := make([]string, 0, len(result.Mismatches))
		for _, mismatch := range result.Mismatches {
			details = append(details, fmt.Sprintf("%s %s: %s is %q in the document and %q in the restore",
				mismatch.Entity, mismatch.Key, mismatch.Field, mismatch.Document, mismatch.Restored))
		}
		result.Err = fmt.Errorf("%w: %s", errors.ErrBackupDrillMismatch, strings.Join(details, "; "))
		return result
	}

	if d.Logger != nil {
		d.Logger.Info().Msgf("Backup drill: %s restored %d products, %d archived products and %d storage locations",
			document.Path, len(backup.Products.Active), len(backup.Products.Archived), len(backupLocationDrafts(&backup)))
	}
	return result
}

// backupDocument is one backup document the drill works on.
type backupDocument struct {
	// Path is the file the document lives in.
	Path string
	// HouseholdID is the household the document belongs to. It is zero for a
	// document whose name carries no household.
	HouseholdID uint
	// HasHousehold reports whether the name carried a household. It keeps a
	// household whose id is zero apart from a document that names none.
	HasHousehold bool
	// WrittenAt is the moment the document was written. It is the
	// modification time of a document whose name carries no timestamp.
	WrittenAt time.Time
	// FromDirectory reports whether the document was picked from the backup
	// directory. A document the operator named by hand is not the backup
	// job's newest output, so the staleness bound does not apply to it.
	FromDirectory bool
	// Err is the failure that made the document unreadable. The document is
	// still drilled — and reported as broken — so one unreadable file cannot
	// hide the households behind it.
	Err error
}

// documentsToDrill returns the document the drill works on for every
// household in the backup directory: the newest one of each. A file whose
// name carries no household stands on its own.
func (d *BackupDrill) documentsToDrill() ([]*backupDocument, error) {
	if named := strings.TrimSpace(d.BackupFile); named != "" {
		document, err := describeBackupDocument(named)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", errors.ErrBackupNotFound, err)
		}
		return []*backupDocument{&document}, nil
	}

	directory := strings.TrimSpace(d.BackupDirectory)
	if directory == "" {
		return nil, fmt.Errorf("%w: no backup directory configured", errors.ErrBackupInvalidDirectory)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: the backup directory %s does not exist", errors.ErrBackupNotFound, directory)
		}
		return nil, fmt.Errorf("%w: %s", errors.ErrBackupNotFound, err)
	}

	newest := map[string]*backupDocument{}
	for i := range entries {
		if entries[i].IsDir() {
			continue
		}
		name := entries[i].Name()
		if !strings.HasPrefix(name, util.BackupFilePrefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		document, err := describeBackupDocument(filepath.Join(directory, name))
		if err != nil {
			document.Err = fmt.Errorf("%w: %s: %s", errors.ErrBackupInvalidDocument, name, err)
		}
		document.FromDirectory = true
		key := document.groupKey()
		current, exists := newest[key]
		if !exists || document.WrittenAt.After(current.WrittenAt) {
			newest[key] = &document
		}
	}
	if len(newest) == 0 {
		return nil, fmt.Errorf("%w: %s holds no backup document", errors.ErrBackupNotFound, directory)
	}

	documents := make([]*backupDocument, 0, len(newest))
	for _, document := range newest {
		documents = append(documents, document)
	}
	slices.SortFunc(documents, compareBackupDocuments)
	return documents, nil
}

// describeBackupDocument reads the household and the write time out of a
// document's name. A name that carries no timestamp falls back to the file's
// modification time, so a document the operator placed by hand still has an
// age.
//
// The name is read before the file: a file the drill cannot stat is still
// the household's document, and the report has to name it.
func describeBackupDocument(path string) (backupDocument, error) {
	document := backupDocument{Path: path}
	householdID, writtenAt, ok := util.ParseBackupDocumentName(filepath.Base(path))
	if ok {
		document.HouseholdID = householdID
		document.HasHousehold = true
		document.WrittenAt = writtenAt
	}
	info, err := os.Stat(path)
	if err != nil {
		return document, err
	}
	if !document.HasHousehold {
		document.WrittenAt = info.ModTime()
	}
	return document, nil
}

// groupKey names the household a document belongs to. A document whose name
// carries no household stands on its own.
func (d *backupDocument) groupKey() string {
	if !d.HasHousehold {
		return "file:" + d.Path
	}
	return "household:" + strconv.Itoa(int(d.HouseholdID))
}

// compareBackupDocuments orders documents by household so the drill reports
// them in a stable order.
func compareBackupDocuments(a, b *backupDocument) int {
	if a.HouseholdID != b.HouseholdID {
		return cmp.Compare(a.HouseholdID, b.HouseholdID)
	}
	return strings.Compare(a.Path, b.Path)
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
func (d *BackupDrill) openScratchDatabase(ctx context.Context) (*gorm.DB, func(), error) {
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

	scoped := db.WithContext(ctx)
	if err := migrations.AutoMigrateAll(scoped); err != nil {
		closeDB()
		return nil, nil, drillFailure(ctx, fmt.Errorf("%w: %s", errors.ErrBackupRestoreFailed, err))
	}
	// The canonical categories are reference data every instance holds, and
	// the restore resolves a product's category against them. The scratch
	// database needs them too, or the drill would restore into a household
	// that production never has. The seed logs through a silent logger: the
	// operator reads the drill's findings, not the scratch setup.
	silent := zerolog.Nop()
	if err := migrations.SeedProductCategoryPrices(&silent, scoped); err != nil {
		closeDB()
		return nil, nil, drillFailure(ctx, fmt.Errorf("%w: %s", errors.ErrBackupRestoreFailed, err))
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

// backupDrillExpectedCount reports the count the document claims for one
// entity it carries. The entities the document does not carry have no
// expectation.
func backupDrillExpectedCount(document *apiModel.FullExportResponse, entity BackupDrillEntity) int {
	switch entity {
	case BackupDrillProducts:
		return len(document.Products.Active)
	case BackupDrillArchivedProducts:
		return len(document.Products.Archived)
	case BackupDrillStorageLocations:
		return len(backupLocationDrafts(document))
	default:
		return 0
	}
}

// compareBackupCounts builds the comparison table and reports the entities
// whose counts differ. The entities the document does not carry are listed
// without an expectation: the backup schema does not hold them, so there is
// nothing to compare them against.
func compareBackupCounts(
	document *apiModel.FullExportResponse,
	restored map[BackupDrillEntity]int,
) []BackupDrillCount {
	counts := make([]BackupDrillCount, 0, len(backupDrillEntities))
	for _, entity := range backupDrillEntities {
		covered := backupDrillCarriedEntities[entity]
		expected := 0
		if covered {
			expected = backupDrillExpectedCount(document, entity)
		}
		counts = append(counts, BackupDrillCount{
			Entity:   entity,
			Expected: expected,
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
//
// A broken backup outranks everything else: it is a defect that needs
// attention now, whatever else the run found. A moved schema comes next, then
// a backup that stopped being written.
func BackupDrillExitCode(err error) int {
	if err == nil {
		return BackupDrillExitOK
	}
	switch {
	case stderrors.Is(err, errors.ErrBackupInvalidDocument),
		stderrors.Is(err, errors.ErrBackupRestoreFailed),
		stderrors.Is(err, errors.ErrBackupDrillMismatch):
		return BackupDrillExitFailed
	case stderrors.Is(err, errors.ErrBackupUnsupportedVersion):
		return BackupDrillExitSchemaMismatch
	case stderrors.Is(err, errors.ErrBackupDrillTimeout):
		return BackupDrillExitTimeout
	case stderrors.Is(err, errors.ErrBackupDrillStale):
		return BackupDrillExitStale
	case stderrors.Is(err, errors.ErrBackupNotFound), stderrors.Is(err, errors.ErrBackupInvalidDirectory):
		return BackupDrillExitUnavailable
	default:
		return BackupDrillExitFailed
	}
}

// backupAge returns how long ago a document was written. A clock that runs
// ahead of the writer's would report a negative age, and an age an operator
// reads never goes below zero. A document whose age is unknown has none.
func backupAge(writtenAt time.Time) time.Duration {
	if writtenAt.IsZero() {
		return 0
	}
	age := time.Since(writtenAt)
	if age < 0 {
		return 0
	}
	return age
}
