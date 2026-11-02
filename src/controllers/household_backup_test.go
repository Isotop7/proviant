package controllers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
	"github.com/Isotop7/proviant/util"

	"github.com/Isotop7/proviant/controllers/database"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// TestBuildFullExportStatsCarriesTheUseFirstList pins that the backup
// document carries the same use-first list the dashboard shows: the products of
// the household ordered by the effective expiry, with the place and the pack
// count of each.
func TestBuildFullExportStatsCarriesTheUseFirstList(t *testing.T) {
	now := time.Now()
	location := dbModel.StorageLocation{Model: gorm.Model{ID: 7}, Name: "Fridge", Icon: "🧊"}

	active := []dbModel.Product{
		{Model: gorm.Model{ID: 1}, ProductName: "Later", Quantity: 1, ExpireAt: now.AddDate(0, 0, 20)},
		{Model: gorm.Model{ID: 2}, ProductName: "Soon", Quantity: 3, PackSize: "250g", ExpireAt: now.AddDate(0, 0, 2),
			StorageLocation: &location, StorageLocationID: &location.ID},
		{Model: gorm.Model{ID: 3}, ProductName: "Undated", Quantity: 1},
	}

	stats := buildFullExportStats(active, nil, 7, now)

	if len(stats.UseFirst) != 3 {
		t.Fatalf("useFirst = %+v, want every active product", stats.UseFirst)
	}
	if stats.UseFirst[0].ProductName != "Soon" {
		t.Errorf("useFirst[0] = %q, want the product that expires first", stats.UseFirst[0].ProductName)
	}
	if stats.UseFirst[2].ProductName != "Undated" {
		t.Errorf("useFirst[2] = %q, want the product without a date last", stats.UseFirst[2].ProductName)
	}
	if stats.UseFirst[0].Quantity != 3 || stats.UseFirst[0].PackSize != "250g" {
		t.Errorf("quantity/packSize = %d/%q, want 3/250g",
			stats.UseFirst[0].Quantity, stats.UseFirst[0].PackSize)
	}
	if stats.UseFirst[0].StorageLocationName != location.Name {
		t.Errorf("storageLocationName = %q, want %q", stats.UseFirst[0].StorageLocationName, location.Name)
	}

	// The expiring list carries the same place and pack count as the use-first
	// list: the backup shows what the dashboard shows.
	if len(stats.ExpiringSoon) != 1 {
		t.Fatalf("expiringSoon = %+v, want the one product in the window", stats.ExpiringSoon)
	}
	entry := stats.ExpiringSoon[0]
	if entry.StorageLocationName != location.Name || entry.StorageLocationIcon != location.Icon {
		t.Errorf("location = %q/%q, want %q/%q",
			entry.StorageLocationName, entry.StorageLocationIcon, location.Name, location.Icon)
	}
	if entry.Quantity != 3 || entry.PackSize != "250g" {
		t.Errorf("quantity/packSize = %d/%q, want 3/250g", entry.Quantity, entry.PackSize)
	}
}

// TestBuildFullExportStatsTrendMatchesTheDashboard pins the backup document's
// expiry trend to the one the dashboard shows: both are computed from the same
// rows, so they have to name the same months with the same counts.
func TestBuildFullExportStatsTrendMatchesTheDashboard(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := database.NewProductRepository(db)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	// The 31st is the day the month arithmetic breaks on, so the anchor is
	// pinned to one: the assertion has to hold on every day of the year. The
	// anchor sits in the container's zone, because the trend buckets the
	// calendar months of that zone.
	anchor := time.Date(2026, time.October, 31, 12, 0, 0, 0, time.Local)

	active := make([]dbModel.Product, 0, 2)
	for _, expiry := range []time.Time{
		time.Date(2026, time.October, 15, 0, 0, 0, 0, time.Local),
		time.Date(2026, time.November, 15, 0, 0, 0, 0, time.Local),
	} {
		product := testutil.CreateTestProduct(db, household.ID, user.ID)
		product.ExpireAt = expiry
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("save product: %v", err)
		}
		active = append(active, *product)
	}

	stats := buildFullExportStats(active, nil, 7, anchor)
	trend, err := repo.GetExpiryTrend(user.ID, anchor)
	if err != nil {
		t.Fatalf("GetExpiryTrend: %v", err)
	}

	if len(stats.ExpiryTrend) != len(trend) {
		t.Fatalf("backup trend has %d months, dashboard trend has %d", len(stats.ExpiryTrend), len(trend))
	}
	for i := range trend {
		if stats.ExpiryTrend[i] != trend[i] {
			t.Errorf("backup trend[%d] = %+v, dashboard trend[%d] = %+v", i, stats.ExpiryTrend[i], i, trend[i])
		}
	}
	if len(trend) == 0 {
		t.Fatal("dashboard trend is empty")
	}
	if trend[0].Count != 1 || trend[1].Count != 1 {
		t.Errorf("dashboard trend counts = %d/%d, want 1/1", trend[0].Count, trend[1].Count)
	}
}

// TestBuildFullExportIncludesPrivateProducts pins the backup scope: a scheduled
// backup must carry every member's stock, including private rows the user-facing
// export deliberately hides from other members.
func TestBuildFullExportIncludesPrivateProducts(t *testing.T) {
	db := testutil.SetupTestDB(t)
	owner := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, owner.ID)
	owner.HouseholdID = household.ID
	if err := db.Save(owner).Error; err != nil {
		t.Fatalf("save owner: %v", err)
	}
	member := testutil.CreateTestUser(db, household.ID)
	member.Username = "member"
	if err := db.Save(member).Error; err != nil {
		t.Fatalf("save member: %v", err)
	}

	shared := testutil.CreateTestProduct(db, household.ID, owner.ID)
	shared.ProductName = "Shared"
	if err := db.Save(shared).Error; err != nil {
		t.Fatalf("save shared product: %v", err)
	}
	private := testutil.CreateTestProduct(db, household.ID, member.ID)
	private.ProductName = "Private"
	private.IsPrivate = true
	if err := db.Save(private).Error; err != nil {
		t.Fatalf("save private product: %v", err)
	}

	logger := zerolog.Nop()
	document, err := BuildFullExport(db, &logger, household.ID, owner.ID, true)
	if err != nil {
		t.Fatalf("BuildFullExport: %v", err)
	}
	if document.Version != apiModel.BackupSchemaVersion {
		t.Errorf("version = %d, want %d", document.Version, apiModel.BackupSchemaVersion)
	}
	if len(document.Products.Active) != 2 {
		t.Errorf("active products = %d, want 2 including the private row", len(document.Products.Active))
	}
	for i := range document.Products.Active {
		if document.Products.Active[i].ProductName == "Private" && document.Products.Active[i].OwnerUsername != "member" {
			t.Errorf("ownerUsername = %q, want member", document.Products.Active[i].OwnerUsername)
		}
	}
}

// TestRestoreHouseholdBackupRejectsUnsupportedVersion keeps a future document
// from being partially applied.
func TestRestoreHouseholdBackupRejectsUnsupportedVersion(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}
	testutil.CreateTestProduct(db, household.ID, user.ID)

	backup := apiModel.FullExportResponse{Version: apiModel.BackupSchemaVersion + 1}
	if _, err := RestoreHouseholdBackup(db, household.ID, user.ID, &backup); err == nil {
		t.Fatal("RestoreHouseholdBackup error = nil, want unsupported version error")
	}

	var count int64
	if err := db.Model(&dbModel.Product{}).Count(&count).Error; err != nil {
		t.Fatalf("count products: %v", err)
	}
	if count != 1 {
		t.Errorf("products = %d, want the existing row untouched", count)
	}
}

// TestRestoreHouseholdBackupKeepsTheCategoryKey pins the restore of the
// canonical category: the restore insert writes an explicit column list, so a
// category that is missing from it is dropped without a word.
func TestRestoreHouseholdBackupKeepsTheCategoryKey(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}
	if err := db.Create(&dbModel.ProductCategoryPrice{
		CategoryKey: "milk",
		DisplayName: "Milk",
	}).Error; err != nil {
		t.Fatalf("seed canonical row: %v", err)
	}

	backup := apiModel.FullExportResponse{
		Version: apiModel.BackupSchemaVersion,
		Products: apiModel.FullExportProducts{
			Active: []apiModel.ExportProductResponse{
				{ProductName: "Milk", Barcode: "1234567890123", Categories: "milk", Quantity: 1},
			},
		},
	}

	if _, err := RestoreHouseholdBackup(db, household.ID, user.ID, &backup); err != nil {
		t.Fatalf("RestoreHouseholdBackup: %v", err)
	}

	var stored dbModel.Product
	if err := db.Where("household_id = ?", household.ID).First(&stored).Error; err != nil {
		t.Fatalf("read the restored product: %v", err)
	}
	if stored.CategoryKey == nil {
		t.Fatal("the restored product has no category")
	}
	if *stored.CategoryKey != "milk" {
		t.Errorf("category key = %q, want milk", *stored.CategoryKey)
	}
}

// TestRestoreHouseholdBackupFillsTheHouseholdShelfLife covers the restore of a
// product that names no date: the household's default for its category fills
// it, the same rule the create paths follow.
func TestRestoreHouseholdBackupFillsTheHouseholdShelfLife(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}
	if err := db.Create(&dbModel.ProductCategoryPrice{
		CategoryKey: "milk",
		DisplayName: "Milk",
	}).Error; err != nil {
		t.Fatalf("seed canonical row: %v", err)
	}
	shelfLife := 5
	if _, err := database.NewHouseholdCategoryRepository(db).Upsert(household.ID, "milk", database.HouseholdCategoryOverride{
		DefaultShelfLifeDays: &shelfLife,
	}); err != nil {
		t.Fatalf("seed the household shelf life: %v", err)
	}

	backup := apiModel.FullExportResponse{
		Version: apiModel.BackupSchemaVersion,
		Products: apiModel.FullExportProducts{
			Active: []apiModel.ExportProductResponse{
				{ProductName: "Milk", Barcode: "1234567890123", Categories: "milk", Quantity: 1},
			},
		},
	}

	if _, err := RestoreHouseholdBackup(db, household.ID, user.ID, &backup); err != nil {
		t.Fatalf("RestoreHouseholdBackup: %v", err)
	}

	var stored dbModel.Product
	if err := db.Where("household_id = ?", household.ID).First(&stored).Error; err != nil {
		t.Fatalf("read the restored product: %v", err)
	}
	if stored.ExpireAt.IsZero() {
		t.Fatal("the restored product has no expiry date")
	}
	if !stored.ExpireAt.After(time.Now().AddDate(0, 0, 4)) {
		t.Errorf("expire at = %v, want about five days from now", stored.ExpireAt)
	}
}

// TestRestoreProductFromRowReadsCounts covers the restore of a current
// document: the pack count and the pack size are carried over as they are.
func TestRestoreProductFromRowReadsCounts(t *testing.T) {
	row := apiModel.ExportProductResponse{ProductName: "Flour", Barcode: "1234567890123", Quantity: 2, PackSize: "1 kg"}

	product, err := restoreProductFromRow(&row, false, 1, map[string]uint{}, 1, map[string]uint{})
	if err != nil {
		t.Fatalf("restoreProductFromRow: %v", err)
	}
	if product.Quantity != 2 || product.PackSize != "1 kg" {
		t.Errorf("count/pack size = %d %q, want 2 \"1 kg\"", product.Quantity, product.PackSize)
	}
}

// TestRestoreProductFromRowReadsLegacyAmounts covers a backup written before
// products were counted in packs: its measured rows describe one pack of that
// size, its counted rows keep their count.
func TestRestoreProductFromRowReadsLegacyAmounts(t *testing.T) {
	measured := apiModel.ExportProductResponse{ProductName: "Flour", Barcode: "1234567890123", Amount: 2000, Unit: "g"}
	product, err := restoreProductFromRow(&measured, false, 1, map[string]uint{}, 1, map[string]uint{})
	if err != nil {
		t.Fatalf("restoreProductFromRow: %v", err)
	}
	if product.Quantity != 1 || product.PackSize != "2000 g" {
		t.Errorf("count/pack size = %d %q, want 1 \"2000 g\"", product.Quantity, product.PackSize)
	}

	counted := apiModel.ExportProductResponse{ProductName: "Eggs", Barcode: "1234567890124", Amount: 12, Unit: "pcs"}
	product, err = restoreProductFromRow(&counted, false, 1, map[string]uint{}, 1, map[string]uint{})
	if err != nil {
		t.Fatalf("restoreProductFromRow: %v", err)
	}
	if product.Quantity != 12 || product.PackSize != "" {
		t.Errorf("count/pack size = %d %q, want 12 and no pack size", product.Quantity, product.PackSize)
	}
}

// TestBackupControllerRunOnceWritesVersionedDocuments covers the scheduled job's
// storage contract: one JSON document per household, carrying the same schema
// version as the export endpoint.
func TestBackupControllerRunOnceWritesVersionedDocuments(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}
	testutil.CreateTestProduct(db, household.ID, user.ID)

	directory := t.TempDir()
	config := &configuration.BackupConfiguration{
		Enabled:       true,
		IntervalHours: 24,
		Directory:     directory,
		Retention:     7,
	}
	logger := zerolog.Nop()
	controller := NewBackupController(&logger, config, db)
	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	matches, err := filepath.Glob(filepath.Join(directory, util.BackupFilePrefix+"*.json"))
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("backup files = %d, want 1", len(matches))
	}
	payload, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	var document apiModel.FullExportResponse
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatalf("decode backup: %v", err)
	}
	if document.Version != apiModel.BackupSchemaVersion {
		t.Errorf("version = %d, want %d", document.Version, apiModel.BackupSchemaVersion)
	}
	if len(document.Products.Active) != 1 {
		t.Errorf("active products = %d, want 1", len(document.Products.Active))
	}
}

// TestBackupControllerPruneKeepsNewestFiles pins the retention behaviour: only
// the newest documents survive, and only for the household they belong to.
func TestBackupControllerPruneKeepsNewestFiles(t *testing.T) {
	directory := t.TempDir()
	config := &configuration.BackupConfiguration{Enabled: true, Directory: directory, Retention: 1}
	logger := zerolog.Nop()
	controller := NewBackupController(&logger, config, nil)

	names := []string{
		util.BackupFilePrefix + "household-1-20260101T000000Z.json",
		util.BackupFilePrefix + "household-1-20260102T000000Z.json",
		util.BackupFilePrefix + "household-2-20260101T000000Z.json",
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("{}"), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	controller.pruneHouseholdBackups(directory, 1)

	remaining := map[string]bool{}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for i := range entries {
		remaining[entries[i].Name()] = true
	}
	if !remaining[util.BackupFilePrefix+"household-1-20260102T000000Z.json"] {
		t.Error("newest household 1 backup was pruned")
	}
	if remaining[util.BackupFilePrefix+"household-1-20260101T000000Z.json"] {
		t.Error("oldest household 1 backup survived retention 1")
	}
	if !remaining[util.BackupFilePrefix+"household-2-20260101T000000Z.json"] {
		t.Error("another household's backup was pruned")
	}
}

// TestBackupControllerDispatchRunsImmediately pins the startup contract: a
// freshly dispatched job writes a backup before waiting for the first interval.
func TestBackupControllerDispatchRunsImmediately(t *testing.T) {
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	directory := t.TempDir()
	config := &configuration.BackupConfiguration{
		Enabled:       true,
		IntervalHours: 24,
		Directory:     directory,
		Retention:     7,
	}
	logger := zerolog.Nop()
	controller := NewBackupController(&logger, config, db)
	defer controller.Stop()
	controller.Dispatch()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		matches, globErr := filepath.Glob(filepath.Join(directory, util.BackupFilePrefix+"*.json"))
		if globErr != nil {
			t.Fatalf("glob backups: %v", globErr)
		}
		if len(matches) > 0 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("Dispatch did not write a backup before the first interval")
}

// TestBackupControllerDisabledWritesNothing keeps the default configuration
// from creating backup files on every install.
func TestBackupControllerDisabledWritesNothing(t *testing.T) {
	directory := t.TempDir()
	config := &configuration.BackupConfiguration{Directory: directory}
	logger := zerolog.Nop()
	controller := NewBackupController(&logger, config, testutil.SetupTestDB(t))

	if err := controller.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("backup files = %d, want 0 while disabled", len(entries))
	}
}
