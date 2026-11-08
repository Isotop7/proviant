package database

import (
	"fmt"
	"testing"
	"time"

	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

func TestProductRepository_GetUserProductsBulk(t *testing.T) {
	db := testutil.SetupTestDB(t)
	listRepo := NewProductListRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	product := dbModel.Product{
		ProductName: "Test Product",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
		ExpireAt:    time.Now().Add(24 * time.Hour),
	}
	db.Create(&product)

	products, err := listRepo.GetUserProductsBulk(user.ID, 10)
	if err != nil {
		t.Fatalf("GetUserProductsBulk() error = %v", err)
	}

	if len(products) != 1 {
		t.Errorf("GetUserProductsBulk() returned %d products, want 1", len(products))
	}
}
func TestProductRepository_GetUserProductsByIDs(t *testing.T) {
	db := testutil.SetupTestDB(t)
	listRepo := NewProductListRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	otherHousehold := dbModel.Household{Name: "Other Household"}
	db.Create(&otherHousehold)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	ownProduct := dbModel.Product{ProductName: "Own", Barcode: "1234567890123", HouseholdID: household.ID}
	db.Create(&ownProduct)
	otherProduct := dbModel.Product{ProductName: "Other", Barcode: "1234567890124", HouseholdID: otherHousehold.ID}
	db.Create(&otherProduct)
	archived := dbModel.Product{ProductName: "Archived", Barcode: "1234567890125", HouseholdID: household.ID}
	db.Create(&archived)
	db.Delete(&archived)

	t.Run("returns only requested, own household, active products", func(t *testing.T) {
		products, err := listRepo.GetUserProductsByIDs(user.ID, []uint{ownProduct.ID, otherProduct.ID, archived.ID, 9999})
		if err != nil {
			t.Fatalf("GetUserProductsByIDs() error = %v", err)
		}
		if len(products) != 1 {
			t.Fatalf("GetUserProductsByIDs() returned %d products, want 1", len(products))
		}
		if products[0].ID != ownProduct.ID {
			t.Errorf("returned product ID = %d, want %d", products[0].ID, ownProduct.ID)
		}
	})

	t.Run("empty ids returns nothing", func(t *testing.T) {
		products, err := listRepo.GetUserProductsByIDs(user.ID, []uint{})
		if err != nil {
			t.Fatalf("GetUserProductsByIDs() error = %v", err)
		}
		if len(products) != 0 {
			t.Errorf("GetUserProductsByIDs() returned %d products, want 0", len(products))
		}
	})
}
func TestProductRepository_GetUserArchivedProductsBulk(t *testing.T) {
	db := testutil.SetupTestDB(t)
	listRepo := NewProductListRepository(db)

	household := dbModel.Household{Name: "Test Household"}
	db.Create(&household)

	user := authentication.User{
		Username:    "testuser",
		Password:    "password",
		MailAddress: "test@example.com",
		HouseholdID: household.ID,
	}
	db.Create(&user)

	product := dbModel.Product{
		ProductName: "Archived Product",
		Barcode:     "1234567890123",
		HouseholdID: household.ID,
	}
	db.Create(&product)
	db.Delete(&product)

	archived, err := listRepo.GetUserArchivedProductsBulk(user.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetUserArchivedProductsBulk() error = %v", err)
	}

	if len(archived) != 1 {
		t.Errorf("GetUserArchivedProductsBulk() returned %d products, want 1", len(archived))
	}
}

func TestProductRepository_GetUserProductsBulkByBarcodesChunking(t *testing.T) {
	db := testutil.SetupTestDB(t)
	listRepo := NewProductListRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	product := testutil.CreateTestProduct(db, household.ID, user.ID)

	// More than one chunk (bulkQueryChunkSize = 500): the matching barcode
	// sits in the second chunk.
	barcodes := make([]string, 0, bulkQueryChunkSize+1)
	for i := range bulkQueryChunkSize {
		barcodes = append(barcodes, fmt.Sprintf("5%012d", i))
	}
	barcodes = append(barcodes, product.Barcode)

	products, err := listRepo.GetUserProductsBulkByBarcodes(user.ID, barcodes)
	if err != nil {
		t.Fatalf("GetUserProductsBulkByBarcodes() error = %v", err)
	}
	if len(products) != 1 || products[0].ID != product.ID {
		t.Errorf("GetUserProductsBulkByBarcodes() = %+v, want product %d", products, product.ID)
	}
}
func TestProductRepository_GetActiveProductProjectionsByLocation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	listRepo := NewProductListRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)
	location := testutil.CreateTestStorageLocation(db, household.ID)

	inLocation := testutil.CreateTestProduct(db, household.ID, user.ID)
	inLocation.StorageLocationID = &location.ID
	if err := db.Save(inLocation).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	testutil.CreateTestProduct(db, household.ID, user.ID)

	projections, err := listRepo.GetActiveProductProjections(user.ID, location.ID)
	if err != nil {
		t.Fatalf("GetActiveProductProjections() error = %v", err)
	}
	if len(projections) != 1 || projections[0].ID != inLocation.ID {
		t.Errorf("GetActiveProductProjections() = %+v, want only product %d", projections, inLocation.ID)
	}

	projections, err = listRepo.GetActiveProductProjections(user.ID, 0)
	if err != nil {
		t.Fatalf("GetActiveProductProjections() error = %v", err)
	}
	if len(projections) != 2 {
		t.Errorf("GetActiveProductProjections() without location returned %d, want 2", len(projections))
	}
}

func TestGetUserProductsBulkByBarcodes(t *testing.T) {
	t.Run("excludes other members' private products", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		listRepo := NewProductListRepository(db)
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

		rows, err := listRepo.GetUserProductsBulkByBarcodes(owner.ID, []string{
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
		listRepo := NewProductListRepository(db)
		rows, err := listRepo.GetUserProductsBulkByBarcodes(1, nil)
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
		listRepo := NewProductListRepository(db)
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

		rows, err := listRepo.GetUserProductsBulkByBarcodes(user.ID, lookups)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(rows) != 1 || rows[0].Barcode != needle.Barcode {
			t.Errorf("got %d rows, want the one product past the chunk boundary", len(rows))
		}
	})
}

func TestProductRepository_GetUserProductsBulkByBarcode(t *testing.T) {
	db := testutil.SetupTestDB(t)
	listRepo := NewProductListRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	otherHousehold := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	match1 := testutil.CreateTestProduct(db, household.ID, user.ID)
	match1.Barcode = "1234567890123"
	if err := db.Save(match1).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	match2 := testutil.CreateTestProduct(db, household.ID, user.ID)
	match2.Barcode = "1234567890123"
	if err := db.Save(match2).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	other := testutil.CreateTestProduct(db, household.ID, user.ID)
	other.Barcode = "5555555555555"
	if err := db.Save(other).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	foreign := testutil.CreateTestProduct(db, otherHousehold.ID)
	foreign.Barcode = "1234567890123"
	if err := db.Save(foreign).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	t.Run("returns only own household matches", func(t *testing.T) {
		products, err := listRepo.GetUserProductsBulkByBarcode(user.ID, 1234567890123)
		if err != nil {
			t.Fatalf("GetUserProductsBulkByBarcode() error = %v", err)
		}
		if len(products) != 2 {
			t.Fatalf("GetUserProductsBulkByBarcode() returned %d products, want 2", len(products))
		}
		for _, p := range products {
			if p.Barcode != "1234567890123" {
				t.Errorf("Barcode = %q, want 1234567890123", p.Barcode)
			}
		}
	})

	t.Run("unknown barcode returns nothing", func(t *testing.T) {
		products, err := listRepo.GetUserProductsBulkByBarcode(user.ID, 42)
		if err != nil {
			t.Fatalf("GetUserProductsBulkByBarcode() error = %v", err)
		}
		if len(products) != 0 {
			t.Errorf("GetUserProductsBulkByBarcode() returned %d products, want 0", len(products))
		}
	})

	t.Run("unknown user errors", func(t *testing.T) {
		if _, err := listRepo.GetUserProductsBulkByBarcode(9999, 1234567890123); err == nil {
			t.Error("GetUserProductsBulkByBarcode(9999) error = nil, want error")
		}
	})
}
func TestProductRepository_GetProductsExpired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	listRepo := NewProductListRepository(db)
	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	past := testutil.CreateTestProduct(db, household.ID, user.ID)
	past.ExpireAt = time.Now().Add(-48 * time.Hour)
	if err := db.Save(past).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	future := testutil.CreateTestProduct(db, household.ID, user.ID)
	future.ExpireAt = time.Now().Add(48 * time.Hour)
	if err := db.Save(future).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}
	// A zero expire_at means no expiry set — never expired (matches
	// GetExpiredProductsCount).
	undated := testutil.CreateTestProduct(db, household.ID, user.ID)
	undated.ExpireAt = time.Time{}
	if err := db.Save(undated).Error; err != nil {
		t.Fatalf("save product: %v", err)
	}

	// The filter keeps products whose effective expiry lies in the past;
	// future-dated and undated ones stay out.
	expired, err := listRepo.GetProductsExpired(user.ID)
	if err != nil {
		t.Fatalf("GetProductsExpired() error = %v", err)
	}
	got := map[uint]bool{}
	for _, p := range expired {
		got[p.ID] = true
	}
	if len(expired) != 1 || !got[past.ID] {
		t.Errorf("GetProductsExpired() = %v, want past %d only", expired, past.ID)
	}
	if got[undated.ID] {
		t.Errorf("GetProductsExpired contains undated product %d", undated.ID)
	}
	if got[future.ID] {
		t.Errorf("GetProductsExpired() contains future product %d", future.ID)
	}

	if _, err := listRepo.GetProductsExpired(9999); err == nil {
		t.Error("GetProductsExpired(9999) error = nil, want error")
	}
}
