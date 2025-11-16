package controllers

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	dbController "gitlab.com/Isotop7/proviant/controllers/database"
	"gitlab.com/Isotop7/proviant/models/configuration"
	dbModel "gitlab.com/Isotop7/proviant/models/database"
	"gitlab.com/Isotop7/proviant/templates"

	"github.com/rs/zerolog"
	gomail "gopkg.in/mail.v2"
)

// NotificationController is the object struct to generate and send notifications for expired products
type NotificationController struct {
	Logger             *zerolog.Logger
	Configuration      configuration.NotificationConfiguration
	DatabaseController *dbController.DatabaseController
}

// Dispatch creates an eternal go routine that periodically checks for pending notifications and sends them.
// The timeout can be configured with the Configuration struct of NotificationController
func (nc *NotificationController) Dispatch() {
	sleepInterval := time.Hour * time.Duration(nc.Configuration.Interval)
	go func() {
		for {
			// Get affected products
			notificationProducts, getError := nc.DatabaseController.GetProductsExpiredAndNotificationPending(sleepInterval)
			if getError != nil {
				nc.Logger.Error().Msg(getError.Error())
			}

			// Generate notifications and send them
			nc.generateNotifications(&notificationProducts)

			// Sleep
			nc.Logger.Info().Msgf("NotificationController is now sleeping for %d hours", nc.Configuration.Interval)
			time.Sleep(sleepInterval)
		}
	}()
}

// generateNotifications uses a list of products and generates a notification for it
func (nc *NotificationController) generateNotifications(notificationProducts *[]dbModel.Product) {
	// Loop through products
	for _, product := range *notificationProducts {
		// Get user object of product
		mailAddresses, getError := nc.DatabaseController.GetHouseholdMembersMailAddressesByID(product.HouseholdID) //TODO: Iterate through users and send mail
		if getError != nil {
			nc.Logger.Error().Msg(getError.Error())
		}

		for _, mailAddress := range mailAddresses {
			// Sending notification
			nc.Logger.Info().Msgf("Sending notification for product with id '%d' and barcode '%s' to '%s'", product.ID, product.Barcode, mailAddress)
			sendError := nc.sendMail(&product, mailAddress)

			// Check for error
			if sendError != nil {
				nc.Logger.Error().Msg(sendError.Error())
				break
			} else {
				nc.Logger.Info().Msg("Notification send successfully")
			}

			// Update notifiedAt timestamp
			updateErr := nc.DatabaseController.SetProductNotifiedAt(product.ID)
			if updateErr != nil {
				nc.Logger.Error().Msg(updateErr.Error())
			} else {
				nc.Logger.Info().Msg("Property NotifiedAt was updated")
			}
		}
	}
}

// sendMail sends the notification for a product to a recipient
func (nc *NotificationController) sendMail(product *dbModel.Product, recipient string) error {
	// Create new mail object
	mail := gomail.NewMessage()

	// Set sender
	mail.SetHeader("From", nc.Configuration.FromAddress)

	// Set recipient
	mail.SetHeader("To", recipient)

	// Set header
	subject := fmt.Sprintf("proviant - Warning - Product '%d' expired", &product.ID)
	mail.SetHeader("Subject", subject)

	// Generate email body from template
	templ, templErr := template.ParseFS(templates.TemplateFiles, "notification/expired.html")
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
	// Check for templating errors
	if templExecErr != nil {
		return templExecErr
	}

	// Set body of mail to generated template output
	mail.SetBody("text/html", bodyBuf.String())

	// Settings for SMTP server
	mailDialer := gomail.Dialer{
		Host: nc.Configuration.SMTP.Host,
		Port: nc.Configuration.SMTP.Port,
	}

	if nc.Configuration.SMTP.User != "" && nc.Configuration.SMTP.Password != "" {
		mailDialer.Username = nc.Configuration.SMTP.User
		mailDialer.Password = nc.Configuration.SMTP.Password
	}

	// Set ssl mode
	mailDialer.SSL = nc.Configuration.SMTP.SSL

	// Send mail and return error

	err := mailDialer.DialAndSend(mail)
	return err
}
