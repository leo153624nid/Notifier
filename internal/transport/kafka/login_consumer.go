package transport_kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	authevents "contracts/events/auth/v1"
	"notifier/internal/core/domain"
	core_logger "notifier/internal/core/logger"
	"notifier/internal/service"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

// messageWriter — часть API *kafka.Writer, нужная для отправки в DLQ;
// messageReader — часть API *kafka.Reader, нужная для основного цикла.
// Обе выделены в интерфейсы, чтобы подставлять моки в тестах без сетевого
// брокера.
type messageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type messageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type LoginConsumer struct {
	reader    messageReader
	dlqWriter messageWriter
	service   *service.NotificationService
	logger    *core_logger.Logger
	groupID   string

	// wg отслеживает фактическое завершение горутины Run — Close ждёт её
	// перед закрытием reader/dlqWriter, чтобы не закрыть их из-под ещё
	// работающего цикла (гонка: cancel() контекста лишь сигнал, а не
	// гарантия, что Run уже вышел).
	wg sync.WaitGroup
}

func NewLoginConsumer(
	brokers []string,
	loginTopic string,
	groupID string,
	dlqTopic string,
	service *service.NotificationService,
	logger *core_logger.Logger,
) *LoginConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   loginTopic,
		GroupID: groupID,
	})

	dlqWriter := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        dlqTopic,
		Balancer:     &kafka.Hash{},
		RequiredAcks: kafka.RequireOne,
		WriteTimeout: 5 * time.Second,
	}

	return &LoginConsumer{
		reader:    reader,
		dlqWriter: dlqWriter,
		service:   service,
		logger:    logger,
		groupID:   groupID,
	}
}

func (c *LoginConsumer) Run(ctx context.Context) {
	const op = "Consumer.Run"

	c.wg.Add(1)
	defer c.wg.Done()

	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}

			c.logger.Error(
				"kafka read failed",
				zap.String("op", op),
				zap.Error(err),
			)
			continue
		}

		if err := c.handleMessage(ctx, msg.Value); err != nil {
			c.logger.Error(
				"handle message failed",
				zap.String("op", op),
				zap.Error(err),
			)

			if dlqErr := c.sendToDLQ(ctx, msg, err); dlqErr != nil {
				c.logger.Error(
					"failed to send msg to dlq",
					zap.String("op", op),
					zap.Error(dlqErr),
				)
				continue
			}
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			c.logger.Error(
				"failed to commit msg",
				zap.String("op", op),
				zap.Error(err),
			)
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
			c.logger.Info(
				"duplicate login event skipped",
				zap.String("op", op),
				zap.String("event_id", event.EventID.String()),
			)
			return nil
		}
		return fmt.Errorf("%s: create notification: %w", op, createErr)
	}

	return nil
}

func (c *LoginConsumer) Close(ctx context.Context) error {
	// Close вызывается сразу после отмены контекста, переданного в Run —
	// сама отмена лишь сигнал, а не гарантия, что горутина Run уже вышла
	// из цикла. Дожидаемся её реального завершения через wg, иначе можно
	// закрыть reader/dlqWriter из-под ещё работающей итерации Run.
	waitDone := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("wait for consumer loop to stop: %w", ctx.Err())
	case <-waitDone:
	}

	closeChan := make(chan error, 1)

	go func() {
		if err := c.reader.Close(); err != nil {
			closeChan <- fmt.Errorf("close reader: %w", err)
			return
		}
		if err := c.dlqWriter.Close(); err != nil {
			closeChan <- fmt.Errorf("close dlq writer: %w", err)
			return
		}
		closeChan <- nil
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-closeChan:
		return err
	}
}

// sendToDLQ переносит недоставленное сообщение в dead-letter топик,
// сохраняя причину ошибки в заголовке, чтобы её можно было разобрать позже.
func (c *LoginConsumer) sendToDLQ(ctx context.Context, msg kafka.Message, cause error) error {
	return c.dlqWriter.WriteMessages(ctx, kafka.Message{
		Key:   msg.Key,
		Value: msg.Value,
		Headers: []kafka.Header{
			{Key: "error", Value: []byte(cause.Error())},
			{Key: "source_topic", Value: []byte(msg.Topic)},
		},
	})
}
