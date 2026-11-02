package api

// BulkProductsAPIModel represents a bulk product operation request
type BulkProductsAPIModel struct {
	ProductIDs []uint `json:"productIDs"`
	// WasteCause is why the products are wasted. It applies to every product
	// of the request and is ignored by a bulk consume.
	WasteCause *string `json:"wasteCause,omitempty"`
}

// CookItemAPIModel is one product entry of a cook request
type CookItemAPIModel struct {
	ProductID uint `json:"productId" binding:"required"`
	Quantity  int  `json:"quantity" binding:"gte=0"`
}

// CookProductsAPIModel represents a cook request
type CookProductsAPIModel struct {
	Items []CookItemAPIModel `json:"items" binding:"required,min=1,max=100"`
}

// CookResponse represents the result of a cook request
type CookResponse struct {
	Consumed int      `json:"consumed"`
	Partial  int      `json:"partial"`
	Errors   []string `json:"errors"`
}

// OnboardingStateResponse represents the current onboarding progress
type OnboardingStateResponse struct {
	ProfileStepDone     bool `json:"profileStepDone"`
	NotificationsSetup  bool `json:"notificationsSetup"`
	HouseholdStepDone   bool `json:"householdStepDone"`
	OnboardingCompleted bool `json:"onboardingCompleted"`
}

// ProductQuantityDTO is the request body for updating a product's pack count
type ProductQuantityDTO struct {
	Delta            int     `json:"delta" binding:"required,min=-1000"`
	ClientMutationID *string `json:"clientMutationId,omitempty"`
}

// ProductActionDTO is the optional request body for consuming or wasting packs.
// A nil Quantity records the whole remaining stock.
type ProductActionDTO struct {
	Quantity         *int    `json:"quantity,omitempty"`
	ClientMutationID *string `json:"clientMutationId,omitempty"`
	// WasteCause is why the packs are wasted. It is ignored by a consume.
	WasteCause *string `json:"wasteCause,omitempty"`
}

// HouseholdListItem represents a household in the discovery list
type HouseholdListItem struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int    `json:"memberCount"`
}
