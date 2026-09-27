package transport_http

import (
	"context"
	"net/http"
	"time"

	core_logger "notifier/internal/core/logger"
	core_http_response "notifier/internal/core/transport/http/response"
)

type HealthResponse struct {
	App     string `json:"app"`
	Version string `json:"version"`
	Status  string `json:"status"`
}

func (h *NotificationsHTTPHandler) healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := core_logger.FromContext(ctx)
	rh := core_http_response.NewHTTPResponseHandler(logger, w)

	ctxDB, cancelDB := context.WithTimeout(ctx, 1*time.Second)
	defer cancelDB()

	if err := h.health.CheckDB(ctxDB); err != nil {
		rh.ErrorResponse("db health check failed", err)
		return
	}

	ctxCache, cancelCache := context.WithTimeout(ctx, 1*time.Second)
	defer cancelCache()

	if err := h.health.CheckCache(ctxCache); err != nil {
		rh.ErrorResponse("cache health check failed", err)
		return
	}

	resp := HealthResponse{
		App:     h.appName,
		Version: h.appVersion,
		Status:  "available",
	}

	rh.JSONResponse(resp, http.StatusOK)
}
