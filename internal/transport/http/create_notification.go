package transport_http

import (
	"errors"
	"net/http"
	"uuid"

	"notifier/internal/core/domain"
	core_errors "notifier/internal/core/errors"
	core_logger "notifier/internal/core/logger"
	core_http_request "notifier/internal/core/transport/http/request"
	core_http_response "notifier/internal/core/transport/http/response"
)

type CreateNotificationRequest struct {
	To      string `json:"to" validate:"required"`
	Subject string `json:"subject" validate:"required"`
	Body    string `json:"body" validate:"required"`
	Channel string `json:"channel" validate:"required"`
	Urgent  bool   `json:"urgent" validate:"required"`
}

type CreateNotificationResponse NotificationResponse

func (r CreateNotificationRequest) toDomain() domain.Notification {
	return domain.NewNotificationUninitialized(
		r.To,
		r.Subject,
		r.Body,
		r.Channel,
		r.Urgent,
	)
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
	ctx := r.Context()
	logger := core_logger.FromContext(ctx)
	rh := core_http_response.NewHTTPResponseHandler(logger, w)

	var req CreateNotificationRequest
	if err := core_http_request.DecodeAndValidateRequest(r, &req); err != nil {
		rh.ErrorResponse("decode and validate request failed", err)
		return
	}

	n, createErr := h.notifications.CreateIdempotent(ctx, httpConsumer, uuid.New(), req.toDomain())
	if createErr != nil {
		switch {
		case errors.Is(createErr, core_errors.ErrInvalidNotification):
			rh.ErrorResponse("invalid request", core_errors.ErrInvalidNotification)
		case errors.Is(createErr, core_errors.ErrUnsupportedChannel):
			rh.ErrorResponse("invalid request: unsupported channel", core_errors.ErrUnsupportedChannel)
		default:
			rh.ErrorResponse("create notification failed", createErr)
		}
		return
	}

	resp := toCreateNotificationResponse(n)
	rh.JSONResponse(resp, http.StatusAccepted)
}
