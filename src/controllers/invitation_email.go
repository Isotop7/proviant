package controllers

import (
	"bytes"
	"fmt"
	"html/template"

	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/templates"

	"github.com/rs/zerolog"
	gomail "gopkg.in/mail.v2"
)

// SendInvitationEmail sends an invitation email to the recipient
func SendInvitationEmail(invitation dbModel.HouseholdInvitation, inviterName, householdName, baseURL string, smtpConfig configuration.SMTPConfiguration, logger *zerolog.Logger) error {
	_ = logger // Reserved for future logging

	// Construct magic link
	magicLink := fmt.Sprintf("%s/web/invite/accept?token=%s", baseURL, invitation.Token)

	// Render HTML template
	templ, err := template.ParseFS(templates.TemplateFiles, "notification/invitation.html")
	if err != nil {
		return err
	}

	var bodyBuf bytes.Buffer
	err = templ.Execute(&bodyBuf, struct {
		InviterName   string
		HouseholdName string
		MagicLink     string
		ExpiresAt     string
	}{
		InviterName:   inviterName,
		HouseholdName: householdName,
		MagicLink:     magicLink,
		ExpiresAt:     invitation.ExpiresAt.Format("2006-01-02 15:04"),
	})
	if err != nil {
		return err
	}

	// Create and send email
	mail := gomail.NewMessage()
	mail.SetHeader("From", smtpConfig.FromAddress)
	mail.SetHeader("To", invitation.Email)
	mail.SetHeader("Subject", fmt.Sprintf("You're invited to join '%s' on Proviant", householdName))
	mail.SetBody("text/html", bodyBuf.String())

	mailDialer := gomail.Dialer{
		Host: smtpConfig.Host,
		Port: smtpConfig.Port,
		SSL:  smtpConfig.SSL,
	}
	if smtpConfig.User != "" && smtpConfig.Password != "" {
		mailDialer.Username = smtpConfig.User
		mailDialer.Password = smtpConfig.Password
	}

	return mailDialer.DialAndSend(mail)
}
