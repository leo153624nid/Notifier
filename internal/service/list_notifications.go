package service

import (
	"context"
	"fmt"

	"notifier/internal/core/domain"
	core_errors "notifier/internal/core/errors"
)

// List возвращает страницу уведомлений, нормализуя page/size к разумным значениям по умолчанию.
func (s *NotificationService) List(
	ctx context.Context,
	page *int,
	size *int,
) ([]domain.Notification, error) {
	const op = "NotificationService.List"

	if page != nil && *page < 0 {
		return nil, fmt.Errorf("%s: %w", op, core_errors.ErrInvalidArgument)
	}
	if size != nil && *size < 0 {
		return nil, fmt.Errorf("%s: %w", op, core_errors.ErrInvalidArgument)
	}

	notifications, err := s.repo.GetList(ctx, page, size)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return notifications, nil
}
