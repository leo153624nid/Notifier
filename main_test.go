package main

import (
	"bytes"
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
