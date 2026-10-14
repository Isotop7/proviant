package controllers

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	dbController "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

var (
	householdSyncLocksMu sync.Mutex
	householdSyncLocks   = make(map[uint]*sync.Mutex)
)

func lockHouseholdSync(householdID uint) func() {
	householdSyncLocksMu.Lock()
	mu, ok := householdSyncLocks[householdID]
	if !ok {
		mu = &sync.Mutex{}
		householdSyncLocks[householdID] = mu
	}
	householdSyncLocksMu.Unlock()

	mu.Lock()
	return mu.Unlock
}

// ScrubIntegrationError redacts any stored integration credentials from an error message.
func ScrubIntegrationError(err error, integration *dbModel.HouseholdIntegration) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if integration == nil {
		return msg
	}
	if token := strings.TrimSpace(integration.AccessToken); token != "" {
		msg = strings.ReplaceAll(msg, token, "[REDACTED]")
	}
	if refresh := strings.TrimSpace(integration.RefreshToken); refresh != "" {
		msg = strings.ReplaceAll(msg, refresh, "[REDACTED]")
	}
	return msg
}

// ReconcileShoppingListIntegration runs one push-and-pull synchronization cycle for a single household integration.
func ReconcileShoppingListIntegration(
	ctx context.Context,
	repos *dbController.RepositoryContainer,
	logger *zerolog.Logger,
	factory ShoppingListSyncProviderFactory,
	httpClient *http.Client,
	integration *dbModel.HouseholdIntegration,
	items []dbModel.ShoppingListItem,
) error {
	if factory == nil {
		factory = NewShoppingListSyncProvider
	}
	provider, err := factory(integration.Provider, integration, logger, httpClient)
	if err != nil {
		recordIntegrationFailure(repos, logger, integration, err)
		return err
	}

	refs, err := repos.ShoppingListSyncRefs.ListByHouseholdAndProvider(integration.HouseholdID, integration.Provider)
	if err != nil {
		recordIntegrationFailure(repos, logger, integration, err)
		return err
	}

	payloads := make([]ShoppingListSyncPayload, len(items))
	itemByID := make(map[uint]*dbModel.ShoppingListItem, len(items))
	for i := range items {
		item := &items[i]
		itemByID[item.ID] = item
		payloads[i] = ShoppingListSyncPayload{
			ID:       item.ID,
			Name:     item.Name,
			Category: item.Category,
			Quantity: item.Quantity,
			Unit:     item.Unit,
			Checked:  item.Checked,
			Notes:    item.Notes,
		}
	}

	pushResult, pushErr := provider.Push(ctx, integration, payloads, refs)
	persistPushRefs(repos, logger, integration, refs, pushResult)
	if pushErr != nil {
		recordIntegrationFailure(repos, logger, integration, pushErr)
		return pushErr
	}

	updatedRefs, err := repos.ShoppingListSyncRefs.ListByHouseholdAndProvider(integration.HouseholdID, integration.Provider)
	if err != nil {
		recordIntegrationFailure(repos, logger, integration, err)
		return err
	}

	checkedByExtID, pullErr := provider.PullChecked(ctx, integration, updatedRefs)
	if pullErr != nil {
		recordIntegrationFailure(repos, logger, integration, pullErr)
		return pullErr
	}

	for i := range updatedRefs {
		ref := &updatedRefs[i]
		remoteChecked, exists := checkedByExtID[ref.ExternalID]
		if !exists {
			continue
		}
		item, ok := itemByID[ref.ShoppingListItemID]
		if !ok || item == nil || item.Checked == remoteChecked {
			continue
		}
		item.Checked = remoteChecked
		if updateErr := repos.ShoppingListItems.Update(item); updateErr != nil && logger != nil {
			logger.Warn().Msgf("Shopping list sync: failed to update checked state for item %d: %s", item.ID, updateErr)
		}
	}

	now := time.Now().UTC()
	integration.LastSyncAt = &now
	integration.LastError = ""
	if saveErr := repos.HouseholdIntegrations.Update(integration); saveErr != nil && logger != nil {
		logger.Warn().Msgf("Shopping list sync: failed to update integration status: %s", saveErr)
	}
	return nil
}

func persistPushRefs(
	repos *dbController.RepositoryContainer,
	logger *zerolog.Logger,
	integration *dbModel.HouseholdIntegration,
	existingRefs []dbModel.ShoppingListItemSyncRef,
	pushResult ShoppingListPushResult,
) {
	now := time.Now().UTC()
	for _, deletedExtID := range pushResult.Deleted {
		if err := repos.ShoppingListSyncRefs.DeleteByExternalID(integration.HouseholdID, integration.Provider, deletedExtID); err != nil && logger != nil {
			logger.Warn().Msgf("Shopping list sync: failed to delete ref %s: %s", deletedExtID, err)
		}
	}

	for itemID, extID := range pushResult.Created {
		ref := &dbModel.ShoppingListItemSyncRef{
			HouseholdID:        integration.HouseholdID,
			ShoppingListItemID: itemID,
			Provider:           integration.Provider,
			ExternalID:         extID,
			ListID:             integration.ListID,
			SyncedAt:           now,
			Status:             util.SyncRefStatusSynced,
		}
		if err := repos.ShoppingListSyncRefs.Upsert(ref); err != nil && logger != nil {
			logger.Warn().Msgf("Shopping list sync: failed to save created ref for item %d: %s", itemID, err)
		}
	}

	refByItemID := make(map[uint]dbModel.ShoppingListItemSyncRef, len(existingRefs))
	for i := range existingRefs {
		if existingRefs[i].ShoppingListItemID > 0 {
			refByItemID[existingRefs[i].ShoppingListItemID] = existingRefs[i]
		}
	}
	for _, itemID := range pushResult.Updated {
		ref, exists := refByItemID[itemID]
		if !exists {
			continue
		}
		ref.ListID = integration.ListID
		ref.SyncedAt = now
		ref.Status = util.SyncRefStatusSynced
		if err := repos.ShoppingListSyncRefs.Upsert(&ref); err != nil && logger != nil {
			logger.Warn().Msgf("Shopping list sync: failed to update ref for item %d: %s", itemID, err)
		}
	}
}

func recordIntegrationFailure(repos *dbController.RepositoryContainer, logger *zerolog.Logger, integration *dbModel.HouseholdIntegration, err error) {
	scrubbed := ScrubIntegrationError(err, integration)
	integration.LastError = scrubbed
	if logger != nil {
		logger.Warn().
			Uint("household_id", integration.HouseholdID).
			Str("provider", integration.Provider).
			Msgf("Shopping list sync failed: %s", scrubbed)
	}
	if updateErr := repos.HouseholdIntegrations.Update(integration); updateErr != nil && logger != nil {
		logger.Warn().Msgf("Shopping list sync: failed to persist integration error: %s", updateErr)
	}
}

// SyncHouseholdIntegrations synchronizes all enabled integrations for a single household.
func SyncHouseholdIntegrations(
	ctx context.Context,
	repos *dbController.RepositoryContainer,
	logger *zerolog.Logger,
	config *configuration.ShoppingListSyncConfiguration,
	factory ShoppingListSyncProviderFactory,
	httpClient *http.Client,
	householdID uint,
) error {
	unlock := lockHouseholdSync(householdID)
	defer unlock()

	integrations, err := repos.HouseholdIntegrations.ListByHousehold(householdID)
	if err != nil {
		return err
	}
	if len(integrations) == 0 {
		return nil
	}

	var firstErr error
	for i := range integrations {
		integration := &integrations[i]
		if !integration.Enabled {
			continue
		}
		if config != nil && !config.IsProviderEnabled(integration.Provider) {
			continue
		}
		items, listErr := repos.ShoppingListItems.ListByHousehold(householdID)
		if listErr != nil {
			return listErr
		}
		if syncErr := ReconcileShoppingListIntegration(ctx, repos, logger, factory, httpClient, integration, items); syncErr != nil && firstErr == nil {
			firstErr = syncErr
		}
	}
	return firstErr
}

// SyncSingleHouseholdIntegration synchronizes one specific provider integration for a household.
func SyncSingleHouseholdIntegration(
	ctx context.Context,
	repos *dbController.RepositoryContainer,
	logger *zerolog.Logger,
	config *configuration.ShoppingListSyncConfiguration,
	factory ShoppingListSyncProviderFactory,
	httpClient *http.Client,
	householdID uint,
	provider string,
) error {
	if config != nil && !config.IsProviderEnabled(provider) {
		return errors.ErrIntegrationProviderDisabled
	}

	unlock := lockHouseholdSync(householdID)
	defer unlock()

	integration, err := repos.HouseholdIntegrations.GetByHouseholdAndProvider(householdID, provider)
	if err != nil {
		return err
	}
	items, err := repos.ShoppingListItems.ListByHousehold(householdID)
	if err != nil {
		return err
	}
	return ReconcileShoppingListIntegration(ctx, repos, logger, factory, httpClient, integration, items)
}

// SyncAllHouseholdsIntegrations synchronizes all households that have at least one enabled integration.
func SyncAllHouseholdsIntegrations(
	ctx context.Context,
	repos *dbController.RepositoryContainer,
	logger *zerolog.Logger,
	config *configuration.ShoppingListSyncConfiguration,
	factory ShoppingListSyncProviderFactory,
	httpClient *http.Client,
) error {
	if config != nil && !config.Enabled {
		return nil
	}
	enabledIntegrations, err := repos.HouseholdIntegrations.ListEnabled()
	if err != nil {
		return err
	}

	seenHouseholds := make(map[uint]bool, len(enabledIntegrations))
	var firstErr error
	for i := range enabledIntegrations {
		householdID := enabledIntegrations[i].HouseholdID
		if seenHouseholds[householdID] {
			continue
		}
		seenHouseholds[householdID] = true
		if syncErr := SyncHouseholdIntegrations(ctx, repos, logger, config, factory, httpClient, householdID); syncErr != nil && firstErr == nil {
			firstErr = syncErr
		}
	}
	return firstErr
}

// ShoppingListSyncController manages the periodic background synchronization of household shopping lists.
type ShoppingListSyncController struct {
	Logger          *zerolog.Logger
	Configuration   *configuration.ShoppingListSyncConfiguration
	DB              *gorm.DB
	ProviderFactory ShoppingListSyncProviderFactory
	HTTPClient      *http.Client
}

// NewShoppingListSyncController constructs a ShoppingListSyncController.
func NewShoppingListSyncController(
	logger *zerolog.Logger,
	config *configuration.ShoppingListSyncConfiguration,
	db *gorm.DB,
) *ShoppingListSyncController {
	return &ShoppingListSyncController{
		Logger:          logger,
		Configuration:   config,
		DB:              db,
		ProviderFactory: NewShoppingListSyncProvider,
	}
}

// RunOnce executes a single synchronization pass across all enabled household integrations.
func (c *ShoppingListSyncController) RunOnce(ctx context.Context) error {
	if c.Configuration == nil || !c.Configuration.Enabled {
		return nil
	}
	if c.DB == nil {
		return nil
	}
	repos := dbController.NewRepositoryContainer(c.DB, c.Logger)
	return SyncAllHouseholdsIntegrations(ctx, repos, c.Logger, c.Configuration, c.ProviderFactory, c.HTTPClient)
}

// Dispatch starts the periodic shopping list sync goroutine when enabled in configuration.
func (c *ShoppingListSyncController) Dispatch() {
	if c.Configuration == nil || !c.Configuration.Enabled {
		if c.Logger != nil {
			c.Logger.Info().Msg("ShoppingListSyncController is disabled")
		}
		return
	}

	intervalMins := c.Configuration.IntervalMinutes
	if intervalMins <= 0 {
		intervalMins = util.DefaultShoppingListSyncIntervalMinutes
	}
	interval := time.Duration(intervalMins) * time.Minute

	go func() {
		if c.Logger != nil {
			c.Logger.Info().Msgf("ShoppingListSyncController dispatched (interval: %d minutes)", intervalMins)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			if err := c.RunOnce(ctx); err != nil && c.Logger != nil {
				c.Logger.Warn().Msgf("ShoppingListSyncController run completed with errors: %s", err)
			}
			cancel()
		}
	}()
}
