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

var notifications []Notification

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
	if r.Method == http.MethodGet {
		// for i, n := range notifications {
		// 	notifications[i].Subject = "testtttttt"
		// 	fmt.Println(n.Subject) // copied instance
		// }

		// validationErr := n1.Validate()
		// if validationErr != nil {
		// 	errResponse := struct {
		// 		Error string `json:"error"`
		// 	}{
		// 		Error: validationErr.Error(),
		// 	}

		// 	js, jsErr := json.Marshal(errResponse)
		// 	if jsErr != nil {
		// 		fmt.Fprintln(w, "Marshal error:", jsErr)
		// 		return
		// 	}

		// 	w.Header().Set("Content-type", "application/json")
		// 	w.Write(js)
		// 	return
		// }

		js, err := json.Marshal(notifications)
		if err != nil {
			fmt.Println(w, "Marshal error:", err)
			return
		}

		w.Header().Set("Content-type", "application/json")
		w.Write(js)
		return
	}

	if r.Method == http.MethodPost {
		var newNotification Notification
		decodeErr := json.NewDecoder(r.Body).Decode(&newNotification)
		if decodeErr != nil {
			errResponse := struct {
				Error string `json:"error"`
			}{
				Error: decodeErr.Error(),
			}

			js, err := json.Marshal(errResponse)
			if err != nil {
				fmt.Println(w, "Marshal error:", err)
				return
			}

			w.Header().Set("Content-type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write(js)
		}

		notifications = append(notifications, newNotification)

		js, jsErr := json.Marshal(newNotification)
		if jsErr != nil {
			fmt.Fprintln(w, "Marshal error:", jsErr)
			return
		}

		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(js)
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func main() {
	fmt.Printf("Starting %s %s on: 8080\n", appName, appVersion)

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/api/notifications", notificationHandler)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
