package main

import "fmt"

const (
	appVersion = "1.0.0"
	appName    = "Notifier"
)

func main() {
	recipient := "user@example.com"
	subject := "Deploy done"
	// retryCount := 3
	// timeout := 3.5 // float64
	// isUrgent := false

	fmt.Println(appVersion)

	message, status := Send(recipient, subject, "Deployment completed successfully.", "email")
	fmt.Println(message, status)
}

func FormatNotification(
	recipient string,
	subject string,
	body string,
	channel string,
) string {
	return fmt.Sprintf(
		"to: %s | subject: %s | body: %s | channel: %s",
		recipient,
		subject,
		body,
		channel,
	)
}

func Send(recipient, subject, body, channel string) (string, string) {
	message := FormatNotification(recipient, subject, "Deployment completed successfully.", "email")
	status := "queued"
	return message, status
}
