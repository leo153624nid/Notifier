package http

import (
	"encoding/json"
	"net/http"
	"notifier/internal/notification"
)

type APIError struct {
	Error string `json:"error"`
}

// CreateNotificationRequest — тело запроса POST /api/v1/notifications.
type CreateNotificationRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Channel string `json:"channel"`
	Urgent  bool   `json:"urgent"`
}

func (r CreateNotificationRequest) toDomain() notification.Notification {
	return notification.Notification{
		Recipient: r.To,
		Subject:   r.Subject,
		Body:      r.Body,
		Channel:   r.Channel,
		IsUrgent:  r.Urgent,
	}
}

// NotificationResponse — представление уведомления в HTTP API.
//
//nolint:govet
type NotificationResponse struct {
	ID        int    `json:"id"`
	Recipient string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Channel   string `json:"channel"`
	Status    string `json:"status"`
	IsUrgent  bool   `json:"urgent"`
}

func toNotificationResponse(n notification.Notification) NotificationResponse {
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

func toNotificationResponses(notifications []notification.Notification) []NotificationResponse {
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
