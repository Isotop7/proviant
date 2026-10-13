package models

// NotificationRecipientInfo contains recipient information for different notification providers
type NotificationRecipientInfo struct {
	EmailAddress              string
	EmailEnabled              bool
	NtfyEnabled               bool
	NtfyURL                   string
	NtfyTopic                 string
	NtfyToken                 string `json:"-"`
	TelegramEnabled           bool
	TelegramChatID            string `json:"-"`
	TelegramBotToken          string `json:"-"`
	NotificationThresholdDays int
	WebPushEnabled            bool
	WebPushSubscriptionJSON   string `json:"-"`
}
