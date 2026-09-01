package main

import "fmt"

const (
	appVersion = "1.0.0"
	appName    = "Notifier"
)

type Notification struct {
	Recipient string
	Subject   string
	Body      string
	Channel   string
	isUrgent  bool
}

func main() {
	// retryCount := 3
	// timeout := 3.5 // float64
	// isUrgent := false

	fmt.Println(appVersion)

	n := Notification{
		Recipient: "user@example.com",
		Subject:   "Deploy done",
		Body:      "Deployment completed successfully.",
		Channel:   "email",
		isUrgent:  true,
	}
	message, status := Send(n)
	fmt.Printf("%s \n %s", message, status)
}

func FormatNotification(n Notification) string {
	return fmt.Sprintf(
		"to: %s | subject: %s | body: %s | channel: %s",
		n.Recipient,
		n.Subject,
		n.Body,
		n.Channel,
	)
}

func Send(n Notification) (message string, status string) {
	message = FormatNotification(n)
	status = "queued"
	return message, status
}
