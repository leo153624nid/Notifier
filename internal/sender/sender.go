package sender

import (
	"fmt"
	"io"
	"log/slog"
	"notifier/internal/notification"
	"os"
)

type MockSender struct {
	Calls []notification.Notification
}

func (m *MockSender) Send(n notification.Notification) error {
	m.Calls = append(m.Calls, n)
	return nil
}

type Sender interface {
	Send(notification.Notification) error
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

func (ls LoggingSender) Send(n notification.Notification) error {
	ls.Logger.Info("sending", "to", n.Recipient, "channel", n.Channel)
	err := ls.Sender.Send(n)
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

func (cs ConsoleSender) Send(n notification.Notification) error {
	_, err := fmt.Fprintf(cs.w, "[console] to %s | %s\n", n.Recipient, n.Subject)
	return err
}

func (es EmailSender) Send(n notification.Notification) error {
	_, err := fmt.Fprintf(es.w, "[email] to %s | %s\n", n.Recipient, n.Subject)
	return err
}

func (tg TelegramSender) Send(n notification.Notification) error {
	fmt.Printf("[telegram] to %s | %s\n", n.Recipient, n.Subject)
	return nil
}
