package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	authevents "contracts/events/auth/v1"
	"notifier/internal/domain"
	"notifier/internal/service"

	"github.com/segmentio/kafka-go"
)

type LoginConsumer struct {
	reader  *kafka.Reader
	service *service.NotificationService
	logger  *slog.Logger
	groupID string
}

func NewLoginConsumer(
	brokers []string,
	groupID string,
	service *service.NotificationService,
	logger *slog.Logger,
) *LoginConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   authevents.TopicUserLoggedIn,
		GroupID: groupID,
	})

	return &LoginConsumer{
		reader:  reader,
		service: service,
		logger:  logger,
		groupID: groupID,
	}
}

func (c *LoginConsumer) Run(ctx context.Context) {
	const op = "Consumer.Run"

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			c.logger.Error("kafka read failed", "op", op, "error", err)
			continue
		}

		if err := c.handleMessage(ctx, msg.Value); err != nil {
			c.logger.Error("handle message failed", "op", op, "error", err)
			continue
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error("failed to commit msg", "op", op, "error", err)
		}
	}
}

// handleMessage разбирает одно Kafka-сообщение из топика auth.user.logged_in
// и создаёт по нему уведомление.
func (c *LoginConsumer) handleMessage(ctx context.Context, value []byte) error {
	const op = "LoginConsumer.handleMessage"

	var event authevents.UserLoggedIn
	if err := json.Unmarshal(value, &event); err != nil {
		return fmt.Errorf("%s: unmarshal: %w", op, err)
	}

	n := domain.Notification{
		Recipient: event.Email,
		Subject:   "New login",
		Body:      "A new login to your account was detected",
		Channel:   "email",
		IsUrgent:  true,
	}

	_, createErr := c.service.CreateIdempotent(ctx, c.groupID, event.EventID, n, event.EventID.String())
	if createErr != nil {
		if errors.Is(createErr, domain.ErrEventAlreadyProcessed) {
			c.logger.Info("duplicate login event skipped", "op", op, "event_id", event.EventID)
			return nil
		}
		return fmt.Errorf("%s: create notification: %w", op, createErr)
	}

	return nil
}

func (c *LoginConsumer) Close() error {
	return c.reader.Close()
}
