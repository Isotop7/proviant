package database

import (
	stderrors "errors"
	"fmt"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	"codeberg.org/isotop7/proviant/util"

	"gorm.io/gorm"
)

func TestProductRepository(t *testing.T) {
	db := testutil.SetupTestDB(t)

	t.Run("GetProductByID not found", func(t *testing.T) {
		repo := NewProductRepository(db)

		_, err := repo.GetProductByID(9999, 1)
		if err != gorm.ErrRecordNotFound {
			t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
		}
	})

	t.Run("GetUserProductsBulk returns empty for no products", func(t *testing.T) {
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		testUser := testutil.CreateTestUser(db, household.ID)

		products, err := repo.GetUserProductsBulk(testUser.ID, 10)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(products) != 0 {
			t.Errorf("len(products) = %v, want 0", len(products))
		}
	})

	t.Run("GetMatchableProductsByHousehold populates only the matched columns", func(t *testing.T) {
		repo := NewProductRepository(db)
		householdID := uint(1)
		product := testutil.CreateTestProduct(db, householdID)
		product.ProductName = "Chicken"
		product.Categories = "meat"
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rows, err := repo.GetMatchableProductsByHousehold(householdID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("len(rows) = %d, want 1", len(rows))
		}
		row := rows[0]
		if row.ID != product.ID {
			t.Errorf("row.ID = %d, want %d", row.ID, product.ID)
		}
		if row.ProductName != "Chicken" {
			t.Errorf("row.ProductName = %q, want %q", row.ProductName, "Chicken")
		}
		if row.Categories != "meat" {
			t.Errorf("row.Categories = %q, want %q", row.Categories, "meat")
		}
		// The projection deliberately leaves everything else zero so a caller
		// reading an unselected column gets an obviously wrong value instead of
		// a plausible one.
		if row.Amount != 0 || row.Barcode != "" || !row.ExpireAt.IsZero() {
			t.Errorf("unselected columns are not zero: amount=%d barcode=%q expireAt=%v",
				row.Amount, row.Barcode, row.ExpireAt)
		}
	})

	t.Run("GetMatchableProductsByHousehold excludes private and deleted products", func(t *testing.T) {
		repo := NewProductRepository(db)
		householdID := uint(2)
		visible := testutil.CreateTestProduct(db, householdID)
		visible.ProductName = "Visible"
		if err := db.Save(visible).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		private := testutil.CreateTestProduct(db, householdID)
		private.ProductName = "Private"
		private.IsPrivate = true
		if err := db.Save(private).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		removed := testutil.CreateTestProduct(db, householdID)
		removed.ProductName = "Removed"
		if err := db.Delete(removed).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rows, err := repo.GetMatchableProductsByHousehold(householdID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 || rows[0].ProductName != "Visible" {
			t.Errorf("got %d rows (%+v), want only the visible product", len(rows), rows)
		}
	})
}

func TestGetActiveExpiryCounts(t *testing.T) {
	now := time.Now()
	criticalDays := 3

	t.Run("classifies expired and critical active products", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		expiredProduct := testutil.CreateTestProduct(db, household.ID)
		expiredProduct.ProductName = "Expired"
		expiredProduct.ExpireAt = now.AddDate(0, 0, -1)
		if err := db.Save(expiredProduct).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		criticalProduct := testutil.CreateTestProduct(db, household.ID)
		criticalProduct.ProductName = "Critical"
		criticalProduct.ExpireAt = now.AddDate(0, 0, 1)
		if err := db.Save(criticalProduct).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		freshProduct := testutil.CreateTestProduct(db, household.ID)
		freshProduct.ProductName = "Fresh"
		freshProduct.ExpireAt = now.AddDate(0, 0, 30)
		if err := db.Save(freshProduct).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expired, critical, err := repo.GetActiveExpiryCounts(user.ID, now, criticalDays)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expired != 1 || critical != 1 {
			t.Errorf("got expired=%d critical=%d, want 1 and 1", expired, critical)
		}
	})

	t.Run("an opened shelf life outranks a far printed expiry", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		// Printed expiry is months out, but opening it two days ago with a
		// one-day shelf life makes it expired. The SQL candidate filter has to
		// admit the row through the opened branch, not the printed one.
		openedAt := now.AddDate(0, 0, -2)
		daysAfterOpening := 1
		product := testutil.CreateTestProduct(db, household.ID)
		product.ExpireAt = now.AddDate(0, 6, 0)
		product.OpenedAt = &openedAt
		product.DaysAfterOpening = &daysAfterOpening
		if err := db.Save(product).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expired, critical, err := repo.GetActiveExpiryCounts(user.ID, now, criticalDays)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expired != 1 || critical != 0 {
			t.Errorf("got expired=%d critical=%d, want 1 and 0", expired, critical)
		}
	})

	t.Run("ignores archived and private products", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		// Soft-deleted: the archived view still renders it as a consumed row,
		// but it must not badge the sidebar.
		archived := testutil.CreateTestProduct(db, household.ID)
		archived.ProductName = "Archived"
		archived.ExpireAt = now.AddDate(0, 0, -5)
		if err := db.Save(archived).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := db.Delete(archived).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Private to another user in the same household.
		other := testutil.CreateTestUser(db, household.ID)
		private := testutil.CreateTestProduct(db, household.ID, other.ID)
		private.ProductName = "Private"
		private.IsPrivate = true
		private.ExpireAt = now.AddDate(0, 0, -5)
		if err := db.Save(private).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expired, critical, err := repo.GetActiveExpiryCounts(user.ID, now, criticalDays)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expired != 0 || critical != 0 {
			t.Errorf("got expired=%d critical=%d, want 0 and 0", expired, critical)
		}
	})
}

func TestGetUserProductsBulkByBarcodes(t *testing.T) {
	t.Run("excludes other members' private products", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		owner := testutil.CreateTestUser(db, household.ID)
		other := testutil.CreateTestUser(db, household.ID)

		shared := testutil.CreateTestProduct(db, household.ID)
		shared.Barcode = "4006381333931"
		shared.UserID = owner.ID
		if err := db.Save(shared).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		ownPrivate := testutil.CreateTestProduct(db, household.ID)
		ownPrivate.Barcode = "4006381333948"
		ownPrivate.UserID = owner.ID
		ownPrivate.IsPrivate = true
		if err := db.Save(ownPrivate).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		otherPrivate := testutil.CreateTestProduct(db, household.ID)
		otherPrivate.Barcode = "4006381333955"
		otherPrivate.UserID = other.ID
		otherPrivate.IsPrivate = true
		if err := db.Save(otherPrivate).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		archived := testutil.CreateTestProduct(db, household.ID)
		archived.Barcode = "4006381333962"
		archived.UserID = owner.ID
		if err := db.Delete(archived).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rows, err := repo.GetUserProductsBulkByBarcodes(owner.ID, []string{
			"4006381333931", "4006381333948", "4006381333955", "4006381333962",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// The import turns every returned barcode into an "already exists"
		// rejection, so a private product of another member showing up here
		// both leaks its existence and rejects a row the caller should own.
		got := make(map[string]bool, len(rows))
		for i := range rows {
			got[rows[i].Barcode] = true
		}
		if len(got) != 2 || !got["4006381333931"] || !got["4006381333948"] {
			t.Errorf("got barcodes %+v, want only the shared and own-private product", got)
		}
		if got["4006381333955"] {
			t.Error("another member's private product leaked into the barcode lookup")
		}
		if got["4006381333962"] {
			t.Error("an archived product leaked into the barcode lookup")
		}
	})

	t.Run("empty barcode set short-circuits", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		rows, err := repo.GetUserProductsBulkByBarcodes(1, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 0 {
			t.Errorf("got %d rows, want none", len(rows))
		}
	})

	// bulkQueryChunkSize exists because SQLite builds have capped bound
	// variables at 999, so the chunked queries have to keep finding matches
	// past the chunk boundary.
	t.Run("finds matches across chunk boundaries", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		// Index bulkQueryChunkSize is the first barcode of the second chunk.
		needle := testutil.CreateTestProduct(db, household.ID)
		needle.Barcode = fmt.Sprintf("400638133%04d", bulkQueryChunkSize)
		needle.UserID = user.ID
		if err := db.Save(needle).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		lookups := make([]string, 0, bulkQueryChunkSize+1)
		for i := 0; i <= bulkQueryChunkSize; i++ {
			lookups = append(lookups, fmt.Sprintf("400638133%04d", i))
		}

		rows, err := repo.GetUserProductsBulkByBarcodes(user.ID, lookups)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 || rows[0].Barcode != needle.Barcode {
			t.Errorf("got %d rows, want the one product past the chunk boundary", len(rows))
		}
	})
}

func TestCreateProductsBulk(t *testing.T) {
	t.Run("creates each distinct new location once and stamps the products", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		// Reusing an existing location is the resolver's job, above this layer;
		// every row here names something the household does not have yet.
		existing := testutil.CreateTestStorageLocation(db, household.ID)

		rows := []ImportedProduct{
			{Product: database.Product{ProductName: "Milk", Barcode: "4006381333931"}, NewLocationName: "Cellar", NewLocationIcon: "📦"},
			{Product: database.Product{ProductName: "Cheese", Barcode: "4006381333948"}, NewLocationName: "cellar", NewLocationIcon: "📦"},
			{Product: database.Product{ProductName: "Butter", Barcode: "4006381333955"}, NewLocationName: "Pantry", NewLocationIcon: "📦"},
		}

		created, err := repo.CreateProductsBulk(user.ID, rows)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(created) != 2 || created[0] != "Cellar" || created[1] != "Pantry" {
			t.Errorf("created = %+v, want [Cellar Pantry] in creation order", created)
		}

		var products []database.Product
		if err := db.Order("id").Find(&products).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(products) != 3 {
			t.Fatalf("got %d products, want 3", len(products))
		}
		for i := range products {
			if products[i].HouseholdID != household.ID || products[i].UserID != user.ID {
				t.Errorf("product %q stamped household %d user %d, want %d/%d",
					products[i].ProductName, products[i].HouseholdID, products[i].UserID, household.ID, user.ID)
			}
			if products[i].StorageLocationID == nil {
				t.Errorf("product %q has no storage location", products[i].ProductName)
			}
		}
		if *products[0].StorageLocationID != *products[1].StorageLocationID {
			t.Error("differently spelled names did not collapse onto one location")
		}
		if *products[0].StorageLocationID == *products[2].StorageLocationID {
			t.Error("two distinct names collapsed onto one location")
		}

		var locationCount int64
		if err := db.Model(&database.StorageLocation{}).Where(util.QueryHouseholdId, household.ID).Count(&locationCount).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// The pre-existing Fridge plus exactly the two new names.
		if locationCount != 3 {
			t.Errorf("got %d locations, want 3 (Fridge, Cellar, Pantry)", locationCount)
		}

		// New locations sort after the ones the household already had, which
		// seeded SortOrder 0.
		var newLocation database.StorageLocation
		if err := db.Where(util.QueryHouseholdId, household.ID).Where("name = ?", "Cellar").First(&newLocation).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if newLocation.SortOrder <= existing.SortOrder {
			t.Errorf("new location SortOrder = %d, want after the existing %d", newLocation.SortOrder, existing.SortOrder)
		}
	})

	t.Run("rejects a CSV naming too many new locations", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewProductRepository(db)
		household := testutil.CreateTestHousehold(db, 0)
		user := testutil.CreateTestUser(db, household.ID)

		rows := make([]ImportedProduct, 0, util.CsvImportMaxNewLocations+1)
		for i := range util.CsvImportMaxNewLocations + 1 {
			rows = append(rows, ImportedProduct{
				Product:         database.Product{ProductName: "Item", Barcode: fmt.Sprintf("40063813339%02d", i)},
				NewLocationName: fmt.Sprintf("Location %d", i),
			})
		}

		created, err := repo.CreateProductsBulk(user.ID, rows)
		if !stderrors.Is(err, errors.ErrImportTooManyLocations) {
			t.Fatalf("err = %v, want ErrImportTooManyLocations", err)
		}
		if created != nil {
			t.Errorf("created = %+v, want none", created)
		}

		// The whole import rolls back: no product and no location may survive.
		var products, locations int64
		if err := db.Model(&database.Product{}).Count(&products).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := db.Model(&database.StorageLocation{}).Count(&locations).Error; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if products != 0 || locations != 0 {
			t.Errorf("rolled back import left %d products and %d locations, want none", products, locations)
		}
	})
}
