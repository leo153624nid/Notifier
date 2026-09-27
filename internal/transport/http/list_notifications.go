package transport_http

import (
	"net/http"
	"strconv"

	"notifier/internal/core/domain"
	core_logger "notifier/internal/core/logger"
	core_http_response "notifier/internal/core/transport/http/response"
)

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

func toListNotificationResponse(notifications []domain.Notification) []NotificationResponse {
	result := make([]NotificationResponse, len(notifications))
	for i, n := range notifications {
		result[i] = toNotificationResponse(n)
	}
	return result
}

func (h *NotificationsHTTPHandler) listNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := core_logger.FromContext(ctx)
	rh := core_http_response.NewHTTPResponseHandler(logger, w)

	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	size, _ := strconv.Atoi(query.Get("size"))

	notifications, err := h.notifications.List(ctx, page, size)
	if err != nil {
		rh.ErrorResponse("get notifications list failed", err)
		return
	}

	resp := toListNotificationResponse(notifications)
	rh.JSONResponse(resp, http.StatusOK)
}
