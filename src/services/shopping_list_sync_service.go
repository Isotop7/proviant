package services

import (
	"context"
	"net/http"

	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/rs/zerolog"
)

// ShoppingListSyncService coordinates shopping list synchronization for households and providers.
type ShoppingListSyncService struct {
	repos           *database.RepositoryContainer
	logger          *zerolog.Logger
	config          *configuration.ShoppingListSyncConfiguration
	providerFactory controllers.ShoppingListSyncProviderFactory
	httpClient      *http.Client
}

// NewShoppingListSyncService constructs a ShoppingListSyncService.
func NewShoppingListSyncService(
	repos *database.RepositoryContainer,
	logger *zerolog.Logger,
	config *configuration.ShoppingListSyncConfiguration,
) *ShoppingListSyncService {
	return &ShoppingListSyncService{
		repos:           repos,
		logger:          logger,
		config:          config,
		providerFactory: controllers.NewShoppingListSyncProvider,
	}
}

// SetProviderFactory overrides the provider factory (used by tests to inject mock or no-op providers).
func (s *ShoppingListSyncService) SetProviderFactory(factory controllers.ShoppingListSyncProviderFactory) {
	s.providerFactory = factory
}

// SetHTTPClient overrides the HTTP client used by external provider adapters.
func (s *ShoppingListSyncService) SetHTTPClient(client *http.Client) {
	s.httpClient = client
}

// TestIntegration tests the connection for a household integration without persisting sync changes.
func (s *ShoppingListSyncService) TestIntegration(ctx context.Context, integration *dbModel.HouseholdIntegration) error {
	if integration == nil {
		return errors.ErrIntegrationMissingCredentials
	}
	if s.config != nil && !s.config.IsProviderEnabled(integration.Provider) {
		return errors.ErrIntegrationProviderDisabled
	}
	factory := s.providerFactory
	if factory == nil {
		factory = controllers.NewShoppingListSyncProvider
	}
	provider, err := factory(integration.Provider, integration, s.logger, s.httpClient)
	if err != nil {
		return err
	}
	return provider.TestConnection(ctx, integration)
}

// SyncHousehold synchronizes all enabled integrations for the given household.
func (s *ShoppingListSyncService) SyncHousehold(householdID uint) error {
	return controllers.SyncHouseholdIntegrations(
		context.Background(),
		s.repos,
		s.logger,
		s.config,
		s.providerFactory,
		s.httpClient,
		householdID,
	)
}

// SyncProvider synchronizes a single provider integration for the given household.
func (s *ShoppingListSyncService) SyncProvider(ctx context.Context, householdID uint, provider string) error {
	return controllers.SyncSingleHouseholdIntegration(
		ctx,
		s.repos,
		s.logger,
		s.config,
		s.providerFactory,
		s.httpClient,
		householdID,
		provider,
	)
}

// SyncAll synchronizes all households with enabled integrations.
func (s *ShoppingListSyncService) SyncAll() error {
	return controllers.SyncAllHouseholdsIntegrations(
		context.Background(),
		s.repos,
		s.logger,
		s.config,
		s.providerFactory,
		s.httpClient,
	)
}
