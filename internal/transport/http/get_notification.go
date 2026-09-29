package transport_http

import (
	"fmt"
	"net/http"

	core_logger "notifier/internal/core/logger"
	core_http_request "notifier/internal/core/transport/http/request"
	core_http_response "notifier/internal/core/transport/http/response"
)

const (
	idKey = "id"
)

type GetNotificationResponse NotificationResponse

func (h *NotificationsHTTPHandler) getNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := core_logger.FromContext(ctx)
	rh := core_http_response.NewHTTPResponseHandler(logger, w)

	id, pathErr := core_http_request.GetIntPathValue(r, idKey)
	if pathErr != nil {
		rh.ErrorResponse(
			fmt.Sprintf("failed to get %s path value", idKey),
			pathErr,
		)
		return
	}

	n, err := h.notifications.Get(ctx, id)
	if err != nil {
		rh.ErrorResponse("get notification failed", err)
		return
	}

	resp := GetNotificationResponse(toNotificationResponse(n))
	rh.JSONResponse(resp, http.StatusOK)
}
