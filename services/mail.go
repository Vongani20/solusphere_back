package services

import (
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"strings"
)

func SendMail(to, subject, body string) error {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	username := strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
	password := os.Getenv("SMTP_PASSWORD")
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))

	if host == "" || port == "" || from == "" {
		return fmt.Errorf("SMTP_HOST, SMTP_PORT, and SMTP_FROM must be configured")
	}
	if to = strings.TrimSpace(to); to == "" {
		return fmt.Errorf("recipient email is required")
	}
	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf("SMTP_PORT must be numeric")
	}

	headers := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
	}
	message := strings.Join(headers, "\r\n") + "\r\n\r\n" + body

	var auth smtp.Auth
	if username != "" || password != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	return smtp.SendMail(host+":"+port, auth, from, []string{to}, []byte(message))
}

func IsMailConfigured() bool {
	return strings.TrimSpace(os.Getenv("SMTP_HOST")) != "" &&
		strings.TrimSpace(os.Getenv("SMTP_PORT")) != "" &&
		strings.TrimSpace(os.Getenv("SMTP_FROM")) != ""
}
