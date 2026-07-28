package utils

import (
	"os"

	"gopkg.in/gomail.v2"
	"sirtom/server/logger"
)

// Send email to the given email. Returns an error if the email could not be
// sent (e.g. missing/invalid SMTP credentials) so callers can surface the
// failure instead of silently swallowing it.
func SendEmail(toEmail string, subject string, body string, contentType string) error {
	if contentType == "" {
		contentType = "text/plain"
	}

	appPassword := os.Getenv("GMAIL_APP_PASSWORD")
	fromEmail := os.Getenv("SCHEJ_EMAIL_ADDRESS")

	m := gomail.NewMessage()
	m.SetHeader("From", fromEmail)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", subject)
	m.SetBody(contentType, body)

	d := gomail.NewDialer("smtp.gmail.com", 587, fromEmail, appPassword)

	if err := d.DialAndSend(m); err != nil {
		logger.StdErr.Println(err)
		return err
	}
	return nil
}

// SendEmailAsync sends in the background, recovering from panics. For
// fire-and-forget notifications, where a mail failure must never fail (or
// slow down) the request that triggered it.
func SendEmailAsync(toEmail string, subject string, body string, contentType string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.StdErr.Println("panic sending email:", r)
			}
		}()
		if err := SendEmail(toEmail, subject, body, contentType); err != nil {
			logger.StdErr.Println("failed to send email:", err)
		}
	}()
}
