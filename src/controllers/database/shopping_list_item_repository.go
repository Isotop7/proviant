package database

import (
	"github.com/Isotop7/proviant/models/database"
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
	r.fillPackSizes([]*database.ShoppingListItem{&item})
	return &item, nil
}

func (r *ShoppingListItemRepositoryImpl) ListByHousehold(householdID uint) ([]database.ShoppingListItem, error) {
	var items []database.ShoppingListItem
	err := r.DB.Where("household_id = ?", householdID).Order("category ASC, name ASC").Find(&items).Error
	if err != nil {
		return nil, err
	}
	pointers := make([]*database.ShoppingListItem, len(items))
	for i := range items {
		pointers[i] = &items[i]
	}
	r.fillPackSizes(pointers)
	return items, nil
}

// fillPackSizes copies the pack size of the linked product onto every item
// that points at one. The size is display metadata of the product, so it is
// read with the items instead of being looked up by every consumer. A failed
// lookup leaves the size empty rather than failing the whole list.
func (r *ShoppingListItemRepositoryImpl) fillPackSizes(items []*database.ShoppingListItem) {
	productIDs := make([]uint, 0, len(items))
	seen := make(map[uint]bool, len(items))
	for _, item := range items {
		if item.ProductID == nil || seen[*item.ProductID] {
			continue
		}
		seen[*item.ProductID] = true
		productIDs = append(productIDs, *item.ProductID)
	}
	if len(productIDs) == 0 {
		return
	}
	var products []database.Product
	if err := r.DB.Select("id, pack_size").Where("id IN ?", productIDs).Find(&products).Error; err != nil {
		return
	}
	sizes := make(map[uint]string, len(products))
	for i := range products {
		if products[i].PackSize != "" {
			sizes[products[i].ID] = products[i].PackSize
		}
	}
	for _, item := range items {
		if item.ProductID == nil {
			continue
		}
		item.PackSize = sizes[*item.ProductID]
	}
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
