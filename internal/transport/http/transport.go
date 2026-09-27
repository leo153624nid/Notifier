package transport_http

import (
	"net/http"

	core_http_server "notifier/internal/core/transport/http/server"
)

// Handler отвечает за перевод HTTP-запросов в вызовы сервисного слоя и обратно.
type NotificationsHTTPHandler struct {
	notifications NotificationService
	health        HealthService
	appName       string
	appVersion    string
}

func NewNotificationsHTTPHandler(
	notifications NotificationService,
	health HealthService,
	appName string,
	appVersion string,
) *NotificationsHTTPHandler {
	return &NotificationsHTTPHandler{
		notifications: notifications,
		health:        health,
		appName:       appName,
		appVersion:    appVersion,
	}
}

func (h *NotificationsHTTPHandler) Routes() []core_http_server.Route {
	return []core_http_server.Route{
		{
			Method:  http.MethodGet,
			Path:    "/health",
			Handler: h.healthHandler,
		},
		{
			Method:  http.MethodGet,
			Path:    "/notifications",
			Handler: h.listNotifications,
		},
		{
			Method:  http.MethodGet,
			Path:    "/notifications/export",
			Handler: h.exportNotifications,
		},
		{
			Method:  http.MethodGet,
			Path:    "/notifications/{id}",
			Handler: h.getNotification,
		},
		{
			Method:  http.MethodDelete,
			Path:    "/notifications/{id}",
			Handler: h.deleteNotification,
		},
		{
			Method:  http.MethodPost,
			Path:    "/notifications",
			Handler: h.createNotification,
		},
	}
}
