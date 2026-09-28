package service

import (
	"context"
	"fmt"
)

// Export выгружает все уведомления в аудит-лог и возвращает количество экспортированных записей.
func (s *NotificationService) Export(ctx context.Context) (int, error) {
	const op = "NotificationService.Export"

	notifications, err := s.repo.GetAll(ctx)
	if err != nil {
		return 0, fmt.Errorf("%s: get all: %w", op, err)
	}

	if err := s.audit.Write(notifications); err != nil {
		return 0, fmt.Errorf("%s: write: %w", op, err)
	}

	return len(notifications), nil
}
