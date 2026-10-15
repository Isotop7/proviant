package controllers

import (
	dbModel "codeberg.org/isotop7/proviant/models/database"
)

// NotificationProvider interface defines methods for sending notifications
type NotificationProvider interface {
	SendNotification(product *dbModel.Product, recipientInfo interface{}, lang string) error
	GetProviderType() string
	IsConfigured() bool
}
