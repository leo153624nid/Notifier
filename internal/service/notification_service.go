package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	"uuid"

	"go.uber.org/zap"

	"notifier/internal/audit"
	"notifier/internal/core/domain"
	core_errors "notifier/internal/core/errors"
	core_logger "notifier/internal/core/logger"
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
	wg      sync.WaitGroup
}

func NewNotificationService(
	repo repository.NotificationRepo,
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

// Create валидирует и сохраняет уведомление, затем асинхронно отправляет его
// через выбранный канал и обновляет статус по завершении отправки.
func (s *NotificationService) Create(
	ctx context.Context,
	n domain.Notification,
) (domain.Notification, error) {
	const op = "NotificationService.Create"

	logger := core_logger.FromContext(ctx)

	if err := n.Validate(); err != nil {
		return domain.Notification{}, fmt.Errorf("%s: %w: %v", op, core_errors.ErrInvalidNotification, err)
	}

	snd, ok := s.senders[n.Channel]
	if !ok {
		return domain.Notification{}, fmt.Errorf("%s: %w", op, core_errors.ErrUnsupportedChannel)
	}

	id, err := s.repo.Save(ctx, n)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: save: %w", op, err)
	}

	n.ID = id
	n.Status = "pending"

	s.wg.Add(1)
	go s.sendAndUpdateStatus(snd, n, logger)

	return n, nil
}

func (s *NotificationService) sendAndUpdateStatus(
	snd sender.Sender,
	n domain.Notification,
	logger *core_logger.Logger,
) {
	defer s.wg.Done()

	sendCtx, sendCancel := context.WithTimeout(context.Background(), sendTimeout)
	defer sendCancel()

	sendCtx = core_logger.ToContext(sendCtx, logger)

	status := "sent"
	if err := snd.Send(sendCtx, n); err != nil {
		logger.Error(
			"notification send failed",
			zap.Error(err),
		)
		status = "failed"
	} else {
		logger.Info(
			"notification sent",
			zap.Int("id", n.ID),
			zap.String("channel", n.Channel),
		)
	}

	updateCtx, updateCancel := context.WithTimeout(context.Background(), sendTimeout)
	defer updateCancel()

	if err := s.repo.UpdateStatus(updateCtx, n.ID, status); err != nil {
		logger.Error(
			"update status failed",
			zap.Int("id", n.ID),
			zap.Error(err),
		)
	}
}

func (s *NotificationService) CreateIdempotent(
	ctx context.Context,
	consumer string,
	eventID uuid.UUID,
	n domain.Notification,
) (domain.Notification, error) {
	const op = "NotificationService.CreateIdempotent"

	logger := core_logger.FromContext(ctx)

	if err := n.Validate(); err != nil {
		return domain.Notification{}, fmt.Errorf("%s: %w: %w", op, core_errors.ErrInvalidNotification, err)
	}

	snd, ok := s.senders[n.Channel]
	if !ok {
		return domain.Notification{}, fmt.Errorf("%s: %w", op, core_errors.ErrUnsupportedChannel)
	}

	id, err := s.repo.SaveIdempotent(ctx, consumer, eventID, n)
	if err != nil {
		if errors.Is(err, core_errors.ErrEventAlreadyProcessed) {
			return domain.Notification{}, fmt.Errorf("%s: %w", op, core_errors.ErrEventAlreadyProcessed)
		}

		logger.Error(
			"save idempotent failed",
			zap.String("op", op),
			zap.Error(err),
		)
		return domain.Notification{}, fmt.Errorf("%s: save idempotent: %w", op, err)
	}

	n.ID = id
	n.Status = "pending"

	s.wg.Add(1)
	go s.sendAndUpdateStatus(snd, n, logger)

	return n, nil
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

// Wait блокируется до завершения всех фоновых отправок — используется при graceful shutdown.
func (s *NotificationService) Wait() {
	s.wg.Wait()
}
