package services

import (
	"strings"
	"time"

	"github.com/Isotop7/proviant/controllers/database"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/rs/zerolog"
)

// MealPlanService turns planned meals into shopping list entries.
type MealPlanService struct {
	repos  *database.RepositoryContainer
	logger *zerolog.Logger
}

func NewMealPlanService(repos *database.RepositoryContainer, logger *zerolog.Logger) *MealPlanService {
	return &MealPlanService{repos: repos, logger: logger}
}

type ingredientKey struct {
	productID uint
	name      string
}

type ingredientNeed struct {
	key       ingredientKey
	productID *uint
	name      string
	quantity  int
}

// ingredientAggregationKey keys a need by product, or by name for an
// ingredient that is not linked to stock.
func ingredientAggregationKey(ingredient *dbModel.RecipeIngredient) ingredientKey {
	if ingredient.ProductID != nil {
		return ingredientKey{productID: *ingredient.ProductID}
	}
	return ingredientKey{name: strings.ToLower(ingredient.Name)}
}

// GenerateShoppingList aggregates the ingredients of every meal planned within
// [from, to] and creates shopping list entries for what is still missing. A
// need counts packs, not portions: every planned use of an ingredient asks
// for the packs the scaled servings need, and the stock on hand is subtracted
// in packs. Needs already on the list are skipped. Returns the number of created entries.
func (s *MealPlanService) GenerateShoppingList(from, to time.Time, userID uint) (int, error) {
	householdID, err := s.repos.Users.GetUserHouseholdByID(userID)
	if err != nil {
		s.logger.Error().Msgf("GenerateShoppingList: could not resolve household for user %d: %s", userID, err)
		return 0, err
	}

	entries, err := s.repos.MealPlans.ListByRange(householdID, from, to)
	if err != nil {
		s.logger.Error().Msgf("GenerateShoppingList: could not load meal plan entries: %s", err)
		return 0, err
	}

	recipes := make(map[uint]*dbModel.Recipe)
	needs := make(map[ingredientKey]*ingredientNeed)
	order := make([]ingredientKey, 0)
	for i := range entries {
		entry := &entries[i]
		recipe, loaded := recipes[entry.RecipeID]
		if !loaded {
			recipe, err = s.repos.Recipes.GetByID(entry.RecipeID, householdID)
			if err != nil {
				s.logger.Warn().Msgf("GenerateShoppingList: skipping recipe %d: %s", entry.RecipeID, err)
				continue
			}
			recipes[entry.RecipeID] = recipe
		}
		for j := range recipe.Ingredients {
			ingredient := &recipe.Ingredients[j]
			key := ingredientAggregationKey(ingredient)
			need, known := needs[key]
			if !known {
				need = &ingredientNeed{key: key, productID: ingredient.ProductID, name: ingredient.Name}
				needs[key] = need
				order = append(order, key)
			}
			// A planned meal asks for one pack of the ingredient, the same
			// number a cook of it takes.
			need.quantity++
		}
	}

	existingItems, err := s.repos.ShoppingListItems.ListByHousehold(householdID)
	if err != nil {
		s.logger.Error().Msgf("GenerateShoppingList: could not load shopping list: %s", err)
		return 0, err
	}
	existingProductIDs := make(map[uint]bool)
	existingNames := make(map[string]bool)
	for i := range existingItems {
		if existingItems[i].ProductID != nil {
			existingProductIDs[*existingItems[i].ProductID] = true
		}
		existingNames[strings.ToLower(existingItems[i].Name)] = true
	}

	created := 0
	for _, key := range order {
		need := needs[key]
		if need.quantity <= 0 {
			continue
		}
		category := ""
		if need.productID != nil {
			if existingProductIDs[*need.productID] {
				continue
			}
			product, productErr := s.repos.Products.GetProductByID(*need.productID, userID)
			if productErr != nil {
				s.logger.Warn().Msgf("GenerateShoppingList: product %d not readable: %s", *need.productID, productErr)
			} else {
				need.name = product.ProductName
				category = product.Categories
				need.quantity -= product.Quantity
				if need.quantity <= 0 {
					continue
				}
			}
		} else if existingNames[strings.ToLower(need.name)] {
			continue
		}

		item := &dbModel.ShoppingListItem{
			HouseholdID: householdID,
			ProductID:   need.productID,
			Name:        need.name,
			Category:    category,
			Quantity:    need.quantity,
			CreatedBy:   userID,
		}
		if createErr := s.repos.ShoppingListItems.Create(item); createErr != nil {
			s.logger.Warn().Msgf("GenerateShoppingList: creating item %s: %s", item.Name, createErr)
			continue
		}
		created++
	}
	return created, nil
}
