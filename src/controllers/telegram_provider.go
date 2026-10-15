package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/i18n"
	"codeberg.org/isotop7/proviant/models"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
)

type TelegramNotificationProvider struct {
	BotToken   string
	Logger     *zerolog.Logger
	HTTPClient *http.Client
}

func (t *TelegramNotificationProvider) GetProviderType() string {
	return "telegram"
}

func (t *TelegramNotificationProvider) IsConfigured() bool {
	return t.BotToken != ""
}

func (t *TelegramNotificationProvider) SendNotification(product *dbModel.Product, recipientInfo any, lang string) error {
	chatID, ok := recipientInfo.(string)
	if !ok {
		return fmt.Errorf("invalid recipient type for telegram provider")
	}
	if chatID == "" {
		return fmt.Errorf("empty chat ID for telegram notification")
	}

	daysUntilExpiry := int(time.Until(product.EffectiveExpireAt()).Hours() / 24)
	var text string
	switch {
	case daysUntilExpiry < 0:
		text = i18n.T(lang, "notify.telegram.expired", product.ProductName, -daysUntilExpiry)
	case daysUntilExpiry == 0:
		text = i18n.T(lang, "notify.telegram.expiresToday", product.ProductName)
	default:
		text = i18n.T(lang, "notify.telegram.expiresInDays", product.ProductName, daysUntilExpiry)
	}

	return t.sendMessage(chatID, text)
}

// SendMonthlyWasteReport sends the monthly waste report to a Telegram chat.
func (t *TelegramNotificationProvider) SendMonthlyWasteReport(chatID string, stats *models.WasteStats, lang string) error {
	text := strings.Join([]string{
		i18n.T(lang, "notify.telegram.monthlyTitle", stats.MonthLabel),
		"",
		i18n.T(lang, "notify.telegram.monthlyHousehold", stats.HouseholdName),
		i18n.T(lang, "notify.telegram.monthlyItems", stats.WastedCount, stats.DeletedCount, stats.ExpiredCount),
		i18n.T(lang, "notify.telegram.monthlyWasteRate", fmt.Sprintf("%.1f%%", stats.WasteRatePct)),
		i18n.T(lang, "notify.telegram.monthlyDelta", stats.DeltaSymbol, fmt.Sprintf("%+.1f%%", stats.Delta)),
	}, "\n")
	return t.sendMessage(chatID, text)
}

// SendStreakMilestone sends a streak milestone notification to a Telegram chat.
func (t *TelegramNotificationProvider) SendStreakMilestone(milestone int, chatID string, lang string) error {
	text := i18n.T(lang, "notify.telegram.streakMilestoneTitle", milestone) + "\n\n" +
		i18n.T(lang, "notify.streakMilestone.body", milestone)
	return t.sendMessage(chatID, text)
}

// SendStreakReset sends a streak-reset notification to a Telegram chat.
func (t *TelegramNotificationProvider) SendStreakReset(previousStreak int, chatID string, lang string) error {
	text := i18n.T(lang, "notify.telegram.streakResetTitle") + "\n\n" +
		i18n.Tn(lang, "notify.telegram.streakResetBody", previousStreak)
	return t.sendMessage(chatID, text)
}

func (t *TelegramNotificationProvider) sendMessage(chatID, text string) error {
	payload := map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set(util.RequestHeaderContentType, "application/json")

	resp, err := t.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Logger.Error().Msgf("Error closing telegram response body: %v", closeErr)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram API returned status %s", resp.Status)
	}
	return nil
}
