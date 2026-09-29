package transport_http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"uuid"

	"notifier/internal/audit"
	"notifier/internal/core/domain"
	core_logger "notifier/internal/core/logger"
	memory_repo "notifier/internal/repository/memory"
	"notifier/internal/sender"
	"notifier/internal/service"
)

type fakeDbPinger struct {
	err error
}

type fakeCachePinger struct {
	err error
}

func (f fakeDbPinger) Ping(context.Context) error {
	return f.err
}

func (f fakeCachePinger) Ping(context.Context) error {
	return f.err
}

// testContext возвращает контекст с no-op логгером: хендлеры и сервис
// достают логгер через core_logger.FromContext, который без него паникует.
func testContext() context.Context {
	return core_logger.ToContext(context.Background(), core_logger.NewNop())
}

func newRequest(method, target string, body io.Reader) *http.Request {
	return httptest.NewRequest(method, target, body).WithContext(testContext())
}

func newTestHandler(
	t *testing.T,
	senders map[string]sender.Sender,
	pingerDB service.Pinger,
	pingerCache service.Pinger,
) (*NotificationsHTTPHandler, *memory_repo.MemoryRepository) {
	t.Helper()

	repo := memory_repo.NewMemoryRepository()
	auditLogger := audit.NewLogger(t.TempDir() + "/audit.log")

	notificationService, err := service.NewNotificationService(
		repo,
		senders,
		auditLogger,
	)
	if err != nil {
		t.Fatalf("NewNotificationService() error: %s", err)
	}
	healthService := service.NewHealthService(pingerDB, pingerCache)

	return NewNotificationsHTTPHandler(
		notificationService,
		healthService,
		"Notifier",
		"test",
	), repo
}

func TestHealthHandler(t *testing.T) {
	h, _ := newTestHandler(
		t,
		map[string]sender.Sender{"console": &sender.MockSender{}},
		fakeDbPinger{},
		fakeCachePinger{},
	)

	r := newRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.healthHandler(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "available") {
		t.Errorf("body = %q, want contains %q", w.Body.String(), "available")
	}
}

func TestHealthHandler_DBUnavailable(t *testing.T) {
	h, _ := newTestHandler(
		t,
		map[string]sender.Sender{"console": &sender.MockSender{}},
		fakeDbPinger{err: errors.New("connection refused")},
		fakeCachePinger{},
	)

	r := newRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.healthHandler(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHealthHandler_CacheUnavailable(t *testing.T) {
	h, _ := newTestHandler(
		t,
		map[string]sender.Sender{"console": &sender.MockSender{}},
		fakeDbPinger{},
		fakeCachePinger{err: errors.New("connection refused")},
	)

	r := newRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.healthHandler(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestCreateNotification(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCalls  int
	}{
		{
			name:       "valid request",
			body:       `{"to":"user@example.com","subject":"some","channel":"email"}`,
			wantStatus: http.StatusAccepted,
			wantCalls:  1,
		},
		{
			name:       "empty recipient",
			body:       `{"to":"","subject":"some","channel":"email"}`,
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		{
			name:       "empty channel",
			body:       `{"to":"user@example.com","subject":"some","channel":""}`,
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		{
			name:       "unknown channel",
			body:       `{"to":"user@example.com","subject":"some","channel":"sms"}`,
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &sender.MockSender{}
			h, _ := newTestHandler(
				t,
				map[string]sender.Sender{"email": mock},
				fakeDbPinger{},
				fakeCachePinger{},
			)

			r := newRequest(http.MethodPost, "/api/v1/notifications", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			h.createNotification(w, r)
			time.Sleep(300 * time.Millisecond)
			h.notifications.Wait()

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if len(mock.Calls) != tt.wantCalls {
				t.Errorf("calls = %d, want %d", len(mock.Calls), tt.wantCalls)
			}
		})
	}
}

func TestGetNotification(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		wantStatus int
		wantErr    bool
	}{
		{
			name:       "valid request",
			id:         "1",
			wantStatus: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "no such id",
			id:         "22",
			wantStatus: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "wrong id",
			id:         "someId",
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &sender.MockSender{}
			h, repo := newTestHandler(
				t,
				map[string]sender.Sender{"email": mock},
				fakeDbPinger{},
				fakeCachePinger{},
			)

			_, err := repo.CreateIdempotent(testContext(), "test", uuid.New(), notificationFixture())
			if err != nil {
				t.Fatalf("save notification error: %s", err)
			}

			r := newRequest(http.MethodGet, "/api/v1/notifications/{id}", nil)
			r.SetPathValue("id", tt.id)
			w := httptest.NewRecorder()

			h.getNotification(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if !tt.wantErr && !strings.Contains(w.Body.String(), "needed recipient") {
				t.Errorf("body = %q, want contains %q", w.Body.String(), "needed recipient")
			}
			if tt.wantErr && !strings.Contains(w.Body.String(), "error") {
				t.Errorf("body = %q, want contains %q", w.Body.String(), "error")
			}
		})
	}
}

func TestDeleteNotification(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		wantStatus int
		wantErr    bool
	}{
		{
			name:       "valid request",
			id:         "1",
			wantStatus: http.StatusNoContent,
			wantErr:    false,
		},
		{
			name:       "no such id",
			id:         "22",
			wantStatus: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "wrong id",
			id:         "someId",
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &sender.MockSender{}
			h, repo := newTestHandler(
				t,
				map[string]sender.Sender{"email": mock},
				fakeDbPinger{},
				fakeCachePinger{},
			)

			_, err := repo.CreateIdempotent(testContext(), "test", uuid.New(), notificationFixture())
			if err != nil {
				t.Fatalf("save notification error: %s", err)
			}

			r := newRequest(http.MethodDelete, "/api/v1/notifications/{id}", nil)
			r.SetPathValue("id", tt.id)
			w := httptest.NewRecorder()

			h.deleteNotification(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantErr && !strings.Contains(w.Body.String(), "error") {
				t.Errorf("body = %q, want contains %q", w.Body.String(), "error")
			}
		})
	}
}

func TestListNotifications(t *testing.T) {
	tests := []struct {
		name       string
		ids        []int
		wantStatus int
	}{
		{
			name:       "valid request",
			ids:        []int{1, 2},
			wantStatus: http.StatusOK,
		},
		{
			name:       "empty array",
			ids:        []int{},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &sender.MockSender{}
			h, repo := newTestHandler(
				t,
				map[string]sender.Sender{"email": mock},
				fakeDbPinger{},
				fakeCachePinger{},
			)

			for _, v := range tt.ids {
				n := notificationFixture()
				n.Recipient = fmt.Sprintf("recipient #%d", v)
				_, _ = repo.CreateIdempotent(testContext(), "test", uuid.New(), n)
			}

			r := newRequest(http.MethodGet, "/api/v1/notifications", nil)
			w := httptest.NewRecorder()

			h.listNotifications(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			for _, v := range tt.ids {
				if !strings.Contains(w.Body.String(), fmt.Sprintf("recipient #%d", v)) {
					t.Errorf("body = %q, want contains %q", w.Body.String(), fmt.Sprintf("recipient #%d", v))
				}
			}
		})
	}
}

func notificationFixture() domain.Notification {
	return domain.Notification{
		Recipient: "needed recipient",
		Subject:   "Test Notification",
		Body:      "This is a test notification.",
		Channel:   "email",
	}
}
