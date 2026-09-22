package kafka

import (
	"context"
	"encoding/json"
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
		}
	}
}

// handleMessage разбирает одно Kafka-сообщение из топика auth.user.logged_in
// и создаёт по нему уведомление. Вынесена из Run отдельно, чтобы её можно
// было протестировать без реального брокера — Run отвечает только за цикл
// чтения, handleMessage — за саму бизнес-обработку события.
func (c *LoginConsumer) handleMessage(ctx context.Context, value []byte) error {
	const op = "Consumer.handleMessage"

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

	if _, err := c.service.Create(ctx, n, ""); err != nil {
		return fmt.Errorf("%s: create notification: %w", op, err)
	}

	return nil
}

func (c *LoginConsumer) Close() error {
	return c.reader.Close()
}
