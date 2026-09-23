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

// newTestConsumerWithSender — как newTestConsumer, но возвращает и сам
// *sender.MockSender, чтобы тесты идемпотентности могли проверить,
// сколько раз реально была вызвана отправка письма.
func newTestConsumerWithSender(t *testing.T) (*LoginConsumer, *repository.MemoryRepository, *sender.MockSender) {
	t.Helper()

	repo := repository.NewMemoryRepository()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	auditLogger := audit.NewLogger(t.TempDir() + "/audit.log")
	mockSnd := &sender.MockSender{}
	senders := map[string]sender.Sender{"email": mockSnd}

	notifSvc, err := service.NewNotificationService(repo, senders, auditLogger, logger)
	if err != nil {
		t.Fatalf("NewNotificationService() error: %s", err)
	}

	return &LoginConsumer{service: notifSvc, logger: logger}, repo, mockSnd
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

// TestLoginConsumer_Idempotency проверяет защиту от повторной обработки
// одного и того же события Kafka (inbox pattern через processed_events /
// MemoryRepository.events): повторная доставка того же event_id не должна
// создавать второе уведомление и не должна повторно отправлять письмо.
func TestLoginConsumer_Idempotency(t *testing.T) {
	t.Run("duplicate event is skipped without error", func(t *testing.T) {
		c, repo, mockSnd := newTestConsumerWithSender(t)

		event := authevents.UserLoggedIn{
			EventID:    uuid.New(),
			UserID:     uuid.New(),
			Email:      "user@mail.com",
			OccurredAt: time.Now(),
		}
		payload := marshalEvent(t, event)

		if err := c.handleMessage(context.Background(), payload); err != nil {
			t.Fatalf("first handleMessage() error: %s", err)
		}
		// Повторная доставка того же сообщения — например, после ретрая
		// producer'а или повторной обработки consumer'ом до коммита offset.
		if err := c.handleMessage(context.Background(), payload); err != nil {
			t.Fatalf("duplicate handleMessage() error = %v, want nil (duplicate must not be treated as failure)", err)
		}

		c.service.Wait()

		list, err := repo.GetList(context.Background(), 1, 10)
		if err != nil {
			t.Fatalf("GetList() error: %s", err)
		}
		if len(list) != 1 {
			t.Fatalf("notifications count = %d, want 1 (duplicate must not create a second notification)", len(list))
		}

		// c.service.Wait() выше дожидается завершения фоновой горутины
		// отправки (sync.WaitGroup даёт здесь happens-before), поэтому
		// Calls можно читать без блокировки mockSnd напрямую.
		if calls := len(mockSnd.Calls); calls != 1 {
			t.Fatalf("sender Send() calls = %d, want 1 (duplicate must not resend the email)", calls)
		}
	})

	t.Run("different events with distinct event_id both create notifications", func(t *testing.T) {
		c, repo, mockSnd := newTestConsumerWithSender(t)

		first := authevents.UserLoggedIn{EventID: uuid.New(), UserID: uuid.New(), Email: "a@mail.com", OccurredAt: time.Now()}
		second := authevents.UserLoggedIn{EventID: uuid.New(), UserID: uuid.New(), Email: "b@mail.com", OccurredAt: time.Now()}

		if err := c.handleMessage(context.Background(), marshalEvent(t, first)); err != nil {
			t.Fatalf("handleMessage(first) error: %s", err)
		}
		if err := c.handleMessage(context.Background(), marshalEvent(t, second)); err != nil {
			t.Fatalf("handleMessage(second) error: %s", err)
		}

		c.service.Wait()

		list, err := repo.GetList(context.Background(), 1, 10)
		if err != nil {
			t.Fatalf("GetList() error: %s", err)
		}
		if len(list) != 2 {
			t.Fatalf("notifications count = %d, want 2 (distinct event_id must not be deduplicated)", len(list))
		}

		if calls := len(mockSnd.Calls); calls != 2 {
			t.Fatalf("sender Send() calls = %d, want 2", calls)
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
