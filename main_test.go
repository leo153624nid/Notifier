package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

type MockSender struct {
	Calls []Notification
}

func (m *MockSender) Send(n Notification) error {
	m.Calls = append(m.Calls, n)
	return nil
}

func TestConsoleSender(t *testing.T) {
	var buf bytes.Buffer
	sender := NewConsoleSender(&buf)
	n := Notification{
		ID:        1,
		Recipient: "test@example.com",
		Subject:   "Test Notification",
		Body:      "This is a test notification.",
		Channel:   "console",
		IsUrgent:  false,
	}

	err := sender.Send(n)
	if err != nil {
		t.Fatalf("Send() error: %s", err)
	}

	got := buf.String()
	want := "[console] to test@example.com | Test Notification\n"
	if got != want {
		t.Errorf("Send() = %q, want %q", got, want)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		n       Notification
		wantErr bool
	}{
		{
			name:    "valid notification",
			n:       Notification{Recipient: "some", Channel: "email"},
			wantErr: false,
		},
		{
			name:    "empty recipient",
			n:       Notification{Recipient: "", Channel: "email"},
			wantErr: true,
		},
		{
			name:    "empty channel",
			n:       Notification{Recipient: "some", Channel: ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.n.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Validate() error: %s", err)
			}
		})
	}
}

func TestHealthHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	senders := map[string]Sender{
		"console": &MockSender{},
	}

	s, err := NewServer(logger, senders)
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
			wantStatus: 201,
			wantCalls:  1,
		},
		{
			name:       "empty recipient",
			body:       `{"to":"","subject":"some","channel":"email"}`,
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
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			mock := &MockSender{}
			senders := map[string]Sender{
				"email": mock,
			}
			s, err := NewServer(
				logger,
				senders,
			)
			if err != nil {
				t.Fatalf("NewSever() error: %s", err)
			}

			r := httptest.NewRequest("POST", "/api/notifications", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			s.createNotification(w, r)

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
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			mock := &MockSender{}
			senders := map[string]Sender{
				"email": mock,
			}
			s, err := NewServer(
				logger,
				senders,
			)
			if err != nil {
				t.Fatalf("NewSever() error: %s", err)
			}
			s.nextID++
			s.notifications[s.nextID] = Notification{
				ID:        s.nextID,
				Recipient: "needed recipient",
				Subject:   "Test Notification",
				Body:      "This is a test notification.",
				Channel:   "email",
				IsUrgent:  false,
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
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			mock := &MockSender{}
			senders := map[string]Sender{
				"email": mock,
			}
			s, err := NewServer(
				logger,
				senders,
			)
			if err != nil {
				t.Fatalf("NewSever() error: %s", err)
			}

			for _, v := range tt.ids {
				s.notifications[v] = Notification{
					ID:        v,
					Recipient: fmt.Sprintf("recipient #%d", v),
				}
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
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mock := &MockSender{}
	senders := map[string]Sender{
		"email": mock,
	}
	s, err := NewServer(
		logger,
		senders,
	)
	if err != nil {
		t.Fatalf("NewSever() error: %s", err)
	}

	_, err = s.findNotification(9999)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error in not ErrNotFound, got: %s", err)
	}
}
