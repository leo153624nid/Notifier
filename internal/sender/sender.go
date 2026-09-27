package sender

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	"notifier/internal/core/domain"
	core_logger "notifier/internal/core/logger"
)

type Sender interface {
	Send(
		ctx context.Context,
		n domain.Notification,
	) error
}

// MockSender — тестовый Sender. Безопасен для конкурентных вызовов Send,
// поскольку в реальном коде отправка выполняется из фоновых горутин.
type MockSender struct {
	Calls []domain.Notification
	mu    sync.Mutex
}

func (m *MockSender) Send(
	ctx context.Context,
	n domain.Notification,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = append(m.Calls, n)
	return nil
}

type ConsoleSender struct {
	w io.Writer
}
type EmailSender struct {
	w io.Writer
}
type TelegramSender struct{}

type SenderService struct {
	Sender
}

func NewSenderService(
	sender Sender,
) *SenderService {
	return &SenderService{
		Sender: sender,
	}
}

func (s SenderService) Send(
	ctx context.Context,
	n domain.Notification,
) error {
	logger := core_logger.FromContext(ctx)

	logger.Info(
		"sending",
		zap.String("to", n.Recipient),
		zap.String("channel", n.Channel),
	)

	if err := s.Sender.Send(ctx, n); err != nil {
		logger.Error(
			"Failed to send notification",
			zap.Error(err),
		)
		return err
	}

	logger.Info(
		"sent",
		zap.String("to", n.Recipient),
		zap.String("channel", n.Channel),
	)

	return nil
}

func NewConsoleSender(w io.Writer) *ConsoleSender {
	if w == nil {
		w = os.Stdout
	}
	return &ConsoleSender{w}
}

func NewEmailSender(w io.Writer) *EmailSender {
	if w == nil {
		w = os.Stdout
	}
	return &EmailSender{w}
}

func (cs ConsoleSender) Send(
	ctx context.Context,
	n domain.Notification,
) error {
	_, err := fmt.Fprintf(cs.w, "[console] to %s | %s\n", n.Recipient, n.Subject)
	return err
}

func (es EmailSender) Send(ctx context.Context, n domain.Notification) error {
	_, err := fmt.Fprintf(es.w, "[email] to %s | %s\n", n.Recipient, n.Subject)
	return err
}

func (tg TelegramSender) Send(ctx context.Context, n domain.Notification) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second): // Immitation of telegram API working
		fmt.Printf("[telegram] to %s | %s\n", n.Recipient, n.Subject)
		return nil
	}
}
