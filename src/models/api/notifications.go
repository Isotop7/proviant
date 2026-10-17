package api

import "github.com/Isotop7/proviant/models/authentication"

// NotificationItem represents a single actionable notification entry.
type NotificationItem struct {
	ID        uint   `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
}

// NotificationsResponse wraps the notification items and a total count.
type NotificationsResponse struct {
	Total int                `json:"total"`
	Items []NotificationItem `json:"items"`
}

// UpdateNotificationPreferencesRequest is the write shape of the notification
// preferences. The two secrets carry normal json tags so the body binds them;
// everything managed elsewhere (chat id, link token, bot username, web push
// subscription) is absent and survives a save untouched.
type UpdateNotificationPreferencesRequest struct {
	EmailEnabled              bool   `json:"emailEnabled"`
	NtfyEnabled               bool   `json:"ntfyEnabled"`
	NtfyURL                   string `json:"ntfyUrl,omitempty"`
	NtfyTopic                 string `json:"ntfyTopic,omitempty"`
	NtfyToken                 string `json:"ntfyToken,omitempty"`
	NotificationThresholdDays int    `json:"notificationThresholdDays"`
	MonthlyWasteReportEnabled bool   `json:"monthlyWasteReportEnabled"`
	TelegramEnabled           bool   `json:"telegramEnabled"`
	TelegramBotToken          string `json:"telegramBotToken"`
	WebPushEnabled            bool   `json:"webPushEnabled"`
	MailDigestFrequency       string `json:"mailDigestFrequency"`
}

// ToNotificationPreferences projects the request onto the stored preferences.
func (r *UpdateNotificationPreferencesRequest) ToNotificationPreferences() authentication.NotificationPreferences {
	return authentication.NotificationPreferences{
		EmailEnabled:              r.EmailEnabled,
		NtfyEnabled:               r.NtfyEnabled,
		NtfyURL:                   r.NtfyURL,
		NtfyTopic:                 r.NtfyTopic,
		NtfyToken:                 r.NtfyToken,
		NotificationThresholdDays: r.NotificationThresholdDays,
		MonthlyWasteReportEnabled: r.MonthlyWasteReportEnabled,
		TelegramEnabled:           r.TelegramEnabled,
		TelegramBotToken:          r.TelegramBotToken,
		WebPushEnabled:            r.WebPushEnabled,
		MailDigestFrequency:       r.MailDigestFrequency,
	}
}

// NotificationPreferencesResponse is the read shape of the notification
// preferences. Secrets never leave the server; the configured booleans stand in
// for them.
type NotificationPreferencesResponse struct {
	EmailEnabled              bool   `json:"emailEnabled"`
	NtfyEnabled               bool   `json:"ntfyEnabled"`
	NtfyURL                   string `json:"ntfyUrl,omitempty"`
	NtfyTopic                 string `json:"ntfyTopic,omitempty"`
	NtfyTokenConfigured       bool   `json:"ntfyTokenConfigured"`
	NotificationThresholdDays int    `json:"notificationThresholdDays"`
	MonthlyWasteReportEnabled bool   `json:"monthlyWasteReportEnabled"`
	TelegramEnabled           bool   `json:"telegramEnabled"`
	TelegramLinked            bool   `json:"telegramLinked"`
	TelegramBotConfigured     bool   `json:"telegramBotConfigured"`
	WebPushEnabled            bool   `json:"webPushEnabled"`
	MailDigestFrequency       string `json:"mailDigestFrequency"`
}

// NewNotificationPreferencesResponse projects the stored preferences onto the
// wire shape.
func NewNotificationPreferencesResponse(prefs *authentication.NotificationPreferences) NotificationPreferencesResponse {
	return NotificationPreferencesResponse{
		EmailEnabled:              prefs.EmailEnabled,
		NtfyEnabled:               prefs.NtfyEnabled,
		NtfyURL:                   prefs.NtfyURL,
		NtfyTopic:                 prefs.NtfyTopic,
		NtfyTokenConfigured:       prefs.NtfyToken != "",
		NotificationThresholdDays: prefs.NotificationThresholdDays,
		MonthlyWasteReportEnabled: prefs.MonthlyWasteReportEnabled,
		TelegramEnabled:           prefs.TelegramEnabled,
		TelegramLinked:            prefs.TelegramChatID != "",
		TelegramBotConfigured:     prefs.TelegramBotToken != "",
		WebPushEnabled:            prefs.WebPushEnabled,
		MailDigestFrequency:       prefs.MailDigestFrequency,
	}
}
