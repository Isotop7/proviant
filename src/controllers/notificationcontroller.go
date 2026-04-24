package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	dbController "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/rs/zerolog"
)

type NotificationController struct {
	Logger           *zerolog.Logger
	Configuration    *configuration.NotificationConfiguration
	NotificationRepo dbController.NotificationRepositoryInterface
	StreakRepo       dbController.StreakRepositoryInterface
	Providers        []NotificationProvider
	telegramClient   *http.Client
	pollerCancels    sync.Map // userID(uint) → context.CancelFunc
	botUsernames     sync.Map // userID(uint) → resolved bot username(string)
}

func NewNotificationController(
	logger *zerolog.Logger,
	config *configuration.NotificationConfiguration,
	notificationRepo dbController.NotificationRepositoryInterface,
) *NotificationController {
	notificationController := &NotificationController{
		Logger:           logger,
		Configuration:    config,
		NotificationRepo: notificationRepo,
		telegramClient:   &http.Client{Timeout: time.Duration(telegramTimeout(config)) * time.Second},
	}

	// Initialize providers
	notificationController.initializeProviders()

	return notificationController
}

func telegramTimeout(config *configuration.NotificationConfiguration) int {
	if config.Telegram.Timeout > 0 {
		return config.Telegram.Timeout
	}
	return 15
}

// GetUserTelegramBotUsername returns the bot username resolved at poller start for a user.
func (nc *NotificationController) GetUserTelegramBotUsername(userID uint) string {
	if v, ok := nc.botUsernames.Load(userID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
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

	// Telegram is per-user; no global provider is initialized here.

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
			webhookService := GetWebhookService()
			if webhookService == nil {
				return
			}
			if expired {
				daysUntilExpiry := int(time.Since(p.ExpireAt).Hours() / 24)
				webhookService.FireEvent("product.expired", map[string]any{
					"id":              p.ID,
					"productName":     p.ProductName,
					"daysUntilExpiry": daysUntilExpiry,
					"householdId":     p.HouseholdID,
				})
			}
			if expiringSoon {
				daysUntilExpiry := int(time.Until(p.ExpireAt).Hours() / 24)
				webhookService.FireEvent("product.expiring_soon", map[string]any{
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

	// Telegram is per-user; use recipient's own bot token.
	if recipientInfo.TelegramEnabled && recipientInfo.TelegramChatID != "" && recipientInfo.TelegramBotToken != "" {
		nc.Logger.Info().Msgf("Attempting telegram notification for product '%s' (ID: %d)", product.ProductName, product.ID)
		telegramProvider := &TelegramNotificationProvider{BotToken: recipientInfo.TelegramBotToken, Logger: nc.Logger, HTTPClient: nc.telegramClient}
		if err := telegramProvider.SendNotification(product, recipientInfo.TelegramChatID); err != nil {
			nc.Logger.Error().Msgf("Failed to send telegram notification: %s", err)
		} else {
			nc.Logger.Info().Msg("Successfully sent telegram notification")
			if !notifiedAtUpdated {
				if updateErr := nc.NotificationRepo.SetProductNotifiedAt(product.ID); updateErr != nil {
					nc.Logger.Error().Msg(updateErr.Error())
				} else {
					nc.Logger.Info().Msg("Property NotifiedAt was updated")
					notifiedAtUpdated = true
				}
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
	emailProvider := &EmailNotificationProvider{Configuration: nc.Configuration.SMTP, Logger: nc.Logger}

	notificationConfig := nc.Configuration.MonthlyWasteReport
	go func() {
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month()+1, notificationConfig.Day, notificationConfig.Hour, 0, 0, 0, time.UTC)
			nc.Logger.Info().Msgf("Monthly waste report: next run at %s", next.Format(time.RFC3339))
			time.Sleep(time.Until(next))
			nc.processMonthlyWasteReports(emailProvider)
		}
	}()
}

func (nc *NotificationController) processMonthlyWasteReports(emailProvider *EmailNotificationProvider) {
	lastMonth := time.Now().UTC().AddDate(0, -1, 0)
	targets, err := nc.NotificationRepo.GetHouseholdsWithMonthlyWasteReportEnabled()
	if err != nil {
		nc.Logger.Error().Msgf("Monthly waste report: failed to fetch households: %s", err)
		return
	}
	nc.Logger.Info().Msgf("Monthly waste report: processing %d household(s)", len(targets))

	for i := range targets {
		target := &targets[i]
		stats, statsErr := nc.NotificationRepo.GetWasteStatsForHousehold(target.HouseholdID, lastMonth)
		if statsErr != nil {
			nc.Logger.Error().Msgf("Monthly waste report: stats error for household %d: %s", target.HouseholdID, statsErr)
			continue
		}
		stats.HouseholdName = target.HouseholdName

		if emailProvider.IsConfigured() {
			for _, recipient := range target.Recipients {
				if sendErr := emailProvider.SendMonthlyWasteReport(recipient, &stats); sendErr != nil {
					nc.Logger.Error().Msgf("Monthly waste report: email send failed to %s: %s", recipient, sendErr)
				} else {
					nc.Logger.Info().Msgf("Monthly waste report: email sent to %s (household %d)", recipient, target.HouseholdID)
				}
			}
		}

		for _, tr := range target.TelegramRecipients {
			telegramProvider := &TelegramNotificationProvider{BotToken: tr.BotToken, Logger: nc.Logger, HTTPClient: nc.telegramClient}
			if sendErr := telegramProvider.SendMonthlyWasteReport(tr.ChatID, &stats); sendErr != nil {
				nc.Logger.Error().Msgf("Monthly waste report: telegram send failed to chat %s: %s", tr.ChatID, sendErr)
			} else {
				nc.Logger.Info().Msgf("Monthly waste report: telegram sent to chat %s (household %d)", tr.ChatID, target.HouseholdID)
			}
		}
	}
}

// DispatchStreakUpdates starts a goroutine that runs daily at midnight UTC to increment
// or reset each household's waste-free streak and send milestone notifications.
func (nc *NotificationController) DispatchStreakUpdates() {
	if nc.StreakRepo == nil {
		nc.Logger.Warn().Msg("StreakRepo not set, skipping streak updates")
		return
	}
	go func() {
		for {
			now := time.Now().UTC()
			nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
			nc.Logger.Info().Msgf("Streak updater: next run at %s", nextMidnight.Format(time.RFC3339))
			time.Sleep(time.Until(nextMidnight))
			nc.processStreakUpdates()
		}
	}()
}

func (nc *NotificationController) processStreakUpdates() {
	streaks, err := nc.StreakRepo.GetAllStreaks()
	if err != nil {
		nc.Logger.Error().Msgf("Streak updater: failed to fetch streaks: %s", err)
		return
	}
	nc.Logger.Info().Msgf("Streak updater: processing %d household(s)", len(streaks))

	milestones := []int{7, 30, 100}
	now := time.Now().UTC()

	for i := range streaks {
		streak := &streaks[i]

		// Waste happened since last check → reset streak
		if streak.LastWastedDate != nil && !streak.LastWastedDate.Before(streak.LastCheckedDate) {
			streak.CurrentStreak = 0
			streak.LastWastedDate = nil
		} else {
			streak.CurrentStreak++
			if streak.CurrentStreak > streak.LongestStreak {
				streak.LongestStreak = streak.CurrentStreak
			}
			for _, m := range milestones {
				if streak.CurrentStreak == m {
					nc.sendStreakMilestoneNotifications(streak.HouseholdID, m)
				}
			}
		}
		streak.LastCheckedDate = now

		if updateErr := nc.StreakRepo.UpdateStreak(streak); updateErr != nil {
			nc.Logger.Error().Msgf("Streak updater: failed to save streak for household %d: %s", streak.HouseholdID, updateErr)
		} else {
			nc.Logger.Info().Msgf("Streak updater: household %d streak = %d", streak.HouseholdID, streak.CurrentStreak)
		}
	}
}

func (nc *NotificationController) sendStreakMilestoneNotifications(householdID uint, milestone int) {
	preferences, err := nc.NotificationRepo.GetHouseholdMembersNotificationPreferences(householdID)
	if err != nil {
		nc.Logger.Error().Msgf("Streak milestone: failed to get preferences for household %d: %s", householdID, err)
		return
	}

	emailProvider := &EmailNotificationProvider{Configuration: nc.Configuration.SMTP, Logger: nc.Logger}
	ntfyTimeout := time.Duration(nc.Configuration.Ntfy.Timeout) * time.Second
	if ntfyTimeout <= 0 {
		ntfyTimeout = 15 * time.Second
	}
	ntfyProvider := &NtfyNotificationProvider{
		Configuration: nc.Configuration.Ntfy,
		Logger:        nc.Logger,
		HTTPClient:    &http.Client{Timeout: ntfyTimeout},
	}

	for _, pref := range preferences {
		if pref.EmailEnabled && pref.EmailAddress != "" && emailProvider.IsConfigured() {
			if sendErr := emailProvider.SendStreakMilestone(milestone, pref.EmailAddress); sendErr != nil {
				nc.Logger.Error().Msgf("Streak milestone: email failed: %s", sendErr)
			}
		}
		if pref.NtfyEnabled && ntfyProvider.IsConfigured() {
			if sendErr := ntfyProvider.SendStreakMilestone(milestone, &pref); sendErr != nil {
				nc.Logger.Error().Msgf("Streak milestone: ntfy failed: %s", sendErr)
			}
		}
		if pref.TelegramEnabled && pref.TelegramChatID != "" && pref.TelegramBotToken != "" {
			telegramProvider := &TelegramNotificationProvider{BotToken: pref.TelegramBotToken, Logger: nc.Logger, HTTPClient: nc.telegramClient}
			if sendErr := telegramProvider.SendStreakMilestone(milestone, pref.TelegramChatID); sendErr != nil {
				nc.Logger.Error().Msgf("Streak milestone: telegram failed: %s", sendErr)
			}
		}
	}
}

// StartAllUserTelegramPollers queries all users with a configured bot token and starts
// a long-poll goroutine for each. Called once at startup.
func (nc *NotificationController) StartAllUserTelegramPollers() {
	users, err := nc.NotificationRepo.GetAllUsersWithTelegramBotToken()
	if err != nil {
		nc.Logger.Error().Msgf("Telegram: failed to load users with bot tokens: %s", err)
		return
	}
	nc.Logger.Info().Msgf("Telegram: starting pollers for %d user(s)", len(users))
	for i := range users {
		user := &users[i]
		nc.StartUserTelegramPoller(user.ID, user.NotificationPreferences.TelegramBotToken)
	}
}

// StartUserTelegramPoller cancels any existing poller for userID, then starts a new goroutine
// that long-polls the Telegram API using botToken and handles /start <token> link commands.
func (nc *NotificationController) StartUserTelegramPoller(userID uint, botToken string) {
	nc.StopUserTelegramPoller(userID)

	ctx, cancel := context.WithCancel(context.Background())
	nc.pollerCancels.Store(userID, cancel)

	pollClient := &http.Client{Timeout: 15 * time.Second}
	baseURL := fmt.Sprintf("https://api.telegram.org/bot%s", botToken)

	go func() {
		defer nc.pollerCancels.Delete(userID)

		var offset int64

		// Resolve bot username via getMe and persist it.
		getMeURL := fmt.Sprintf("%s/getMe", baseURL)
		if resp, err := pollClient.Get(getMeURL); err == nil {
			var result struct {
				OK     bool `json:"ok"`
				Result struct {
					Username string `json:"username"`
				} `json:"result"`
			}
			if body, readErr := io.ReadAll(resp.Body); readErr == nil {
				if jsonErr := json.Unmarshal(body, &result); jsonErr == nil && result.OK && result.Result.Username != "" {
					nc.botUsernames.Store(userID, result.Result.Username)
					if dbErr := nc.NotificationRepo.SetTelegramBotUsername(userID, result.Result.Username); dbErr != nil {
						nc.Logger.Warn().Msgf("Telegram poller (user %d): failed to persist bot username: %s", userID, dbErr)
					}
					nc.Logger.Info().Msgf("Telegram poller (user %d): resolved bot username @%s", userID, result.Result.Username)
				}
			}
			_ = resp.Body.Close()
		}

		nc.Logger.Info().Msgf("Telegram poller (user %d): started", userID)

		for {
			select {
			case <-ctx.Done():
				nc.Logger.Info().Msgf("Telegram poller (user %d): stopped", userID)
				return
			default:
			}

			url := fmt.Sprintf("%s/getUpdates?timeout=10&offset=%d", baseURL, offset)
			resp, err := pollClient.Get(url)
			if err != nil {
				nc.Logger.Error().Msgf("Telegram poller (user %d): getUpdates error: %s", userID, err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
				continue
			}

			body, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				nc.Logger.Error().Msgf("Telegram poller (user %d): read error: %s", userID, readErr)
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
				nc.Logger.Error().Msgf("Telegram poller (user %d): parse error: %s", userID, jsonErr)
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
					nc.sendTelegramText(pollClient, baseURL, chatID,
						"Send `/start <token>` with the token from your Proviant notification settings to link this chat.")
					continue
				}

				token := parts[1]
				user, findErr := nc.NotificationRepo.FindUserByTelegramLinkToken(token)
				if findErr != nil || user.ID != userID {
					nc.Logger.Warn().Msgf("Telegram poller (user %d): invalid link token from chat %s", userID, chatID)
					nc.sendTelegramText(pollClient, baseURL, chatID,
						"Invalid or expired token. Please generate a new one in Proviant settings.")
					continue
				}

				if setErr := nc.NotificationRepo.SetTelegramChatID(userID, chatID); setErr != nil {
					nc.Logger.Error().Msgf("Telegram poller (user %d): failed to save chat ID: %s", userID, setErr)
					nc.sendTelegramText(pollClient, baseURL, chatID,
						"Something went wrong. Please try again.")
					continue
				}

				nc.Logger.Info().Msgf("Telegram poller (user %d): linked chat %s", userID, chatID)
				nc.sendTelegramText(pollClient, baseURL, chatID,
					"✅ Linked! You will now receive Proviant notifications here.")
			}
		}
	}()
}

// StopUserTelegramPoller cancels the long-poll goroutine for the given user, if running.
func (nc *NotificationController) StopUserTelegramPoller(userID uint) {
	if v, ok := nc.pollerCancels.Load(userID); ok {
		if cancel, ok := v.(context.CancelFunc); ok {
			cancel()
		}
		nc.pollerCancels.Delete(userID)
	}
	nc.botUsernames.Delete(userID)
}

func (nc *NotificationController) sendTelegramText(client *http.Client, baseURL, chatID, text string) {
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
