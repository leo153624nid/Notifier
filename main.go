package main

import (
	"fmt"
	"net/http"
)

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

func (n Notification) Format() string {
	return fmt.Sprintf(
		"to: %s | subject: %s | body: %s | channel: %s",
		n.Recipient,
		n.Subject,
		n.Body,
		n.Channel,
	)
}

func (n Notification) Send() (message string, status string) {
	message = n.Format()
	status = "queued"
	return message, status
}

func (n Notification) Validate() error {
	if n.Recipient == "" {
		return fmt.Errorf("recipient is required")
	}
	return nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, appName, appVersion)
	fmt.Fprintln(w, "status: available")
}

func notificationHandler(w http.ResponseWriter, r *http.Request) {
	n := Notification{
		Recipient: "user@example.com",
		Subject:   "Deploy done",
		Body:      "Deployment completed successfully.",
		Channel:   "email",
		isUrgent:  true,
	}

	err := n.Validate()
	if err != nil {
		fmt.Fprintln(w, "Validation failed:", err)
		return
	}

	message, status := n.Send()
	fmt.Fprintln(w, message)
	fmt.Fprintln(w, "status:", status)
}

func main() {
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/api/notifications", notificationHandler)

	fmt.Println("Starting server on: 8080")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}

}
