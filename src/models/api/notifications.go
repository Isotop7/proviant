package api

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
