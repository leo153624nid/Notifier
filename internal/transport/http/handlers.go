package transport_http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"notifier/internal/core/domain"

	"go.uber.org/zap"
)

func (h *NotificationsHTTPHandler) healthHandler(w http.ResponseWriter, r *http.Request) {
	const op = "Handler.health"

	ctxDB, cancelDB := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancelDB()

	if err := h.health.CheckDB(ctxDB); err != nil {
		h.logger.Error(
			"db health check failed",
			zap.String("op", op),
			zap.Error(err),
		)
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "database unavailable", "error": err.Error()})
		return
	}

	ctxCache, cancelCache := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancelCache()

	if err := h.health.CheckCache(ctxCache); err != nil {
		h.logger.Error(
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
		h.logger.Error(
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
	const op = "Handler.getNotification"

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendJSONError(w, domain.ErrInvalidID.Error(), http.StatusBadRequest)
		return
	}

	n, err := h.notifications.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			SendJSONError(w, domain.ErrNotFound.Error(), http.StatusNotFound)
			return
		}

		h.logger.Error(
			"get notification failed",
			zap.String("op", op),
			zap.Error(err),
		)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	js, err := json.Marshal(toNotificationResponse(n))
	if err != nil {
		h.logger.Error(
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
	const op = "Handler.deleteNotification"

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendJSONError(w, domain.ErrInvalidID.Error(), http.StatusBadRequest)
		return
	}

	delErr := h.notifications.Delete(r.Context(), id)
	if delErr != nil {
		if errors.Is(delErr, domain.ErrNotFound) {
			SendJSONError(w, domain.ErrNotFound.Error(), http.StatusNotFound)
			return
		}

		h.logger.Error(
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
	const op = "Handler.listNotifications"

	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	size, _ := strconv.Atoi(query.Get("size"))

	notifications, err := h.notifications.List(r.Context(), page, size)
	if err != nil {
		h.logger.Error(
			"list notifications failed",
			zap.String("op", op),
			zap.Error(err),
		)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	js, err := json.Marshal(toNotificationResponses(notifications))
	if err != nil {
		h.logger.Error(
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

func (h *NotificationsHTTPHandler) exportNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Handler.exportNotification"

	count, err := h.notifications.Export(r.Context())
	if err != nil {
		h.logger.Error(
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
