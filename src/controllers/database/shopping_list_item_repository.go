package database

import (
	"codeberg.org/isotop7/proviant/models/database"
	"gorm.io/gorm"
)

type ShoppingListItemRepository interface {
	Create(item *database.ShoppingListItem) error
	GetByID(id, householdID uint) (*database.ShoppingListItem, error)
	ListByHousehold(householdID uint) ([]database.ShoppingListItem, error)
	Update(item *database.ShoppingListItem) error
	Delete(id, householdID uint) error
	ToggleChecked(id, householdID uint) error
}

var _ ShoppingListItemRepository = (*ShoppingListItemRepositoryImpl)(nil)

type ShoppingListItemRepositoryImpl struct {
	DB *gorm.DB
}

func NewShoppingListItemRepository(db *gorm.DB) *ShoppingListItemRepositoryImpl {
	return &ShoppingListItemRepositoryImpl{DB: db}
}

func (r *ShoppingListItemRepositoryImpl) Create(item *database.ShoppingListItem) error {
	return r.DB.Create(item).Error
}

func (r *ShoppingListItemRepositoryImpl) GetByID(id, householdID uint) (*database.ShoppingListItem, error) {
	var item database.ShoppingListItem
	err := r.DB.Where("id = ? AND household_id = ?", id, householdID).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *ShoppingListItemRepositoryImpl) ListByHousehold(householdID uint) ([]database.ShoppingListItem, error) {
	var items []database.ShoppingListItem
	err := r.DB.Where("household_id = ?", householdID).Order("category ASC, name ASC").Find(&items).Error
	return items, err
}

func (r *ShoppingListItemRepositoryImpl) Update(item *database.ShoppingListItem) error {
	return r.DB.Save(item).Error
}

func (r *ShoppingListItemRepositoryImpl) Delete(id, householdID uint) error {
	return r.DB.Where("id = ? AND household_id = ?", id, householdID).Delete(&database.ShoppingListItem{}).Error
}

func (r *ShoppingListItemRepositoryImpl) ToggleChecked(id, householdID uint) error {
	return r.DB.Model(&database.ShoppingListItem{}).
		Where("id = ? AND household_id = ?", id, householdID).
		Update("checked", gorm.Expr("NOT checked")).Error
}
