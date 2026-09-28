package service

import (
	"context"
	"fmt"

	"notifier/internal/core/domain"
)

// List возвращает страницу уведомлений, нормализуя page/size к разумным значениям по умолчанию.
func (s *NotificationService) List(
	ctx context.Context,
	page int,
	size int,
) ([]domain.Notification, error) {
	const op = "NotificationService.List"

	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	notifications, err := s.repo.GetList(ctx, page, size)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return notifications, nil
}
