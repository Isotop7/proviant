package controllers

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/templates"

	"github.com/rs/zerolog"
	gomail "gopkg.in/mail.v2"
)

type EmailNotificationProvider struct {
	Configuration configuration.SMTPConfiguration
	Logger        *zerolog.Logger
}

func (e *EmailNotificationProvider) GetProviderType() string {
	return "email"
}

func (e *EmailNotificationProvider) IsConfigured() bool {
	return e.Configuration.Host != "" && e.Configuration.Port > 0
}

func (e *EmailNotificationProvider) SendNotification(product *dbModel.Product, recipientInfo interface{}) error {
	recipient, ok := recipientInfo.(string)
	if !ok {
		return fmt.Errorf("invalid recipient type for email provider")
	}

	// Create new mail object
	mail := gomail.NewMessage()

	// Set sender
	mail.SetHeader("From", e.Configuration.FromAddress)

	// Set recipient
	mail.SetHeader("To", recipient)

	// Set header
	subject := fmt.Sprintf("proviant - Warning - Product '%s' expired", product.ProductName)
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
		Host: e.Configuration.Host,
		Port: e.Configuration.Port,
	}

	if e.Configuration.User != "" && e.Configuration.Password != "" {
		mailDialer.Username = e.Configuration.User
		mailDialer.Password = e.Configuration.Password
	}

	// Set ssl mode
	mailDialer.SSL = e.Configuration.SSL

	// Send mail and return error
	return mailDialer.DialAndSend(mail)
}
