package transport_http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	core_errors "notifier/internal/core/errors"
	core_logger "notifier/internal/core/logger"

	"go.uber.org/zap"
)

func (h *NotificationsHTTPHandler) healthHandler(w http.ResponseWriter, r *http.Request) {
	const op = "NotificationsHTTPHandler.health"

	ctx := r.Context()
	logger := core_logger.FromContext(ctx)

	ctxDB, cancelDB := context.WithTimeout(ctx, 1*time.Second)
	defer cancelDB()

	if err := h.health.CheckDB(ctxDB); err != nil {
		logger.Error(
			"db health check failed",
			zap.String("op", op),
			zap.Error(err),
		)
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "database unavailable", "error": err.Error()})
		return
	}

	ctxCache, cancelCache := context.WithTimeout(ctx, 1*time.Second)
	defer cancelCache()

	if err := h.health.CheckCache(ctxCache); err != nil {
		logger.Error(
			"cache health check failed",
			zap.String("op", op),
			zap.Error(err),
		)
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "cache unavailable", "error": err.Error()})
		return
	}

	resp := HealthResponse{
		App:     h.appName,
		Version: h.appVersion,
		Status:  "available",
	}

	js, err := json.Marshal(resp)
	if err != nil {
		logger.Error(
			"marshal failed",
			zap.String("op", op),
			zap.Error(err),
		)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(js)
}

func (h *NotificationsHTTPHandler) getNotification(w http.ResponseWriter, r *http.Request) {
	const op = "NotificationsHTTPHandler.getNotification"

	ctx := r.Context()
	logger := core_logger.FromContext(ctx)

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendJSONError(w, core_errors.ErrInvalidArgument.Error(), http.StatusBadRequest)
		return
	}

	n, err := h.notifications.Get(ctx, id)
	if err != nil {
		if errors.Is(err, core_errors.ErrNotFound) {
			SendJSONError(w, core_errors.ErrNotFound.Error(), http.StatusNotFound)
			return
		}

		logger.Error(
			"get notification failed",
			zap.String("op", op),
			zap.Error(err),
		)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	js, err := json.Marshal(toNotificationResponse(n))
	if err != nil {
		logger.Error(
			"marshal failed",
			zap.String("op", op),
			zap.Error(err),
		)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(js)
}

func (h *NotificationsHTTPHandler) deleteNotification(w http.ResponseWriter, r *http.Request) {
	const op = "NotificationsHTTPHandler.deleteNotification"

	ctx := r.Context()
	logger := core_logger.FromContext(ctx)

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendJSONError(w, core_errors.ErrInvalidArgument.Error(), http.StatusBadRequest)
		return
	}

	delErr := h.notifications.Delete(ctx, id)
	if delErr != nil {
		if errors.Is(delErr, core_errors.ErrNotFound) {
			SendJSONError(w, core_errors.ErrNotFound.Error(), http.StatusNotFound)
			return
		}

		logger.Error(
			"delete notification failed",
			zap.String("op", op),
			zap.Error(delErr),
		)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationsHTTPHandler) listNotifications(w http.ResponseWriter, r *http.Request) {
	const op = "NotificationsHTTPHandler.listNotifications"

	ctx := r.Context()
	logger := core_logger.FromContext(ctx)

	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	size, _ := strconv.Atoi(query.Get("size"))

	notifications, err := h.notifications.List(ctx, page, size)
	if err != nil {
		logger.Error(
			"list notifications failed",
			zap.String("op", op),
			zap.Error(err),
		)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	js, err := json.Marshal(toNotificationResponses(notifications))
	if err != nil {
		logger.Error(
			"marshal failed",
			zap.String("op", op),
			zap.Error(err),
		)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(js)
}

func (h *NotificationsHTTPHandler) exportNotifications(w http.ResponseWriter, r *http.Request) {
	const op = "NotificationsHTTPHandler.exportNotification"

	ctx := r.Context()
	logger := core_logger.FromContext(ctx)

	count, err := h.notifications.Export(ctx)
	if err != nil {
		logger.Error(
			"export failed",
			zap.String("op", op),
			zap.Error(err),
		)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ExportResponse{Exported: count})
}
