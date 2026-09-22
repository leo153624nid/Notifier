package kafka

import (
	"context"
	"encoding/json"
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

		var event authevents.UserLoggedIn
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			c.logger.Error("unmarshal failed", "op", op, "error", err)
			continue
		}

		n := domain.Notification{
			Recipient: event.Email,
			Subject:   "New login",
			Body:      "A new login to your account was detected",
			Channel:   "email",
			IsUrgent:  true,
		}

		if _, err := c.service.Create(ctx, n, ""); err != nil {
			c.logger.Error("create notification failed", "op", op, "error", err)
			continue
		}
	}
}

func (c *LoginConsumer) Close() error {
	return c.reader.Close()
}
