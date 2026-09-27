package transport_http

import (
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

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

	ctx := r.Context()
	logger := core_logger.FromContext(ctx)
	responseHandler := core_http_response.NewHTTPResponseHandler(logger, w)

	var req CreateNotificationRequest
	if err := core_http_request.DecodeAndValidateRequest(r, &req); err != nil {
		responseHandler.ErrorResponse("decode and validate request failed", err)
		return
	}

	n, createErr := h.notifications.Create(ctx, req.toDomain())
	if createErr != nil {
		switch {
		case errors.Is(createErr, core_errors.ErrInvalidNotification):
			responseHandler.ErrorResponse("invalid request", core_errors.ErrInvalidNotification)
		case errors.Is(createErr, core_errors.ErrUnsupportedChannel):
			responseHandler.ErrorResponse("invalid request: unsupported channel", core_errors.ErrUnsupportedChannel)
		default:
			logger.Error(
				"create notification failed",
				zap.String("op", op),
				zap.Error(createErr),
			)
			responseHandler.ErrorResponse("internal error", createErr)
		}
		return
	}

	js, marshalErr := json.Marshal(toCreateNotificationResponse(n))
	if marshalErr != nil {
		logger.Error(
			"marshal failed",
			zap.String("op", op),
			zap.Error(marshalErr),
		)
		responseHandler.ErrorResponse("internal error", marshalErr)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write(js)
}
