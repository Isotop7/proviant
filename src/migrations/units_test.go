package migrations

import (
	"testing"

	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func TestNormalizeStoredUnits(t *testing.T) {
	db := testutil.SetupTestDB(t)

	household := testutil.CreateTestHousehold(db, 0)
	user := testutil.CreateTestUser(db, household.ID)

	kilogramProduct := testutil.CreateTestProduct(db, household.ID, user.ID)
	seedAmountUnit(t, db, kilogramProduct, 2, "kg")
	litreProduct := testutil.CreateTestProduct(db, household.ID, user.ID)
	seedAmountUnit(t, db, litreProduct, 1, "L")
	countProduct := testutil.CreateTestProduct(db, household.ID, user.ID)
	seedAmountUnit(t, db, countProduct, 3, "pkg")
	baseProduct := testutil.CreateTestProduct(db, household.ID, user.ID)
	seedAmountUnit(t, db, baseProduct, 500, "g")
	// A pre-normalisation row typed with the shift key held: its unit must be
	// treated as an unnormalised storage symbol, not skipped as one.
	caseVariantProduct := testutil.CreateTestProduct(db, household.ID, user.ID)
	seedAmountUnit(t, db, caseVariantProduct, 2, "KG")

	recipe := database.Recipe{HouseholdID: household.ID, Name: "Cake", Servings: 1, CreatedBy: user.ID}
	if err := db.Create(&recipe).Error; err != nil {
		t.Fatalf("seeding recipe: %v", err)
	}
	ounceIngredient := database.RecipeIngredient{RecipeID: recipe.ID, Name: "Sugar", Amount: 4, Unit: "oz"}
	if err := db.Create(&ounceIngredient).Error; err != nil {
		t.Fatalf("seeding ounce ingredient: %v", err)
	}
	shoppingItem := database.ShoppingListItem{HouseholdID: household.ID, Name: "Milk", Quantity: 2, Unit: "l"}
	if err := db.Create(&shoppingItem).Error; err != nil {
		t.Fatalf("seeding shopping list item: %v", err)
	}

	logger := zerolog.Nop()
	if err := NormalizeStoredUnits(&logger, db); err != nil {
		t.Fatalf("NormalizeStoredUnits() error = %v", err)
	}

	assertProductAmountUnit(t, db, kilogramProduct.ID, 2000, "g")
	assertProductAmountUnit(t, db, litreProduct.ID, 1000, "ml")
	assertProductAmountUnit(t, db, countProduct.ID, 3, "pkg")
	assertProductAmountUnit(t, db, baseProduct.ID, 500, "g")
	assertProductAmountUnit(t, db, caseVariantProduct.ID, 2000, "g")
	assertIngredientAmountUnit(t, db, ounceIngredient.ID, 113, "g")
	assertShoppingItemQuantityUnit(t, db, shoppingItem.ID, 2000, "ml")

	if err := NormalizeStoredUnits(&logger, db); err != nil {
		t.Fatalf("second NormalizeStoredUnits() error = %v", err)
	}
	assertProductAmountUnit(t, db, kilogramProduct.ID, 2000, "g")
	assertIngredientAmountUnit(t, db, ounceIngredient.ID, 113, "g")
	assertShoppingItemQuantityUnit(t, db, shoppingItem.ID, 2000, "ml")
}

func seedAmountUnit(t *testing.T, db *gorm.DB, product *database.Product, amount int, unit string) {
	t.Helper()
	if err := db.Model(product).UpdateColumns(map[string]any{"amount": amount, "unit": unit}).Error; err != nil {
		t.Fatalf("seeding product amount/unit: %v", err)
	}
}

func assertProductAmountUnit(t *testing.T, db *gorm.DB, id uint, wantAmount int, wantUnit string) {
	t.Helper()
	var row database.Product
	if err := db.First(&row, id).Error; err != nil {
		t.Fatalf("reloading product %d: %v", id, err)
	}
	if row.Amount != wantAmount || row.Unit != wantUnit {
		t.Errorf("product %d = %d %q, want %d %q", id, row.Amount, row.Unit, wantAmount, wantUnit)
	}
}

func assertIngredientAmountUnit(t *testing.T, db *gorm.DB, id uint, wantAmount int, wantUnit string) {
	t.Helper()
	var row database.RecipeIngredient
	if err := db.First(&row, id).Error; err != nil {
		t.Fatalf("reloading recipe ingredient %d: %v", id, err)
	}
	if row.Amount != wantAmount || row.Unit != wantUnit {
		t.Errorf("recipe ingredient %d = %d %q, want %d %q", id, row.Amount, row.Unit, wantAmount, wantUnit)
	}
}

func assertShoppingItemQuantityUnit(t *testing.T, db *gorm.DB, id uint, wantQuantity int, wantUnit string) {
	t.Helper()
	var row database.ShoppingListItem
	if err := db.First(&row, id).Error; err != nil {
		t.Fatalf("reloading shopping list item %d: %v", id, err)
	}
	if row.Quantity != wantQuantity || row.Unit != wantUnit {
		t.Errorf("shopping list item %d = %d %q, want %d %q", id, row.Quantity, row.Unit, wantQuantity, wantUnit)
	}
}

// TestUnitScanPredicateIsCaseSensitiveOnMySQL pins the dialect split: a plain
// NOT IN is correct on SQLite and PostgreSQL but silently skips case-variant
// storage units under MySQL/MariaDB's default case-insensitive collations, so
// the MySQL predicate must force byte comparison.
func TestUnitScanPredicateIsCaseSensitiveOnMySQL(t *testing.T) {
	if got := unitScanPredicate("mysql"); got == unitScanPredicate("sqlite") {
		t.Errorf("mysql predicate = %q, want a case-sensitive variant distinct from %q", got, unitScanPredicate("sqlite"))
	}
	if got := unitScanPredicate("mysql"); got != "unit <> '' AND CAST(unit AS BINARY) NOT IN ?" {
		t.Errorf("mysql predicate = %q, want binary-cast comparison", got)
	}
}
