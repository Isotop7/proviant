package services

import (
	"testing"
	"time"

	"github.com/Isotop7/proviant/controllers/database"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/testutil"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func newTestMealPlanService(db *gorm.DB) *MealPlanService {
	logger := zerolog.Nop()
	repos := database.NewRepositoryContainer(db, nil)
	return NewMealPlanService(repos, &logger)
}

func TestMealPlanService_GenerateShoppingList(t *testing.T) {
	setup := func(t *testing.T) (*gorm.DB, *MealPlanService, uint, uint) {
		t.Helper()
		db := testutil.SetupTestDB(t)
		svc := newTestMealPlanService(db)
		household := testutil.CreateTestHousehold(db, 1)
		user := testutil.CreateTestUser(db, household.ID)
		return db, svc, household.ID, user.ID
	}

	weekStart := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	weekEnd := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)

	t.Run("aggregates scaled ingredients, subtracts stock and adds free-text items", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Flour", "amount": 100, "unit": "g"})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Bread",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Flour", Amount: 250, Unit: "g", SortOrder: 0},
				{Name: "Salt", Amount: 5, Unit: "g", SortOrder: 1},
			},
		}
		if err := db.Create(recipe).Error; err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		testutil.CreateTestMealPlanEntry(db, householdID, recipe.ID, weekStart, 2, userID)
		testutil.CreateTestMealPlanEntry(db, householdID, recipe.ID, weekStart.Add(24*time.Hour), 4, userID)

		created, err := svc.GenerateShoppingList(weekStart, weekEnd, userID)
		if err != nil {
			t.Fatalf("GenerateShoppingList() error = %v", err)
		}
		if created != 2 {
			t.Fatalf("created = %d, want 2", created)
		}

		items, err := database.NewShoppingListItemRepository(db).ListByHousehold(householdID)
		if err != nil {
			t.Fatalf("list shopping items: %v", err)
		}
		byName := make(map[string]dbModel.ShoppingListItem)
		for i := range items {
			byName[items[i].Name] = items[i]
		}
		if flour, ok := byName["Flour"]; !ok || flour.Quantity != 650 {
			t.Errorf("flour item = %+v, want quantity 650 (250 + 500 planned - 100 in stock)", flour)
		}
		if salt, ok := byName["Salt"]; !ok || salt.Quantity != 15 {
			t.Errorf("salt item = %+v, want quantity 15 (5 + 10 planned)", salt)
		}
	})

	t.Run("skips needs already on the shopping list", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Bread",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Flour", Amount: 250, Unit: "g", SortOrder: 0},
				{Name: "Salt", Amount: 5, Unit: "g", SortOrder: 1},
			},
		}
		if err := db.Create(recipe).Error; err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		testutil.CreateTestMealPlanEntry(db, householdID, recipe.ID, weekStart, 2, userID)
		shoppingRepo := database.NewShoppingListItemRepository(db)
		if err := shoppingRepo.Create(&dbModel.ShoppingListItem{HouseholdID: householdID, ProductID: &product.ID, Name: "Flour", CreatedBy: userID}); err != nil {
			t.Fatalf("seed shopping item: %v", err)
		}
		if err := shoppingRepo.Create(&dbModel.ShoppingListItem{HouseholdID: householdID, Name: "SALT", CreatedBy: userID}); err != nil {
			t.Fatalf("seed shopping item: %v", err)
		}

		created, err := svc.GenerateShoppingList(weekStart, weekEnd, userID)
		if err != nil {
			t.Fatalf("GenerateShoppingList() error = %v", err)
		}
		if created != 0 {
			t.Errorf("created = %d, want 0 (everything already listed)", created)
		}
	})

	t.Run("keeps the full amount when units do not match", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Cream", "amount": 100, "unit": "ml"})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Soup",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Cream", Amount: 200, Unit: "g", SortOrder: 0},
			},
		}
		if err := db.Create(recipe).Error; err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		testutil.CreateTestMealPlanEntry(db, householdID, recipe.ID, weekStart, 2, userID)

		created, err := svc.GenerateShoppingList(weekStart, weekEnd, userID)
		if err != nil {
			t.Fatalf("GenerateShoppingList() error = %v", err)
		}
		if created != 1 {
			t.Fatalf("created = %d, want 1", created)
		}
		items, err := database.NewShoppingListItemRepository(db).ListByHousehold(householdID)
		if err != nil {
			t.Fatalf("list shopping items: %v", err)
		}
		if items[0].Quantity != 200 || items[0].Unit != "g" {
			t.Errorf("item = %+v, want quantity 200 in g (no stock subtraction)", items[0])
		}
	})

	t.Run("aggregates one product across mixed units", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Flour", "amount": 100, "unit": "g"})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Bread",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Flour", Amount: 250, Unit: "g", SortOrder: 0},
				{ProductID: &product.ID, Name: "Flour", Amount: 1, Unit: "kg", SortOrder: 1},
			},
		}
		if err := db.Create(recipe).Error; err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		testutil.CreateTestMealPlanEntry(db, householdID, recipe.ID, weekStart, 2, userID)

		created, err := svc.GenerateShoppingList(weekStart, weekEnd, userID)
		if err != nil {
			t.Fatalf("GenerateShoppingList() error = %v", err)
		}
		if created != 1 {
			t.Fatalf("created = %d, want 1 (one aggregated need)", created)
		}
		items, err := database.NewShoppingListItemRepository(db).ListByHousehold(householdID)
		if err != nil {
			t.Fatalf("list shopping items: %v", err)
		}
		if items[0].Quantity != 1150 || items[0].Unit != "g" {
			t.Errorf("item = %+v, want quantity 1150 in g (250 + 1000 planned - 100 in stock)", items[0])
		}
	})

	t.Run("keeps a unit-less ingredient separate from a united one", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Flour", "amount": 100, "unit": "g"})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Bread",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Flour", Amount: 2, Unit: "", SortOrder: 0},
				{ProductID: &product.ID, Name: "Flour", Amount: 500, Unit: "g", SortOrder: 1},
			},
		}
		if err := db.Create(recipe).Error; err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		testutil.CreateTestMealPlanEntry(db, householdID, recipe.ID, weekStart, 2, userID)

		created, err := svc.GenerateShoppingList(weekStart, weekEnd, userID)
		if err != nil {
			t.Fatalf("GenerateShoppingList() error = %v", err)
		}
		if created != 2 {
			t.Fatalf("created = %d, want 2 (one unit-less, one gram need)", created)
		}
		items, err := database.NewShoppingListItemRepository(db).ListByHousehold(householdID)
		if err != nil {
			t.Fatalf("list shopping items: %v", err)
		}
		var unitless, grams *dbModel.ShoppingListItem
		for i := range items {
			if items[i].Unit == "" {
				unitless = &items[i]
			} else {
				grams = &items[i]
			}
		}
		if unitless == nil || unitless.Quantity != 2 {
			t.Errorf("unit-less item = %+v, want quantity 2 without a unit", unitless)
		}
		if grams == nil || grams.Quantity != 400 || grams.Unit != "g" {
			t.Errorf("gram item = %+v, want 400 g (500 planned - 100 in stock)", grams)
		}
	})

	// A product row carrying a non-base unit must not drag the created
	// shopping list item out of base units: the stock is converted instead.
	t.Run("converts stock in a non-base unit into the need's base unit", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Flour", "amount": 1, "unit": "kg"})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Bread",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Flour", Amount: 2500, Unit: "g", SortOrder: 0},
			},
		}
		if err := db.Create(recipe).Error; err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		testutil.CreateTestMealPlanEntry(db, householdID, recipe.ID, weekStart, 2, userID)

		created, err := svc.GenerateShoppingList(weekStart, weekEnd, userID)
		if err != nil {
			t.Fatalf("GenerateShoppingList() error = %v", err)
		}
		if created != 1 {
			t.Fatalf("created = %d, want 1", created)
		}
		items, err := database.NewShoppingListItemRepository(db).ListByHousehold(householdID)
		if err != nil {
			t.Fatalf("list shopping items: %v", err)
		}
		if items[0].Quantity != 1500 || items[0].Unit != "g" {
			t.Errorf("item = %+v, want 1500 g (2500 planned - 1 kg in stock, stored in base units)", items[0])
		}
	})

	t.Run("deducts a unit-less product from a unit-less need", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Eggs", "amount": 2, "unit": ""})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Pancakes",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Eggs", Amount: 5, Unit: "", SortOrder: 0},
			},
		}
		if err := db.Create(recipe).Error; err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		testutil.CreateTestMealPlanEntry(db, householdID, recipe.ID, weekStart, 2, userID)

		created, err := svc.GenerateShoppingList(weekStart, weekEnd, userID)
		if err != nil {
			t.Fatalf("GenerateShoppingList() error = %v", err)
		}
		if created != 1 {
			t.Fatalf("created = %d, want 1", created)
		}
		items, err := database.NewShoppingListItemRepository(db).ListByHousehold(householdID)
		if err != nil {
			t.Fatalf("list shopping items: %v", err)
		}
		if items[0].Quantity != 3 || items[0].Unit != "" {
			t.Errorf("item = %+v, want quantity 3 without a unit (5 planned - 2 in stock)", items[0])
		}
	})

	t.Run("needs covered by stock are skipped", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Flour", "amount": 500, "unit": "g"})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Bread",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Flour", Amount: 250, Unit: "g", SortOrder: 0},
			},
		}
		if err := db.Create(recipe).Error; err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		testutil.CreateTestMealPlanEntry(db, householdID, recipe.ID, weekStart, 2, userID)

		created, err := svc.GenerateShoppingList(weekStart, weekEnd, userID)
		if err != nil {
			t.Fatalf("GenerateShoppingList() error = %v", err)
		}
		if created != 0 {
			t.Errorf("created = %d, want 0 (stock covers the need)", created)
		}
	})
}
