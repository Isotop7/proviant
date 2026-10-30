package database

import (
	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"
	"gorm.io/gorm"
)

type ShoppingListItemRepository interface {
	Create(item *database.ShoppingListItem) error
	GetByID(id, householdID uint) (*database.ShoppingListItem, error)
	ListByHousehold(householdID uint) ([]database.ShoppingListItem, error)
	Update(item *database.ShoppingListItem) error
	Delete(id, householdID uint) error
	ToggleChecked(id, householdID uint) error
	SetChecked(id, householdID uint, checked bool) error
	PurchaseWithProduct(item *database.ShoppingListItem, product *database.Product) error
	PurchaseIntoProduct(item *database.ShoppingListItem, productID uint, packs int) error
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

// SetChecked writes the checked state of an item without touching the rest of
// the row. A write that only needs the checked state must not carry the rest
// of the row with it: a stale copy would undo a purchase's product link.
func (r *ShoppingListItemRepositoryImpl) SetChecked(id, householdID uint, checked bool) error {
	return r.DB.Model(&database.ShoppingListItem{}).
		Where("id = ? AND household_id = ?", id, householdID).
		Update("checked", checked).Error
}

// PurchaseWithProduct creates the bought product and checks the item in one
// transaction: either both land, or the item stays unchecked and no product
// exists. The product belongs to the household of the item, so the link can
// never point at a product another household can see.
func (r *ShoppingListItemRepositoryImpl) PurchaseWithProduct(item *database.ShoppingListItem, product *database.Product) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		product.HouseholdID = item.HouseholdID
		product.Barcode = database.NormalizeBarcode(product.Barcode)
		product.PackSize = util.PackSizeLabel(product.PackSize)
		product.ProductName = util.ProductNameLabel(product.ProductName)
		product.Quantity = util.ClampProductQuantity(product.Quantity)
		if err := tx.Create(product).Error; err != nil {
			return err
		}
		itemResult := tx.Model(&database.ShoppingListItem{}).
			Where("id = ? AND household_id = ?", item.ID, item.HouseholdID).
			Updates(map[string]any{"checked": true, "product_id": product.ID})
		if itemResult.Error != nil {
			return itemResult.Error
		}
		// An item that was deleted since the caller read it takes the product
		// nowhere: the purchase must not leave a row no item points at.
		if itemResult.RowsAffected == 0 {
			return errors.ErrShoppingListItemNotFound
		}
		return nil
	})
}

// PurchaseIntoProduct adds the bought packs to the product the item already
// points at and checks the item in one transaction. No second row is written:
// the household already tracks this product, so the packs belong to it.
func (r *ShoppingListItemRepositoryImpl) PurchaseIntoProduct(item *database.ShoppingListItem, productID uint, packs int) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		// The count stays inside the range a product may hold. The bounds are
		// spelled as a CASE and not as MIN or LEAST because no scalar minimum
		// is portable across SQLite, MySQL and PostgreSQL.
		countResult := tx.Model(&database.Product{}).
			Where("id = ? AND household_id = ?", productID, item.HouseholdID).
			Update("quantity", gorm.Expr(
				"CASE WHEN quantity + ? < 0 THEN 0 WHEN quantity + ? > ? THEN ? ELSE quantity + ? END",
				packs, packs, util.MaxProductQuantity, util.MaxProductQuantity, packs))
		if countResult.Error != nil {
			return countResult.Error
		}
		// A product that was deleted or archived since the caller read it
		// takes the packs nowhere: the purchase must not check the item.
		if countResult.RowsAffected == 0 {
			return errors.ErrShoppingListItemProductGone
		}
		itemResult := tx.Model(&database.ShoppingListItem{}).
			Where("id = ? AND household_id = ?", item.ID, item.HouseholdID).
			Updates(map[string]any{"checked": true, "product_id": productID})
		if itemResult.Error != nil {
			return itemResult.Error
		}
		// An item that was deleted since the caller read it takes the packs
		// nowhere: the count must not move either.
		if itemResult.RowsAffected == 0 {
			return errors.ErrShoppingListItemNotFound
		}
		return nil
	})
}
