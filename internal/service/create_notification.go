package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"go.uber.org/zap"

	"notifier/internal/core/domain"
	core_errors "notifier/internal/core/errors"
	core_logger "notifier/internal/core/logger"
	"notifier/internal/sender"
)

const sendTimeout = 2 * time.Second

// Create валидирует и сохраняет уведомление, затем асинхронно отправляет его
// через выбранный канал и обновляет статус по завершении отправки.
func (s *NotificationService) CreateIdempotent(
	ctx context.Context,
	consumer string,
	eventID uuid.UUID,
	n domain.Notification,
) (domain.Notification, error) {
	const op = "NotificationService.CreateIdempotent"

	logger := core_logger.FromContext(ctx)

	if err := n.Validate(); err != nil {
		return domain.Notification{}, fmt.Errorf("%s: %w: %v", op, core_errors.ErrInvalidNotification, err)
	}

	snd, ok := s.senders[n.Channel]
	if !ok {
		return domain.Notification{}, fmt.Errorf("%s: %w", op, core_errors.ErrUnsupportedChannel)
	}

	n, err := s.repo.CreateIdempotent(ctx, consumer, eventID, n)
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
