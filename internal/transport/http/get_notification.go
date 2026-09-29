package transport_http

import (
	"net/http"

	core_logger "notifier/internal/core/logger"
	core_http_response "notifier/internal/core/transport/http/response"
	core_http_utils "notifier/internal/core/transport/http/utils"
)

type GetNotificationResponse NotificationResponse

func (h *NotificationsHTTPHandler) getNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := core_logger.FromContext(ctx)
	rh := core_http_response.NewHTTPResponseHandler(logger, w)

	id, pathErr := core_http_utils.GetIntPathValue(r, "id")
	if pathErr != nil {
		rh.ErrorResponse(
			"failed to get `id` path value",
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
