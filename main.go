package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

const (
	appVersion = "0.1.0"
	appName    = "Notifier"
)

// var notifications []Notification // slice
var notifications = map[int]Notification{} // map
var nextID int

type Notification struct {
	ID        int    `json:"id"`
	Recipient string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Channel   string `json:"channel"`
	IsUrgent  bool   `json:"urgent"`
}

func (n Notification) Format() string {
	return fmt.Sprintf(
		"id: %d | to: %s | subject: %s | body: %s | channel: %s",
		n.ID,
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
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
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

		// notifications = append(notifications, newNotification)
		notifications[nextID] = newNotification
		nextID++

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

func getNotification(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Invalid notification ID"}`))
		return
	}

	n, ok := notifications[id]
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "Notification not found"}`))
		return
	}

	js, err := json.Marshal(n)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Failed to marshal notification"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(js)
}

func listNotifications(w http.ResponseWriter, r *http.Request) {
	var all []Notification
	for _, n := range notifications {
		all = append(all, n)
	}

	js, err := json.Marshal(all)
	if err != nil {
		fmt.Println("Marshal error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(js)
}

func createNotification(w http.ResponseWriter, r *http.Request) {
	var n Notification
	err := json.NewDecoder(r.Body).Decode(&n)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Invalid request payload"}`))
		return
	}

	err = n.Validate()
	if err != nil {
		errResponse := struct {
			Error string `json:"error"`
		}{
			Error: err.Error(),
		}

		js, _ := json.Marshal(errResponse)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write(js)
		return
	}

	nextID++
	n.ID = nextID
	notifications[n.ID] = n

	js, err := json.Marshal(n)
	if err != nil {
		fmt.Println("Marshal error:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	w.Write(js)
}

func main() {
	nextID++
	notifications[nextID] = Notification{
		ID:        nextID,
		Recipient: "111",
		Subject:   "Test Notification",
		Body:      "This is a test notification.",
		Channel:   "email",
		IsUrgent:  false,
	}
	nextID++
	notifications[nextID] = Notification{
		ID:        nextID,
		Recipient: "222",
	}

	fmt.Printf("Starting %s %s on: 8080\n", appName, appVersion)

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("GET /api/notifications", listNotifications)
	http.HandleFunc("GET /api/notifications/{id}", getNotification)
	http.HandleFunc("POST /api/notifications", createNotification)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
