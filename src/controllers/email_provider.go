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

const (
	mimeTypeHTML  = "text/html"
	headerFrom    = "From"
	headerTo      = "To"
	headerSubject = "Subject"
)

var emailSendFunc = func(d *gomail.Dialer, m *gomail.Message) error {
	return d.DialAndSend(m)
}

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

	mail := gomail.NewMessage()
	mail.SetHeader(headerFrom, e.Configuration.FromAddress)
	mail.SetHeader(headerTo, recipient)
	subject := fmt.Sprintf("proviant - Warning - Product '%s' expired", product.ProductName)
	mail.SetHeader(headerSubject, subject)

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
	if templExecErr != nil {
		return templExecErr
	}

	mail.SetBody(mimeTypeHTML, bodyBuf.String())

	mailDialer := gomail.Dialer{
		Host: e.Configuration.Host,
		Port: e.Configuration.Port,
	}

	if e.Configuration.User != "" && e.Configuration.Password != "" {
		mailDialer.Username = e.Configuration.User
		mailDialer.Password = e.Configuration.Password
	}

	mailDialer.SSL = e.Configuration.SSL

	return emailSendFunc(&mailDialer, mail)
}

// SendMonthlyWasteReport sends the monthly household waste report to a single recipient.
func (e *EmailNotificationProvider) SendMonthlyWasteReport(recipient string, stats *models.WasteStats) error {
	templ, err := template.ParseFS(templates.TemplateFiles, "notification/monthly_waste_report.html")
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := templ.Execute(&buf, stats); err != nil {
		return err
	}

	mail := gomail.NewMessage()
	mail.SetHeader(headerFrom, e.Configuration.FromAddress)
	mail.SetHeader(headerTo, recipient)
	mail.SetHeader(headerSubject, fmt.Sprintf("%s — Monthly Waste Report for %s",
		stats.HouseholdName, stats.MonthLabel))
	mail.SetBody(mimeTypeHTML, buf.String())

	dialer := gomail.Dialer{Host: e.Configuration.Host, Port: e.Configuration.Port, SSL: e.Configuration.SSL}
	if e.Configuration.User != "" && e.Configuration.Password != "" {
		dialer.Username = e.Configuration.User
		dialer.Password = e.Configuration.Password
	}
	return emailSendFunc(&dialer, mail)
}

// SendStreakMilestone sends a streak milestone notification email.
func (e *EmailNotificationProvider) SendStreakMilestone(milestone int, recipient string) error {
	templ, err := template.ParseFS(templates.TemplateFiles, "notification/streak_milestone.html")
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := templ.Execute(&buf, struct{ Milestone int }{Milestone: milestone}); err != nil {
		return err
	}

	mail := gomail.NewMessage()
	mail.SetHeader(headerFrom, e.Configuration.FromAddress)
	mail.SetHeader(headerTo, recipient)
	mail.SetHeader(headerSubject, fmt.Sprintf("proviant - %d-day waste-free streak", milestone))
	mail.SetBody(mimeTypeHTML, buf.String())

	dialer := gomail.Dialer{Host: e.Configuration.Host, Port: e.Configuration.Port, SSL: e.Configuration.SSL}
	if e.Configuration.User != "" && e.Configuration.Password != "" {
		dialer.Username = e.Configuration.User
		dialer.Password = e.Configuration.Password
	}
	return emailSendFunc(&dialer, mail)
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
	mail.SetHeader(headerFrom, e.Configuration.FromAddress)
	mail.SetHeader(headerTo, email)
	mail.SetHeader(headerSubject, "Verify your email address for Proviant")
	mail.SetBody(mimeTypeHTML, bodyBuf.String())

	mailDialer := gomail.Dialer{
		Host: e.Configuration.Host,
		Port: e.Configuration.Port,
		SSL:  e.Configuration.SSL,
	}

	if e.Configuration.User != "" && e.Configuration.Password != "" {
		mailDialer.Username = e.Configuration.User
		mailDialer.Password = e.Configuration.Password
	}

	return emailSendFunc(&mailDialer, mail)
}

// SendExpiryDigestEmail sends an expiry digest email to a single recipient.
// Returns nil if all product groups are empty (no email sent).
func (e *EmailNotificationProvider) SendExpiryDigestEmail(productGroups interface {
	GetToday() []dbModel.Product
	GetThisWeek() []dbModel.Product
	GetNextWeek() []dbModel.Product
}, recipientEmail, householdName, unsubscribeURL string) error {
	groups := productGroups
	if len(groups.GetToday()) == 0 && len(groups.GetThisWeek()) == 0 && len(groups.GetNextWeek()) == 0 {
		return nil
	}

	templ, err := template.ParseFS(templates.TemplateFiles, "notification/expiry_mail_digest.html")
	if err != nil {
		return err
	}

	data := struct {
		HouseholdName    string
		GeneratedDate    string
		ProductsToday    []dbModel.Product
		ProductsThisWeek []dbModel.Product
		ProductsNextWeek []dbModel.Product
		UnsubscribeURL   string
	}{
		HouseholdName:    householdName,
		GeneratedDate:    time.Now().Format("January 2, 2006"),
		ProductsToday:    groups.GetToday(),
		ProductsThisWeek: groups.GetThisWeek(),
		ProductsNextWeek: groups.GetNextWeek(),
		UnsubscribeURL:   unsubscribeURL,
	}

	var buf bytes.Buffer
	if err := templ.Execute(&buf, data); err != nil {
		return err
	}

	mail := gomail.NewMessage()
	mail.SetHeader(headerFrom, e.Configuration.FromAddress)
	mail.SetHeader(headerTo, recipientEmail)
	mail.SetHeader(headerSubject, fmt.Sprintf("Proviant — Expiry Digest for %s", householdName))
	mail.SetBody(mimeTypeHTML, buf.String())

	dialer := gomail.Dialer{Host: e.Configuration.Host, Port: e.Configuration.Port, SSL: e.Configuration.SSL}
	if e.Configuration.User != "" && e.Configuration.Password != "" {
		dialer.Username = e.Configuration.User
		dialer.Password = e.Configuration.Password
	}
	return emailSendFunc(&dialer, mail)
}

// SendInvitationEmail sends an invitation email to the recipient
func (e *EmailNotificationProvider) SendInvitationEmail(invitation *dbModel.HouseholdInvitation, inviterName, householdName, baseURL string) error {
	magicLink := fmt.Sprintf("%s/web/invite/accept?token=%s", baseURL, invitation.Token)

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

	mail := gomail.NewMessage()
	mail.SetHeader(headerFrom, e.Configuration.FromAddress)
	mail.SetHeader(headerTo, invitation.Email)
	mail.SetHeader(headerSubject, fmt.Sprintf("You're invited to join '%s' on Proviant", householdName))
	mail.SetBody(mimeTypeHTML, bodyBuf.String())

	mailDialer := gomail.Dialer{
		Host: e.Configuration.Host,
		Port: e.Configuration.Port,
		SSL:  e.Configuration.SSL,
	}

	if e.Configuration.User != "" && e.Configuration.Password != "" {
		mailDialer.Username = e.Configuration.User
		mailDialer.Password = e.Configuration.Password
	}

	return emailSendFunc(&mailDialer, mail)
}
