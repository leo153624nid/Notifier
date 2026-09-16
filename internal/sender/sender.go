package sender

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"notifier/internal/domain"
)

type Sender interface {
	Send(ctx context.Context, n domain.Notification) error
}

// MockSender — тестовый Sender. Безопасен для конкурентных вызовов Send,
// поскольку в реальном коде отправка выполняется из фоновых горутин.
type MockSender struct {
	Calls []domain.Notification
	mu    sync.Mutex
}

func (m *MockSender) Send(ctx context.Context, n domain.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = append(m.Calls, n)
	return nil
}

type LoggingSender struct {
	Sender
	Logger *slog.Logger
}

type ConsoleSender struct {
	w io.Writer
}
type EmailSender struct {
	w io.Writer
}
type TelegramSender struct{}

func (ls LoggingSender) Send(ctx context.Context, n domain.Notification) error {
	ls.Logger.Info("sending", "to", n.Recipient, "channel", n.Channel)

	err := ls.Sender.Send(ctx, n)
	if err != nil {
		ls.Logger.Error("Failed to send notification", "error", err)
		return err
	}

	ls.Logger.Info("sent", "to", n.Recipient, "channel", n.Channel)

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

func (cs ConsoleSender) Send(ctx context.Context, n domain.Notification) error {
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
