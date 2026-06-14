package controllers

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"time"

	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/templates"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
)

type NtfyNotificationProvider struct {
	Configuration configuration.NtfyConfiguration
	Logger        *zerolog.Logger
	HTTPClient    *http.Client
}

func (n *NtfyNotificationProvider) GetProviderType() string {
	return "ntfy"
}

func (n *NtfyNotificationProvider) IsConfigured() bool {
	return n.Configuration.URL != "" || n.Configuration.Topic != ""
}

func (n *NtfyNotificationProvider) SendNotification(product *dbModel.Product, recipientInfo any) error {
	recipient, ok := recipientInfo.(models.NotificationRecipientInfo)
	if !ok {
		return fmt.Errorf("invalid recipient type for ntfy provider")
	}

	// Determine URL and topic
	ntfyURL := n.Configuration.URL
	ntfyTopic := n.Configuration.Topic

	if recipient.NtfyURL != "" {
		ntfyURL = recipient.NtfyURL
	}
	if recipient.NtfyTopic != "" {
		ntfyTopic = recipient.NtfyTopic
	}

	// Validate URL
	parsedURL, err := url.Parse(ntfyURL)
	if err != nil {
		return fmt.Errorf("invalid ntfy URL: %w", err)
	}

	// Create full endpoint URL
	endpoint := fmt.Sprintf("%s://%s/%s", parsedURL.Scheme, parsedURL.Host, ntfyTopic)

	// Generate message from template
	templ, templErr := template.ParseFS(templates.TemplateFiles, "notification/expired.txt")
	if templErr != nil {
		return templErr
	}

	var bodyBuf bytes.Buffer
	templExecErr := templ.Execute(&bodyBuf, struct {
		ProductName string
		ID          uint
		Barcode     string
		ExpireAt    time.Time
	}{
		ProductName: product.ProductName,
		ID:          product.ID,
		Barcode:     product.Barcode,
		ExpireAt:    product.ExpireAt,
	})

	if templExecErr != nil {
		return templExecErr
	}

	// Create HTTP request
	req, reqErr := http.NewRequest("POST", endpoint, &bodyBuf)
	if reqErr != nil {
		return reqErr
	}

	// Set headers
	req.Header.Set(util.RequestHeaderContentType, "text/plain")
	req.Header.Set("Title", fmt.Sprintf("proviant - Product '%s' expired", product.ProductName))
	req.Header.Set("Priority", "high")
	req.Header.Set("Tags", "warning")

	// Add authentication if token is provided
	if recipient.NtfyToken != "" {
		req.Header.Set("Authorization", "Bearer "+recipient.NtfyToken)
	}

	// Send request
	resp, httpErr := n.HTTPClient.Do(req)
	if httpErr != nil {
		return httpErr
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			n.Logger.Error().Msgf("Error closing response body: %v", closeErr)
		}
	}()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy.sh request failed with status: %s", resp.Status)
	}

	return nil
}

// SendStreakMilestone sends a streak milestone push notification via ntfy.
func (n *NtfyNotificationProvider) SendStreakMilestone(milestone int, recipient *models.NotificationRecipientInfo) error {
	ntfyURL := n.Configuration.URL
	ntfyTopic := n.Configuration.Topic
	if recipient.NtfyURL != "" {
		ntfyURL = recipient.NtfyURL
	}
	if recipient.NtfyTopic != "" {
		ntfyTopic = recipient.NtfyTopic
	}

	parsedURL, err := url.Parse(ntfyURL)
	if err != nil {
		return fmt.Errorf("invalid ntfy URL: %w", err)
	}
	endpoint := fmt.Sprintf("%s://%s/%s", parsedURL.Scheme, parsedURL.Host, ntfyTopic)

	body := fmt.Sprintf("%d-day waste-free streak — your household has gone %d consecutive days without wasting food.", milestone, milestone)
	req, err := http.NewRequest("POST", endpoint, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	req.Header.Set(util.RequestHeaderContentType, "text/plain")
	req.Header.Set("Title", fmt.Sprintf("proviant - %d-day waste-free streak", milestone))
	req.Header.Set("Tags", "tada")
	if recipient.NtfyToken != "" {
		req.Header.Set("Authorization", "Bearer "+recipient.NtfyToken)
	}

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			n.Logger.Error().Msgf("Error closing response body: %v", closeErr)
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy.sh request failed with status: %s", resp.Status)
	}
	return nil
}

// SendStreakReset sends a streak-reset push notification via ntfy.
func (n *NtfyNotificationProvider) SendStreakReset(previousStreak int, recipient *models.NotificationRecipientInfo) error {
	ntfyURL := n.Configuration.URL
	ntfyTopic := n.Configuration.Topic
	if recipient.NtfyURL != "" {
		ntfyURL = recipient.NtfyURL
	}
	if recipient.NtfyTopic != "" {
		ntfyTopic = recipient.NtfyTopic
	}

	parsedURL, err := url.Parse(ntfyURL)
	if err != nil {
		return fmt.Errorf("invalid ntfy URL: %w", err)
	}
	endpoint := fmt.Sprintf("%s://%s/%s", parsedURL.Scheme, parsedURL.Host, ntfyTopic)

	body := fmt.Sprintf("Your %d-day waste-free streak has been reset. Time to start a new one!", previousStreak)
	req, err := http.NewRequest("POST", endpoint, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	req.Header.Set(util.RequestHeaderContentType, "text/plain")
	req.Header.Set("Title", "proviant - waste-free streak reset")
	req.Header.Set("Tags", "warning")
	if recipient.NtfyToken != "" {
		req.Header.Set("Authorization", "Bearer "+recipient.NtfyToken)
	}

	resp, err := n.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			n.Logger.Error().Msgf("Error closing response body: %v", closeErr)
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy.sh request failed with status: %s", resp.Status)
	}
	return nil
}
