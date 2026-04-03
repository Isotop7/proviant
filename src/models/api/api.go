package api

// BulkProductsAPIModel represents a bulk product operation request
type BulkProductsAPIModel struct {
	ProductIDs []string `json:"productIDs"`
}

// OnboardingStateResponse represents the current onboarding progress
type OnboardingStateResponse struct {
	NotificationsSetup  bool `json:"notificationsSetup"`
	HouseholdStepDone   bool `json:"householdStepDone"`
	OnboardingCompleted bool `json:"onboardingCompleted"`
}

// HouseholdListItem represents a household in the discovery list
type HouseholdListItem struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int    `json:"memberCount"`
}
