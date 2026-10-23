package controllers

import (
	"fmt"
	"testing"

	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"github.com/rs/zerolog"
)

// stubDatasetGetter answers a barcode lookup from a fixed table and counts the
// lookups, so a test can tell a cache hit from a live request.
type stubDatasetGetter struct {
	datasets map[string]dbModel.Product
	lookups  int
}

func (s *stubDatasetGetter) GetDataset(barcode string) (dbModel.Product, error) {
	s.lookups++
	product, ok := s.datasets[barcode]
	if !ok {
		return dbModel.Product{}, errNoSuchBarcode
	}
	return product, nil
}

var errNoSuchBarcode = &barcodeError{}

type barcodeError struct{}

func (e *barcodeError) Error() string { return "no such barcode" }

// A cache entry without a pack size is no answer: every entry written before
// the pack size column existed has none, so the live lookup has to answer it.
func TestPackSizeControllerFallsThroughToTheLiveLookup(t *testing.T) {
	logger := zerolog.Nop()

	t.Run("a cached entry without a pack size is answered by the live lookup", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		product := testutil.CreateTestProduct(db, 1, 1)
		if err := db.Model(product).Update("barcode", "4006381333931").Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
		if err := db.Create(&dbModel.OpenFoodFactsCache{
			Barcode: "4006381333931", ProductName: "Milk",
		}).Error; err != nil {
			t.Fatalf("seed cache: %v", err)
		}
		getter := &stubDatasetGetter{datasets: map[string]dbModel.Product{
			"4006381333931": {Barcode: "4006381333931", PackSize: "1 l"},
		}}
		controller := NewPackSizeController(db, getter, &logger)

		if err := controller.RunOnce(t.Context()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}

		var stored dbModel.Product
		if err := db.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.PackSize != "1 l" {
			t.Errorf("pack size = %q, want 1 l", stored.PackSize)
		}
		if getter.lookups != 1 {
			t.Errorf("live lookups = %d, want 1", getter.lookups)
		}
	})

	t.Run("a cached entry with a pack size needs no live lookup", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		product := testutil.CreateTestProduct(db, 1, 1)
		if err := db.Model(product).Update("barcode", "4006381333931").Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
		if err := db.Create(&dbModel.OpenFoodFactsCache{
			Barcode: "4006381333931", ProductName: "Milk", PackSize: "500 ml",
		}).Error; err != nil {
			t.Fatalf("seed cache: %v", err)
		}
		getter := &stubDatasetGetter{datasets: map[string]dbModel.Product{
			"4006381333931": {Barcode: "4006381333931", PackSize: "1 l"},
		}}
		controller := NewPackSizeController(db, getter, &logger)

		if err := controller.RunOnce(t.Context()); err != nil {
			t.Fatalf("RunOnce: %v", err)
		}

		var stored dbModel.Product
		if err := db.First(&stored, product.ID).Error; err != nil {
			t.Fatalf("load product: %v", err)
		}
		if stored.PackSize != "500 ml" {
			t.Errorf("pack size = %q, want 500 ml", stored.PackSize)
		}
		if getter.lookups != 0 {
			t.Errorf("live lookups = %d, want 0", getter.lookups)
		}
	})
}

// One pass covers the whole backlog: the walk continues in batches until the
// table is walked to its end, so a household with more products than one
// batch does not wait days for the pack sizes the shopping list shows.
func TestPackSizeControllerPassCoversTheWholeBacklog(t *testing.T) {
	logger := zerolog.Nop()
	db := testutil.SetupTestDB(t)
	const products = packSizeBackfillBatch + 1
	datasets := make(map[string]dbModel.Product, products)
	for i := range products {
		barcode := fmt.Sprintf("4006381333%03d", i)
		product := testutil.CreateTestProduct(db, 1, 1)
		if err := db.Model(product).Update("barcode", barcode).Error; err != nil {
			t.Fatalf("seed product: %v", err)
		}
		datasets[barcode] = dbModel.Product{Barcode: barcode, PackSize: "1 l"}
	}
	getter := &stubDatasetGetter{datasets: datasets}
	controller := NewPackSizeController(db, getter, &logger)

	if err := controller.RunOnce(t.Context()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	var filled int64
	if err := db.Model(&dbModel.Product{}).Where("pack_size <> ''").Count(&filled).Error; err != nil {
		t.Fatalf("count the products with a pack size: %v", err)
	}
	if filled != products {
		t.Errorf("products with a pack size = %d, want %d (one pass covers the whole backlog)", filled, products)
	}
}

// A barcode the data source does not know is looked up once, not on every
// pass over the table: the walk starts over at the end of the table, and a
// barcode without a cooldown would be asked about again and again.
func TestPackSizeControllerKeepsUnknownBarcodesOnACooldown(t *testing.T) {
	logger := zerolog.Nop()
	db := testutil.SetupTestDB(t)
	product := testutil.CreateTestProduct(db, 1, 1)
	if err := db.Model(product).Update("barcode", "4006381333931").Error; err != nil {
		t.Fatalf("seed product: %v", err)
	}
	getter := &stubDatasetGetter{datasets: map[string]dbModel.Product{}}
	controller := NewPackSizeController(db, getter, &logger)

	// Three runs: the first walks the table, the second finds the table
	// walked to its end and starts over, the third walks it again.
	for i := range 3 {
		if err := controller.RunOnce(t.Context()); err != nil {
			t.Fatalf("RunOnce %d: %v", i, err)
		}
	}

	if getter.lookups != 1 {
		t.Errorf("live lookups = %d, want 1 (a known miss is not asked about again)", getter.lookups)
	}

	var stored dbModel.Product
	if err := db.First(&stored, product.ID).Error; err != nil {
		t.Fatalf("load product: %v", err)
	}
	if stored.PackSize != "" {
		t.Errorf("pack size = %q, want an empty string", stored.PackSize)
	}
}
