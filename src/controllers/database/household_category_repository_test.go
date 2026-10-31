package database

import (
	"testing"

	"github.com/Isotop7/proviant/errors"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"
)

func TestHouseholdCategoryRepositoryListsTheCanonicalCategories(t *testing.T) {
	db := testutil.SetupTestDB(t)
	for _, row := range canonicalPriceRows() {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}
	household := testutil.CreateTestHousehold(db, 0)

	repo := NewHouseholdCategoryRepository(db)
	categories, err := repo.List(household.ID)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(categories) != len(canonicalPriceRows()) {
		t.Fatalf("List() returned %d categories, want %d", len(categories), len(canonicalPriceRows()))
	}
	if categories[0].CategoryKey != "dairy products" {
		t.Errorf("first category = %q, want dairy products", categories[0].CategoryKey)
	}
	if categories[0].DisplayName != "Dairy Products" {
		t.Errorf("display name = %q, want Dairy Products", categories[0].DisplayName)
	}
	if categories[0].Hidden {
		t.Error("a category without an override must not be hidden")
	}
}

// A household hides a category, it never removes the canonical row.
func TestHouseholdCategoryRepositoryKeepsTheCanonicalRows(t *testing.T) {
	db := testutil.SetupTestDB(t)
	for _, row := range canonicalPriceRows() {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}
	household := testutil.CreateTestHousehold(db, 0)
	hidden := true

	repo := NewHouseholdCategoryRepository(db)
	if _, err := repo.Upsert(household.ID, "milk", HouseholdCategoryOverride{Hidden: &hidden}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	var canonical dbModel.ProductCategoryPrice
	if err := db.Where("category_key = ?", "milk").First(&canonical).Error; err != nil {
		t.Fatalf("the canonical row is gone: %v", err)
	}
}

func TestHouseholdCategoryRepositoryUpsertStoresTheOverrides(t *testing.T) {
	db := testutil.SetupTestDB(t)
	for _, row := range canonicalPriceRows() {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}
	household := testutil.CreateTestHousehold(db, 0)
	shelfLife := 5
	daysAfterOpening := 3
	displayName := "Our milk"

	repo := NewHouseholdCategoryRepository(db)
	sortOrder := 2
	category, err := repo.Upsert(household.ID, "milk", HouseholdCategoryOverride{
		DisplayName:             &displayName,
		SortOrder:               &sortOrder,
		DefaultShelfLifeDays:    &shelfLife,
		DefaultDaysAfterOpening: &daysAfterOpening,
	})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if category.DisplayName != "Our milk" {
		t.Errorf("display name = %q, want Our milk", category.DisplayName)
	}
	if category.DefaultShelfLifeDays == nil || *category.DefaultShelfLifeDays != 5 {
		t.Errorf("shelf life = %v, want 5", category.DefaultShelfLifeDays)
	}
	if category.DefaultDaysAfterOpening == nil || *category.DefaultDaysAfterOpening != 3 {
		t.Errorf("days after opening = %v, want 3", category.DefaultDaysAfterOpening)
	}

	// A second write replaces the override instead of adding a second one.
	renamed := "Milk again"
	if _, err := repo.Upsert(household.ID, "milk", HouseholdCategoryOverride{DisplayName: &renamed}); err != nil {
		t.Fatalf("second Upsert() error = %v", err)
	}
	var overrides []dbModel.HouseholdCategory
	if err := db.Where("household_id = ?", household.ID).Find(&overrides).Error; err != nil {
		t.Fatalf("read overrides: %v", err)
	}
	if len(overrides) != 1 {
		t.Fatalf("stored %d overrides, want 1", len(overrides))
	}
	if overrides[0].DisplayName != "Milk again" {
		t.Errorf("display name = %q, want Milk again", overrides[0].DisplayName)
	}
	// A write that does not carry an order keeps the place the category had.
	if overrides[0].SortOrder != 2 {
		t.Errorf("sort order = %d, want 2", overrides[0].SortOrder)
	}
	// A write that does not carry a shelf life keeps the one the category had.
	if overrides[0].DefaultShelfLifeDays == nil || *overrides[0].DefaultShelfLifeDays != 5 {
		t.Errorf("shelf life = %v, want 5", overrides[0].DefaultShelfLifeDays)
	}
}

// A write that names no field leaves the stored values alone: a patch of the
// shelf life must not wipe the household's display name or unhide the
// category.
func TestHouseholdCategoryRepositoryUpsertKeepsTheUnmentionedValues(t *testing.T) {
	db := testutil.SetupTestDB(t)
	for _, row := range canonicalPriceRows() {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}
	household := testutil.CreateTestHousehold(db, 0)
	displayName := "Our milk"
	hidden := true
	shelfLife := 5

	repo := NewHouseholdCategoryRepository(db)
	if _, err := repo.Upsert(household.ID, "milk", HouseholdCategoryOverride{
		DisplayName:          &displayName,
		Hidden:               &hidden,
		DefaultShelfLifeDays: &shelfLife,
	}); err != nil {
		t.Fatalf("first Upsert() error = %v", err)
	}

	newShelfLife := 9
	if _, err := repo.Upsert(household.ID, "milk", HouseholdCategoryOverride{DefaultShelfLifeDays: &newShelfLife}); err != nil {
		t.Fatalf("second Upsert() error = %v", err)
	}

	var stored dbModel.HouseholdCategory
	if err := db.Where("household_id = ? AND category_key = ?", household.ID, "milk").First(&stored).Error; err != nil {
		t.Fatalf("read the stored override: %v", err)
	}
	if stored.DisplayName != "Our milk" {
		t.Errorf("display name = %q, want Our milk", stored.DisplayName)
	}
	if !stored.Hidden {
		t.Error("the category is no longer hidden")
	}
	if stored.DefaultShelfLifeDays == nil || *stored.DefaultShelfLifeDays != 9 {
		t.Errorf("shelf life = %v, want 9", stored.DefaultShelfLifeDays)
	}
}

// A shelf life of zero clears the stored one: a stored zero would read as a
// shelf life of zero days and expire the products of the category on the day
// they were bought.
func TestHouseholdCategoryRepositoryUpsertClearsTheShelfLifeOnZero(t *testing.T) {
	db := testutil.SetupTestDB(t)
	for _, row := range canonicalPriceRows() {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}
	household := testutil.CreateTestHousehold(db, 0)
	shelfLife := 5

	repo := NewHouseholdCategoryRepository(db)
	if _, err := repo.Upsert(household.ID, "milk", HouseholdCategoryOverride{DefaultShelfLifeDays: &shelfLife}); err != nil {
		t.Fatalf("first Upsert() error = %v", err)
	}

	cleared := 0
	if _, err := repo.Upsert(household.ID, "milk", HouseholdCategoryOverride{DefaultShelfLifeDays: &cleared}); err != nil {
		t.Fatalf("second Upsert() error = %v", err)
	}

	defaults, err := repo.Defaults(household.ID)
	if err != nil {
		t.Fatalf("Defaults() error = %v", err)
	}
	if _, ok := defaults["milk"]; ok {
		t.Errorf("the cleared shelf life is still a default: %v", defaults["milk"])
	}
}

// An empty display name falls back to the canonical one.
func TestHouseholdCategoryRepositoryFallsBackToTheCanonicalName(t *testing.T) {
	db := testutil.SetupTestDB(t)
	for _, row := range canonicalPriceRows() {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}
	household := testutil.CreateTestHousehold(db, 0)

	repo := NewHouseholdCategoryRepository(db)
	category, err := repo.Upsert(household.ID, "milk", HouseholdCategoryOverride{})
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if category.DisplayName != "Milk" {
		t.Errorf("display name = %q, want Milk", category.DisplayName)
	}
}
func TestHouseholdCategoryRepositoryRefusesAnUnknownCategory(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)

	repo := NewHouseholdCategoryRepository(db)
	if _, err := repo.Upsert(household.ID, "unicorns", HouseholdCategoryOverride{}); err != errors.ErrCategoryNotFound {
		t.Errorf("Upsert() error = %v, want ErrCategoryNotFound", err)
	}
}

func TestHouseholdCategoryRepositoryRefusesAnOutOfRangeShelfLife(t *testing.T) {
	db := testutil.SetupTestDB(t)
	for _, row := range canonicalPriceRows() {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}
	household := testutil.CreateTestHousehold(db, 0)

	repo := NewHouseholdCategoryRepository(db)
	tooLong := dbModel.MaxCategoryShelfLifeDays + 1
	if _, err := repo.Upsert(household.ID, "milk", HouseholdCategoryOverride{DefaultShelfLifeDays: &tooLong}); err != errors.ErrCategoryShelfLifeOutOfRange {
		t.Errorf("Upsert() error = %v, want ErrCategoryShelfLifeOutOfRange", err)
	}
	tooManyDays := dbModel.MaxDaysAfterOpening + 1
	if _, err := repo.Upsert(household.ID, "milk", HouseholdCategoryOverride{DefaultDaysAfterOpening: &tooManyDays}); err != errors.ErrCategoryDaysAfterOpeningOutOfRange {
		t.Errorf("Upsert() error = %v, want ErrCategoryDaysAfterOpeningOutOfRange", err)
	}
}

func TestHouseholdCategoryRepositoryReorderWritesTheOrder(t *testing.T) {
	db := testutil.SetupTestDB(t)
	for _, row := range canonicalPriceRows() {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}
	household := testutil.CreateTestHousehold(db, 0)

	repo := NewHouseholdCategoryRepository(db)
	if err := repo.Reorder(household.ID, []string{"milk", "cheese", "dairy products", "fresh vegetables", "rice"}); err != nil {
		t.Fatalf("Reorder() error = %v", err)
	}

	categories, err := repo.List(household.ID)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if categories[0].CategoryKey != "milk" {
		t.Errorf("first category = %q, want milk", categories[0].CategoryKey)
	}
	if categories[1].CategoryKey != "cheese" {
		t.Errorf("second category = %q, want cheese", categories[1].CategoryKey)
	}
}

func TestHouseholdCategoryRepositoryReorderRefusesAnUnknownCategory(t *testing.T) {
	db := testutil.SetupTestDB(t)
	household := testutil.CreateTestHousehold(db, 0)

	repo := NewHouseholdCategoryRepository(db)
	if err := repo.Reorder(household.ID, []string{"unicorns"}); err != errors.ErrCategoryNotFound {
		t.Errorf("Reorder() error = %v, want ErrCategoryNotFound", err)
	}
}

func TestHouseholdCategoryRepositoryDefaultsSkipTheCategoriesWithoutOne(t *testing.T) {
	db := testutil.SetupTestDB(t)
	for _, row := range canonicalPriceRows() {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}
	household := testutil.CreateTestHousehold(db, 0)
	shelfLife := 7

	repo := NewHouseholdCategoryRepository(db)
	if _, err := repo.Upsert(household.ID, "milk", HouseholdCategoryOverride{DefaultShelfLifeDays: &shelfLife}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}
	if _, err := repo.Upsert(household.ID, "cheese", HouseholdCategoryOverride{}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	defaults, err := repo.Defaults(household.ID)
	if err != nil {
		t.Fatalf("Defaults() error = %v", err)
	}
	if len(defaults) != 1 {
		t.Fatalf("Defaults() returned %d entries, want 1", len(defaults))
	}
	milk, ok := defaults["milk"]
	if !ok {
		t.Fatal("Defaults() has no entry for milk")
	}
	if milk.ShelfLifeDays == nil || *milk.ShelfLifeDays != 7 {
		t.Errorf("shelf life = %v, want 7", milk.ShelfLifeDays)
	}
}

// A household only ever sees its own overrides.
func TestHouseholdCategoryRepositoryKeepsHouseholdsApart(t *testing.T) {
	db := testutil.SetupTestDB(t)
	for _, row := range canonicalPriceRows() {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed canonical row: %v", err)
		}
	}
	first := testutil.CreateTestHousehold(db, 0)
	second := testutil.CreateTestHousehold(db, 0)

	repo := NewHouseholdCategoryRepository(db)
	displayName := "Our milk"
	if _, err := repo.Upsert(first.ID, "milk", HouseholdCategoryOverride{DisplayName: &displayName}); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	categories, err := repo.List(second.ID)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	for _, category := range categories {
		if category.CategoryKey == "milk" && category.DisplayName != "Milk" {
			t.Errorf("the second household sees %q, want the canonical name", category.DisplayName)
		}
	}
}
