package database

import (
	"sort"
	"strings"
	"time"

	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"gorm.io/gorm"
)

// HouseholdCategoryOverride is what a household changes about a canonical
// category. A nil field leaves the stored value alone; a zero shelf life
// clears it.
type HouseholdCategoryOverride struct {
	DisplayName             *string
	SortOrder               *int
	Hidden                  *bool
	DefaultShelfLifeDays    *int
	DefaultDaysAfterOpening *int
}

// HouseholdCategoryDefaults are the shelf lives a household gave a category.
type HouseholdCategoryDefaults struct {
	ShelfLifeDays    *int
	DaysAfterOpening *int
}

// ApplyHouseholdShelfLife fills the expiry date and the days-after-opening of
// a product from the household's default for its category. A date the request
// carried is never overwritten: the user may have corrected the suggestion.
//
// A category the household gave no shelf life for keeps the product without a
// date — a zero-day shelf life would expire the product on the day it was
// bought.
func ApplyHouseholdShelfLife(product *dbModel.Product, defaults map[string]HouseholdCategoryDefaults, categoryKey *string) {
	if product == nil || categoryKey == nil {
		return
	}
	defaultsForCategory, ok := defaults[*categoryKey]
	if !ok {
		return
	}
	if product.ExpireAt.IsZero() && defaultsForCategory.ShelfLifeDays != nil && *defaultsForCategory.ShelfLifeDays > 0 {
		product.ExpireAt = time.Now().AddDate(0, 0, *defaultsForCategory.ShelfLifeDays)
	}
	if product.DaysAfterOpening == nil && defaultsForCategory.DaysAfterOpening != nil && *defaultsForCategory.DaysAfterOpening > 0 {
		daysAfterOpening := *defaultsForCategory.DaysAfterOpening
		product.DaysAfterOpening = &daysAfterOpening
	}
}

type HouseholdCategoryRepositoryInterface interface {
	List(householdID uint) ([]dbModel.HouseholdCategoryView, error)
	ListByUser(userID uint) ([]dbModel.HouseholdCategoryView, error)
	Upsert(householdID uint, categoryKey string, override HouseholdCategoryOverride) (dbModel.HouseholdCategoryView, error)
	Reorder(householdID uint, categoryKeys []string) error
	Defaults(householdID uint) (map[string]HouseholdCategoryDefaults, error)
}

var _ HouseholdCategoryRepositoryInterface = (*HouseholdCategoryRepository)(nil)

// HouseholdCategoryRepository holds the household's overrides of the global
// canonical categories.
type HouseholdCategoryRepository struct {
	DB *gorm.DB
}

func NewHouseholdCategoryRepository(db *gorm.DB) *HouseholdCategoryRepository {
	return &HouseholdCategoryRepository{DB: db}
}

// canonicalCategories reads the canonical categories in the order they were
// seeded, which is the order a household's list falls back to.
func (r *HouseholdCategoryRepository) canonicalCategories() ([]dbModel.ProductCategoryPrice, error) {
	var prices []dbModel.ProductCategoryPrice
	if err := r.DB.Order("id ASC").Find(&prices).Error; err != nil {
		return nil, err
	}
	return prices, nil
}

// householdOverrides reads the overrides of one household.
func (r *HouseholdCategoryRepository) householdOverrides(householdID uint) (map[string]*dbModel.HouseholdCategory, error) {
	var overrides []dbModel.HouseholdCategory
	if err := r.DB.Where(util.QueryHouseholdId, householdID).Find(&overrides).Error; err != nil {
		return nil, err
	}
	byKey := make(map[string]*dbModel.HouseholdCategory, len(overrides))
	for i := range overrides {
		byKey[overrides[i].CategoryKey] = &overrides[i]
	}
	return byKey, nil
}

// List returns every canonical category with the household's overrides
// applied, ordered the way the household arranged them.
func (r *HouseholdCategoryRepository) List(householdID uint) ([]dbModel.HouseholdCategoryView, error) {
	prices, err := r.canonicalCategories()
	if err != nil {
		return nil, err
	}
	overrides, err := r.householdOverrides(householdID)
	if err != nil {
		return nil, err
	}

	categories := make([]dbModel.HouseholdCategoryView, 0, len(prices))
	canonicalIndex := make(map[string]int, len(prices))
	for index := range prices {
		canonical := prices[index]
		canonicalIndex[canonical.CategoryKey] = index
		category := dbModel.HouseholdCategoryView{
			CategoryKey: canonical.CategoryKey,
			DisplayName: canonical.DisplayName,
			SortOrder:   index,
		}
		if override, ok := overrides[canonical.CategoryKey]; ok {
			if override.DisplayName != "" {
				category.DisplayName = override.DisplayName
			}
			category.SortOrder = override.SortOrder
			category.Hidden = override.Hidden
			category.DefaultShelfLifeDays = override.DefaultShelfLifeDays
			category.DefaultDaysAfterOpening = override.DefaultDaysAfterOpening
		}
		categories = append(categories, category)
	}

	// The sort order decides, and the canonical order breaks a tie: a
	// category the household never touched keeps the place the seeded
	// taxonomy gave it.
	sort.SliceStable(categories, func(i, j int) bool {
		if categories[i].SortOrder != categories[j].SortOrder {
			return categories[i].SortOrder < categories[j].SortOrder
		}
		return canonicalIndex[categories[i].CategoryKey] < canonicalIndex[categories[j].CategoryKey]
	})
	return categories, nil
}

// ListByUser lists the categories of the household of a user.
func (r *HouseholdCategoryRepository) ListByUser(userID uint) ([]dbModel.HouseholdCategoryView, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}
	return r.List(user.HouseholdID)
}

// Upsert stores the household's override of one canonical category.
func (r *HouseholdCategoryRepository) Upsert(householdID uint, categoryKey string, override HouseholdCategoryOverride) (dbModel.HouseholdCategoryView, error) {
	canonical, canonicalIndex, err := r.canonicalCategory(categoryKey)
	if err != nil {
		return dbModel.HouseholdCategoryView{}, err
	}
	if err := validateShelfLife(override.DefaultShelfLifeDays, override.DefaultDaysAfterOpening); err != nil {
		return dbModel.HouseholdCategoryView{}, err
	}

	var stored dbModel.HouseholdCategory
	err = r.DB.Where(util.QueryHouseholdId, householdID).Where("category_key = ?", categoryKey).First(&stored).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return dbModel.HouseholdCategoryView{}, err
	}

	if err == gorm.ErrRecordNotFound {
		stored = dbModel.HouseholdCategory{
			HouseholdID: householdID,
			CategoryKey: categoryKey,
			SortOrder:   canonicalIndex,
		}
	}
	if override.DisplayName != nil {
		stored.DisplayName = strings.TrimSpace(*override.DisplayName)
	}
	if override.SortOrder != nil {
		stored.SortOrder = *override.SortOrder
	}
	if override.Hidden != nil {
		stored.Hidden = *override.Hidden
	}
	if override.DefaultShelfLifeDays != nil {
		stored.DefaultShelfLifeDays = shelfLifeOrNil(*override.DefaultShelfLifeDays)
	}
	if override.DefaultDaysAfterOpening != nil {
		stored.DefaultDaysAfterOpening = shelfLifeOrNil(*override.DefaultDaysAfterOpening)
	}

	if stored.ID == 0 {
		if err := r.DB.Create(&stored).Error; err != nil {
			return dbModel.HouseholdCategoryView{}, err
		}
	} else if err := r.DB.Save(&stored).Error; err != nil {
		return dbModel.HouseholdCategoryView{}, err
	}

	return dbModel.HouseholdCategoryView{
		CategoryKey:             canonical.CategoryKey,
		DisplayName:             displayName(&canonical, &stored),
		SortOrder:               stored.SortOrder,
		Hidden:                  stored.Hidden,
		DefaultShelfLifeDays:    stored.DefaultShelfLifeDays,
		DefaultDaysAfterOpening: stored.DefaultDaysAfterOpening,
	}, nil
}

// Reorder writes the household's order over the canonical categories.
func (r *HouseholdCategoryRepository) Reorder(householdID uint, categoryKeys []string) error {
	canonicalKeys := make(map[string]bool)
	prices, err := r.canonicalCategories()
	if err != nil {
		return err
	}
	for i := range prices {
		canonicalKeys[prices[i].CategoryKey] = true
	}

	return r.DB.Transaction(func(tx *gorm.DB) error {
		for index, key := range categoryKeys {
			if !canonicalKeys[key] {
				return errors.ErrCategoryNotFound
			}
			var stored dbModel.HouseholdCategory
			err := tx.Where(util.QueryHouseholdId, householdID).Where("category_key = ?", key).First(&stored).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}
			if err == gorm.ErrRecordNotFound {
				stored = dbModel.HouseholdCategory{HouseholdID: householdID, CategoryKey: key}
			}
			stored.SortOrder = index
			if stored.ID == 0 {
				if err := tx.Create(&stored).Error; err != nil {
					return err
				}
				continue
			}
			if err := tx.Save(&stored).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Defaults returns the shelf lives the household gave its categories.
func (r *HouseholdCategoryRepository) Defaults(householdID uint) (map[string]HouseholdCategoryDefaults, error) {
	overrides, err := r.householdOverrides(householdID)
	if err != nil {
		return nil, err
	}
	defaults := make(map[string]HouseholdCategoryDefaults, len(overrides))
	for key, override := range overrides {
		shelfLife := shelfLifeOrNilValue(override.DefaultShelfLifeDays)
		daysAfterOpening := shelfLifeOrNilValue(override.DefaultDaysAfterOpening)
		if shelfLife == nil && daysAfterOpening == nil {
			continue
		}
		defaults[key] = HouseholdCategoryDefaults{
			ShelfLifeDays:    shelfLife,
			DaysAfterOpening: daysAfterOpening,
		}
	}
	return defaults, nil
}

// canonicalCategory returns the canonical row of a key and the place it has
// in the seeded order.
func (r *HouseholdCategoryRepository) canonicalCategory(categoryKey string) (dbModel.ProductCategoryPrice, int, error) {
	var prices []dbModel.ProductCategoryPrice
	if err := r.DB.Order("id ASC").Find(&prices).Error; err != nil {
		return dbModel.ProductCategoryPrice{}, 0, err
	}
	for index := range prices {
		if prices[index].CategoryKey == categoryKey {
			return prices[index], index, nil
		}
	}
	return dbModel.ProductCategoryPrice{}, 0, errors.ErrCategoryNotFound
}

// validateShelfLife refuses a shelf life outside the bounds the model
// documents. A zero clears the stored value, so it never reaches the check.
func validateShelfLife(shelfLifeDays, daysAfterOpening *int) error {
	if shelfLifeDays != nil && *shelfLifeDays != 0 {
		if *shelfLifeDays < dbModel.MinCategoryShelfLifeDays || *shelfLifeDays > dbModel.MaxCategoryShelfLifeDays {
			return errors.ErrCategoryShelfLifeOutOfRange
		}
	}
	if daysAfterOpening != nil && *daysAfterOpening != 0 {
		if *daysAfterOpening < 1 || *daysAfterOpening > dbModel.MaxDaysAfterOpening {
			return errors.ErrCategoryDaysAfterOpeningOutOfRange
		}
	}
	return nil
}

// shelfLifeOrNil turns a cleared shelf life into no value at all. A stored
// zero would read as "a shelf life of zero days" and expire the products of
// the category on the day they were bought.
func shelfLifeOrNil(days int) *int {
	if days == 0 {
		return nil
	}
	value := days
	return &value
}

// shelfLifeOrNilValue drops a stored zero, which a household that cleared the
// shelf life before the zero was stored as "no value" may still carry.
func shelfLifeOrNilValue(days *int) *int {
	if days == nil || *days == 0 {
		return nil
	}
	return days
}

// displayName falls back to the canonical name when the household gave none.
func displayName(canonical *dbModel.ProductCategoryPrice, override *dbModel.HouseholdCategory) string {
	if override.DisplayName != "" {
		return override.DisplayName
	}
	return canonical.DisplayName
}
