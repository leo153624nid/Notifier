package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"notifier/internal/audit"
	"notifier/internal/domain"
	"notifier/internal/repository"
	"notifier/internal/sender"
)

const sendTimeout = 2 * time.Second

// NotificationService содержит бизнес-логику работы с уведомлениями:
// валидацию, сохранение, асинхронную отправку через нужный канал и экспорт в аудит-лог.
type NotificationService struct {
	repo    repository.NotificationRepo
	senders map[string]sender.Sender
	audit   *audit.Logger
	logger  *slog.Logger
	wg      sync.WaitGroup
}

func NewNotificationService(
	repo repository.NotificationRepo,
	senders map[string]sender.Sender,
	auditLogger *audit.Logger,
	logger *slog.Logger,
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
	if logger == nil {
		return nil, fmt.Errorf("%s: logger is required", op)
	}

	return &NotificationService{
		repo:    repo,
		senders: senders,
		audit:   auditLogger,
		logger:  logger,
	}, nil
}

// Create валидирует и сохраняет уведомление, затем асинхронно отправляет его
// через выбранный канал и обновляет статус по завершении отправки.
func (s *NotificationService) Create(
	ctx context.Context,
	n domain.Notification,
	requestID string,
) (domain.Notification, error) {
	const op = "NotificationService.Create"

	if err := n.Validate(); err != nil {
		return domain.Notification{}, fmt.Errorf("%s: %w: %w", op, ErrInvalidNotification, err)
	}

	snd, ok := s.senders[n.Channel]
	if !ok {
		return domain.Notification{}, fmt.Errorf("%s: %w", op, ErrUnsupportedChannel)
	}

	id, err := s.repo.Save(ctx, n)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: save: %w", op, err)
	}

	n.ID = id
	n.Status = "pending"

	s.wg.Add(1)
	go s.sendAndUpdateStatus(snd, n, requestID)

	return n, nil
}

func (s *NotificationService) sendAndUpdateStatus(
	snd sender.Sender,
	n domain.Notification,
	requestID string,
) {
	defer s.wg.Done()

	reqLogger := s.logger.With("request_id", requestID)

	sendCtx, sendCancel := context.WithTimeout(context.Background(), sendTimeout)
	defer sendCancel()

	status := "sent"
	if err := snd.Send(sendCtx, n); err != nil {
		reqLogger.Error("send failed", "id", n.ID, "error", err)
		status = "failed"
	} else {
		reqLogger.Info("notification sent", "id", n.ID, "channel", n.Channel)
	}

	updateCtx, updateCancel := context.WithTimeout(context.Background(), sendTimeout)
	defer updateCancel()

	if err := s.repo.UpdateStatus(updateCtx, n.ID, status); err != nil {
		reqLogger.Error("update status failed", "id", n.ID, "error", err)
	}
}

func (s *NotificationService) Get(ctx context.Context, id int) (domain.Notification, error) {
	const op = "NotificationService.Get"

	n, err := s.repo.GetById(ctx, id)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: %w", op, err)
	}

	return n, nil
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

// Wait блокируется до завершения всех фоновых отправок — используется при graceful shutdown.
func (s *NotificationService) Wait() {
	s.wg.Wait()
}
