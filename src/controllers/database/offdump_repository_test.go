package database_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Isotop7/proviant/controllers/database"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupOffDumpRepository(t *testing.T) *database.OffDumpRepository {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "off_dump.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open dump database: %v", err)
	}
	repo := database.NewOffDumpRepository(db)
	if err := repo.Migrate(); err != nil {
		t.Fatalf("failed to migrate dump database: %v", err)
	}
	return repo
}

// TestOffDumpRepositoryUpsertAndLookup verifies batch upsert replaces existing
// rows and lookups report a clean miss for unknown barcodes.
func TestOffDumpRepositoryUpsertAndLookup(t *testing.T) {
	repo := setupOffDumpRepository(t)
	ctx := context.Background()

	t.Run("insert and lookup", func(t *testing.T) {
		rows := []dbModel.OffProduct{
			{Barcode: "111", Data: `{"product_name":"Milk"}`},
			{Barcode: "222", Data: `{"product_name":"Bread"}`},
		}
		if err := repo.UpsertBatch(ctx, rows); err != nil {
			t.Fatalf("UpsertBatch: %v", err)
		}

		data, found, err := repo.Lookup("111")
		if err != nil {
			t.Fatalf("Lookup: %v", err)
		}
		if !found || data != `{"product_name":"Milk"}` {
			t.Errorf("Lookup(111) = %q, %v; want Milk, true", data, found)
		}
	})

	t.Run("upsert replaces existing row", func(t *testing.T) {
		if err := repo.UpsertBatch(ctx, []dbModel.OffProduct{{Barcode: "111", Data: `{"product_name":"Oat Milk"}`}}); err != nil {
			t.Fatalf("UpsertBatch replace: %v", err)
		}
		data, found, err := repo.Lookup("111")
		if err != nil {
			t.Fatalf("Lookup: %v", err)
		}
		if !found || data != `{"product_name":"Oat Milk"}` {
			t.Errorf("Lookup(111) = %q, %v; want Oat Milk, true", data, found)
		}

		count, err := repo.Count()
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 2 {
			t.Errorf("Count = %d, want 2", count)
		}
	})

	t.Run("unknown barcode is a clean miss", func(t *testing.T) {
		_, found, err := repo.Lookup("999")
		if err != nil {
			t.Fatalf("Lookup: %v", err)
		}
		if found {
			t.Errorf("found = true, want false for unknown barcode")
		}
	})
}

// TestOffDumpRepositoryState verifies state defaults to empty and survives
// overwrites.
func TestOffDumpRepositoryState(t *testing.T) {
	repo := setupOffDumpRepository(t)

	t.Run("unset state is empty", func(t *testing.T) {
		value, err := repo.GetState(util.OffDumpStateLastFullImport)
		if err != nil {
			t.Fatalf("GetState: %v", err)
		}
		if value != "" {
			t.Errorf("value = %q, want empty", value)
		}
	})

	t.Run("set, overwrite and read back", func(t *testing.T) {
		if err := repo.SetState(util.OffDumpStateLastDeltaEnd, "100"); err != nil {
			t.Fatalf("SetState: %v", err)
		}
		if err := repo.SetState(util.OffDumpStateLastDeltaEnd, "200"); err != nil {
			t.Fatalf("SetState overwrite: %v", err)
		}
		value, err := repo.GetState(util.OffDumpStateLastDeltaEnd)
		if err != nil {
			t.Fatalf("GetState: %v", err)
		}
		if value != "200" {
			t.Errorf("value = %q, want 200", value)
		}
	})
}
