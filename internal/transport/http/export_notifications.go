package transport_http

import (
	"net/http"

	core_logger "notifier/internal/core/logger"
	core_http_response "notifier/internal/core/transport/http/response"
)

type ExportResponse struct {
	Exported int `json:"exported"`
}

func (h *NotificationsHTTPHandler) exportNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := core_logger.FromContext(ctx)
	rh := core_http_response.NewHTTPResponseHandler(logger, w)

	count, err := h.notifications.Export(ctx)
	if err != nil {
		rh.ErrorResponse("export failed", err)
		return
	}

	resp := ExportResponse{
		Exported: count,
	}
	rh.JSONResponse(resp, http.StatusOK)
}
