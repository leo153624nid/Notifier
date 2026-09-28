package service

import (
	"context"
	"fmt"

	"notifier/internal/core/domain"
)

func (s *NotificationService) Get(
	ctx context.Context,
	id int,
) (domain.Notification, error) {
	const op = "NotificationService.Get"

	n, err := s.repo.GetById(ctx, id)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: %w", op, err)
	}

	return n, nil
}
