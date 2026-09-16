package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"notifier/internal/notification"
	"notifier/internal/service"
)

// Handler отвечает за перевод HTTP-запросов в вызовы сервисного слоя и обратно.
type Handler struct {
	notifications *service.NotificationService
	health        *service.HealthService
	logger        *slog.Logger
	appName       string
	appVersion    string
}

func NewHandler(
	notifications *service.NotificationService,
	health *service.HealthService,
	logger *slog.Logger,
	appName string,
	appVersion string,
) *Handler {
	return &Handler{
		notifications: notifications,
		health:        health,
		logger:        logger,
		appName:       appName,
		appVersion:    appVersion,
	}
}

func (h *Handler) healthHandler(w http.ResponseWriter, r *http.Request) {
	const op = "Handler.health"

	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()

	if err := h.health.Check(ctx); err != nil {
		h.logger.Error("health check failed", "op", op, "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "service unavailable", "error": err.Error()})
		return
	}

	resp := HealthResponse{
		App:     h.appName,
		Version: h.appVersion,
		Status:  "available",
	}

	js, err := json.Marshal(resp)
	if err != nil {
		h.logger.Error("marshal failed", "op", op, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(js)
}

func (h *Handler) getNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Handler.getNotification"

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		SendJSONError(w, notification.ErrInvalidId.Error(), http.StatusBadRequest)
		return
	}

	n, err := h.notifications.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, notification.ErrNotFound) {
			SendJSONError(w, notification.ErrNotFound.Error(), http.StatusNotFound)
			return
		}

		h.logger.Error("get notification failed", "op", op, "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	js, err := json.Marshal(toNotificationResponse(n))
	if err != nil {
		h.logger.Error("marshal failed", "op", op, "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(js)
}

func (h *Handler) listNotifications(w http.ResponseWriter, r *http.Request) {
	const op = "Handler.listNotifications"

	query := r.URL.Query()
	page, _ := strconv.Atoi(query.Get("page"))
	size, _ := strconv.Atoi(query.Get("size"))

	notifications, err := h.notifications.List(r.Context(), page, size)
	if err != nil {
		h.logger.Error("list notifications failed", "op", op, "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	js, err := json.Marshal(toNotificationResponses(notifications))
	if err != nil {
		h.logger.Error("marshal failed", "op", op, "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(js)
}

func (h *Handler) createNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Handler.createNotification"

	var req CreateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendJSONError(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	requestID := getRequestID(r.Context())

	n, err := h.notifications.Create(r.Context(), req.toDomain(), requestID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidNotification):
			SendJSONError(w, "invalid request", http.StatusBadRequest)
		case errors.Is(err, service.ErrUnsupportedChannel):
			SendJSONError(w, "invalid request: unsupported channel", http.StatusBadRequest)
		default:
			h.logger.Error("create notification failed", "op", op, "error", err)
			SendJSONError(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	js, err := json.Marshal(toNotificationResponse(n))
	if err != nil {
		h.logger.Error("marshal failed", "op", op, "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write(js)
}

func (h *Handler) exportNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Handler.exportNotification"

	count, err := h.notifications.Export(r.Context())
	if err != nil {
		h.logger.Error("export failed", "op", op, "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ExportResponse{Exported: count})
}
