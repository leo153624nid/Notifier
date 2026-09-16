package repository

import (
	"context"

	"notifier/internal/domain"
)

type Repository interface {
	Save(ctx context.Context, n domain.Notification) (int, error)
	GetAll(ctx context.Context) ([]domain.Notification, error)
	GetList(ctx context.Context, page int, size int) ([]domain.Notification, error)
	GetById(ctx context.Context, id int) (domain.Notification, error)
	UpdateStatus(ctx context.Context, id int, status string) error
}
