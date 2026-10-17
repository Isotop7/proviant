package api

import (
	"time"

	dbModel "github.com/Isotop7/proviant/models/database"
)

// HouseholdIntegrationResponse is the DTO returned for a household's shopping list provider integration.
type HouseholdIntegrationResponse struct {
	ID                     uint       `json:"id"`
	Provider               string     `json:"provider"`
	Enabled                bool       `json:"enabled"`
	AccessTokenConfigured  bool       `json:"accessTokenConfigured"`
	RefreshTokenConfigured bool       `json:"refreshTokenConfigured"`
	AccountEmail           string     `json:"accountEmail,omitempty"`
	ListID                 string     `json:"listId,omitempty"`
	ListName               string     `json:"listName,omitempty"`
	LastSyncAt             *time.Time `json:"lastSyncAt,omitempty"`
	LastError              string     `json:"lastError,omitempty"`
}

// NewHouseholdIntegrationResponse builds a HouseholdIntegrationResponse from a database model, omitting secret tokens.
func NewHouseholdIntegrationResponse(integration *dbModel.HouseholdIntegration) HouseholdIntegrationResponse {
	if integration == nil {
		return HouseholdIntegrationResponse{}
	}
	return HouseholdIntegrationResponse{
		ID:                     integration.ID,
		Provider:               integration.Provider,
		Enabled:                integration.Enabled,
		AccessTokenConfigured:  integration.AccessToken != "",
		RefreshTokenConfigured: integration.RefreshToken != "",
		AccountEmail:           integration.AccountEmail,
		ListID:                 integration.ListID,
		ListName:               integration.ListName,
		LastSyncAt:             integration.LastSyncAt,
		LastError:              integration.LastError,
	}
}

// NewHouseholdIntegrationResponses converts a slice of HouseholdIntegration models into response DTOs.
func NewHouseholdIntegrationResponses(integrations []dbModel.HouseholdIntegration) []HouseholdIntegrationResponse {
	responses := make([]HouseholdIntegrationResponse, len(integrations))
	for i := range integrations {
		responses[i] = NewHouseholdIntegrationResponse(&integrations[i])
	}
	return responses
}

// CreateHouseholdIntegrationRequest is the request body for POST /api/v1/household/integrations/{provider}.
type CreateHouseholdIntegrationRequest struct {
	Enabled          bool   `json:"enabled"`
	AccessToken      string `json:"accessToken"`
	ClearAccessToken bool   `json:"clearAccessToken"`
	RefreshToken     string `json:"refreshToken"`
	AccountEmail     string `json:"accountEmail"`
	ListID           string `json:"listId"`
	ListName         string `json:"listName"`
}

// UpdateHouseholdIntegrationRequest is the request body for PATCH /api/v1/household/integrations/{provider}.
type UpdateHouseholdIntegrationRequest struct {
	Enabled           *bool   `json:"enabled"`
	AccessToken       string  `json:"accessToken"`
	ClearAccessToken  bool    `json:"clearAccessToken"`
	RefreshToken      string  `json:"refreshToken"`
	ClearRefreshToken bool    `json:"clearRefreshToken"`
	AccountEmail      *string `json:"accountEmail"`
	ListID            *string `json:"listId"`
	ListName          *string `json:"listName"`
}
