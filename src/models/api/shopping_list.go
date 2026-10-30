package api

// CreateShoppingListItemRequest is the request body for POST /api/v1/shopping-list.
type CreateShoppingListItemRequest struct {
	ProductID *uint  `json:"productId"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Quantity  int    `json:"quantity"`
	Notes     string `json:"notes"`
}

// UpdateShoppingListItemRequest is the request body for PATCH /api/v1/shopping-list/{id}.
type UpdateShoppingListItemRequest struct {
	Name     *string `json:"name"`
	Category *string `json:"category"`
	Quantity *int    `json:"quantity"`
	Checked  *bool   `json:"checked"`
	Notes    *string `json:"notes"`
}

// PurchaseShoppingListItemRequest is the request body for
// POST /api/v1/shopping-list/{id}/purchase.
type PurchaseShoppingListItemRequest struct {
	// Quantity is the number of packs bought. It defaults to the number of
	// packs the item asked for; the two are not the same number.
	Quantity *int `json:"quantity"`
	// ExpireAt is the expiry date the user confirmed, as "2006-01-02" or an
	// RFC 3339 timestamp. An empty value leaves the date to the household's
	// category default.
	ExpireAt *string `json:"expireAt"`
	// CategoryKey is the canonical category the bought product belongs to.
	// It defaults to the category of the item.
	CategoryKey *string `json:"categoryKey"`
	// StorageLocationID is where the bought packs go.
	StorageLocationID *uint `json:"storageLocationId"`
	// PackSize is the size of one pack. It defaults to the size of the
	// product the item points at.
	PackSize *string `json:"packSize"`
	// Barcode identifies the bought product. An empty barcode leaves the
	// product without one.
	Barcode *string `json:"barcode"`
	// Scanned says the barcode was scanned rather than typed.
	Scanned bool `json:"scanned"`
	// AddToProductID is the product the bought packs go into when it is not
	// the product the item points at: the caller resolved a same-item
	// conflict by hand.
	AddToProductID *uint `json:"addToProductId"`
	// AllowDuplicate writes a second product instead of refusing when the
	// bought product matches one the household already has.
	AllowDuplicate bool `json:"allowDuplicate"`
}

// PurchaseShoppingListItemResponse is the answer to a purchase: the item as
// it now stands and the product the purchase landed in.
type PurchaseShoppingListItemResponse struct {
	Item    ShoppingListItemResponse `json:"item"`
	Product *ProductResponse         `json:"product,omitempty"`
	// Created says the purchase created a product. It is false when the
	// bought packs went into the product the item already pointed at.
	Created bool `json:"created"`
}
