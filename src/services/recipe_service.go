package services

import (
	"errors"
	"fmt"
	"math"

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

// CookRecipe consumes the ingredients of a recipe. Every linked ingredient
// costs a pack: the stock is counted in packs, and a recipe states how much
// of something a dish needs, not how many packs of it that is. A cook for
// more servings than the recipe states needs more packs of every ingredient,
// so the requested servings scale the pack count. Free-text ingredients and
// missing products are reported as per-ingredient gaps and skipped;
// everything else is delegated to CookProducts. internalErr carries the first
// server-side failure so the handler can answer 5xx instead of blaming the
// client.
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

	items := make([]apiModel.CookItemAPIModel, 0, len(recipe.Ingredients))
	// A cook for more servings than the recipe states needs more packs of every
	// ingredient: the stock is counted in packs, so the ratio of the requested
	// servings to the recipe's own becomes the number of packs. The recipe's
	// ingredient amount stays display metadata and never re-enters the
	// arithmetic.
	packs := 1
	if servings > 0 && recipe.Servings > 0 && servings > recipe.Servings {
		packs = int(math.Ceil(float64(servings) / float64(recipe.Servings)))
	}
	for i := range recipe.Ingredients {
		ingredient := &recipe.Ingredients[i]
		if ingredient.ProductID == nil {
			errs = append(errs, fmt.Sprintf("%s: not linked to a product", ingredient.Name))
			continue
		}
		_, productErr := s.repos.Products.GetProductByID(*ingredient.ProductID, userID)
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
		items = append(items, apiModel.CookItemAPIModel{
			ProductID: *ingredient.ProductID,
			Quantity:  packs,
		})
	}

	if len(items) == 0 {
		return 0, 0, errs, internalErr
	}

	cooked, cookedPartial, cookErrs, cookInternalErr := s.products.CookProducts(items, userID)
	return cooked, cookedPartial, append(errs, cookErrs...), cookInternalErr
}
