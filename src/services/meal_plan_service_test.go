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

	t.Run("asks for one pack per planned meal, subtracts the stock and adds free-text items", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Flour", "quantity": 1})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Bread",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Flour", Portion: "250 g", SortOrder: 0},
				{Name: "Salt", Portion: "5 g", SortOrder: 1},
			},
		}
		if err := db.Create(recipe).Error; err != nil {
			t.Fatalf("create recipe: %v", err)
		}
		// Every planned meal asks for one pack of the ingredient, the same
		// number a cook of it takes.
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
		if flour, ok := byName["Flour"]; !ok || flour.Quantity != 1 {
			t.Errorf("flour item = %+v, want quantity 1 (two packs needed - one pack in stock)", flour)
		}
		if salt, ok := byName["Salt"]; !ok || salt.Quantity != 2 {
			t.Errorf("salt item = %+v, want quantity 2 (one pack per planned meal)", salt)
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
				{ProductID: &product.ID, Name: "Flour", Portion: "250 g", SortOrder: 0},
				{Name: "Salt", Portion: "5 g", SortOrder: 1},
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

	t.Run("the stock in packs covers the need", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Cream", "quantity": 2})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Soup",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Cream", Portion: "200 g", SortOrder: 0},
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
			t.Fatalf("created = %d, want 0 (two packs in stock cover the one planned use)", created)
		}
	})

	t.Run("aggregates one product across its ingredients", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Flour", "quantity": 1})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Bread",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Flour", Portion: "250 g", SortOrder: 0},
				{ProductID: &product.ID, Name: "Flour", Portion: "1 kg", SortOrder: 1},
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
		if items[0].Quantity != 1 {
			t.Errorf("item = %+v, want quantity 1 (two ingredients - one pack in stock)", items[0])
		}
	})

	t.Run("aggregates the ingredients of one product into one need", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Flour", "quantity": 1})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Bread",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Flour", Portion: "2", SortOrder: 0},
				{ProductID: &product.ID, Name: "Flour", Portion: "500 g", SortOrder: 1},
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
		if items[0].Quantity != 1 {
			t.Errorf("item = %+v, want quantity 1 (two ingredients - one pack in stock)", items[0])
		}
	})

	t.Run("subtracts the packs in stock from the need", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Flour", "quantity": 1})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Bread",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Flour", Portion: "2500 g", SortOrder: 0},
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
			t.Fatalf("created = %d, want 0 (one pack in stock covers the planned use)", created)
		}
	})

	t.Run("deducts the product in stock from the need", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Eggs", "quantity": 2})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Pancakes",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Eggs", Portion: "5", SortOrder: 0},
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
			t.Fatalf("created = %d, want 0 (two packs in stock cover the planned use)", created)
		}
	})

	t.Run("needs covered by stock are skipped", func(t *testing.T) {
		db, svc, householdID, userID := setup(t)
		product := testutil.CreateTestProduct(db, householdID, userID)
		db.Model(product).Updates(map[string]any{"product_name": "Flour", "quantity": 2})
		recipe := &dbModel.Recipe{
			HouseholdID: householdID,
			Name:        "Bread",
			Servings:    2,
			CreatedBy:   userID,
			Ingredients: []dbModel.RecipeIngredient{
				{ProductID: &product.ID, Name: "Flour", Portion: "250 g", SortOrder: 0},
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
