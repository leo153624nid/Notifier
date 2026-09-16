package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"notifier/internal/audit"
	"notifier/internal/notification"
	"notifier/internal/repository"
	"notifier/internal/sender"
)

func newTestService(t *testing.T, senders map[string]sender.Sender) *NotificationService {
	t.Helper()

	repo := repository.NewMemoryRepository()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	auditLogger := audit.NewLogger(t.TempDir() + "/audit.log")

	s, err := NewNotificationService(repo, senders, auditLogger, logger)
	if err != nil {
		t.Fatalf("NewNotificationService() error: %s", err)
	}

	return s
}

func TestNewNotificationService(t *testing.T) {
	repo := repository.NewMemoryRepository()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	senders := map[string]sender.Sender{"console": &sender.MockSender{}}
	auditLogger := audit.NewLogger(t.TempDir() + "/audit.log")

	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "valid", wantErr: false},
		{name: "no repo", wantErr: true},
		{name: "no senders", wantErr: true},
		{name: "no auditLogger", wantErr: true},
		{name: "no logger", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			switch tt.name {
			case "no repo":
				_, err = NewNotificationService(nil, senders, auditLogger, logger)
			case "no senders":
				_, err = NewNotificationService(repo, nil, auditLogger, logger)
			case "no auditLogger":
				_, err = NewNotificationService(repo, senders, nil, logger)
			case "no logger":
				_, err = NewNotificationService(repo, senders, auditLogger, nil)
			default:
				_, err = NewNotificationService(repo, senders, auditLogger, logger)
			}

			if err != nil && !tt.wantErr {
				t.Errorf("NewNotificationService() error: %s", err)
			}
			if err == nil && tt.wantErr {
				t.Errorf("err is nil, want error")
			}
		})
	}
}

func TestNotificationService_Create(t *testing.T) {
	//nolint:govet
	tests := []struct {
		name      string
		n         notification.Notification
		wantErr   error
		wantCalls int
	}{
		{
			name:      "valid request",
			n:         notification.Notification{Recipient: "user@example.com", Subject: "some", Channel: "email"},
			wantErr:   nil,
			wantCalls: 1,
		},
		{
			name:      "empty recipient",
			n:         notification.Notification{Recipient: "", Subject: "some", Channel: "email"},
			wantErr:   ErrInvalidNotification,
			wantCalls: 0,
		},
		{
			name:      "empty channel",
			n:         notification.Notification{Recipient: "user@example.com", Subject: "some", Channel: ""},
			wantErr:   ErrInvalidNotification,
			wantCalls: 0,
		},
		{
			name:      "unknown channel",
			n:         notification.Notification{Recipient: "user@example.com", Subject: "some", Channel: "sms"},
			wantErr:   ErrUnsupportedChannel,
			wantCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &sender.MockSender{}
			s := newTestService(t, map[string]sender.Sender{"email": mock})

			_, err := s.Create(context.Background(), tt.n, "req-1")
			s.Wait()

			if tt.wantErr == nil && err != nil {
				t.Errorf("Create() error: %s", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("Create() error = %v, want %v", err, tt.wantErr)
			}
			if len(mock.Calls) != tt.wantCalls {
				t.Errorf("calls = %d, want %d", len(mock.Calls), tt.wantCalls)
			}
		})
	}
}

func TestNotificationService_Create_UpdatesStatusAfterSend(t *testing.T) {
	mock := &sender.MockSender{}
	s := newTestService(t, map[string]sender.Sender{"email": mock})

	n, err := s.Create(context.Background(), notification.Notification{
		Recipient: "user@example.com",
		Channel:   "email",
	}, "req-1")
	if err != nil {
		t.Fatalf("Create() error: %s", err)
	}
	if n.Status != "pending" {
		t.Errorf("initial status = %q, want %q", n.Status, "pending")
	}

	s.Wait()

	got, err := s.Get(context.Background(), n.ID)
	if err != nil {
		t.Fatalf("Get() error: %s", err)
	}
	if got.Status != "sent" {
		t.Errorf("status after send = %q, want %q", got.Status, "sent")
	}
}

func TestNotificationService_Get(t *testing.T) {
	mock := &sender.MockSender{}
	s := newTestService(t, map[string]sender.Sender{"email": mock})

	created, err := s.Create(context.Background(), notification.Notification{
		Recipient: "needed recipient",
		Channel:   "email",
	}, "req-1")
	if err != nil {
		t.Fatalf("Create() error: %s", err)
	}
	s.Wait()

	got, err := s.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() error: %s", err)
	}
	if got.Recipient != "needed recipient" {
		t.Errorf("recipient = %q, want %q", got.Recipient, "needed recipient")
	}

	_, err = s.Get(context.Background(), 9999)
	if !errors.Is(err, notification.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestNotificationService_List(t *testing.T) {
	mock := &sender.MockSender{}
	s := newTestService(t, map[string]sender.Sender{"email": mock})

	for i := range 3 {
		_, err := s.Create(context.Background(), notification.Notification{
			Recipient: fmt.Sprintf("recipient #%d", i),
			Channel:   "email",
		}, "req-1")
		if err != nil {
			t.Fatalf("Create() error: %s", err)
		}
	}
	s.Wait()

	list, err := s.List(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("List() error: %s", err)
	}
	if len(list) != 3 {
		t.Errorf("len = %d, want %d", len(list), 3)
	}
}

func TestNotificationService_Export(t *testing.T) {
	mock := &sender.MockSender{}
	s := newTestService(t, map[string]sender.Sender{"email": mock})

	for i := range 2 {
		_, err := s.Create(context.Background(), notification.Notification{
			Recipient: fmt.Sprintf("recipient #%d", i),
			Channel:   "email",
		}, "req-1")
		if err != nil {
			t.Fatalf("Create() error: %s", err)
		}
	}
	s.Wait()

	count, err := s.Export(context.Background())
	if err != nil {
		t.Fatalf("Export() error: %s", err)
	}
	if count != 2 {
		t.Errorf("exported = %d, want %d", count, 2)
	}
}
