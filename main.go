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

	err := n.Validate()
	if err != nil {
		fmt.Println("Validation failed:", err)
		return
	}

	message, status := n.Send()

	fmt.Printf("%s \n %s", message, status)
}
