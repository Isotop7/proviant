package services

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/Isotop7/proviant/controllers/database"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// RecipeService turns saved recipes into bulk consumes.
type RecipeService struct {
	repos    *database.RepositoryContainer
	logger   *zerolog.Logger
	products *ProductService
}

func NewRecipeService(repos *database.RepositoryContainer, logger *zerolog.Logger) *RecipeService {
	return &RecipeService{repos: repos, logger: logger, products: NewProductService(repos, logger)}
}

// scaleIngredientAmount scales a recipe amount to the requested servings,
// rounding half up. baseServings <= 0 is treated as 1.
func scaleIngredientAmount(amount, servings, baseServings int) int {
	if baseServings <= 0 {
		baseServings = 1
	}
	if servings <= 0 {
		servings = baseServings
	}
	scaled := (int64(amount)*int64(servings) + int64(baseServings)/2) / int64(baseServings)
	if scaled < 0 {
		return 0
	}
	if scaled > int64(math.MaxInt) {
		return math.MaxInt
	}
	return int(scaled)
}

// CookRecipe consumes the ingredients of a recipe scaled to the requested
// servings (0 = the recipe's own serving count). Free-text ingredients, missing
// products and unit mismatches are reported as per-ingredient gaps and skipped;
// everything else is delegated to CookProducts. internalErr carries the first
// server-side failure so the handler can answer 5xx instead of blaming the client.
func (s *RecipeService) CookRecipe(recipeID uint, servings int, userID uint) (consumed, partial int, errs []string, internalErr error) {
	householdID, householdErr := s.repos.Users.GetUserHouseholdByID(userID)
	if householdErr != nil {
		s.logger.Error().Msgf("CookRecipe: could not resolve household for user %d: %s", userID, householdErr)
		return 0, 0, nil, householdErr
	}

	recipe, err := s.repos.Recipes.GetByID(recipeID, householdID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn().Msgf("CookRecipe: recipe %d not found", recipeID)
			return 0, 0, nil, err
		}
		s.logger.Error().Msgf("CookRecipe: recipe %d lookup failed: %s", recipeID, err)
		return 0, 0, nil, err
	}

	if servings <= 0 {
		servings = recipe.Servings
	}

	items := make([]apiModel.CookItemAPIModel, 0, len(recipe.Ingredients))
	for i := range recipe.Ingredients {
		ingredient := &recipe.Ingredients[i]
		if ingredient.ProductID == nil {
			errs = append(errs, fmt.Sprintf("%s: not linked to a product", ingredient.Name))
			continue
		}
		// A zero amount would mean "consume everything" to CookProducts; for a
		// recipe ingredient with no amount set that would silently archive the
		// whole product, so report it as a gap instead.
		if ingredient.Amount <= 0 {
			errs = append(errs, fmt.Sprintf("%s: no amount set", ingredient.Name))
			continue
		}
		product, productErr := s.repos.Products.GetProductByID(*ingredient.ProductID, userID)
		if productErr != nil {
			if errors.Is(productErr, gorm.ErrRecordNotFound) {
				errs = append(errs, fmt.Sprintf("%s: product not found", ingredient.Name))
				continue
			}
			s.logger.Error().Msgf("CookRecipe: product %d lookup failed: %s", *ingredient.ProductID, productErr)
			errs = append(errs, fmt.Sprintf("%s: product lookup failed", ingredient.Name))
			if internalErr == nil {
				internalErr = productErr
			}
			continue
		}
		if ingredient.Unit != "" && product.Unit != "" && !strings.EqualFold(ingredient.Unit, product.Unit) {
			errs = append(errs, fmt.Sprintf("%s: unit %s does not match product unit %s", ingredient.Name, ingredient.Unit, product.Unit))
			continue
		}
		items = append(items, apiModel.CookItemAPIModel{
			ProductID: *ingredient.ProductID,
			Amount:    scaleIngredientAmount(ingredient.Amount, servings, recipe.Servings),
		})
	}

	if len(items) == 0 {
		return 0, 0, errs, internalErr
	}

	cooked, cookedPartial, cookErrs, cookInternalErr := s.products.CookProducts(items, userID)
	return cooked, cookedPartial, append(errs, cookErrs...), cookInternalErr
}
