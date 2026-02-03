package models

// NotificationRecipientInfo contains recipient information for different notification providers
type NotificationRecipientInfo struct {
	EmailAddress string
	NtfyURL      string
	NtfyTopic    string
	NtfyToken    string
}
