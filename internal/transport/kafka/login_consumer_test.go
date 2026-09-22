package kafka

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	authevents "contracts/events/auth/v1"
	"notifier/internal/audit"
	"notifier/internal/repository"
	"notifier/internal/sender"
	"notifier/internal/service"

	"uuid"
)

// newTestConsumer собирает LoginConsumer вокруг настоящего
// NotificationService (in-memory репозиторий + mock sender), а не мока —
// так handleMessage проверяется как единое целое с бизнес-логикой создания
// уведомления, а не только сам по себе разбор JSON. reader оставляем nil:
// тесты вызывают handleMessage напрямую и никогда не обращаются к брокеру.
func newTestConsumer(t *testing.T) (*LoginConsumer, *repository.MemoryRepository) {
	t.Helper()

	repo := repository.NewMemoryRepository()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	auditLogger := audit.NewLogger(t.TempDir() + "/audit.log")
	senders := map[string]sender.Sender{"email": &sender.MockSender{}}

	notifSvc, err := service.NewNotificationService(repo, senders, auditLogger, logger)
	if err != nil {
		t.Fatalf("NewNotificationService() error: %s", err)
	}

	return &LoginConsumer{service: notifSvc, logger: logger}, repo
}

func TestLoginConsumer_HandleMessage(t *testing.T) {
	t.Run("valid event creates notification", func(t *testing.T) {
		c, repo := newTestConsumer(t)

		event := authevents.UserLoggedIn{
			UserID:     uuid.New(),
			Email:      "user@mail.com",
			OccurredAt: time.Now(),
		}
		payload := marshalEvent(t, event)

		if err := c.handleMessage(context.Background(), payload); err != nil {
			t.Fatalf("handleMessage() error: %s", err)
		}

		c.service.Wait()

		list, err := repo.GetList(context.Background(), 1, 10)
		if err != nil {
			t.Fatalf("GetList() error: %s", err)
		}
		if len(list) != 1 {
			t.Fatalf("notifications count = %d, want 1", len(list))
		}
		if list[0].Recipient != event.Email {
			t.Errorf("Recipient = %q, want %q", list[0].Recipient, event.Email)
		}
		if list[0].Channel != "email" {
			t.Errorf("Channel = %q, want %q", list[0].Channel, "email")
		}
	})

	t.Run("invalid JSON returns error and creates nothing", func(t *testing.T) {
		c, repo := newTestConsumer(t)

		err := c.handleMessage(context.Background(), []byte("not json"))
		if err == nil {
			t.Fatal("handleMessage() error = nil, want error for invalid JSON")
		}

		list, err := repo.GetList(context.Background(), 1, 10)
		if err != nil {
			t.Fatalf("GetList() error: %s", err)
		}
		if len(list) != 0 {
			t.Errorf("notifications count = %d, want 0", len(list))
		}
	})

	t.Run("empty email fails validation and returns error", func(t *testing.T) {
		c, repo := newTestConsumer(t)

		event := authevents.UserLoggedIn{UserID: uuid.New(), Email: "", OccurredAt: time.Now()}
		payload := marshalEvent(t, event)

		if err := c.handleMessage(context.Background(), payload); err == nil {
			t.Fatal("handleMessage() error = nil, want error for empty recipient")
		}

		list, err := repo.GetList(context.Background(), 1, 10)
		if err != nil {
			t.Fatalf("GetList() error: %s", err)
		}
		if len(list) != 0 {
			t.Errorf("notifications count = %d, want 0", len(list))
		}
	})
}

func marshalEvent(t *testing.T, event authevents.UserLoggedIn) []byte {
	t.Helper()

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %s", err)
	}

	return payload
}
