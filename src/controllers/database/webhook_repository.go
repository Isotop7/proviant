package database

import (
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/database"
	"encoding/json"
	"gorm.io/gorm"
)

type WebhookRepositoryInterface interface {
	CreateWebhook(webhook *database.Webhook) error
	GetWebhooksByUserID(userID uint) ([]database.Webhook, error)
	GetWebhookByID(webhookID uint) (database.Webhook, error)
	GetActiveWebhooksByEvent(event string) ([]database.Webhook, error)
	UpdateWebhook(webhook *database.Webhook) error
	DeleteWebhook(webhookID uint) error
	CreateDeliveryLog(log *database.WebhookDeliveryLog) error
	GetDeliveryLogs(webhookID uint, limit int) ([]database.WebhookDeliveryLog, error)
	TrimDeliveryLogs(webhookID uint, keep int) error
	CheckOwnership(webhookID, userID uint) error
}

var _ WebhookRepositoryInterface = (*WebhookRepository)(nil)

type WebhookRepository struct {
	DB *gorm.DB
}

func NewWebhookRepository(db *gorm.DB) *WebhookRepository {
	return &WebhookRepository{DB: db}
}

func (r *WebhookRepository) CreateWebhook(webhook *database.Webhook) error {
	return r.DB.Create(&webhook).Error
}

func (r *WebhookRepository) GetWebhooksByUserID(userID uint) ([]database.Webhook, error) {
	var webhooks []database.Webhook
	err := r.DB.Where("user_id = ?", userID).Find(&webhooks).Error
	return webhooks, err
}

func (r *WebhookRepository) GetWebhookByID(webhookID uint) (database.Webhook, error) {
	var webhook database.Webhook
	err := r.DB.First(&webhook, webhookID).Error
	return webhook, err
}

func (r *WebhookRepository) GetActiveWebhooksByEvent(event string) ([]database.Webhook, error) {
	var webhooks []database.Webhook
	err := r.DB.Where("active = ?", true).Find(&webhooks).Error
	if err != nil {
		return nil, err
	}
	var filtered []database.Webhook
	for i := range webhooks {
		if containsEvent(webhooks[i].Events, event) {
			filtered = append(filtered, webhooks[i])
		}
	}
	return filtered, nil
}

func (r *WebhookRepository) UpdateWebhook(webhook *database.Webhook) error {
	return r.DB.Save(&webhook).Error
}

func (r *WebhookRepository) DeleteWebhook(webhookID uint) error {
	tx := r.DB.Begin()
	if err := tx.Where("webhook_id = ?", webhookID).Delete(&database.WebhookDeliveryLog{}).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Delete(&database.Webhook{}, webhookID).Error; err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit().Error
}

func (r *WebhookRepository) CreateDeliveryLog(log *database.WebhookDeliveryLog) error {
	if err := r.DB.Create(&log).Error; err != nil {
		return err
	}
	return r.TrimDeliveryLogs(log.WebhookID, 50)
}

func (r *WebhookRepository) GetDeliveryLogs(webhookID uint, limit int) ([]database.WebhookDeliveryLog, error) {
	var logs []database.WebhookDeliveryLog
	query := r.DB.Where("webhook_id = ?", webhookID).Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&logs).Error
	return logs, err
}

func (r *WebhookRepository) TrimDeliveryLogs(webhookID uint, keep int) error {
	var logs []database.WebhookDeliveryLog
	if err := r.DB.Where("webhook_id = ?", webhookID).Order("created_at DESC").Limit(keep).Find(&logs).Error; err != nil {
		return err
	}
	if len(logs) == 0 {
		return nil
	}
	lastKeepID := logs[len(logs)-1].ID
	return r.DB.Where("webhook_id = ? AND id < ?", webhookID, lastKeepID).Delete(&database.WebhookDeliveryLog{}).Error
}

func (r *WebhookRepository) CheckOwnership(webhookID, userID uint) error {
	webhook, err := r.GetWebhookByID(webhookID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrWebhookNotFound
		}
		return err
	}
	if webhook.UserID != userID {
		return errors.ErrWebhookNotOwner
	}
	return nil
}

func containsEvent(eventsJSON, event string) bool {
	var events []string
	if err := json.Unmarshal([]byte(eventsJSON), &events); err != nil {
		return false
	}
	for _, e := range events {
		if e == event {
			return true
		}
	}
	return false
}
