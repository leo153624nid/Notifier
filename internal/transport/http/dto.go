package transport_http

import (
	"encoding/json"
	"net/http"

	"notifier/internal/core/domain"
)

type APIError struct {
	Error string `json:"error"`
}

type NotificationResponse struct {
	Recipient string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Channel   string `json:"channel"`
	Status    string `json:"status"`
	ID        int    `json:"id"`
	IsUrgent  bool   `json:"urgent"`
}

func toNotificationResponse(n domain.Notification) NotificationResponse {
	return NotificationResponse{
		ID:        n.ID,
		Recipient: n.Recipient,
		Subject:   n.Subject,
		Body:      n.Body,
		Channel:   n.Channel,
		Status:    n.Status,
		IsUrgent:  n.IsUrgent,
	}
}

func toNotificationResponses(notifications []domain.Notification) []NotificationResponse {
	result := make([]NotificationResponse, len(notifications))
	for i, n := range notifications {
		result[i] = toNotificationResponse(n)
	}
	return result
}

type HealthResponse struct {
	App     string `json:"app"`
	Version string `json:"version"`
	Status  string `json:"status"`
}

type ExportResponse struct {
	Exported int `json:"exported"`
}

func SendJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(APIError{Error: message})
}
