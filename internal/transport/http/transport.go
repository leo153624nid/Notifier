package transport_http

import (
	"context"
	"net/http"
	"uuid"

	"notifier/internal/core/domain"
	core_logger "notifier/internal/core/logger"
	core_http_server "notifier/internal/core/transport/http/server"
)

// Handler отвечает за перевод HTTP-запросов в вызовы сервисного слоя и обратно.
type NotificationsHTTPHandler struct {
	notifications NotificationService
	health        HealthService
	logger        *core_logger.Logger // TODO: delete ?
	appName       string
	appVersion    string
}

type NotificationService interface {
	Create(
		ctx context.Context,
		n domain.Notification,
		requestID string,
	) (domain.Notification, error)

	CreateIdempotent(
		ctx context.Context,
		consumer string,
		eventID uuid.UUID,
		n domain.Notification,
		requestID string,
	) (domain.Notification, error)

	Get(
		ctx context.Context,
		id int,
	) (domain.Notification, error)

	Delete(
		ctx context.Context,
		id int,
	) error

	// List возвращает страницу уведомлений, нормализуя page/size к разумным значениям по умолчанию.
	List(
		ctx context.Context,
		page int,
		size int,
	) ([]domain.Notification, error)

	// Export выгружает все уведомления в аудит-лог и возвращает количество экспортированных записей.
	Export(ctx context.Context) (int, error)

	// Wait блокируется до завершения всех фоновых отправок — используется при graceful shutdown.
	Wait()
}

type HealthService interface {
	CheckDB(ctx context.Context) error
	CheckCache(ctx context.Context) error
}

func NewNotificationsHTTPHandler(
	notifications NotificationService,
	health HealthService,
	logger *core_logger.Logger,
	appName string,
	appVersion string,
) *NotificationsHTTPHandler {
	return &NotificationsHTTPHandler{
		notifications: notifications,
		health:        health,
		logger:        logger,
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
		// TODO
	}
}
