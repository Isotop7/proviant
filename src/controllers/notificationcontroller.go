package controllers

import (
	"time"

	dbController "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"net/http"

	"github.com/rs/zerolog"
)

// NotificationController is the object struct to generate and send notifications for expired products
type NotificationController struct {
	Logger             *zerolog.Logger
	Configuration      *configuration.NotificationConfiguration
	DatabaseController dbController.DatabaseControllerInterface
	Providers          []NotificationProvider
}

// NewNotificationController creates a new NotificationController with configured providers
func NewNotificationController(
	logger *zerolog.Logger,
	config *configuration.NotificationConfiguration,
	dbc dbController.DatabaseControllerInterface,
) *NotificationController {
	nc := &NotificationController{
		Logger:             logger,
		Configuration:      config,
		DatabaseController: dbc,
	}

	// Initialize providers
	nc.initializeProviders()

	return nc
}

func (nc *NotificationController) initializeProviders() {
	// Add email provider if configured
	emailProvider := &EmailNotificationProvider{
		Configuration: nc.Configuration.SMTP,
		Logger:        nc.Logger,
	}

	if emailProvider.IsConfigured() {
		nc.Providers = append(nc.Providers, emailProvider)
	}

	// Add ntfy provider if configured
	ntfyProvider := &NtfyNotificationProvider{
		Configuration: nc.Configuration.Ntfy,
		Logger:        nc.Logger,
		HTTPClient:    &http.Client{Timeout: time.Duration(nc.Configuration.Ntfy.Timeout) * time.Second},
	}

	if ntfyProvider.IsConfigured() {
		nc.Providers = append(nc.Providers, ntfyProvider)
	}

	nc.Logger.Info().Msgf("Initialized %d notification providers", len(nc.Providers))
}

// Dispatch creates an eternal go routine that periodically checks for pending notifications and sends them.
// The timeout can be configured with the Configuration struct of NotificationController
func (nc *NotificationController) Dispatch() {
	if !nc.Configuration.Enabled {
		nc.Logger.Info().Msg("NotificationController is disabled")
		return
	}

	sleepInterval := time.Hour * time.Duration(nc.Configuration.Interval)
	go func() {
		nc.Logger.Debug().Msg("New NotificationController run dispatched")
		for {
			// Get affected products
			notificationProducts, getError := nc.DatabaseController.GetProductsExpiredAndNotificationPending(sleepInterval)
			if getError != nil {
				nc.Logger.Error().Msg(getError.Error())
				continue
			}

			// Generate notifications and send them
			nc.Logger.Info().Msgf("Sending notifications for %d products", len(notificationProducts))
			nc.generateNotifications(&notificationProducts)

			// Sleep
			nc.Logger.Info().Msgf("NotificationController is now sleeping for %d hours", nc.Configuration.Interval)
			time.Sleep(sleepInterval)
		}
	}()
}

// generateNotifications uses a list of products and generates a notification for it
func (nc *NotificationController) generateNotifications(notificationProducts *[]dbModel.Product) {
	// Loop through products
	for idx := range *notificationProducts {
		product := &(*notificationProducts)[idx]

		// Get notification preferences for household members
		preferences, getError := nc.DatabaseController.GetHouseholdMembersNotificationPreferences(product.HouseholdID)
		if getError != nil {
			nc.Logger.Error().Msg(getError.Error())
			continue
		}

		// Send notifications for each recipient
		for _, pref := range preferences {
			nc.sendNotificationsForRecipient(product, pref)
		}
	}
}

func (nc *NotificationController) sendNotificationsForRecipient(product *dbModel.Product, recipientInfo models.NotificationRecipientInfo) {
	success := false

	// Try each provider in order
	for _, provider := range nc.Providers {
		providerType := provider.GetProviderType()
		nc.Logger.Info().Msgf("Attempting to send %s notification for product '%s' (ID: %d)",
			providerType, product.ProductName, product.ID)

		var sendError error
		switch providerType {
		case "email":
			if recipientInfo.EmailAddress != "" {
				sendError = provider.SendNotification(product, recipientInfo.EmailAddress)
			}
		case "ntfy":
			sendError = provider.SendNotification(product, recipientInfo)
		}

		if sendError != nil {
			nc.Logger.Error().Msgf("Failed to send %s notification: %s", providerType, sendError.Error())
			continue
		}

		nc.Logger.Info().Msgf("Successfully sent %s notification", providerType)
		success = true

		// Only update NotifiedAt on first successful notification
		if success {
			updateErr := nc.DatabaseController.SetProductNotifiedAt(product.ID)
			if updateErr != nil {
				nc.Logger.Error().Msg(updateErr.Error())
			} else {
				nc.Logger.Info().Msg("Property NotifiedAt was updated")
			}
		}
	}

	if !success {
		nc.Logger.Warn().Msgf("Failed to send any notifications for product '%s' (ID: %d)",
			product.ProductName, product.ID)
	}
}
