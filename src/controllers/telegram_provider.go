package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/models"
	dbModel "codeberg.org/isotop7/proviant/models/database"

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

func (t *TelegramNotificationProvider) SendNotification(product *dbModel.Product, recipientInfo any) error {
	chatID, ok := recipientInfo.(string)
	if !ok {
		return fmt.Errorf("invalid recipient type for telegram provider")
	}
	if chatID == "" {
		return fmt.Errorf("empty chat ID for telegram notification")
	}

	daysUntilExpiry := int(time.Until(product.ExpireAt).Hours() / 24)
	var text string
	switch {
	case daysUntilExpiry < 0:
		text = fmt.Sprintf("⚠️ *%s* has expired (%d days ago)", product.ProductName, -daysUntilExpiry)
	case daysUntilExpiry == 0:
		text = fmt.Sprintf("⚠️ *%s* expires today", product.ProductName)
	default:
		text = fmt.Sprintf("⚠️ *%s* expires in %d day(s)", product.ProductName, daysUntilExpiry)
	}

	return t.sendMessage(chatID, text)
}

// SendMonthlyWasteReport sends the monthly waste report to a Telegram chat.
func (t *TelegramNotificationProvider) SendMonthlyWasteReport(chatID string, stats *models.WasteStats) error {
	text := fmt.Sprintf(
		"📊 *Monthly Waste Report — %s*\n\n"+
			"Household: *%s*\n"+
			"Items wasted: *%d* (removed: %d, expired in pantry: %d)\n"+
			"Waste rate: *%.1f%%*\n"+
			"vs. previous month: *%s%.1f%%*",
		stats.MonthLabel,
		stats.HouseholdName,
		stats.WastedCount, stats.DeletedCount, stats.ExpiredCount,
		stats.WasteRatePct,
		stats.DeltaSymbol, stats.Delta,
	)
	return t.sendMessage(chatID, text)
}

// SendStreakMilestone sends a streak milestone notification to a Telegram chat.
func (t *TelegramNotificationProvider) SendStreakMilestone(milestone int, chatID string) error {
	text := fmt.Sprintf("*%d-day waste-free streak!*\n\nYour household has gone %d consecutive days without wasting food.", milestone, milestone)
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
	req.Header.Set("Content-Type", "application/json")

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
