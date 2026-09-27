package transport_http

import (
	"context"
	"notifier/internal/core/domain"
	"uuid"
)

type NotificationService interface {
	Create(
		ctx context.Context,
		n domain.Notification,
	) (domain.Notification, error)

	CreateIdempotent(
		ctx context.Context,
		consumer string,
		eventID uuid.UUID,
		n domain.Notification,
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
