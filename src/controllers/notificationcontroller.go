package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	dbController "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/rs/zerolog"
)

type NotificationController struct {
	Logger              *zerolog.Logger
	Configuration       *configuration.NotificationConfiguration
	NotificationRepo    dbController.NotificationRepositoryInterface
	Providers           []NotificationProvider
	TelegramBotUsername string
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

// GetTelegramBotUsername returns the resolved bot username (from getMe or config).
func (nc *NotificationController) GetTelegramBotUsername() string {
	return nc.TelegramBotUsername
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

	// Add Telegram provider if configured
	telegramTimeout := nc.Configuration.Telegram.Timeout
	if telegramTimeout <= 0 {
		telegramTimeout = 10
	}
	telegramProvider := &TelegramNotificationProvider{
		Configuration: nc.Configuration.Telegram,
		Logger:        nc.Logger,
		HTTPClient:    &http.Client{Timeout: time.Duration(telegramTimeout) * time.Second},
	}
	if telegramProvider.IsConfigured() {
		nc.Providers = append(nc.Providers, telegramProvider)
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

		// Determine if product is expired or expiring soon
		now := time.Now()
		isExpired := product.ExpireAt.Before(now)
		isExpiringSoon := !isExpired && product.ExpireAt.Before(now.AddDate(0, 0, nc.Configuration.Interval*24))

		// Fire webhooks asynchronously
		go func(p *dbModel.Product, expired, expiringSoon bool) {
			ws := GetWebhookService()
			if ws == nil {
				return
			}
			if expired {
				daysUntilExpiry := int(time.Since(p.ExpireAt).Hours() / 24)
				ws.FireEvent("product.expired", map[string]any{
					"id":              p.ID,
					"productName":     p.ProductName,
					"daysUntilExpiry": daysUntilExpiry,
					"householdId":     p.HouseholdID,
				})
			}
			if expiringSoon {
				daysUntilExpiry := int(time.Until(p.ExpireAt).Hours() / 24)
				ws.FireEvent("product.expiring_soon", map[string]any{
					"id":              p.ID,
					"productName":     p.ProductName,
					"daysUntilExpiry": daysUntilExpiry,
					"householdId":     p.HouseholdID,
				})
			}
		}(product, isExpired, isExpiringSoon)

		// Get notification preferences for household members
		preferences, getError := nc.NotificationRepo.GetHouseholdMembersNotificationPreferences(product.HouseholdID)
		if getError != nil {
			nc.Logger.Error().Msg(getError.Error())
			continue
		}

		// Send notifications for each recipient
		for _, pref := range preferences {
			nc.sendNotificationsForRecipient(product, &pref)
		}
	}
}

func (nc *NotificationController) sendNotificationsForRecipient(product *dbModel.Product, recipientInfo *models.NotificationRecipientInfo) {
	// Skip if the product's expiry date is still beyond this user's threshold window.
	// A threshold of 0 means notify on/after expiry (current behaviour).
	cutoff := time.Now().AddDate(0, 0, recipientInfo.NotificationThresholdDays)
	if product.ExpireAt.After(cutoff) {
		nc.Logger.Debug().Msgf("Product '%s' (ID: %d) not yet within threshold for recipient, skipping",
			product.ProductName, product.ID)
		return
	}

	notifiedAtUpdated := false

	for _, provider := range nc.Providers {
		providerType := provider.GetProviderType()

		var sendError error
		switch providerType {
		case "email":
			if recipientInfo.EmailEnabled && recipientInfo.EmailAddress != "" {
				nc.Logger.Info().Msgf("Attempting email notification for product '%s' (ID: %d)", product.ProductName, product.ID)
				sendError = provider.SendNotification(product, recipientInfo.EmailAddress)
			}
		case "ntfy":
			if recipientInfo.NtfyEnabled {
				nc.Logger.Info().Msgf("Attempting ntfy notification for product '%s' (ID: %d)", product.ProductName, product.ID)
				sendError = provider.SendNotification(product, recipientInfo)
			}
		case "telegram":
			if recipientInfo.TelegramEnabled && recipientInfo.TelegramChatID != "" {
				nc.Logger.Info().Msgf("Attempting telegram notification for product '%s' (ID: %d)", product.ProductName, product.ID)
				sendError = provider.SendNotification(product, recipientInfo.TelegramChatID)
			}
		}

		if sendError != nil {
			nc.Logger.Error().Msgf("Failed to send %s notification: %s", providerType, sendError.Error())
			continue
		}

		nc.Logger.Info().Msgf("Successfully sent %s notification", providerType)

		if !notifiedAtUpdated {
			updateErr := nc.NotificationRepo.SetProductNotifiedAt(product.ID)
			if updateErr != nil {
				nc.Logger.Error().Msg(updateErr.Error())
			} else {
				nc.Logger.Info().Msg("Property NotifiedAt was updated")
				notifiedAtUpdated = true
			}
		}
	}

	if !notifiedAtUpdated {
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

// DispatchMonthlyWasteReports starts a goroutine that sends household waste reports
// on the configured day/hour (UTC) of each month to opted-in members via all enabled providers.
func (nc *NotificationController) DispatchMonthlyWasteReports() {
	ep := &EmailNotificationProvider{Configuration: nc.Configuration.SMTP, Logger: nc.Logger}
	telegramTimeout := nc.Configuration.Telegram.Timeout
	if telegramTimeout <= 0 {
		telegramTimeout = 10
	}
	tp := &TelegramNotificationProvider{
		Configuration: nc.Configuration.Telegram,
		Logger:        nc.Logger,
		HTTPClient:    &http.Client{Timeout: time.Duration(telegramTimeout) * time.Second},
	}

	if !ep.IsConfigured() && !tp.IsConfigured() {
		nc.Logger.Info().Msg("Monthly waste report: no providers configured, skipping")
		return
	}

	cfg := nc.Configuration.MonthlyWasteReport
	go func() {
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month()+1, cfg.Day, cfg.Hour, 0, 0, 0, time.UTC)
			nc.Logger.Info().Msgf("Monthly waste report: next run at %s", next.Format(time.RFC3339))
			time.Sleep(time.Until(next))
			nc.processMonthlyWasteReports(ep, tp)
		}
	}()
}

func (nc *NotificationController) processMonthlyWasteReports(ep *EmailNotificationProvider, tp *TelegramNotificationProvider) {
	lastMonth := time.Now().UTC().AddDate(0, -1, 0)
	targets, err := nc.NotificationRepo.GetHouseholdsWithMonthlyWasteReportEnabled()
	if err != nil {
		nc.Logger.Error().Msgf("Monthly waste report: failed to fetch households: %s", err)
		return
	}
	nc.Logger.Info().Msgf("Monthly waste report: processing %d household(s)", len(targets))

	for i := range targets {
		t := &targets[i]
		stats, statsErr := nc.NotificationRepo.GetWasteStatsForHousehold(t.HouseholdID, lastMonth)
		if statsErr != nil {
			nc.Logger.Error().Msgf("Monthly waste report: stats error for household %d: %s", t.HouseholdID, statsErr)
			continue
		}
		stats.HouseholdName = t.HouseholdName

		if ep.IsConfigured() {
			for _, recipient := range t.Recipients {
				if sendErr := ep.SendMonthlyWasteReport(recipient, &stats); sendErr != nil {
					nc.Logger.Error().Msgf("Monthly waste report: email send failed to %s: %s", recipient, sendErr)
				} else {
					nc.Logger.Info().Msgf("Monthly waste report: email sent to %s (household %d)", recipient, t.HouseholdID)
				}
			}
		}

		if tp.IsConfigured() {
			for _, chatID := range t.TelegramChatIDs {
				if sendErr := tp.SendMonthlyWasteReport(chatID, &stats); sendErr != nil {
					nc.Logger.Error().Msgf("Monthly waste report: telegram send failed to chat %s: %s", chatID, sendErr)
				} else {
					nc.Logger.Info().Msgf("Monthly waste report: telegram sent to chat %s (household %d)", chatID, t.HouseholdID)
				}
			}
		}
	}
}

// StartTelegramPoller starts a long-polling goroutine that listens for Telegram bot updates.
// It handles /start <token> commands to link a Telegram chat to a Proviant user account.
func (nc *NotificationController) StartTelegramPoller() {
	if nc.Configuration.Telegram.BotToken == "" {
		nc.Logger.Info().Msg("Telegram poller: bot token not configured, skipping")
		return
	}

	pollTimeout := nc.Configuration.Telegram.Timeout
	if pollTimeout <= 0 {
		pollTimeout = 10
	}

	client := &http.Client{Timeout: time.Duration(pollTimeout+5) * time.Second}
	baseURL := fmt.Sprintf("https://api.telegram.org/bot%s", nc.Configuration.Telegram.BotToken)

	go func() {
		var offset int64

		// Resolve bot username via getMe so the UI can build deep links
		if nc.TelegramBotUsername == "" {
			getMeURL := fmt.Sprintf("%s/getMe", baseURL)
			if resp, err := client.Get(getMeURL); err == nil {
				var result struct {
					OK     bool `json:"ok"`
					Result struct {
						Username string `json:"username"`
					} `json:"result"`
				}
				if body, readErr := io.ReadAll(resp.Body); readErr == nil {
					if jsonErr := json.Unmarshal(body, &result); jsonErr == nil && result.OK {
						nc.TelegramBotUsername = result.Result.Username
						nc.Logger.Info().Msgf("Telegram poller: resolved bot username @%s", nc.TelegramBotUsername)
					}
				}
				_ = resp.Body.Close()
			}
		}
		// Fall back to config value if getMe failed
		if nc.TelegramBotUsername == "" {
			nc.TelegramBotUsername = nc.Configuration.Telegram.BotUsername
		}

		nc.Logger.Info().Msg("Telegram poller: started")

		for {
			url := fmt.Sprintf("%s/getUpdates?timeout=%d&offset=%d", baseURL, pollTimeout, offset)
			resp, err := client.Get(url)
			if err != nil {
				nc.Logger.Error().Msgf("Telegram poller: getUpdates error: %s", err)
				time.Sleep(5 * time.Second)
				continue
			}

			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				nc.Logger.Error().Msgf("Telegram poller: read error: %s", readErr)
				continue
			}

			var result struct {
				OK     bool `json:"ok"`
				Result []struct {
					UpdateID int64 `json:"update_id"`
					Message  *struct {
						Chat struct {
							ID int64 `json:"id"`
						} `json:"chat"`
						Text string `json:"text"`
					} `json:"message"`
				} `json:"result"`
			}

			if jsonErr := json.Unmarshal(body, &result); jsonErr != nil {
				nc.Logger.Error().Msgf("Telegram poller: parse error: %s", jsonErr)
				continue
			}

			for _, update := range result.Result {
				offset = update.UpdateID + 1

				if update.Message == nil {
					continue
				}

				text := strings.TrimSpace(update.Message.Text)
				chatID := fmt.Sprintf("%d", update.Message.Chat.ID)

				if !strings.HasPrefix(text, "/start") {
					continue
				}

				parts := strings.Fields(text)
				if len(parts) < 2 {
					nc.sendTelegramMessage(client, baseURL, chatID,
						"Send `/start <token>` with the token from your Proviant notification settings to link this chat.")
					continue
				}

				token := parts[1]
				user, findErr := nc.NotificationRepo.FindUserByTelegramLinkToken(token)
				if findErr != nil {
					nc.Logger.Warn().Msgf("Telegram poller: unknown link token from chat %s", chatID)
					nc.sendTelegramMessage(client, baseURL, chatID,
						"Invalid or expired token. Please generate a new one in Proviant settings.")
					continue
				}

				if setErr := nc.NotificationRepo.SetTelegramChatID(user.ID, chatID); setErr != nil {
					nc.Logger.Error().Msgf("Telegram poller: failed to save chat ID for user %d: %s", user.ID, setErr)
					nc.sendTelegramMessage(client, baseURL, chatID,
						"Something went wrong. Please try again.")
					continue
				}

				nc.Logger.Info().Msgf("Telegram poller: linked chat %s to user %d", chatID, user.ID)
				nc.sendTelegramMessage(client, baseURL, chatID,
					"✅ Linked! You will now receive Proviant notifications here.")
			}
		}
	}()
}

func (nc *NotificationController) sendTelegramMessage(client *http.Client, baseURL, chatID, text string) {
	payload := map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		nc.Logger.Error().Msgf("Telegram: marshal error: %s", err)
		return
	}
	resp, err := client.Post(baseURL+"/sendMessage", "application/json", bytes.NewReader(body))
	if err != nil {
		nc.Logger.Error().Msgf("Telegram: sendMessage error: %s", err)
		return
	}
	_ = resp.Body.Close()
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
			inviterName = user.EffectiveName()
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
