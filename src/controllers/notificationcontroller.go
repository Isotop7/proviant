package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	dbController "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// pollerState holds the polling state for a single user.
type pollerState struct {
	botToken string
	offset   int64
}

// telegramPollerPool manages the worker pool for polling Telegram updates for all users.
type telegramPollerPool struct {
	mu           sync.RWMutex
	users        map[uint]*pollerState
	numWorkers   int
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	ticker       *time.Ticker
	nc           *NotificationController
	pollInterval time.Duration
}

type NotificationController struct {
	Logger           *zerolog.Logger
	Configuration    *configuration.NotificationConfiguration
	NotificationRepo dbController.NotificationRepositoryInterface
	ProductRepo      dbController.ProductRepositoryInterface
	StreakRepo       dbController.StreakRepositoryInterface
	Providers        []NotificationProvider
	telegramClient   *http.Client
	telegramAPIBase  string
	pollerPool       *telegramPollerPool
	botUsernames     sync.Map // userID(uint) → resolved bot username(string)
}

type telegramResponse struct {
	OK     bool             `json:"ok"`
	Result []telegramUpdate `json:"result"`
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
}

type telegramMessage struct {
	Chat telegramChat `json:"chat"`
	Text string       `json:"text"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

const MsgEmailProviderNotConfigured = "email provider not configured"

func NewNotificationController(
	logger *zerolog.Logger,
	config *configuration.NotificationConfiguration,
	notificationRepo dbController.NotificationRepositoryInterface,
	productRepo dbController.ProductRepositoryInterface,
) *NotificationController {
	notificationController := &NotificationController{
		Logger:           logger,
		Configuration:    config,
		NotificationRepo: notificationRepo,
		ProductRepo:      productRepo,
		telegramClient:   &http.Client{Timeout: time.Duration(telegramTimeout(config)) * time.Second},
		telegramAPIBase:  "https://api.telegram.org",
	}

	notificationController.initializeProviders()

	return notificationController
}

func telegramTimeout(config *configuration.NotificationConfiguration) int {
	if config.Telegram.Timeout > 0 {
		return config.Telegram.Timeout
	}
	return 15
}

func (nc *NotificationController) newEmailProvider() *EmailNotificationProvider {
	return &EmailNotificationProvider{
		Configuration: nc.Configuration.SMTP,
		Logger:        nc.Logger,
	}
}

func (nc *NotificationController) newNtfyProvider() *NtfyNotificationProvider {
	timeout := time.Duration(nc.Configuration.Ntfy.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &NtfyNotificationProvider{
		Configuration: nc.Configuration.Ntfy,
		Logger:        nc.Logger,
		HTTPClient:    &http.Client{Timeout: timeout},
	}
}

func (nc *NotificationController) newTelegramProvider(botToken string) *TelegramNotificationProvider {
	return &TelegramNotificationProvider{
		BotToken:   botToken,
		Logger:     nc.Logger,
		HTTPClient: nc.telegramClient,
	}
}

func (nc *NotificationController) newWebPushProvider() *WebPushNotificationProvider {
	return &WebPushNotificationProvider{
		NotificationRepo: nc.NotificationRepo,
		Logger:           nc.Logger,
	}
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
	emailProvider := nc.newEmailProvider()
	if emailProvider.IsConfigured() {
		nc.Providers = append(nc.Providers, emailProvider)
	}

	// Add ntfy provider if configured
	ntfyProvider := nc.newNtfyProvider()
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
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			webhookService := GetWebhookService()
			if webhookService == nil {
				return
			}
			if expired {
				daysUntilExpiry := int(time.Since(p.ExpireAt).Hours() / 24)
				webhookService.FireEventContext(ctx, "product.expired", map[string]any{
					"id":              p.ID,
					"productName":     p.ProductName,
					"daysUntilExpiry": daysUntilExpiry,
					"householdId":     p.HouseholdID,
				})
			}
			if expiringSoon {
				daysUntilExpiry := int(time.Until(p.ExpireAt).Hours() / 24)
				webhookService.FireEventContext(ctx, "product.expiring_soon", map[string]any{
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
		for i := range preferences {
			nc.sendNotificationsForRecipient(product, &preferences[i])
		}
	}
}

func (nc *NotificationController) sendNotificationsForRecipient(product *dbModel.Product, recipientInfo *models.NotificationRecipientInfo) {
	if !nc.isWithinNotificationThreshold(product, recipientInfo) {
		return
	}

	notified := false

	for _, provider := range nc.Providers {
		if err := nc.sendViaProvider(provider, product, recipientInfo); err != nil {
			nc.Logger.Error().Msgf("Failed to send %s notification: %s", provider.GetProviderType(), err.Error())
			continue
		}
		nc.Logger.Info().Msgf("Successfully sent %s notification", provider.GetProviderType())
		if !notified {
			notified = nc.markNotified(product.ID)
		}
	}

	if nc.sendTelegram(product, recipientInfo) && !notified {
		notified = nc.markNotified(product.ID)
	}

	if nc.sendWebPush(product, recipientInfo) && !notified {
		notified = nc.markNotified(product.ID)
	}

	if !notified {
		nc.Logger.Warn().Msgf("Failed to send any notifications for product '%s' (ID: %d)",
			product.ProductName, product.ID)
	}
}

func (nc *NotificationController) isWithinNotificationThreshold(product *dbModel.Product, recipientInfo *models.NotificationRecipientInfo) bool {
	var threshold int
	if product.NotificationLeadDays != nil {
		threshold = *product.NotificationLeadDays
	} else {
		threshold = recipientInfo.NotificationThresholdDays
	}
	cutoff := time.Now().AddDate(0, 0, threshold)
	if product.ExpireAt.After(cutoff) {
		nc.Logger.Debug().Msgf("Product '%s' (ID: %d) not yet within threshold for recipient, skipping",
			product.ProductName, product.ID)
		return false
	}
	return true
}

func (nc *NotificationController) sendViaProvider(provider NotificationProvider, product *dbModel.Product, recipientInfo *models.NotificationRecipientInfo) error {
	switch provider.GetProviderType() {
	case "email":
		if recipientInfo.EmailEnabled && recipientInfo.EmailAddress != "" {
			nc.Logger.Info().Msgf("Attempting email notification for product '%s' (ID: %d)", product.ProductName, product.ID)
			return provider.SendNotification(product, recipientInfo.EmailAddress)
		}
	case "ntfy":
		if recipientInfo.NtfyEnabled {
			nc.Logger.Info().Msgf("Attempting ntfy notification for product '%s' (ID: %d)", product.ProductName, product.ID)
			return provider.SendNotification(product, recipientInfo)
		}
	}
	return nil
}

func (nc *NotificationController) sendTelegram(product *dbModel.Product, recipientInfo *models.NotificationRecipientInfo) bool {
	if recipientInfo.TelegramEnabled && recipientInfo.TelegramChatID != "" && recipientInfo.TelegramBotToken != "" {
		nc.Logger.Info().Msgf("Attempting telegram notification for product '%s' (ID: %d)", product.ProductName, product.ID)
		telegramProvider := nc.newTelegramProvider(recipientInfo.TelegramBotToken)
		if err := telegramProvider.SendNotification(product, recipientInfo.TelegramChatID); err != nil {
			nc.Logger.Error().Msgf("Failed to send telegram notification: %s", err)
			return false
		}
		nc.Logger.Info().Msg("Successfully sent telegram notification")
		return true
	}
	return false
}

func (nc *NotificationController) sendWebPush(product *dbModel.Product, recipientInfo *models.NotificationRecipientInfo) bool {
	if recipientInfo.WebPushEnabled && recipientInfo.WebPushSubscriptionJSON != "" {
		nc.Logger.Info().Msgf("Attempting webpush notification for product '%s' (ID: %d)", product.ProductName, product.ID)
		webPushProvider := nc.newWebPushProvider()
		if webPushProvider == nil {
			return false
		}
		if err := webPushProvider.SendNotification(product, recipientInfo.WebPushSubscriptionJSON); err != nil {
			nc.Logger.Error().Msgf("Failed to send webpush notification: %s", err)
			return false
		}
		nc.Logger.Info().Msg("Successfully sent webpush notification")
		return true
	}
	return false
}

func (nc *NotificationController) markNotified(productID uint) bool {
	if err := nc.NotificationRepo.SetProductNotifiedAt(productID); err != nil {
		nc.Logger.Error().Msg(err.Error())
		return false
	}
	nc.Logger.Info().Msg("Property NotifiedAt was updated")
	return true
}

// DispatchInvitations starts a background goroutine that periodically retries sending pending invitation emails.
// It runs once immediately on startup, then every Interval hours (reusing the same config as product notifications).
func (nc *NotificationController) DispatchInvitations(baseURL string) {
	emailProvider := nc.newEmailProvider()

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
// If tx is provided (non-nil), the "mark as sent" update will run within that transaction to avoid
// SQLite "database is locked" conflicts when the transaction holds a write lock.
func (nc *NotificationController) SendInvitationEmail(invitation *dbModel.HouseholdInvitation, inviterName, householdName, baseURL string, tx *gorm.DB) error {
	emailProvider := nc.newEmailProvider()

	if !emailProvider.IsConfigured() {
		return errors.New(MsgEmailProviderNotConfigured)
	}

	if err := emailProvider.SendInvitationEmail(invitation, inviterName, householdName, baseURL); err != nil {
		nc.Logger.Error().Msgf("Failed to send invitation email to %s: %s", invitation.Email, err)
		if markErr := nc.NotificationRepo.MarkInvitationSendFailed(invitation.ID); markErr != nil {
			nc.Logger.Error().Msgf("Failed to mark invitation %d as send-failed: %s", invitation.ID, markErr)
		}
		return err
	}

	if tx != nil {
		if err := nc.NotificationRepo.MarkInvitationSentTx(tx, invitation.ID); err != nil {
			nc.Logger.Error().Msgf("Failed to mark invitation %d as sent: %s", invitation.ID, err)
			return err
		}
	} else {
		if err := nc.NotificationRepo.MarkInvitationSent(invitation.ID); err != nil {
			nc.Logger.Error().Msgf("Failed to mark invitation %d as sent: %s", invitation.ID, err)
			return err
		}
	}

	nc.Logger.Info().Msgf("Invitation email sent successfully to %s", invitation.Email)
	return nil
}

// SendVerificationEmail sends an email verification link using the invitation email system.
func (nc *NotificationController) SendVerificationEmail(invitation *dbModel.HouseholdInvitation, username, baseURL string) error {
	emailProvider := nc.newEmailProvider()

	if !emailProvider.IsConfigured() {
		return errors.New(MsgEmailProviderNotConfigured)
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
	emailProvider := nc.newEmailProvider()

	if !emailProvider.IsConfigured() {
		return errors.New(MsgEmailProviderNotConfigured)
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
	emailProvider := nc.newEmailProvider()

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

		nc.sendMonthlyWasteReportToEmailRecipients(emailProvider, target.Recipients, &stats, target.HouseholdID)
		nc.sendMonthlyWasteReportToTelegramRecipients(target.TelegramRecipients, &stats, target.HouseholdID)
	}
}

func (nc *NotificationController) sendMonthlyWasteReportToEmailRecipients(
	provider *EmailNotificationProvider,
	recipients []string,
	stats *models.WasteStats,
	householdID uint,
) {
	if !provider.IsConfigured() {
		return
	}
	for _, recipient := range recipients {
		if sendErr := provider.SendMonthlyWasteReport(recipient, stats); sendErr != nil {
			nc.Logger.Error().Msgf("Monthly waste report: email send failed to %s: %s", recipient, sendErr)
		} else {
			nc.Logger.Info().Msgf("Monthly waste report: email sent to %s (household %d)", recipient, householdID)
		}
	}
}

func (nc *NotificationController) sendMonthlyWasteReportToTelegramRecipients(
	recipients []models.TelegramRecipient,
	stats *models.WasteStats,
	householdID uint,
) {
	for _, tr := range recipients {
		telegramProvider := &TelegramNotificationProvider{BotToken: tr.BotToken, Logger: nc.Logger, HTTPClient: nc.telegramClient}
		if sendErr := telegramProvider.SendMonthlyWasteReport(tr.ChatID, stats); sendErr != nil {
			nc.Logger.Error().Msgf("Monthly waste report: telegram send failed to chat %s: %s", tr.ChatID, sendErr)
		} else {
			nc.Logger.Info().Msgf("Monthly waste report: telegram sent to chat %s (household %d)", tr.ChatID, householdID)
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
	if nc.StreakRepo == nil {
		nc.Logger.Warn().Msg("StreakRepo not set, skipping streak updates")
		return
	}
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
		nc.processHouseholdStreak(streak, now, milestones)
		if updateErr := nc.StreakRepo.UpdateStreak(streak); updateErr != nil {
			nc.Logger.Error().Msgf("Streak updater: failed to save streak for household %d: %s", streak.HouseholdID, updateErr)
		} else {
			nc.Logger.Info().Msgf("Streak updater: household %d streak = %d", streak.HouseholdID, streak.CurrentStreak)
		}
	}
}

func (nc *NotificationController) processHouseholdStreak(streak *dbModel.WasteStreak, now time.Time, milestones []int) {
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
}

func (nc *NotificationController) sendStreakMilestoneNotifications(householdID uint, milestone int) {
	preferences, err := nc.NotificationRepo.GetHouseholdMembersNotificationPreferences(householdID)
	if err != nil {
		nc.Logger.Error().Msgf("Streak milestone: failed to get preferences for household %d: %s", householdID, err)
		return
	}

	emailProvider := nc.newEmailProvider()
	ntfyProvider := nc.newNtfyProvider()

	for i := range preferences {
		nc.sendStreakEmailIfEnabled(milestone, &preferences[i], emailProvider)
		nc.sendStreakNtfyIfEnabled(milestone, &preferences[i], ntfyProvider)
		nc.sendStreakTelegramIfEnabled(milestone, &preferences[i])
	}
}

func (nc *NotificationController) sendStreakEmailIfEnabled(
	milestone int,
	pref *models.NotificationRecipientInfo,
	provider *EmailNotificationProvider,
) {
	if pref.EmailEnabled && pref.EmailAddress != "" && provider.IsConfigured() {
		if sendErr := provider.SendStreakMilestone(milestone, pref.EmailAddress); sendErr != nil {
			nc.Logger.Error().Msgf("Streak milestone: email failed: %s", sendErr)
		}
	}
}

func (nc *NotificationController) sendStreakNtfyIfEnabled(
	milestone int,
	pref *models.NotificationRecipientInfo,
	provider *NtfyNotificationProvider,
) {
	if pref.NtfyEnabled && provider.IsConfigured() {
		if sendErr := provider.SendStreakMilestone(milestone, pref); sendErr != nil {
			nc.Logger.Error().Msgf("Streak milestone: ntfy failed: %s", sendErr)
		}
	}
}

func (nc *NotificationController) sendStreakTelegramIfEnabled(
	milestone int,
	pref *models.NotificationRecipientInfo,
) {
	if pref.TelegramEnabled && pref.TelegramChatID != "" && pref.TelegramBotToken != "" {
		telegramProvider := nc.newTelegramProvider(pref.TelegramBotToken)
		if sendErr := telegramProvider.SendStreakMilestone(milestone, pref.TelegramChatID); sendErr != nil {
			nc.Logger.Error().Msgf("Streak milestone: telegram failed: %s", sendErr)
		}
	}
}

// StartDigestScheduler starts a goroutine that checks every minute whether
// any household's digest is due, and sends expiry digest emails.
func (nc *NotificationController) StartMailDigestScheduler(baseURL string) {
	if !nc.Configuration.MailDigest.Enabled {

		nc.Logger.Info().Msg("Mail digest scheduler: disabled by config")

		return
	}

	defaultTime := nc.Configuration.MailDigest.DefaultTime
	if defaultTime == "" {
		defaultTime = "08:00"
	}

	go func() {
		for {
			nc.processMailDigestEmails(baseURL, defaultTime)
			time.Sleep(1 * time.Minute)
		}
	}()
}

func (nc *NotificationController) processMailDigestEmails(baseURL, defaultTime string) {
	targets, err := nc.NotificationRepo.GetHouseholdsWithMailDigestEnabled()
	if err != nil {
		nc.Logger.Error().Msgf("Digest scheduler: failed to fetch households: %s", err)
		return
	}

	now := time.Now()

	for i := range targets {
		target := &targets[i]
		for j := range target.Users {
			user := &target.Users[j]
			if !nc.shouldSendDigestNow(user.MailDigestFrequency, defaultTime, now) {
				continue
			}

			productGroups, prodErr := nc.ProductRepo.GetExpiringProductsForMailDigest(target.HouseholdID)
			if prodErr != nil {
				nc.Logger.Error().Msgf("Digest scheduler: failed to get products for household %d: %s", target.HouseholdID, prodErr)
				continue
			}

			if len(productGroups.Today) == 0 && len(productGroups.ThisWeek) == 0 && len(productGroups.NextWeek) == 0 {
				continue
			}

			unsubscribeURL := ""
			if user.MailDigestToken != "" {
				unsubscribeURL = fmt.Sprintf("%s/web/unsubscribe?token=%s", baseURL, user.MailDigestToken)
			}

			emailProvider := nc.newEmailProvider()
			if !emailProvider.IsConfigured() {
				continue
			}

			digestGroups := &digestProductGroupAdapter{group: productGroups}
			if sendErr := emailProvider.SendExpiryDigestEmail(digestGroups, user.Email, target.HouseholdName, unsubscribeURL); sendErr != nil {
				nc.Logger.Error().Msgf("Digest scheduler: failed to send digest to %s: %s", user.Email, sendErr)
			} else {
				nc.Logger.Info().Msgf("Digest scheduler: sent digest to %s (household %d)", user.Email, target.HouseholdID)
			}
		}
	}
}

type digestProductGroupAdapter struct {
	group dbController.MailDigestProductGroup
}

func (a *digestProductGroupAdapter) GetToday() []dbModel.Product    { return a.group.Today }
func (a *digestProductGroupAdapter) GetThisWeek() []dbModel.Product { return a.group.ThisWeek }
func (a *digestProductGroupAdapter) GetNextWeek() []dbModel.Product { return a.group.NextWeek }

func (nc *NotificationController) shouldSendDigestNow(frequency, defaultTime string, now time.Time) bool {
	hour, min, _ := now.Clock()
	currentMins := hour*60 + min

	parsedHour, parsedMin, _ := parseTime(defaultTime)
	targetMins := parsedHour*60 + parsedMin

	switch frequency {
	case authentication.MailDigestFrequencyDaily:
		return currentMins == targetMins
	case authentication.MailDigestFrequencyWeekly:
		if now.Weekday() != time.Monday {
			return false
		}
		return currentMins == targetMins
	default:
		return false
	}
}

func parseTime(t string) (hour, min, sec int) {
	parts := strings.Split(t, ":")
	if len(parts) >= 2 {
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		return h, m, 0
	}
	return 8, 0, 0
}

// StartAllUserTelegramPollers queries all users with a configured bot token and starts
// the worker pool for Telegram polling. Called once at startup.
func (nc *NotificationController) StartAllUserTelegramPollers() {
	nc.StartTelegramPollerPool()
}

// StartTelegramPollerPool initializes the worker pool and registers all users with bot tokens.
func (nc *NotificationController) StartTelegramPollerPool() {
	users, err := nc.NotificationRepo.GetAllUsersWithTelegramBotToken()
	if err != nil {
		nc.Logger.Error().Msgf("Telegram: failed to load users with bot tokens: %s", err)
		return
	}

	numWorkers := nc.Configuration.Telegram.PollerWorkers
	if numWorkers <= 0 {
		numWorkers = 10
	}

	nc.pollerPool = &telegramPollerPool{
		users:        make(map[uint]*pollerState),
		numWorkers:   numWorkers,
		nc:           nc,
		pollInterval: 2 * time.Second,
		ticker:       time.NewTicker(2 * time.Second),
	}
	nc.pollerPool.ctx, nc.pollerPool.cancel = context.WithCancel(context.Background()) //nolint:gosec // cancel stored and called in StopTelegramPollerPool

	nc.Logger.Info().Msgf("Telegram: starting poller pool with %d worker(s) for %d user(s)", numWorkers, len(users))

	for i := range users {
		user := &users[i]
		nc.registerUserInPool(user.ID, user.NotificationPreferences.TelegramBotToken)
	}

	nc.pollerPool.wg.Add(numWorkers)
	for workerID := 0; workerID < numWorkers; workerID++ {
		go nc.poolWorker(workerID)
	}
}

// StartUserTelegramPoller registers or updates a user in the pool and resolves the bot username.
func (nc *NotificationController) StartUserTelegramPoller(userID uint, botToken string) {
	if nc.pollerPool == nil {
		nc.StartTelegramPollerPool()
	}
	nc.registerUserInPool(userID, botToken)
}

// registerUserInPool adds or updates a user in the pool and resolves their bot username.
func (nc *NotificationController) registerUserInPool(userID uint, botToken string) {
	baseURL := fmt.Sprintf("%s/bot%s", nc.telegramAPIBase, botToken)
	nc.resolveTelegramBotUsername(baseURL, userID)

	nc.pollerPool.mu.Lock()
	defer nc.pollerPool.mu.Unlock()
	nc.pollerPool.users[userID] = &pollerState{botToken: botToken}
}

// StopUserTelegramPoller removes a user from the pool.
func (nc *NotificationController) StopUserTelegramPoller(userID uint) {
	if nc.pollerPool == nil {
		return
	}
	nc.pollerPool.mu.Lock()
	delete(nc.pollerPool.users, userID)
	nc.pollerPool.mu.Unlock()
	nc.botUsernames.Delete(userID)
}

// StopTelegramPollerPool stops all workers and cleans up.
func (nc *NotificationController) StopTelegramPollerPool() {
	if nc.pollerPool == nil {
		return
	}
	nc.pollerPool.cancel()
	nc.pollerPool.ticker.Stop()
	nc.pollerPool.wg.Wait()
	nc.pollerPool = nil
}

// poolWorker runs as a single worker goroutine, polling assigned users on each tick.
func (nc *NotificationController) poolWorker(workerID int) {
	defer nc.pollerPool.wg.Done()
	nc.Logger.Info().Msgf("Telegram: worker %d started", workerID)

	for {
		select {
		case <-nc.pollerPool.ctx.Done():
			nc.Logger.Info().Msgf("Telegram: worker %d stopped", workerID)
			return
		case <-nc.pollerPool.ticker.C:
			nc.processPoolUsers(workerID)
		}
	}
}

// processPoolUsers iterates the pool and processes users assigned to this worker.
func (nc *NotificationController) processPoolUsers(workerID int) {
	nc.pollerPool.mu.RLock()
	users := make([]uint, 0, len(nc.pollerPool.users))
	for userID := range nc.pollerPool.users {
		if userID%uint(nc.pollerPool.numWorkers) == uint(workerID) { //nolint:gosec // numWorkers validated >0 above, workerID >=0 from loop
			users = append(users, userID)
		}
	}
	nc.pollerPool.mu.RUnlock()

	for _, userID := range users {
		nc.pollerPool.mu.RLock()
		state, exists := nc.pollerPool.users[userID]
		nc.pollerPool.mu.RUnlock()

		if !exists {
			continue
		}

		baseURL := fmt.Sprintf("%s/bot%s", nc.telegramAPIBase, state.botToken)
		nc.pollUser(userID, baseURL, state)
	}
}

// pollUser performs a single getUpdates call for a user and handles any updates.
func (nc *NotificationController) pollUser(userID uint, baseURL string, state *pollerState) {
	nc.pollerPool.mu.RLock()
	currentOffset := state.offset
	nc.pollerPool.mu.RUnlock()

	url := fmt.Sprintf("%s/getUpdates?timeout=1&offset=%d", baseURL, currentOffset)
	resp, err := nc.telegramClient.Get(url)
	if err != nil {
		nc.Logger.Error().Msgf("Telegram poller (user %d): getUpdates error: %s", userID, err)
		return
	}

	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		nc.Logger.Error().Msgf("Telegram poller (user %d): read error: %s", userID, readErr)
		return
	}

	var result telegramResponse
	if jsonErr := json.Unmarshal(body, &result); jsonErr != nil {
		nc.Logger.Error().Msgf("Telegram poller (user %d): parse error: %s", userID, jsonErr)
		return
	}

	for _, update := range result.Result {
		nc.pollerPool.mu.Lock()
		state.offset = update.UpdateID + 1
		nc.pollerPool.mu.Unlock()

		if update.Message == nil {
			continue
		}

		text := strings.TrimSpace(update.Message.Text)
		chatID := fmt.Sprintf("%d", update.Message.Chat.ID)
		nc.handleTelegramStartCommand(text, chatID, baseURL, userID)
	}
}

// resolveTelegramBotUsername resolves and caches the bot username for a user.
func (nc *NotificationController) resolveTelegramBotUsername(baseURL string, userID uint) {
	getMeURL := fmt.Sprintf("%s/getMe", baseURL)
	if resp, err := nc.telegramClient.Get(getMeURL); err == nil {
		var result struct {
			OK     bool `json:"ok"`
			Result struct {
				Username string `json:"username"`
			} `json:"result"`
		}
		if body, readErr := io.ReadAll(resp.Body); readErr == nil {
			if jsonErr := json.Unmarshal(body, &result); jsonErr == nil && result.OK && result.Result.Username != "" {
				nc.botUsernames.Store(userID, result.Result.Username)
				if err := nc.NotificationRepo.SetTelegramBotUsername(userID, result.Result.Username); err != nil {
					nc.Logger.Warn().Msgf("Telegram poller (user %d): failed to persist bot username: %s", userID, err)
				}
				nc.Logger.Info().Msgf("Telegram poller (user %d): resolved bot username @%s", userID, result.Result.Username)
			}
		}
		_ = resp.Body.Close()
	}
}

func (nc *NotificationController) handleTelegramStartCommand(text, chatID, baseURL string, userID uint) bool {
	if !strings.HasPrefix(text, "/start") {
		return false
	}

	parts := strings.Fields(text)
	if len(parts) < 2 {
		nc.sendTelegramText(nc.telegramClient, baseURL, chatID,
			"Send `/start <token>` with the token from your Proviant notification settings to link this chat.")
		return true
	}

	token := parts[1]
	user, findErr := nc.NotificationRepo.FindUserByTelegramLinkToken(token)
	if findErr != nil || user.ID != userID {
		nc.Logger.Warn().Msgf("Telegram poller (user %d): invalid link token from chat %s", userID, chatID)
		nc.sendTelegramText(nc.telegramClient, baseURL, chatID,
			"Invalid or expired token. Please generate a new one in Proviant settings.")
		return true
	}

	if setErr := nc.NotificationRepo.SetTelegramChatID(userID, chatID); setErr != nil {
		nc.Logger.Error().Msgf("Telegram poller (user %d): failed to save chat ID: %s", userID, setErr)
		nc.sendTelegramText(nc.telegramClient, baseURL, chatID,
			"Something went wrong. Please try again.")
		return true
	}

	nc.Logger.Info().Msgf("Telegram poller (user %d): linked chat %s", userID, chatID)
	nc.sendTelegramText(nc.telegramClient, baseURL, chatID,
		"✅ Linked! You will now receive Proviant notifications here.")
	return true
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
		inviterName := nc.resolveInviterName(invitation.InviterID)
		householdName := nc.resolveHouseholdName(invitation.HouseholdID)
		nc.dispatchPendingInvitation(emailProvider, invitation, inviterName, householdName, baseURL)
	}
}

func (nc *NotificationController) resolveInviterName(inviterID uint) string {
	user, err := nc.NotificationRepo.GetUserByID(inviterID)
	if err != nil {
		return "A household member"
	}
	return user.EffectiveName()
}

func (nc *NotificationController) resolveHouseholdName(householdID uint) string {
	household, err := nc.NotificationRepo.GetHouseholdByID(householdID)
	if err != nil {
		return fmt.Sprintf("Household #%d", householdID)
	}
	return household.Name
}

func (nc *NotificationController) dispatchPendingInvitation(
	emailProvider *EmailNotificationProvider,
	invitation *dbModel.HouseholdInvitation,
	inviterName, householdName, baseURL string,
) {
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
