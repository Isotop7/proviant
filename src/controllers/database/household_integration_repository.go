package database

import (
	"errors"

	apperrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"gorm.io/gorm"
)

// HouseholdIntegrationRepository defines persistence operations for household shopping list integrations.
type HouseholdIntegrationRepository interface {
	GetByHouseholdAndProvider(householdID uint, provider string) (*database.HouseholdIntegration, error)
	ListByHousehold(householdID uint) ([]database.HouseholdIntegration, error)
	ListEnabled() ([]database.HouseholdIntegration, error)
	Create(integration *database.HouseholdIntegration) error
	Update(integration *database.HouseholdIntegration) error
	Upsert(integration *database.HouseholdIntegration) error
	Delete(householdID uint, provider string) error
}

var _ HouseholdIntegrationRepository = (*HouseholdIntegrationRepositoryImpl)(nil)

// HouseholdIntegrationRepositoryImpl implements HouseholdIntegrationRepository using GORM.
type HouseholdIntegrationRepositoryImpl struct {
	DB *gorm.DB
}

// NewHouseholdIntegrationRepository constructs a HouseholdIntegrationRepositoryImpl backed by db.
func NewHouseholdIntegrationRepository(db *gorm.DB) *HouseholdIntegrationRepositoryImpl {
	return &HouseholdIntegrationRepositoryImpl{DB: db}
}

func (r *HouseholdIntegrationRepositoryImpl) GetByHouseholdAndProvider(householdID uint, provider string) (*database.HouseholdIntegration, error) {
	var integration database.HouseholdIntegration
	err := r.DB.Where("household_id = ? AND provider = ?", householdID, provider).First(&integration).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrIntegrationNotFound
		}
		return nil, err
	}
	return &integration, nil
}

func (r *HouseholdIntegrationRepositoryImpl) ListByHousehold(householdID uint) ([]database.HouseholdIntegration, error) {
	var integrations []database.HouseholdIntegration
	err := r.DB.Where(util.QueryHouseholdId, householdID).Order("provider ASC").Find(&integrations).Error
	return integrations, err
}

func (r *HouseholdIntegrationRepositoryImpl) ListEnabled() ([]database.HouseholdIntegration, error) {
	var integrations []database.HouseholdIntegration
	err := r.DB.Where("enabled = ?", true).Order("household_id ASC, provider ASC").Find(&integrations).Error
	return integrations, err
}

func (r *HouseholdIntegrationRepositoryImpl) Create(integration *database.HouseholdIntegration) error {
	return r.DB.Create(integration).Error
}

func (r *HouseholdIntegrationRepositoryImpl) Update(integration *database.HouseholdIntegration) error {
	return r.DB.Save(integration).Error
}

func (r *HouseholdIntegrationRepositoryImpl) Upsert(integration *database.HouseholdIntegration) error {
	var existing database.HouseholdIntegration
	err := r.DB.Unscoped().Where("household_id = ? AND provider = ?", integration.HouseholdID, integration.Provider).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.DB.Create(integration).Error
		}
		return err
	}
	integration.ID = existing.ID
	integration.CreatedAt = existing.CreatedAt
	integration.DeletedAt = gorm.DeletedAt{}
	return r.DB.Unscoped().Save(integration).Error
}

func (r *HouseholdIntegrationRepositoryImpl) Delete(householdID uint, provider string) error {
	result := r.DB.Unscoped().Where("household_id = ? AND provider = ?", householdID, provider).Delete(&database.HouseholdIntegration{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrIntegrationNotFound
	}
	return nil
}

// ShoppingListItemSyncRefRepository defines persistence operations for shopping list item external references.
type ShoppingListItemSyncRefRepository interface {
	ListByHouseholdAndProvider(householdID uint, provider string) ([]database.ShoppingListItemSyncRef, error)
	Upsert(ref *database.ShoppingListItemSyncRef) error
	DeleteByID(id, householdID uint) error
	DeleteByExternalID(householdID uint, provider, externalID string) error
	DeleteByHouseholdAndProvider(householdID uint, provider string) error
}

var _ ShoppingListItemSyncRefRepository = (*ShoppingListItemSyncRefRepositoryImpl)(nil)

// ShoppingListItemSyncRefRepositoryImpl implements ShoppingListItemSyncRefRepository using GORM.
type ShoppingListItemSyncRefRepositoryImpl struct {
	DB *gorm.DB
}

// NewShoppingListItemSyncRefRepository constructs a ShoppingListItemSyncRefRepositoryImpl backed by db.
func NewShoppingListItemSyncRefRepository(db *gorm.DB) *ShoppingListItemSyncRefRepositoryImpl {
	return &ShoppingListItemSyncRefRepositoryImpl{DB: db}
}

func (r *ShoppingListItemSyncRefRepositoryImpl) ListByHouseholdAndProvider(householdID uint, provider string) ([]database.ShoppingListItemSyncRef, error) {
	var refs []database.ShoppingListItemSyncRef
	err := r.DB.Where("household_id = ? AND provider = ?", householdID, provider).Order("id ASC").Find(&refs).Error
	return refs, err
}

func (r *ShoppingListItemSyncRefRepositoryImpl) Upsert(ref *database.ShoppingListItemSyncRef) error {
	var existing database.ShoppingListItemSyncRef
	query := r.DB.Where("household_id = ? AND provider = ? AND external_id = ?", ref.HouseholdID, ref.Provider, ref.ExternalID)
	if ref.ShoppingListItemID > 0 {
		query = r.DB.Where("household_id = ? AND provider = ? AND (shopping_list_item_id = ? OR external_id = ?)",
			ref.HouseholdID, ref.Provider, ref.ShoppingListItemID, ref.ExternalID)
	}
	err := query.First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.DB.Create(ref).Error
		}
		return err
	}
	ref.ID = existing.ID
	ref.CreatedAt = existing.CreatedAt
	return r.DB.Save(ref).Error
}

func (r *ShoppingListItemSyncRefRepositoryImpl) DeleteByID(id, householdID uint) error {
	return r.DB.Unscoped().Where("id = ? AND household_id = ?", id, householdID).Delete(&database.ShoppingListItemSyncRef{}).Error
}

func (r *ShoppingListItemSyncRefRepositoryImpl) DeleteByExternalID(householdID uint, provider, externalID string) error {
	return r.DB.Unscoped().Where("household_id = ? AND provider = ? AND external_id = ?", householdID, provider, externalID).Delete(&database.ShoppingListItemSyncRef{}).Error
}

func (r *ShoppingListItemSyncRefRepositoryImpl) DeleteByHouseholdAndProvider(householdID uint, provider string) error {
	return r.DB.Unscoped().Where("household_id = ? AND provider = ?", householdID, provider).Delete(&database.ShoppingListItemSyncRef{}).Error
}
