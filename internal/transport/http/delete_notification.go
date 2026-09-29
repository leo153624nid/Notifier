package transport_http

import (
	"net/http"

	core_logger "notifier/internal/core/logger"
	core_http_request "notifier/internal/core/transport/http/request"
	core_http_response "notifier/internal/core/transport/http/response"
)

func (h *NotificationsHTTPHandler) deleteNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := core_logger.FromContext(ctx)
	rh := core_http_response.NewHTTPResponseHandler(logger, w)

	id, pathErr := core_http_request.GetIntPathValue(r, "id")
	if pathErr != nil {
		rh.ErrorResponse(
			"failed to get `id` path value",
			pathErr,
		)
		return
	}

	delErr := h.notifications.Delete(ctx, id)
	if delErr != nil {
		rh.ErrorResponse("delete notification failed", delErr)
		return
	}

	rh.NoContentResponse()
}
