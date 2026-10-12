package api

// CreateShoppingListItemRequest is the request body for POST /api/v1/shopping-list.
type CreateShoppingListItemRequest struct {
	ProductID *uint  `json:"productId"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Quantity  int    `json:"quantity"`
	Unit      string `json:"unit"`
	Notes     string `json:"notes"`
}

// UpdateShoppingListItemRequest is the request body for PATCH /api/v1/shopping-list/{id}.
type UpdateShoppingListItemRequest struct {
	Name     *string `json:"name"`
	Category *string `json:"category"`
	Quantity *int    `json:"quantity"`
	Unit     *string `json:"unit"`
	Checked  *bool   `json:"checked"`
	Notes    *string `json:"notes"`
}
