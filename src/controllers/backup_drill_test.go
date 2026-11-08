package controllers

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	dbController "github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"

	"github.com/rs/zerolog"
)

// TestBackupDrillRestoresTheNewestBackupAndMatchesTheCounts covers the drill's
// contract: the document the scheduler wrote restores into a scratch database
// and comes back with the rows it says it carries.
func TestBackupDrillRestoresTheNewestBackupAndMatchesTheCounts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}
	testutil.CreateTestProduct(db, household.ID, user.ID)
	testutil.SeedConsumedProduct(t, db, household.ID, user.ID, false, "1234567890124", "Archived", 1, time.Now())
	testutil.CreateTestStorageLocation(db, household.ID)

	directory := t.TempDir()
	logger := zerolog.Nop()
	controller := NewBackupController(&logger, &configuration.BackupConfiguration{
		Enabled:   true,
		Directory: directory,
		Retention: 7,
	}, db, dbController.NewRepositoryContainer(db, &logger))
	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	drill := &BackupDrill{
		Logger:          &logger,
		BackupDirectory: directory,
		ScratchRoot:     t.TempDir(),
	}
	run, err := drill.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(run.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(run.Results))
	}
	result := run.Results[0]

	if result.SchemaVersion != apiModel.BackupSchemaVersion {
		t.Errorf("schema version = %d, want %d", result.SchemaVersion, apiModel.BackupSchemaVersion)
	}
	if result.BackupPath == "" {
		t.Error("the drill did not name the document it drilled")
	}
	if result.BackupAge <= 0 {
		t.Errorf("backup age = %s, want the age of the document", result.BackupAge)
	}
	if len(result.Mismatches) != 0 {
		t.Errorf("mismatches = %+v, want the restored rows to match the document", result.Mismatches)
	}

	counts := map[BackupDrillEntity]BackupDrillCount{}
	for _, count := range result.Counts {
		counts[count.Entity] = count
	}
	active := counts[BackupDrillProducts]
	if !active.Covered {
		t.Error("the active products are not covered by the comparison")
	}
	if active.Expected != 1 || active.Restored != 1 {
		t.Errorf("products = %d expected, %d restored, want 1 and 1", active.Expected, active.Restored)
	}
	archived := counts[BackupDrillArchivedProducts]
	if archived.Expected != 1 || archived.Restored != 1 {
		t.Errorf("archived products = %d expected, %d restored, want 1 and 1", archived.Expected, archived.Restored)
	}
	locations := counts[BackupDrillStorageLocations]
	if locations.Expected != 1 || locations.Restored != 1 {
		t.Errorf("storage locations = %d expected, %d restored, want 1 and 1", locations.Expected, locations.Restored)
	}
}

// TestBackupDrillReportsASchemaMismatchDistinctly keeps a document written
// against another schema apart from a broken one: the schema moved, the
// backup did not break.
func TestBackupDrillReportsASchemaMismatchDistinctly(t *testing.T) {
	directory := t.TempDir()
	writeBackupDocument(t, directory, `{"version":`+strconv.Itoa(apiModel.BackupSchemaVersion+1)+`}`)

	logger := zerolog.Nop()
	drill := &BackupDrill{Logger: &logger, BackupDirectory: directory, ScratchRoot: t.TempDir()}
	_, err := drill.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want a schema mismatch")
	}
	if !stderrors.Is(err, errors.ErrBackupUnsupportedVersion) {
		t.Errorf("error = %v, want %v", err, errors.ErrBackupUnsupportedVersion)
	}
	if code := BackupDrillExitCode(err); code != BackupDrillExitSchemaMismatch {
		t.Errorf("exit code = %d, want %d", code, BackupDrillExitSchemaMismatch)
	}
}

// TestBackupDrillReportsACorruptDocument covers a backup that cannot be read
// at all: a truncated or half-written file.
func TestBackupDrillReportsACorruptDocument(t *testing.T) {
	directory := t.TempDir()
	writeBackupDocument(t, directory, `{"version":`+strconv.Itoa(apiModel.BackupSchemaVersion)+`,"products":`)

	logger := zerolog.Nop()
	drill := &BackupDrill{Logger: &logger, BackupDirectory: directory, ScratchRoot: t.TempDir()}
	_, err := drill.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want a broken document")
	}
	if !stderrors.Is(err, errors.ErrBackupInvalidDocument) {
		t.Errorf("error = %v, want %v", err, errors.ErrBackupInvalidDocument)
	}
	if code := BackupDrillExitCode(err); code != BackupDrillExitFailed {
		t.Errorf("exit code = %d, want %d", code, BackupDrillExitFailed)
	}
}

// TestBackupDrillReportsAForeignDocumentAsBroken keeps a file that is not a
// backup apart from a backup written against another schema: a document that
// names no schema version is broken, not moved.
func TestBackupDrillReportsAForeignDocumentAsBroken(t *testing.T) {
	directory := t.TempDir()
	writeBackupDocument(t, directory, `{"household":{"name":"Not a backup"}}`)

	logger := zerolog.Nop()
	drill := &BackupDrill{Logger: &logger, BackupDirectory: directory, ScratchRoot: t.TempDir()}
	_, err := drill.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want a broken document")
	}
	if !stderrors.Is(err, errors.ErrBackupInvalidDocument) {
		t.Errorf("error = %v, want %v", err, errors.ErrBackupInvalidDocument)
	}
	if code := BackupDrillExitCode(err); code != BackupDrillExitFailed {
		t.Errorf("exit code = %d, want %d", code, BackupDrillExitFailed)
	}
}

// TestBackupDrillRemovesTheScratchDatabaseOnFailure keeps a failed drill from
// leaving a restored household behind on disk.
func TestBackupDrillRemovesTheScratchDatabaseOnFailure(t *testing.T) {
	directory := t.TempDir()
	writeBackupDocument(t, directory, "not a backup document")

	scratchRoot := t.TempDir()
	logger := zerolog.Nop()
	drill := &BackupDrill{Logger: &logger, BackupDirectory: directory, ScratchRoot: scratchRoot}
	if _, err := drill.Run(context.Background()); err == nil {
		t.Fatal("Run error = nil, want a broken document")
	}

	entries, err := os.ReadDir(scratchRoot)
	if err != nil {
		t.Fatalf("read scratch root: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("scratch entries = %d, want the scratch database to be gone", len(entries))
	}
}

// TestBackupDrillNeverTouchesTheLiveDatabase pins the promise the drill makes:
// the live database file is read by nobody and written by nobody. The scratch
// database is a different file, and the live one keeps its bytes.
func TestBackupDrillNeverTouchesTheLiveDatabase(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}
	testutil.CreateTestProduct(db, household.ID, user.ID)

	directory := t.TempDir()
	logger := zerolog.Nop()
	controller := NewBackupController(&logger, &configuration.BackupConfiguration{
		Enabled:   true,
		Directory: directory,
		Retention: 7,
	}, db, dbController.NewRepositoryContainer(db, &logger))
	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	livePath := filepath.Join(t.TempDir(), "live.db")
	liveBytes := []byte("the live database")
	if err := os.WriteFile(livePath, liveBytes, 0o600); err != nil {
		t.Fatalf("write the live database: %v", err)
	}

	drill := &BackupDrill{
		Logger:           &logger,
		BackupDirectory:  directory,
		ScratchRoot:      t.TempDir(),
		LiveDatabasePath: livePath,
	}
	if _, err := drill.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	readBack, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatalf("read the live database: %v", err)
	}
	if string(readBack) != string(liveBytes) {
		t.Errorf("the live database changed: %q, want %q", readBack, liveBytes)
	}
}

// TestBackupDrillReportsTheEntitiesTheDocumentDoesNotCover keeps the gap
// visible: the backup document carries no recipes, no meal plan and no
// shopping list, so the drill reports those counts without an expectation.
func TestBackupDrillReportsTheEntitiesTheDocumentDoesNotCover(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}
	testutil.CreateTestProduct(db, household.ID, user.ID)

	directory := t.TempDir()
	logger := zerolog.Nop()
	controller := NewBackupController(&logger, &configuration.BackupConfiguration{
		Enabled:   true,
		Directory: directory,
		Retention: 7,
	}, db, dbController.NewRepositoryContainer(db, &logger))
	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	drill := &BackupDrill{Logger: &logger, BackupDirectory: directory, ScratchRoot: t.TempDir()}
	run, err := drill.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(run.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(run.Results))
	}
	result := run.Results[0]

	for _, entity := range []BackupDrillEntity{BackupDrillRecipes, BackupDrillMealPlanEntries, BackupDrillShoppingListItems} {
		found := false
		for _, count := range result.Counts {
			if count.Entity != entity {
				continue
			}
			found = true
			if count.Covered {
				t.Errorf("%s is covered by the comparison, want it reported as not covered", entity)
			}
		}
		if !found {
			t.Errorf("%s is missing from the comparison table", entity)
		}
	}
}

// TestBackupDrillComparesTheProductRows covers the row comparison: a restore
// that keeps the count but writes a different value is caught by the field it
// changed, not only by the count.
func TestBackupDrillComparesTheProductRows(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	expiry := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	document := &apiModel.FullExportResponse{
		Products: apiModel.FullExportProducts{
			Active: []apiModel.ExportProductResponse{{
				Barcode:       "1234567890123",
				ProductName:   "Milk",
				Categories:    "milk",
				Quantity:      1,
				PackSize:      "1 l",
				ExpireAt:      expiry,
				OwnerUsername: user.Username,
			}},
		},
	}

	seeded := scratchHousehold{
		ID:          household.ID,
		AdminUserID: user.ID,
		Usernames:   map[uint]string{user.ID: user.Username},
	}
	restored := dbModel.Product{
		Barcode:     "1234567890123",
		ProductName: "Milk",
		Categories:  "milk",
		Quantity:    2,
		PackSize:    "1 l",
		ExpireAt:    expiry,
		HouseholdID: household.ID,
		UserID:      user.ID,
	}
	if err := db.Create(&restored).Error; err != nil {
		t.Fatalf("seed the restored product: %v", err)
	}

	mismatches, err := compareProductRows(document, db, seeded)
	if err != nil {
		t.Fatalf("compareProductRows: %v", err)
	}
	if len(mismatches) != 1 {
		t.Fatalf("mismatches = %d, want 1", len(mismatches))
	}
	if mismatches[0].Field != "quantity" {
		t.Errorf("field = %q, want %q", mismatches[0].Field, "quantity")
	}
	if mismatches[0].Document != "1" || mismatches[0].Restored != "2" {
		t.Errorf("mismatch = %q/%q, want 1/2", mismatches[0].Document, mismatches[0].Restored)
	}
}

// TestBackupDrillComparesTheProductRowsOfAHealthyDocument covers the other
// side of the comparison: a document and a restore that agree on every field
// produce no mismatch.
func TestBackupDrillComparesTheProductRowsOfAHealthyDocument(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	expiry := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	document := &apiModel.FullExportResponse{
		Products: apiModel.FullExportProducts{
			Active: []apiModel.ExportProductResponse{{
				Barcode:       "1234567890123",
				ProductName:   "Milk",
				Categories:    "milk",
				Quantity:      1,
				PackSize:      "1 l",
				ExpireAt:      expiry,
				OwnerUsername: user.Username,
			}},
		},
	}

	seeded := scratchHousehold{
		ID:          household.ID,
		AdminUserID: user.ID,
		Usernames:   map[uint]string{user.ID: user.Username},
	}
	restored := dbModel.Product{
		Barcode:     "1234567890123",
		ProductName: "Milk",
		Categories:  "milk",
		Quantity:    1,
		PackSize:    "1 l",
		ExpireAt:    expiry,
		HouseholdID: household.ID,
		UserID:      user.ID,
	}
	if err := db.Create(&restored).Error; err != nil {
		t.Fatalf("seed the restored product: %v", err)
	}

	mismatches, err := compareProductRows(document, db, seeded)
	if err != nil {
		t.Fatalf("compareProductRows: %v", err)
	}
	if len(mismatches) != 0 {
		t.Errorf("mismatches = %+v, want none", mismatches)
	}
}

// TestBackupDrillNamesTheBarcodeThatDiffers covers the report of a row whose
// barcode differs: the drill has to name the field, not report an empty one.
func TestBackupDrillNamesTheBarcodeThatDiffers(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	document := &apiModel.FullExportResponse{
		Products: apiModel.FullExportProducts{
			Active: []apiModel.ExportProductResponse{{Barcode: "1111111111111", ProductName: "Milk", Quantity: 1}},
		},
	}
	seeded := scratchHousehold{
		ID:          household.ID,
		AdminUserID: user.ID,
		Usernames:   map[uint]string{user.ID: user.Username},
	}
	restored := dbModel.Product{
		Barcode:     "2222222222222",
		ProductName: "Milk",
		Quantity:    1,
		HouseholdID: household.ID,
		UserID:      user.ID,
	}
	if err := db.Create(&restored).Error; err != nil {
		t.Fatalf("seed the restored product: %v", err)
	}

	mismatches, err := compareProductRows(document, db, seeded)
	if err != nil {
		t.Fatalf("compareProductRows: %v", err)
	}
	if len(mismatches) != 1 {
		t.Fatalf("mismatches = %d, want 1", len(mismatches))
	}
	if mismatches[0].Field != "barcode" {
		t.Errorf("field = %q, want %q", mismatches[0].Field, "barcode")
	}
	if mismatches[0].Document != "1111111111111" || mismatches[0].Restored != "2222222222222" {
		t.Errorf("mismatch = %q/%q, want 1111111111111/2222222222222", mismatches[0].Document, mismatches[0].Restored)
	}
}

// TestBackupDrillComparesTheStorageLocations covers a location that came back
// under another name, including one no product refers to.
func TestBackupDrillComparesTheStorageLocations(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	document := &apiModel.FullExportResponse{
		StorageLocations: []apiModel.StorageLocationResponse{{Name: "Fridge"}},
	}
	restored := dbModel.StorageLocation{Name: "Pantry", HouseholdID: household.ID}
	if err := db.Create(&restored).Error; err != nil {
		t.Fatalf("seed the restored location: %v", err)
	}

	mismatches, err := compareStorageLocations(document, db, household.ID)
	if err != nil {
		t.Fatalf("compareStorageLocations: %v", err)
	}
	if len(mismatches) != 1 {
		t.Fatalf("mismatches = %d, want 1", len(mismatches))
	}
	if mismatches[0].Entity != BackupDrillMismatchStorageLocation {
		t.Errorf("entity = %q, want %q", mismatches[0].Entity, BackupDrillMismatchStorageLocation)
	}
	if mismatches[0].Document != "fridge" || mismatches[0].Restored != "pantry" {
		t.Errorf("mismatch = %q/%q, want fridge/pantry", mismatches[0].Document, mismatches[0].Restored)
	}
}

// TestBackupDrillComparesTheIconOfAStorageLocation covers a location that
// came back with the right name and the wrong icon.
func TestBackupDrillComparesTheIconOfAStorageLocation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	document := &apiModel.FullExportResponse{
		StorageLocations: []apiModel.StorageLocationResponse{{Name: "Fridge", Icon: "🧊"}},
	}
	restored := dbModel.StorageLocation{Name: "Fridge", Icon: "📦", HouseholdID: household.ID}
	if err := db.Create(&restored).Error; err != nil {
		t.Fatalf("seed the restored location: %v", err)
	}

	mismatches, err := compareStorageLocations(document, db, household.ID)
	if err != nil {
		t.Fatalf("compareStorageLocations: %v", err)
	}
	if len(mismatches) != 1 {
		t.Fatalf("mismatches = %d, want 1", len(mismatches))
	}
	if mismatches[0].Field != "icon" {
		t.Errorf("field = %q, want %q", mismatches[0].Field, "icon")
	}
	if mismatches[0].Document != "🧊" || mismatches[0].Restored != "📦" {
		t.Errorf("mismatch = %q/%q, want the two icons", mismatches[0].Document, mismatches[0].Restored)
	}
}

// TestBackupDrillComparesTheHouseholdMetadata covers a household whose name
// the restore dropped: the counts would not show it.
func TestBackupDrillComparesTheHouseholdMetadata(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}
	household.Name = "A different household"
	if err := db.Save(household).Error; err != nil {
		t.Fatalf("save household: %v", err)
	}

	document := &apiModel.FullExportResponse{
		Household: &apiModel.FullExportHousehold{Name: "Drill Household", Description: "The drill"},
	}

	mismatches, err := compareHouseholdMetadata(document, db, household.ID)
	if err != nil {
		t.Fatalf("compareHouseholdMetadata: %v", err)
	}
	if len(mismatches) != 2 {
		t.Fatalf("mismatches = %d, want the name and the description", len(mismatches))
	}
	if mismatches[0].Entity != BackupDrillMismatchHousehold {
		t.Errorf("entity = %q, want %q", mismatches[0].Entity, BackupDrillMismatchHousehold)
	}
	if mismatches[0].Field != "name" {
		t.Errorf("field = %q, want %q", mismatches[0].Field, "name")
	}
	if mismatches[0].Document != "Drill Household" || mismatches[0].Restored != "A different household" {
		t.Errorf("mismatch = %q/%q, want the household's two names", mismatches[0].Document, mismatches[0].Restored)
	}
}

// TestSeedScratchHouseholdDoesNotPreFillTheName keeps the household name
// comparison honest: the scratch household starts with the drill's own name,
// so a restore that never writes the document's name is caught.
func TestSeedScratchHouseholdDoesNotPreFillTheName(t *testing.T) {
	db := testutil.SetupTestDB(t)
	document := &apiModel.FullExportResponse{
		Household: &apiModel.FullExportHousehold{Name: "Drill Household"},
	}

	seeded, err := seedScratchHousehold(db, document)
	if err != nil {
		t.Fatalf("seedScratchHousehold: %v", err)
	}

	var household dbModel.Household
	if err := db.First(&household, seeded.ID).Error; err != nil {
		t.Fatalf("read the household: %v", err)
	}
	if household.Name != util.BackupDrillHouseholdName {
		t.Errorf("household name = %q, want %q", household.Name, util.BackupDrillHouseholdName)
	}
}

// TestRestoreAppliesTheHouseholdNameTheSeedDidNot covers the other side: the
// restore writes the document's name into the scratch household, and the
// comparison finds nothing wrong with it.
func TestRestoreAppliesTheHouseholdNameTheSeedDidNot(t *testing.T) {
	db := testutil.SetupTestDB(t)
	document := &apiModel.FullExportResponse{
		Version: apiModel.BackupSchemaVersion,
		Household: &apiModel.FullExportHousehold{
			Name:        "Drill Household",
			Description: "The drill",
		},
	}

	seeded, err := seedScratchHousehold(db, document)
	if err != nil {
		t.Fatalf("seedScratchHousehold: %v", err)
	}
	if _, err := RestoreHouseholdBackup(db, seeded.ID, seeded.AdminUserID, document); err != nil {
		t.Fatalf("RestoreHouseholdBackup: %v", err)
	}

	var household dbModel.Household
	if err := db.First(&household, seeded.ID).Error; err != nil {
		t.Fatalf("read the household: %v", err)
	}
	if household.Name != "Drill Household" {
		t.Errorf("household name = %q, want %q", household.Name, "Drill Household")
	}

	mismatches, err := compareHouseholdMetadata(document, db, seeded.ID)
	if err != nil {
		t.Fatalf("compareHouseholdMetadata: %v", err)
	}
	if len(mismatches) != 0 {
		t.Errorf("mismatches = %+v, want none", mismatches)
	}
}

// TestBackupDrillExitCodes pins the exit codes a scheduled job reads: a
// broken backup, a moved schema, a backup that stopped being written and
// nothing to drill are four different outcomes.
func TestBackupDrillExitCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{name: "no error", err: nil, want: BackupDrillExitOK},
		{name: "broken document", err: errors.ErrBackupInvalidDocument, want: BackupDrillExitFailed},
		{name: "restore failed", err: errors.ErrBackupRestoreFailed, want: BackupDrillExitFailed},
		{name: "counts differ", err: errors.ErrBackupDrillMismatch, want: BackupDrillExitFailed},
		{name: "schema moved", err: errors.ErrBackupUnsupportedVersion, want: BackupDrillExitSchemaMismatch},
		{name: "stale backup", err: errors.ErrBackupDrillStale, want: BackupDrillExitStale},
		{name: "out of time", err: errors.ErrBackupDrillTimeout, want: BackupDrillExitTimeout},
		{name: "a broken backup outranks a stale one", err: stderrors.Join(errors.ErrBackupDrillStale, errors.ErrBackupDrillMismatch), want: BackupDrillExitFailed},
		{name: "a broken backup outranks a moved schema", err: stderrors.Join(errors.ErrBackupUnsupportedVersion, errors.ErrBackupInvalidDocument), want: BackupDrillExitFailed},
		{name: "a moved schema outranks a timeout", err: stderrors.Join(errors.ErrBackupDrillTimeout, errors.ErrBackupUnsupportedVersion), want: BackupDrillExitSchemaMismatch},
		{name: "a timeout outranks a stale backup", err: stderrors.Join(errors.ErrBackupDrillStale, errors.ErrBackupDrillTimeout), want: BackupDrillExitTimeout},
		{name: "no backup", err: errors.ErrBackupNotFound, want: BackupDrillExitUnavailable},
		{name: "no directory", err: errors.ErrBackupInvalidDirectory, want: BackupDrillExitUnavailable},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if code := BackupDrillExitCode(testCase.err); code != testCase.want {
				t.Errorf("exit code = %d, want %d", code, testCase.want)
			}
		})
	}
}

// TestBackupDrillExitCodePrefersABrokenBackupOverEverythingElse covers the
// precedence of the exit codes: a broken backup is a defect that needs
// attention now, whatever else the run found.
func TestBackupDrillExitCodePrefersABrokenBackupOverEverythingElse(t *testing.T) {
	joined := stderrors.Join(errors.ErrBackupDrillStale, errors.ErrBackupDrillMismatch)

	if code := BackupDrillExitCode(joined); code != BackupDrillExitFailed {
		t.Errorf("exit code = %d, want %d", code, BackupDrillExitFailed)
	}
}

// TestBackupDrillPicksTheNewestDocumentOfEachHousehold covers the selection:
// every household is drilled, and the document it is drilled with is the
// newest one the directory holds for it.
func TestBackupDrillPicksTheNewestDocumentOfEachHousehold(t *testing.T) {
	directory := t.TempDir()
	older := time.Now().Add(-48 * time.Hour)
	newer := time.Now().Add(-time.Hour)
	writeBackupDocumentAs(t, directory, 1, older, healthyBackupDocument)
	writeBackupDocumentAs(t, directory, 1, newer, healthyBackupDocument)
	writeBackupDocumentAs(t, directory, 10, older, healthyBackupDocument)

	logger := zerolog.Nop()
	drill := &BackupDrill{Logger: &logger, BackupDirectory: directory, ScratchRoot: t.TempDir()}
	run, err := drill.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(run.Results) != 2 {
		t.Fatalf("results = %d, want 2 (one per household)", len(run.Results))
	}
	if run.Results[0].HouseholdID != 1 {
		t.Errorf("household = %d, want 1", run.Results[0].HouseholdID)
	}
	if want := util.BackupDocumentName(1, newer); filepath.Base(run.Results[0].BackupPath) != want {
		t.Errorf("document = %s, want the newest one of the household (%s)",
			filepath.Base(run.Results[0].BackupPath), want)
	}
	if run.Results[1].HouseholdID != 10 {
		t.Errorf("household = %d, want 10", run.Results[1].HouseholdID)
	}
}

// TestBackupDrillDrillsEveryHousehold covers the other side of the
// selection: a household whose newest document is broken does not stop the
// drill, and the healthy household is still proven.
func TestBackupDrillDrillsEveryHousehold(t *testing.T) {
	directory := t.TempDir()
	writtenAt := time.Now().Add(-time.Hour)
	writeBackupDocumentAs(t, directory, 1, writtenAt, healthyBackupDocument)
	writeBackupDocumentAs(t, directory, 2, writtenAt, "not a backup document")

	logger := zerolog.Nop()
	drill := &BackupDrill{Logger: &logger, BackupDirectory: directory, ScratchRoot: t.TempDir()}
	run, err := drill.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want the broken household")
	}

	if len(run.Results) != 2 {
		t.Fatalf("results = %d, want 2 (one per household)", len(run.Results))
	}
	if run.Results[0].Err != nil {
		t.Errorf("the healthy household failed: %v", run.Results[0].Err)
	}
	if run.Results[1].Err == nil {
		t.Error("the broken household did not fail")
	}
	if code := BackupDrillExitCode(err); code != BackupDrillExitFailed {
		t.Errorf("exit code = %d, want %d", code, BackupDrillExitFailed)
	}
}

// TestBackupDrillFailsOnAStaleBackup covers the second silent failure: the
// backup restores, but the job that writes the backups stopped.
func TestBackupDrillFailsOnAStaleBackup(t *testing.T) {
	directory := t.TempDir()
	writeBackupDocumentAs(t, directory, 1, time.Now().Add(-72*time.Hour), healthyBackupDocument)

	logger := zerolog.Nop()
	drill := &BackupDrill{
		Logger:          &logger,
		BackupDirectory: directory,
		ScratchRoot:     t.TempDir(),
		MaxAge:          48 * time.Hour,
	}
	run, err := drill.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want a stale backup")
	}
	if !stderrors.Is(err, errors.ErrBackupDrillStale) {
		t.Errorf("error = %v, want %v", err, errors.ErrBackupDrillStale)
	}
	if code := BackupDrillExitCode(err); code != BackupDrillExitStale {
		t.Errorf("exit code = %d, want %d", code, BackupDrillExitStale)
	}
	if len(run.Results) != 1 || !run.Results[0].Stale {
		t.Errorf("results = %+v, want one stale document", run.Results)
	}
}

// TestBackupDrillAcceptsAFreshBackup covers the other side of the staleness
// bound: a backup within the bound is not a failure.
func TestBackupDrillAcceptsAFreshBackup(t *testing.T) {
	directory := t.TempDir()
	writeBackupDocumentAs(t, directory, 1, time.Now().Add(-time.Hour), healthyBackupDocument)

	logger := zerolog.Nop()
	drill := &BackupDrill{
		Logger:          &logger,
		BackupDirectory: directory,
		ScratchRoot:     t.TempDir(),
		MaxAge:          48 * time.Hour,
	}
	run, err := drill.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(run.Results) != 1 || run.Results[0].Stale {
		t.Errorf("results = %+v, want one fresh document", run.Results)
	}
}

// TestBackupDrillIgnoresStalenessForAnExplicitDocument covers the document
// the operator named by hand: it is not the backup job's newest output, so
// its age says nothing about the job.
func TestBackupDrillIgnoresStalenessForAnExplicitDocument(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, util.BackupDocumentName(1, time.Now().Add(-72*time.Hour)))
	if err := os.WriteFile(path, []byte(healthyBackupDocument), 0o600); err != nil {
		t.Fatalf("write the backup document: %v", err)
	}

	logger := zerolog.Nop()
	drill := &BackupDrill{
		Logger:          &logger,
		BackupFile:      path,
		ScratchRoot:     t.TempDir(),
		MaxAge:          time.Hour,
	}
	run, err := drill.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(run.Results) != 1 || run.Results[0].Stale {
		t.Errorf("results = %+v, want one document that is not stale", run.Results)
	}
}

// TestBackupDrillFailsWhenTheCountsDiffer covers a document that restores
// into fewer rows than it carries: the drill has to name the entity.
func TestBackupDrillFailsWhenTheCountsDiffer(t *testing.T) {
	document := apiModel.FullExportResponse{
		Version: apiModel.BackupSchemaVersion,
		Stats: apiModel.ProductStatsResponse{
			TotalActive:   1,
			TotalArchived: 0,
		},
		Products: apiModel.FullExportProducts{
			Active: []apiModel.ExportProductResponse{
				{ProductName: "Milk", Barcode: "1234567890123", Quantity: 1},
			},
		},
	}
	counts := compareBackupCounts(&document, map[BackupDrillEntity]int{
		BackupDrillProducts:         0,
		BackupDrillArchivedProducts: 0,
		BackupDrillStorageLocations: 0,
	})

	mismatches := backupDrillMismatches(counts)
	if len(mismatches) != 1 {
		t.Fatalf("mismatches = %d, want 1", len(mismatches))
	}
	if mismatches[0].Entity != BackupDrillProducts {
		t.Errorf("mismatch = %s, want %s", mismatches[0].Entity, BackupDrillProducts)
	}
}

// TestBackupDrillRejectsADocumentWhoseStatsDisagree keeps a document whose
// halves contradict each other out of the drill: it was not written by the
// scheduler.
func TestBackupDrillRejectsADocumentWhoseStatsDisagree(t *testing.T) {
	document := apiModel.FullExportResponse{
		Version: apiModel.BackupSchemaVersion,
		Stats: apiModel.ProductStatsResponse{
			TotalActive: 5,
		},
		Products: apiModel.FullExportProducts{
			Active: []apiModel.ExportProductResponse{{Barcode: "1234567890123"}},
		},
	}

	if err := checkDocumentStats(&document); err == nil {
		t.Fatal("checkDocumentStats error = nil, want a mismatch")
	} else if !stderrors.Is(err, errors.ErrBackupDrillMismatch) {
		t.Errorf("error = %v, want %v", err, errors.ErrBackupDrillMismatch)
	}
}

// TestBackupDrillReportsAMissingDocument keeps a document that is not there
// apart from a broken one: there is nothing to drill.
func TestBackupDrillReportsAMissingDocument(t *testing.T) {
	logger := zerolog.Nop()
	missing := filepath.Join(t.TempDir(), util.BackupDocumentName(1, time.Now()))
	drill := &BackupDrill{Logger: &logger, BackupFile: missing, ScratchRoot: t.TempDir()}
	_, err := drill.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want a missing document")
	}
	if !stderrors.Is(err, errors.ErrBackupNotFound) {
		t.Errorf("error = %v, want %v", err, errors.ErrBackupNotFound)
	}
	if code := BackupDrillExitCode(err); code != BackupDrillExitUnavailable {
		t.Errorf("exit code = %d, want %d", code, BackupDrillExitUnavailable)
	}
}

// TestBackupDrillReportsADocumentItCannotRead keeps a backup the drill cannot
// open out of the report: a file it cannot read is a backup it cannot prove,
// and it must not vanish from the drill — nor take the households behind it
// down with it.
func TestBackupDrillReportsADocumentItCannotRead(t *testing.T) {
	directory := t.TempDir()
	writeBackupDocumentAs(t, directory, 1, time.Now(), healthyBackupDocument)
	dangling := filepath.Join(directory, util.BackupDocumentName(2, time.Now()))
	if err := os.Symlink(filepath.Join(directory, "gone.json"), dangling); err != nil {
		t.Fatalf("symlink the backup document: %v", err)
	}

	logger := zerolog.Nop()
	drill := &BackupDrill{Logger: &logger, BackupDirectory: directory, ScratchRoot: t.TempDir()}
	run, err := drill.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want the unreadable document")
	}
	if !stderrors.Is(err, errors.ErrBackupInvalidDocument) {
		t.Errorf("error = %v, want %v", err, errors.ErrBackupInvalidDocument)
	}

	if len(run.Results) != 2 {
		t.Fatalf("results = %d, want 2 (one per household)", len(run.Results))
	}
	if run.Results[0].Err != nil {
		t.Errorf("the readable household failed: %v", run.Results[0].Err)
	}
	if run.Results[1].Err == nil {
		t.Error("the unreadable household did not fail")
	}
	if run.Results[1].HouseholdID != 2 {
		t.Errorf("household = %d, want 2", run.Results[1].HouseholdID)
	}
}

// TestBackupDrillNamesTheHouseholdTheDocumentCarries covers the label of a
// drilled document: the household knows itself by its name, and the id in the
// file name is the bookkeeping of the instance that wrote it.
func TestBackupDrillNamesTheHouseholdTheDocumentCarries(t *testing.T) {
	directory := t.TempDir()
	writeBackupDocumentAs(t, directory, 7, time.Now(), healthyBackupDocument)

	logger := zerolog.Nop()
	drill := &BackupDrill{Logger: &logger, BackupDirectory: directory, ScratchRoot: t.TempDir()}
	run, err := drill.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(run.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(run.Results))
	}
	if run.Results[0].HouseholdName != "Drill Household" {
		t.Errorf("household name = %q, want %q", run.Results[0].HouseholdName, "Drill Household")
	}
	if run.Results[0].HouseholdID != 7 {
		t.Errorf("household id = %d, want 7", run.Results[0].HouseholdID)
	}
}

// TestBackupDrillReportsATimedOutRestoreAsATimeout keeps a restore that ran
// out of time apart from a broken backup: the operator has to know which of
// the two it was, or they go hunting for a corrupt file that is fine.
func TestBackupDrillReportsATimedOutRestoreAsATimeout(t *testing.T) {
	directory := t.TempDir()
	writeBackupDocumentAs(t, directory, 1, time.Now(), healthyBackupDocument)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	logger := zerolog.Nop()
	drill := &BackupDrill{Logger: &logger, BackupDirectory: directory, ScratchRoot: t.TempDir()}
	_, err := drill.Run(ctx)
	if err == nil {
		t.Fatal("Run error = nil, want a timeout")
	}
	if !stderrors.Is(err, errors.ErrBackupDrillTimeout) {
		t.Errorf("error = %v, want %v", err, errors.ErrBackupDrillTimeout)
	}
	if stderrors.Is(err, errors.ErrBackupRestoreFailed) {
		t.Errorf("error = %v, want it kept apart from a broken backup", err)
	}
	if code := BackupDrillExitCode(err); code != BackupDrillExitTimeout {
		t.Errorf("exit code = %d, want %d", code, BackupDrillExitTimeout)
	}
}

// TestDrillFailureNamesATimeout covers the classification of a failure: a
// restore that ran out of time is not a broken backup.
func TestDrillFailureNamesATimeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := drillFailure(ctx, fmt.Errorf("%w: %s", errors.ErrBackupRestoreFailed, "context deadline exceeded"))

	if !stderrors.Is(err, errors.ErrBackupDrillTimeout) {
		t.Errorf("error = %v, want %v", err, errors.ErrBackupDrillTimeout)
	}
	if stderrors.Is(err, errors.ErrBackupRestoreFailed) {
		t.Errorf("error = %v, want it kept apart from a broken backup", err)
	}
}

// TestBackupDrillUncoveredEntitiesAreTheSchemaGap pins the entities the
// backup document does not carry. They are the gap issue #469 tracks: a new
// one is a change to the backup schema, not to the drill.
func TestBackupDrillUncoveredEntitiesAreTheSchemaGap(t *testing.T) {
	want := []BackupDrillEntity{BackupDrillRecipes, BackupDrillMealPlanEntries, BackupDrillShoppingListItems}

	got := BackupDrillUncoveredEntities()
	if len(got) != len(want) {
		t.Fatalf("uncovered = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("uncovered = %v, want %v", got, want)
		}
	}
}

// TestBackupDrillExpectedCountCoversEveryCarriedEntity keeps the count of
// every carried entity in step with the list of carried entities. An entity
// the comparison claims to carry but cannot count is compared against
// nothing, and the drill would pass on rows the restore dropped.
func TestBackupDrillExpectedCountCoversEveryCarriedEntity(t *testing.T) {
	document := apiModel.FullExportResponse{
		Products: apiModel.FullExportProducts{
			Active:   []apiModel.ExportProductResponse{{Barcode: "1234567890123"}},
			Archived: []apiModel.ExportProductResponse{{Barcode: "1234567890124"}},
		},
		StorageLocations: []apiModel.StorageLocationResponse{{Name: "Fridge"}},
	}

	for entity := range backupDrillCarriedEntities {
		if backupDrillExpectedCount(&document, entity) == 0 {
			t.Errorf("%s is carried, but the document's rows are not counted", entity)
		}
	}
}

// TestBackupDrillReportsNoBackup keeps an empty backup directory apart from a
// broken document: there is nothing to drill.
func TestBackupDrillReportsNoBackup(t *testing.T) {
	logger := zerolog.Nop()
	drill := &BackupDrill{Logger: &logger, BackupDirectory: t.TempDir(), ScratchRoot: t.TempDir()}
	_, err := drill.Run(context.Background())
	if err == nil {
		t.Fatal("Run error = nil, want a missing backup")
	}
	if !stderrors.Is(err, errors.ErrBackupNotFound) {
		t.Errorf("error = %v, want %v", err, errors.ErrBackupNotFound)
	}
	if code := BackupDrillExitCode(err); code != BackupDrillExitUnavailable {
		t.Errorf("exit code = %d, want %d", code, BackupDrillExitUnavailable)
	}
}

// TestBackupDrillCountsTheStorageLocationsTheRestoreCreates covers the rule
// the drill shares with the restore: a product that names a place the
// document's own list does not carry still gets that place.
func TestBackupDrillCountsTheStorageLocationsTheRestoreCreates(t *testing.T) {
	document := apiModel.FullExportResponse{
		StorageLocations: []apiModel.StorageLocationResponse{{Name: "Fridge"}},
		Products: apiModel.FullExportProducts{
			Active: []apiModel.ExportProductResponse{
				{Barcode: "1234567890123", StorageLocation: &apiModel.StorageLocationResponse{Name: "Pantry"}},
			},
			Archived: []apiModel.ExportProductResponse{
				{Barcode: "1234567890124", StorageLocation: &apiModel.StorageLocationResponse{Name: "fridge"}},
			},
		},
	}

	counts := compareBackupCounts(&document, map[BackupDrillEntity]int{})
	for _, count := range counts {
		if count.Entity != BackupDrillStorageLocations {
			continue
		}
		if count.Expected != 2 {
			t.Errorf("storage locations = %d, want 2 (the pantry and the fridge, named once each)", count.Expected)
		}
		return
	}
	t.Fatal("the storage locations are missing from the comparison table")
}

// healthyBackupDocument is a backup document that restores: it carries no
// products and no locations, so the drill's comparison has nothing to
// disagree about. Its version is the one this release writes, so a schema
// bump does not turn every test into a "the document names no schema
// version" failure.
var healthyBackupDocument = `{"version":` + strconv.Itoa(apiModel.BackupSchemaVersion) +
	`,"household":{"name":"Drill Household"},"members":[],` +
	`"products":{"active":[],"archived":[]},"stats":{"totalActive":0,"totalArchived":0}}`

// writeBackupDocument writes a backup document into a backup directory under
// the name the scheduler gives it.
func writeBackupDocument(t *testing.T, directory, content string) {
	t.Helper()
	writeBackupDocumentAs(t, directory, 1, time.Now(), content)
}

// writeBackupDocumentAs writes a backup document under the name the
// scheduler gives the backup of one household.
func writeBackupDocumentAs(t *testing.T, directory string, householdID uint, writtenAt time.Time, content string) {
	t.Helper()
	name := filepath.Join(directory, util.BackupDocumentName(householdID, writtenAt))
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatalf("write the backup document: %v", err)
	}
}
