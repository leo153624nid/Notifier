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

type Server struct {
	notifications map[int]Notification
	nextID        int
}

// var notifications []Notification // slice
// var notifications map[int]Notification{} // empty map

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

func (s *Server) getNotification(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Invalid notification ID"}`))
		return
	}

	n, ok := s.notifications[id]
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

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	var all []Notification
	for _, n := range s.notifications {
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

func (s *Server) createNotification(w http.ResponseWriter, r *http.Request) {
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

	s.nextID++
	n.ID = s.nextID
	s.notifications[n.ID] = n

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
	s := &Server{
		notifications: map[int]Notification{},
		nextID:        0,
	}

	s.nextID++
	s.notifications[s.nextID] = Notification{
		ID:        s.nextID,
		Recipient: "111",
		Subject:   "Test Notification",
		Body:      "This is a test notification.",
		Channel:   "email",
		IsUrgent:  false,
	}
	s.nextID++
	s.notifications[s.nextID] = Notification{
		ID:        s.nextID,
		Recipient: "222",
	}

	fmt.Printf("Starting %s %s on: 8080\n", appName, appVersion)

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("GET /api/notifications", s.listNotifications)
	http.HandleFunc("GET /api/notifications/{id}", s.getNotification)
	http.HandleFunc("POST /api/notifications", s.createNotification)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
