package controllers

import (
	"bytes"
	"expiro/backend/models"
	"fmt"
	"html/template"
	"time"

	gomail "gopkg.in/mail.v2"
	"gorm.io/gorm"
)

type NotificationController struct {
	Configuration models.NotificationConfiguration
	DB            *gorm.DB
}

func (nc NotificationController) Dispatch() {
	sleepInterval := time.Hour * time.Duration(nc.Configuration.Interval)
	go func() {
		for {
			// Get products with pending notification
			var notificationProducts []models.Product
			getError := nc.DB.Where("expire_at < ?", time.Now()).Find(&notificationProducts)
			if getError.Error != nil {
				fmt.Println(getError.Error)
			}

			for _, product := range notificationProducts {
				fmt.Printf("Sending notification for product with id '%d' and barcode '%s'\n", product.ID, product.Barcode)
				sendError := nc.SendMail(product)
				if sendError != nil {
					fmt.Println(sendError)
				}
			}

			// Sleep
			time.Sleep(sleepInterval)
		}
	}()
}

func (nc NotificationController) SendMail(product models.Product) error {
	m := gomail.NewMessage()

	// Set E-Mail sender
	m.SetHeader("From", nc.Configuration.FromAddress)

	// Set E-Mail receivers
	m.SetHeader("To", nc.Configuration.ToAddress...)

	subject := fmt.Sprintf("expiro - Warning - Product '%d' expired", product.ID)
	m.SetHeader("Subject", subject)

	// Generate email body from template
	templ, templErr := template.ParseFiles("templates/expired.html")
	if templErr != nil {
		return templErr
	}
	var bodyBuf bytes.Buffer
	templ.Execute(&bodyBuf, struct {
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
	body := bodyBuf.String()

	m.SetBody("text/html", body)

	// Settings for SMTP server
	d := gomail.NewDialer(
		nc.Configuration.SMTP.Host,
		nc.Configuration.SMTP.Port,
		nc.Configuration.SMTP.User,
		nc.Configuration.SMTP.Password,
	)

	d.SSL = nc.Configuration.SMTP.SSL

	// Now send E-Mail
	err := d.DialAndSend(m)
	return err
}
