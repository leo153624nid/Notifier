package transport_http

import (
	"context"
	"log/slog"
	"uuid"

	"notifier/internal/core/domain"
)

// Handler отвечает за перевод HTTP-запросов в вызовы сервисного слоя и обратно.
type NotificationsHTTPHandler struct {
	notifications NotificationService
	health        HealthService
	logger        *slog.Logger // TODO: delete ?
	appName       string       // TODO: delete ?
	appVersion    string       // TODO: delete ?
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
	logger *slog.Logger,
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
