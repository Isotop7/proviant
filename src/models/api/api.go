package api

// BulkProductsAPIModel represents a bulk product operation request
type BulkProductsAPIModel struct {
	ProductIDs []uint `json:"productIDs"`
}

// CookItemAPIModel is one product entry of a cook request
type CookItemAPIModel struct {
	ProductID uint `json:"productId" binding:"required"`
	Amount    int  `json:"amount" binding:"gte=0"`
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

// ProductAmountDTO is the request body for updating a product's amount
type ProductAmountDTO struct {
	Delta int `json:"delta" binding:"required,min=-1000"`
}

// HouseholdListItem represents a household in the discovery list
type HouseholdListItem struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int    `json:"memberCount"`
}
