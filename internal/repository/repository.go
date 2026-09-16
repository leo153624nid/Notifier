package repository

import (
	"context"

	"notifier/internal/notification"
)

type Repository interface {
	Save(ctx context.Context, n notification.Notification) (int, error)
	GetAll(ctx context.Context) ([]notification.Notification, error)
	GetList(ctx context.Context, page int, size int) ([]notification.Notification, error)
	GetById(ctx context.Context, id int) (notification.Notification, error)
	UpdateStatus(ctx context.Context, id int, status string) error
}
