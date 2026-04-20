package models

// NotificationRecipientInfo contains recipient information for different notification providers
type NotificationRecipientInfo struct {
	EmailAddress              string
	EmailEnabled              bool
	NtfyEnabled               bool
	NtfyURL                   string
	NtfyTopic                 string
	NtfyToken                 string
	TelegramEnabled           bool
	TelegramChatID            string
	TelegramBotToken          string
	NotificationThresholdDays int
}
