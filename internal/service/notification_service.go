package service

import (
	"fmt"
	"sync"

	"notifier/internal/audit"
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
