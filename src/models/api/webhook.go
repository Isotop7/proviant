package api

type CreateWebhookRequest struct {
	URL    string   `json:"url" binding:"required,url"`
	Secret string   `json:"secret" binding:"required,min=16"`
	Events []string `json:"events" binding:"required,min=1"`
	Active *bool    `json:"active"`
}

type UpdateWebhookRequest struct {
	URL    string   `json:"url" binding:"omitempty,url"`
	Secret string   `json:"secret" binding:"omitempty,min=16"`
	Events []string `json:"events" binding:"omitempty,min=1"`
	Active *bool    `json:"active"`
}

type WebhookResponse struct {
	ID        uint     `json:"id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	Active    bool     `json:"active"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

type WebhookListResponse struct {
	Webhooks []WebhookResponse `json:"webhooks"`
}

type DeliveryLogResponse struct {
	ID           uint   `json:"id"`
	StatusCode   int    `json:"statusCode"`
	ResponseBody string `json:"responseBody,omitempty"`
	Error        string `json:"error,omitempty"`
	Attempt      int    `json:"attempt"`
	CreatedAt    string `json:"createdAt"`
}

type DeliveryLogListResponse struct {
	Deliveries []DeliveryLogResponse `json:"deliveries"`
}

var ValidWebhookEvents = []string{
	"product.expiring_soon",
	"product.expired",
	"product.created",
	"product.wasted",
	"household.member_joined",
}
