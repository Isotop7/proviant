package controllers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"codeberg.org/isotop7/proviant/controllers/database"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

var (
	webhookService *WebhookService
	webhookOnce    sync.Once
)

// webhookDeliveryTimeout bounds a single webhook's full delivery (all retry
// attempts) and is threaded into the HTTP request, so a slow endpoint cannot
// pin a goroutine indefinitely.
const webhookDeliveryTimeout = 10 * time.Second

type WebhookService struct {
	DB         *gorm.DB
	HTTPClient *http.Client
	Logger     *zerolog.Logger
	Repo       *database.WebhookRepository
}

func InitWebhookService(db *gorm.DB, logger *zerolog.Logger) {
	webhookOnce.Do(func() {
		webhookService = &WebhookService{
			DB:         db,
			HTTPClient: &http.Client{Timeout: 10 * time.Second},
			Logger:     logger,
			Repo:       database.NewWebhookRepository(db),
		}
	})
}

func GetWebhookService() *WebhookService {
	return webhookService
}

func (s *WebhookService) FireEvent(event string, payload map[string]any) {
	s.FireEventContext(context.Background(), event, payload)
}

func (s *WebhookService) FireEventContext(ctx context.Context, event string, payload map[string]any) {
	webhooks, err := s.Repo.GetActiveWebhooksByEvent(event)
	if err != nil {
		s.Logger.Error().Msgf("Error fetching webhooks for event %s: %v", event, err)
		return
	}

	// Each delivery gets its own timeout context derived from ctx: one slow
	// webhook cannot consume a shared budget, and this function returning does
	// not abort deliveries still in flight. The context is threaded into the
	// HTTP request, so the timeout actually bounds the delivery.
	for i := range webhooks {
		go func(webhook *dbModel.Webhook) {
			dctx, cancel := context.WithTimeout(ctx, webhookDeliveryTimeout)
			defer cancel()
			s.deliverWebhook(dctx, webhook, event, payload)
		}(&webhooks[i])
	}
}

func (s *WebhookService) deliverWebhook(ctx context.Context, webhook *dbModel.Webhook, event string, payload map[string]any) {
	select {
	case <-ctx.Done():
		s.Logger.Warn().Msgf("Webhook delivery timed out for event %s to URL %s", event, webhook.URL)
		return
	default:
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		s.Logger.Error().Msgf("Error marshaling webhook payload: %v", err)
		return
	}

	signature := s.computeSignature(webhook.Secret, payloadBytes)

	for attempt := 1; attempt <= 3; attempt++ {
		statusCode, responseBody, deliveryErr := s.doDelivery(ctx, webhook.URL, payloadBytes, signature)

		deliveryLog := dbModel.WebhookDeliveryLog{
			WebhookID:    webhook.ID,
			StatusCode:   statusCode,
			ResponseBody: truncateString(responseBody, 1024),
			Attempt:      attempt,
		}
		if deliveryErr != nil {
			deliveryLog.Error = deliveryErr.Error()
		}

		if err := s.Repo.CreateDeliveryLog(&deliveryLog); err != nil {
			s.Logger.Error().Msgf("Error creating delivery log: %v", err)
		}

		if deliveryErr == nil && statusCode >= 200 && statusCode < 300 {
			return
		}

		if attempt < 3 {
			select {
			case <-ctx.Done():
				s.Logger.Warn().Msgf("Webhook delivery cancelled for event %s to URL %s", event, webhook.URL)
				return
			case <-time.After(time.Duration(1<<(attempt-1)) * time.Second):
			}
		}
	}
}

func (s *WebhookService) doDelivery(ctx context.Context, url string, payload []byte, signature string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return 0, "", err
	}

	req.Header.Set(util.RequestHeaderContentType, "application/json")
	req.Header.Set("X-Proviant-Signature", "sha256="+signature)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			s.Logger.Error().Msgf("Error closing response body: %v", closeErr)
		}
	}()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return resp.StatusCode, "", err
	}

	return resp.StatusCode, string(bodyBytes), nil
}

func (s *WebhookService) computeSignature(secret string, payload []byte) string {
	hmacHash := hmac.New(sha256.New, []byte(secret))
	hmacHash.Write(payload)
	return hex.EncodeToString(hmacHash.Sum(nil))
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func ParseWebhookEvents(eventsJSON string) []string {
	eventsJSON = strings.TrimSpace(eventsJSON)
	if len(eventsJSON) < 2 {
		return nil
	}
	if eventsJSON[0] == '[' && eventsJSON[len(eventsJSON)-1] == ']' {
		eventsJSON = eventsJSON[1 : len(eventsJSON)-1]
	}
	var events []string
	parts := strings.Split(eventsJSON, ",")
	for _, part := range parts {
		event := strings.Trim(strings.Trim(part, ` `), `"`)
		if event != "" {
			events = append(events, event)
		}
	}
	return events
}
