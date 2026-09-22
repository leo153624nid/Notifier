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

type Producer struct {
	writer *kafka.Writer
}

func New(brokers []string, topic string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireOne,
			WriteTimeout: 5 * time.Second,
		},
	}
}

func (p *Producer) CLose() error {
	return p.writer.Close()
}

func (p *Producer) PublishUserLoggedInEvent(ctx context.Context, userID uuid.UUID, email string) error {
	const op = "Producer.PublishUserLoggedInEvent"

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
