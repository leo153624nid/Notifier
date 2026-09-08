package sender

import (
	"bytes"
	"testing"

	"notifier/internal/notification"
)

func TestConsoleSender(t *testing.T) {
	var buf bytes.Buffer
	sender := NewConsoleSender(&buf)
	n := notification.Notification{
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
