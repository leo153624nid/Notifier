package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"notifier/internal/audit"
	"notifier/internal/domain"
	"notifier/internal/repository"
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

func newTestHandler(
	t *testing.T,
	senders map[string]sender.Sender,
	pingerDB service.Pinger,
	pingerCache service.Pinger,
) (*Handler, *repository.MemoryRepository) {
	t.Helper()

	repo := repository.NewMemoryRepository()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	auditLogger := audit.NewLogger(t.TempDir() + "/audit.log")

	notificationService, err := service.NewNotificationService(repo, senders, auditLogger, logger)
	if err != nil {
		t.Fatalf("NewNotificationService() error: %s", err)
	}
	healthService := service.NewHealthService(pingerDB, pingerCache)

	return NewHandler(notificationService, healthService, logger, "Notifier", "test"), repo
}

func TestHealthHandler(t *testing.T) {
	h, _ := newTestHandler(
		t,
		map[string]sender.Sender{"console": &sender.MockSender{}},
		fakeDbPinger{},
		fakeCachePinger{},
	)

	r := httptest.NewRequest("GET", "/health", nil)
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

	r := httptest.NewRequest("GET", "/health", nil)
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

	r := httptest.NewRequest("GET", "/health", nil)
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

			r := httptest.NewRequest("POST", "/api/v1/notifications", strings.NewReader(tt.body))
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

			_, err := repo.Save(context.Background(), notificationFixture())
			if err != nil {
				t.Fatalf("save notification error: %s", err)
			}

			r := httptest.NewRequest("GET", "/api/v1/notifications/{id}", nil)
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

			_, err := repo.Save(context.Background(), notificationFixture())
			if err != nil {
				t.Fatalf("save notification error: %s", err)
			}

			r := httptest.NewRequest("DELETE", "/api/v1/notifications/{id}", nil)
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
				_, _ = repo.Save(context.Background(), n)
			}

			r := httptest.NewRequest("GET", "/api/v1/notifications", nil)
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
