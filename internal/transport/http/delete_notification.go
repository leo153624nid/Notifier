package transport_http

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	core_errors "notifier/internal/core/errors"
	core_logger "notifier/internal/core/logger"
	core_http_response "notifier/internal/core/transport/http/response"
)

func (h *NotificationsHTTPHandler) deleteNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := core_logger.FromContext(ctx)
	rh := core_http_response.NewHTTPResponseHandler(logger, w)

	idStr := r.PathValue("id")
	id, pathErr := strconv.Atoi(idStr)
	if pathErr != nil {
		rh.ErrorResponse(
			core_errors.ErrInvalidArgument.Error(),
			fmt.Errorf("%w: %v", core_errors.ErrInvalidArgument, pathErr),
		)
		return
	}

	delErr := h.notifications.Delete(ctx, id)
	if delErr != nil {
		if errors.Is(delErr, core_errors.ErrNotFound) {
			rh.ErrorResponse(
				core_errors.ErrNotFound.Error(),
				core_errors.ErrNotFound,
			)
			return
		}

		rh.ErrorResponse("delete notification failed", delErr)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
