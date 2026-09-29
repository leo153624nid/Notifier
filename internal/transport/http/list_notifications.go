package transport_http

import (
	"fmt"
	"net/http"

	core_logger "notifier/internal/core/logger"
	core_http_request "notifier/internal/core/transport/http/request"
	core_http_response "notifier/internal/core/transport/http/response"
)

const (
	pageKey = "page"
	sizeKey = "size"
)

func (h *NotificationsHTTPHandler) listNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := core_logger.FromContext(ctx)
	rh := core_http_response.NewHTTPResponseHandler(logger, w)

	page, size, err := getPaginationParams(r)
	if err != nil {
		rh.ErrorResponse("wrong pagination params: %w", err)
		return
	}

	notifications, err := h.notifications.List(ctx, page, size)
	if err != nil {
		rh.ErrorResponse("get notifications list failed", err)
		return
	}

	resp := toListNotificationResponse(notifications)
	rh.JSONResponse(resp, http.StatusOK)
}

func getPaginationParams(r *http.Request) (page *int, size *int, err error) {
	page, pageErr := core_http_request.GetIntQueryParam(r, pageKey)
	if pageErr != nil {
		return nil, nil, fmt.Errorf("`page` param failed: %w", pageErr)
	}

	size, sizeErr := core_http_request.GetIntQueryParam(r, sizeKey)
	if sizeErr != nil {
		return nil, nil, fmt.Errorf("`size` param failed: %w", sizeErr)
	}

	return page, size, nil
}
