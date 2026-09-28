package service

import (
	"context"
	"fmt"
	"sync"

	"notifier/internal/audit"
	"notifier/internal/core/domain"
	"notifier/internal/sender"
)

// NotificationService содержит бизнес-логику работы с уведомлениями:
// валидацию, сохранение, асинхронную отправку через нужный канал и экспорт в аудит-лог.
type NotificationService struct {
	repo    NotificationRepo
	senders map[string]sender.Sender
	audit   *audit.Logger
	wg      sync.WaitGroup
}

func NewNotificationService(
	repo NotificationRepo,
	senders map[string]sender.Sender,
	auditLogger *audit.Logger,
) (*NotificationService, error) {
	const op = "NewNotificationService"

	if repo == nil {
		return nil, fmt.Errorf("%s: repo is required", op)
	}
	if len(senders) == 0 {
		return nil, fmt.Errorf("%s: senders is required", op)
	}
	if auditLogger == nil {
		return nil, fmt.Errorf("%s: auditLogger is required", op)
	}

	return &NotificationService{
		repo:    repo,
		senders: senders,
		audit:   auditLogger,
	}, nil
}

// Wait блокируется до завершения всех фоновых отправок — используется при graceful shutdown.
func (s *NotificationService) Wait() {
	s.wg.Wait()
}


func (s *NotificationService) Get(ctx context.Context, id int) (domain.Notification, error) {
	const op = "NotificationService.Get"

	n, err := s.repo.GetById(ctx, id)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: %w", op, err)
	}

	return n, nil
}

func (s *NotificationService) Delete(ctx context.Context, id int) error {
	const op = "NotificationService.Delete"

	err := s.repo.DeleteById(ctx, id)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

// List возвращает страницу уведомлений, нормализуя page/size к разумным значениям по умолчанию.
func (s *NotificationService) List(ctx context.Context, page int, size int) ([]domain.Notification, error) {
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
