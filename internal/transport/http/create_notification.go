package transport_http

import (
	"encoding/json"
	"errors"
	"net/http"

	"notifier/internal/core/domain"

	"go.uber.org/zap"
)

type CreateNotificationRequest struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	Channel string `json:"channel"`
	Urgent  bool   `json:"urgent"`
}

type CreateNotificationResponse NotificationResponse

func (r CreateNotificationRequest) toDomain() domain.Notification {
	return domain.Notification{
		Recipient: r.To,
		Subject:   r.Subject,
		Body:      r.Body,
		Channel:   r.Channel,
		IsUrgent:  r.Urgent,
	}
}

func toCreateNotificationResponse(n domain.Notification) CreateNotificationResponse {
	return CreateNotificationResponse{
		ID:        n.ID,
		Recipient: n.Recipient,
		Subject:   n.Subject,
		Body:      n.Body,
		Channel:   n.Channel,
		Status:    n.Status,
		IsUrgent:  n.IsUrgent,
	}
}

func (h *NotificationsHTTPHandler) createNotification(w http.ResponseWriter, r *http.Request) {
	const op = "NotificationsHTTPHandler.createNotification"

	var req CreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendJSONError(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	requestID := getRequestID(r.Context())

	n, err := h.notifications.Create(r.Context(), req.toDomain(), requestID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidNotification):
			SendJSONError(w, "invalid request", http.StatusBadRequest)
		case errors.Is(err, domain.ErrUnsupportedChannel):
			SendJSONError(w, "invalid request: unsupported channel", http.StatusBadRequest)
		default:
			h.logger.Error(
				"create notification failed",
				zap.String("op", op),
				zap.Error(err),
			)
			SendJSONError(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	js, err := json.Marshal(toCreateNotificationResponse(n))
	if err != nil {
		h.logger.Error(
			"marshal failed",
			zap.String("op", op),
			zap.Error(err),
		)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write(js)
}
