package kafkaproducer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"uuid"

	authevents "contracts/events/auth/v1"

	"github.com/segmentio/kafka-go"
)

type LoginProducer struct {
	writer *kafka.Writer
}

func NewLoginProducer(brokers []string) *LoginProducer {
	return &LoginProducer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        authevents.TopicUserLoggedIn,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireOne,
			WriteTimeout: 5 * time.Second,
		},
	}
}

func (p *LoginProducer) CLose() error {
	return p.writer.Close()
}

func (p *LoginProducer) PublishUserLoggedInEvent(
	ctx context.Context,
	userID uuid.UUID,
	email string,
) error {
	const op = "LoginProducer.PublishUserLoggedInEvent"

	event := authevents.UserLoggedIn{
		UserID:     userID,
		Email:      email,
		OccurredAt: time.Now(),
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("%s: marshal: %w", op, err)
	}

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   event.UserID[:],
		Value: payload,
	})
}
