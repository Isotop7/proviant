package controllers

import (
	"fmt"
	"time"

	dbController "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"net/http"

	"github.com/rs/zerolog"
)

type NotificationController struct {
	Logger           *zerolog.Logger
	Configuration    *configuration.NotificationConfiguration
	NotificationRepo dbController.NotificationRepositoryInterface
	Providers        []NotificationProvider
}

func NewNotificationController(
	logger *zerolog.Logger,
	config *configuration.NotificationConfiguration,
	notificationRepo dbController.NotificationRepositoryInterface,
) *NotificationController {
	nc := &NotificationController{
		Logger:           logger,
		Configuration:    config,
		NotificationRepo: notificationRepo,
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
			// Get affected products (widen query window to cover the highest per-user threshold)
			maxThreshold := nc.NotificationRepo.GetMaxNotificationThresholdDays()
			notificationProducts, getError := nc.NotificationRepo.GetProductsExpiredAndNotificationPending(sleepInterval, maxThreshold)
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
		preferences, getError := nc.NotificationRepo.GetHouseholdMembersNotificationPreferences(product.HouseholdID)
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
	// Skip if the product's expiry date is still beyond this user's threshold window.
	// A threshold of 0 means notify on/after expiry (current behaviour).
	cutoff := time.Now().AddDate(0, 0, recipientInfo.NotificationThresholdDays)
	if product.ExpireAt.After(cutoff) {
		nc.Logger.Debug().Msgf("Product '%s' (ID: %d) not yet within threshold for recipient, skipping",
			product.ProductName, product.ID)
		return
	}

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
			updateErr := nc.NotificationRepo.SetProductNotifiedAt(product.ID)
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

// DispatchInvitations starts a background goroutine that periodically retries sending pending invitation emails.
// It runs once immediately on startup, then every Interval hours (reusing the same config as product notifications).
func (nc *NotificationController) DispatchInvitations(baseURL string) {
	emailProvider := &EmailNotificationProvider{
		Configuration: nc.Configuration.SMTP,
		Logger:        nc.Logger,
	}

	if !emailProvider.IsConfigured() {
		nc.Logger.Info().Msg("Invitation dispatch: email provider not configured, skipping")
		return
	}

	sleepInterval := time.Hour * time.Duration(nc.Configuration.Interval)
	go func() {
		nc.Logger.Info().Msg("Invitation dispatch: running initial dispatch on startup")
		nc.processPendingInvitations(emailProvider, baseURL)

		for {
			nc.Logger.Info().Msgf("Invitation dispatch: sleeping for %v before next retry cycle", sleepInterval)
			time.Sleep(sleepInterval)
			nc.processPendingInvitations(emailProvider, baseURL)
		}
	}()
}

// SendInvitationEmail sends a single invitation email and marks it as sent or failed in the database.
func (nc *NotificationController) SendInvitationEmail(invitation *dbModel.HouseholdInvitation, inviterName, householdName, baseURL string) error {
	emailProvider := &EmailNotificationProvider{
		Configuration: nc.Configuration.SMTP,
		Logger:        nc.Logger,
	}

	if !emailProvider.IsConfigured() {
		return fmt.Errorf("email provider not configured")
	}

	if err := emailProvider.SendInvitationEmail(invitation, inviterName, householdName, baseURL); err != nil {
		nc.Logger.Error().Msgf("Failed to send invitation email to %s: %s", invitation.Email, err)
		if markErr := nc.NotificationRepo.MarkInvitationSendFailed(invitation.ID); markErr != nil {
			nc.Logger.Error().Msgf("Failed to mark invitation %d as send-failed: %s", invitation.ID, markErr)
		}
		return err
	}

	if err := nc.NotificationRepo.MarkInvitationSent(invitation.ID); err != nil {
		nc.Logger.Error().Msgf("Failed to mark invitation %d as sent: %s", invitation.ID, err)
		return err
	}

	nc.Logger.Info().Msgf("Invitation email sent successfully to %s", invitation.Email)
	return nil
}

// SendVerificationEmail sends an email verification link using the invitation email system.
func (nc *NotificationController) SendVerificationEmail(invitation *dbModel.HouseholdInvitation, username, baseURL string) error {
	emailProvider := &EmailNotificationProvider{
		Configuration: nc.Configuration.SMTP,
		Logger:        nc.Logger,
	}

	if !emailProvider.IsConfigured() {
		return fmt.Errorf("email provider not configured")
	}

	if err := emailProvider.SendInvitationEmail(invitation, username, "your household", baseURL); err != nil {
		nc.Logger.Error().Msgf("Failed to send verification email to %s: %s", invitation.Email, err)
		if markErr := nc.NotificationRepo.MarkInvitationSendFailed(invitation.ID); markErr != nil {
			nc.Logger.Error().Msgf("Failed to mark verification invitation %d as send-failed: %s", invitation.ID, markErr)
		}
		return err
	}

	if err := nc.NotificationRepo.MarkInvitationSent(invitation.ID); err != nil {
		nc.Logger.Error().Msgf("Failed to mark verification invitation %d as sent: %s", invitation.ID, err)
		return err
	}

	nc.Logger.Info().Msgf("Verification email sent successfully to %s", invitation.Email)
	return nil
}

// SendEmailVerification sends a verification email directly to the user with a verification token.
func (nc *NotificationController) SendEmailVerification(email, username, token, baseURL string, expiresAt time.Time) error {
	emailProvider := &EmailNotificationProvider{
		Configuration: nc.Configuration.SMTP,
		Logger:        nc.Logger,
	}

	if !emailProvider.IsConfigured() {
		return fmt.Errorf("email provider not configured")
	}

	if err := emailProvider.SendEmailVerificationEmail(email, username, token, baseURL, expiresAt); err != nil {
		nc.Logger.Error().Msgf("Failed to send email verification to %s: %s", email, err)
		return err
	}

	nc.Logger.Info().Msgf("Email verification sent successfully to %s", email)
	return nil
}

// processPendingInvitations fetches all pending invitations that need to be sent or retried.
func (nc *NotificationController) processPendingInvitations(emailProvider *EmailNotificationProvider, baseURL string) {
	sleepInterval := time.Hour * time.Duration(nc.Configuration.Interval)

	invitations, err := nc.NotificationRepo.GetPendingInvitationsNotSent(sleepInterval)
	if err != nil {
		nc.Logger.Error().Msgf("Failed to fetch pending invitations: %s", err)
		return
	}

	if len(invitations) == 0 {
		nc.Logger.Debug().Msg("Invitation dispatch: no pending invitations to send")
		return
	}

	nc.Logger.Info().Msgf("Invitation dispatch: processing %d pending invitation(s)", len(invitations))

	for i := range invitations {
		invitation := &invitations[i]
		user, userErr := nc.NotificationRepo.GetUserByID(invitation.InviterID)
		inviterName := "A household member"
		if userErr == nil {
			inviterName = user.Username
		}

		household, householdErr := nc.NotificationRepo.GetHouseholdByID(invitation.HouseholdID)
		householdName := fmt.Sprintf("Household #%d", invitation.HouseholdID)
		if householdErr == nil {
			householdName = household.Name
		}

		if err := emailProvider.SendInvitationEmail(invitation, inviterName, householdName, baseURL); err != nil {
			nc.Logger.Error().Msgf("Failed to send invitation %d to %s: %s", invitation.ID, invitation.Email, err)
			if markErr := nc.NotificationRepo.MarkInvitationSendFailed(invitation.ID); markErr != nil {
				nc.Logger.Error().Msgf("Failed to mark invitation %d as send-failed: %s", invitation.ID, markErr)
			}
		} else {
			if markErr := nc.NotificationRepo.MarkInvitationSent(invitation.ID); markErr != nil {
				nc.Logger.Error().Msgf("Failed to mark invitation %d as sent: %s", invitation.ID, markErr)
			} else {
				nc.Logger.Info().Msgf("Invitation email sent successfully to %s", invitation.Email)
			}
		}
	}
}
