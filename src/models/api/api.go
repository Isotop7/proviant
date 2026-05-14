package api

// BulkProductsAPIModel represents a bulk product operation request
type BulkProductsAPIModel struct {
	ProductIDs []uint `json:"productIDs"`
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
	Delta int `json:"delta" binding:"required,min=0"`
}

// HouseholdListItem represents a household in the discovery list
type HouseholdListItem struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int    `json:"memberCount"`
}
