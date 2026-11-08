package controllers

import (
	"context"
	stderrors "errors"
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
	result, err := drill.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

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
	writeBackupDocument(t, directory, `{"version":3,"products":`)

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
	result, err := drill.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

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
// broken backup, a moved schema and nothing to drill are three different
// outcomes.
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

// writeBackupDocument writes a backup document into a backup directory under
// the name the scheduler gives it.
func writeBackupDocument(t *testing.T, directory, content string) {
	t.Helper()
	name := filepath.Join(directory, util.BackupFilePrefix+"household-1-20260101T000000.000Z.json")
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatalf("write the backup document: %v", err)
	}
}
