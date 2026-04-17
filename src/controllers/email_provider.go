package controllers

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"codeberg.org/isotop7/proviant/models"
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

// SendMonthlyWasteReport sends the monthly household waste report to a single recipient.
func (e *EmailNotificationProvider) SendMonthlyWasteReport(recipient string, stats models.WasteStats) error {
	templ, err := template.ParseFS(templates.TemplateFiles, "notification/monthly_waste_report.html")
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := templ.Execute(&buf, stats); err != nil {
		return err
	}

	mail := gomail.NewMessage()
	mail.SetHeader("From", e.Configuration.FromAddress)
	mail.SetHeader("To", recipient)
	mail.SetHeader("Subject", fmt.Sprintf("%s — Monthly Waste Report for %s",
		stats.HouseholdName, stats.MonthLabel))
	mail.SetBody("text/html", buf.String())

	d := gomail.Dialer{Host: e.Configuration.Host, Port: e.Configuration.Port, SSL: e.Configuration.SSL}
	if e.Configuration.User != "" && e.Configuration.Password != "" {
		d.Username = e.Configuration.User
		d.Password = e.Configuration.Password
	}
	return d.DialAndSend(mail)
}

// SendEmailVerificationEmail sends an email verification email to the recipient
func (e *EmailNotificationProvider) SendEmailVerificationEmail(email, username, token, baseURL string, expiresAt time.Time) error {
	magicLink := fmt.Sprintf("%s/web/verify-email?token=%s", baseURL, token)

	templ, templErr := template.ParseFS(templates.TemplateFiles, "notification/verification.html")
	if templErr != nil {
		return templErr
	}
	var bodyBuf bytes.Buffer
	templExecErr := templ.Execute(&bodyBuf, struct {
		Username  string
		MagicLink string
		ExpiresAt string
		Email     string
	}{
		Username:  username,
		MagicLink: magicLink,
		ExpiresAt: expiresAt.Format("2006-01-02 15:04"),
		Email:     email,
	})
	if templExecErr != nil {
		return templExecErr
	}

	mail := gomail.NewMessage()
	mail.SetHeader("From", e.Configuration.FromAddress)
	mail.SetHeader("To", email)
	mail.SetHeader("Subject", "Verify your email address for Proviant")
	mail.SetBody("text/html", bodyBuf.String())

	mailDialer := gomail.Dialer{
		Host: e.Configuration.Host,
		Port: e.Configuration.Port,
		SSL:  e.Configuration.SSL,
	}

	if e.Configuration.User != "" && e.Configuration.Password != "" {
		mailDialer.Username = e.Configuration.User
		mailDialer.Password = e.Configuration.Password
	}

	return mailDialer.DialAndSend(mail)
}

// SendInvitationEmail sends an invitation email to the recipient
func (e *EmailNotificationProvider) SendInvitationEmail(invitation *dbModel.HouseholdInvitation, inviterName, householdName, baseURL string) error {
	// Construct magic link
	magicLink := fmt.Sprintf("%s/web/invite/accept?token=%s", baseURL, invitation.Token)

	// Generate email body from template
	templ, templErr := template.ParseFS(templates.TemplateFiles, "notification/invitation.html")
	if templErr != nil {
		return templErr
	}
	var bodyBuf bytes.Buffer
	templExecErr := templ.Execute(&bodyBuf, struct {
		InviterName   string
		HouseholdName string
		MagicLink     string
		ExpiresAt     string
		Email         string
	}{
		InviterName:   inviterName,
		HouseholdName: householdName,
		MagicLink:     magicLink,
		ExpiresAt:     invitation.ExpiresAt.Format("2006-01-02 15:04"),
		Email:         invitation.Email,
	})
	if templExecErr != nil {
		return templExecErr
	}

	// Create new mail object
	mail := gomail.NewMessage()

	// Set sender
	mail.SetHeader("From", e.Configuration.FromAddress)

	// Set recipient
	mail.SetHeader("To", invitation.Email)

	// Set subject
	mail.SetHeader("Subject", fmt.Sprintf("You're invited to join '%s' on Proviant", householdName))

	// Set body of mail to generated template output
	mail.SetBody("text/html", bodyBuf.String())

	// Settings for SMTP server
	mailDialer := gomail.Dialer{
		Host: e.Configuration.Host,
		Port: e.Configuration.Port,
		SSL:  e.Configuration.SSL,
	}

	if e.Configuration.User != "" && e.Configuration.Password != "" {
		mailDialer.Username = e.Configuration.User
		mailDialer.Password = e.Configuration.Password
	}

	// Send mail and return error
	return mailDialer.DialAndSend(mail)
}
