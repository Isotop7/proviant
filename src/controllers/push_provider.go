package controllers

import (
	"encoding/json"
	"fmt"

	"codeberg.org/isotop7/proviant/models/database"
	"github.com/SherClockHolmes/webpush-go"
	"github.com/rs/zerolog"
)

type WebPushKeyProvider interface {
	GetVAPIDKeys() (publicKey, privateKey string, err error)
}

type WebPushNotificationProvider struct {
	NotificationRepo WebPushKeyProvider
	VAPIDPublicKey    string
	VAPIDPrivateKey   string
	Logger            *zerolog.Logger
}

func (p *WebPushNotificationProvider) GetProviderType() string {
	return "webpush"
}

func (p *WebPushNotificationProvider) IsConfigured() bool {
	return p.VAPIDPublicKey != "" && p.VAPIDPrivateKey != ""
}

type webPushMessage struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Tag   string `json:"tag"`
	URL   string `json:"url"`
}

func (p *WebPushNotificationProvider) ensureVAPIDKeys() error {
	if p.VAPIDPublicKey != "" && p.VAPIDPrivateKey != "" {
		return nil
	}

	publicKey, privateKey, err := p.NotificationRepo.GetVAPIDKeys()
	if err != nil {
		return fmt.Errorf("failed to get VAPID keys: %w", err)
	}
	p.VAPIDPublicKey = publicKey
	p.VAPIDPrivateKey = privateKey
	return nil
}

func (p *WebPushNotificationProvider) SendNotification(product *database.Product, recipientInfo any) error {
	if err := p.ensureVAPIDKeys(); err != nil {
		return err
	}

	subscription, ok := recipientInfo.(string)
	if !ok {
		return fmt.Errorf("invalid recipient type for webpush provider")
	}

	var sub webpush.Subscription
	if unmarshalErr := json.Unmarshal([]byte(subscription), &sub); unmarshalErr != nil {
		return fmt.Errorf("invalid subscription JSON: %w", unmarshalErr)
	}

	days := formatWebPushExpiryDays(product.ExpireAt)
	msg := webPushMessage{
		Title: fmt.Sprintf("⚠️ %s", product.ProductName),
		Body:  fmt.Sprintf("expires in %s", days),
		Tag:   "proviant-notification",
		URL:   "/web/products",
	}

	payload, payloadErr := json.Marshal(msg)
	if payloadErr != nil {
		return payloadErr
	}

	resp, err := webpush.SendNotification(payload, &sub, &webpush.Options{
		VAPIDPublicKey:  p.VAPIDPublicKey,
		VAPIDPrivateKey: p.VAPIDPrivateKey,
		TTL:             86400,
	})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webpush notification failed with status: %s", resp.Status)
	}

	return nil
}

func formatWebPushExpiryDays(expireAt any) string {
	switch v := expireAt.(type) {
	case string:
		return v
	default:
		return "soon"
	}
}