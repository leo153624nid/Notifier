package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	appVersion = "0.1.0"
	appName    = "Notifier"
)

type Notification struct {
	Recipient string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Channel   string `json:"channel"`
	IsUrgent  bool   `json:"urgent"`
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
	data := struct {
		App     string `json:"app"`
		Version string `json:"version"`
		Status  string `json:"status"`
	}{
		App:     appName,
		Version: appVersion,
		Status:  "available",
	}

	js, err := json.Marshal(data)
	if err != nil {
		fmt.Println(w, "Marshal error:", err)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.Write(js)
}

func notificationHandler(w http.ResponseWriter, r *http.Request) {
	n := Notification{
		Recipient: "user@example.com",
		Subject:   "Deploy done",
		Body:      "Deployment completed successfully.",
		Channel:   "email",
		IsUrgent:  true,
	}

	validationErr := n.Validate()
	if validationErr != nil {
		errResponse := struct {
			Error string `json:"error"`
		}{
			Error: validationErr.Error(),
		}

		js, jsErr := json.Marshal(errResponse)
		if jsErr != nil {
			fmt.Fprintln(w, "Marshal error:", jsErr)
			return
		}

		w.Header().Set("Content-type", "application/json")
		w.Write(js)
		return
	}

	js, jsErr := json.Marshal(n)
	if jsErr != nil {
		fmt.Fprintln(w, "Marshal error:", jsErr)
		return
	}

	// message, status := n.Send()
	// fmt.Fprintln(w, message)
	// fmt.Fprintln(w, "status:", status)
	w.Header().Set("Content-type", "application/json")
	w.Write(js)
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
