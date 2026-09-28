package service

import (
	"context"
	"uuid"

	"notifier/internal/core/domain"
)

type NotificationRepo interface {
	CreateIdempotent(
		ctx context.Context,
		consumer string,
		eventID uuid.UUID,
		n domain.Notification,
	) (int, error)

	GetAll(ctx context.Context) ([]domain.Notification, error)

	GetList(
		ctx context.Context,
		page int,
		size int,
	) ([]domain.Notification, error)

	GetById(
		ctx context.Context,
		id int,
	) (domain.Notification, error)

	DeleteById(
		ctx context.Context,
		id int,
	) error

	UpdateStatus(
		ctx context.Context,
		id int,
		status string,
	) error
}
