package main

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"notifier/internal/config"
	"notifier/internal/notification"
	"notifier/internal/sender"
	"notifier/internal/store"
)

func TestHealthHandler(t *testing.T) {
	_ = os.Setenv("API_KEY", "secret")
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() error: %s", err)
	}
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		t.Fatalf("DB error: %s", err)
	}

	store := store.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	senders := map[string]sender.Sender{
		"console": &sender.MockSender{},
	}
	auditLogger := NewAuditLogger(t.TempDir() + "/audit.log")

	s, err := NewServer(store, db, logger, senders, auditLogger)
	if err != nil {
		t.Fatalf("NewServer() error: %s", err)
	}

	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	s.healthHandler(w, r)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "available") {
		t.Errorf("body = %q, want contains %q", w.Body.String(), "available")
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
			wantStatus: 202,
			wantCalls:  1,
		},
		{
			name:       "empty recipient",
			body:       `{"to":"","subject":"some","channel":"email"}`,
			wantStatus: 400,
			wantCalls:  0,
		},
		{
			name:       "empty channel",
			body:       `{"to":"user@example.com","subject":"some","channel":""}`,
			wantStatus: 400,
			wantCalls:  0,
		},
		{
			name:       "unknown channel",
			body:       `{"to":"user@example.com","subject":"some","channel":"sms"}`,
			wantStatus: 400,
			wantCalls:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := store.NewMemoryStore()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			mock := &sender.MockSender{}
			senders := map[string]sender.Sender{
				"email": mock,
			}
			auditLogger := NewAuditLogger(t.TempDir() + "/audit.log")

			s, err := NewServer(store, nil, logger, senders, auditLogger)
			if err != nil {
				t.Fatalf("NewSever() error: %s", err)
			}

			r := httptest.NewRequest("POST", "/api/notifications", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			s.createNotification(w, r)
			time.Sleep(300 * time.Millisecond)

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
			wantStatus: 200,
			wantErr:    false,
		},
		{
			name:       "no such id",
			id:         "22",
			wantStatus: 404,
			wantErr:    true,
		},
		{
			name:       "wrong id",
			id:         "someId",
			wantStatus: 400,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := store.NewMemoryStore()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			mock := &sender.MockSender{}
			senders := map[string]sender.Sender{
				"email": mock,
			}
			auditLogger := NewAuditLogger(t.TempDir() + "/audit.log")

			s, err := NewServer(store, nil, logger, senders, auditLogger)
			if err != nil {
				t.Fatalf("NewSever() error: %s", err)
			}

			n := notification.Notification{
				ID:        0,
				Recipient: "needed recipient",
				Subject:   "Test Notification",
				Body:      "This is a test notification.",
				Channel:   "email",
				IsUrgent:  false,
			}
			_, err = s.store.Save(n)
			if err != nil {
				t.Fatalf("save notification error: %s", err)
			}

			r := httptest.NewRequest("GET", "/api/notifications/{id}", nil)
			r.SetPathValue("id", tt.id)
			w := httptest.NewRecorder()

			s.getNotification(w, r)

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

func TestListNotifications(t *testing.T) {
	tests := []struct {
		name       string
		ids        []int
		wantStatus int
		wantErr    bool
	}{
		{
			name:       "valid request",
			ids:        []int{1, 2},
			wantStatus: 200,
			wantErr:    false,
		},
		{
			name:       "empty array",
			ids:        []int{},
			wantStatus: 200,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := store.NewMemoryStore()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			mock := &sender.MockSender{}
			senders := map[string]sender.Sender{
				"email": mock,
			}
			auditLogger := NewAuditLogger(t.TempDir() + "/audit.log")

			s, err := NewServer(store, nil, logger, senders, auditLogger)
			if err != nil {
				t.Fatalf("NewSever() error: %s", err)
			}

			for _, v := range tt.ids {
				n := notification.Notification{
					ID:        v,
					Recipient: fmt.Sprintf("recipient #%d", v),
					Channel:   "email",
				}
				_, _ = s.store.Save(n)
			}

			r := httptest.NewRequest("GET", "/api/notifications", nil)
			w := httptest.NewRecorder()

			s.listNotifications(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			for _, v := range tt.ids {
				if !tt.wantErr && !strings.Contains(w.Body.String(), fmt.Sprintf("recipient #%d", v)) {
					t.Errorf("body = %q, want contains %q", w.Body.String(), fmt.Sprintf("recipient #%d", v))
				}
			}
			if tt.wantErr && !strings.Contains(w.Body.String(), "error") {
				t.Errorf("body = %q, want contains %q", w.Body.String(), "error")
			}
		})
	}
}

func TestErrNotFoundThroughChain(t *testing.T) {
	store := store.NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mock := &sender.MockSender{}
	senders := map[string]sender.Sender{
		"email": mock,
	}
	auditLogger := NewAuditLogger(t.TempDir() + "/audit.log")

	s, err := NewServer(store, nil, logger, senders, auditLogger)
	if err != nil {
		t.Fatalf("NewSever() error: %s", err)
	}

	_, err = s.store.GetById(9999)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, notification.ErrNotFound) {
		t.Errorf("error in not ErrNotFound, got: %s", err)
	}
}
