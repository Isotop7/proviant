package controllers

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"gitlab.com/Isotop7/expiro/models/configuration"
	"gitlab.com/Isotop7/expiro/models/database"

	"github.com/rs/zerolog"
	gomail "gopkg.in/mail.v2"
	"gorm.io/gorm"
)

// NotificationController is the object struct to generate and send notifications for expired products
type NotificationController struct {
	Logger        *zerolog.Logger
	Configuration configuration.NotificationConfiguration
	DB            *gorm.DB
}

// Dispatch creates an eternal go routine that periodically checks for pending notifications and sends them.
// The timeout can be configured with the Configuration struct of NotificationController
func (nc NotificationController) Dispatch() {
	sleepInterval := time.Hour * time.Duration(nc.Configuration.Interval)
	go func() {
		for {
			// Get products with pending notification
			var notificationProducts []database.Product
			getError := nc.DB.
				Where("expire_at < ?", time.Now()).
				Where("notified_at < ?", time.Now().Add(-(sleepInterval))).
				Find(&notificationProducts)

			if getError.Error != nil {
				nc.Logger.Error().Msg(getError.Error.Error())
			}

			for _, product := range notificationProducts {
				nc.Logger.Info().Msgf("Sending notification for product with id '%d' and barcode '%s'", product.ID, product.Barcode)
				sendError := nc.SendMail(product)
				if sendError != nil {
					nc.Logger.Error().Msg(sendError.Error())
				} else {
					nc.Logger.Info().Msg("Notification send successfully")
					if nc.updateNotifiedAt(product.ID) {
						nc.Logger.Info().Msg("Property NotifiedAt was updated")
					}
				}
			}

			// Sleep
			nc.Logger.Info().Msgf("NotificationController is now sleeping for %d hours", nc.Configuration.Interval)
			time.Sleep(sleepInterval)
		}
	}()
}

func (nc NotificationController) SendMail(product database.Product) error {
	m := gomail.NewMessage()

	// Set E-Mail sender
	m.SetHeader("From", nc.Configuration.FromAddress)

	// Set E-Mail receivers
	m.SetHeader("To", nc.Configuration.CCAddresses...)

	subject := fmt.Sprintf("expiro - Warning - Product '%d' expired", product.ID)
	m.SetHeader("Subject", subject)

	// Generate email body from template
	templ, templErr := template.ParseFiles("templates/expired.html")
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

	body := bodyBuf.String()

	// Set body of mail to generated template output
	m.SetBody("text/html", body)

	// Settings for SMTP server
	d := gomail.NewDialer(
		nc.Configuration.SMTP.Host,
		nc.Configuration.SMTP.Port,
		nc.Configuration.SMTP.User,
		nc.Configuration.SMTP.Password,
	)

	// Set ssl mode
	d.SSL = nc.Configuration.SMTP.SSL

	// Send mail and return error
	err := d.DialAndSend(m)
	return err
}

func (nc NotificationController) updateNotifiedAt(id uint) bool {
	// Product by id
	var dbProduct database.Product
	selectErr := nc.DB.First(&dbProduct, id)

	if selectErr.Error != nil {
		nc.Logger.Error().Msgf("Product with ID '%d' was not found in database", int(id))
		return false
	}

	// Update notified_at
	dbProduct.NotifiedAt = time.Now()

	// Save changes to database
	nc.DB.Save(&dbProduct)
	return true
}
