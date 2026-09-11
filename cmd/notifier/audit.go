package main

import (
	"fmt"
	"os"
	"sync"

	"notifier/internal/notification"
)

type AuditLogger struct {
	path string
	mu   sync.Mutex
}

func NewAuditLogger(path string) *AuditLogger {
	return &AuditLogger{path: path}
}

func (a *AuditLogger) Write(notifications []notification.Notification) error {
	const op = "AuditLogger.Write"

	a.mu.Lock()
	defer a.mu.Unlock()

	f, err := os.Create(a.path)
	if err != nil {
		return fmt.Errorf("%s: create: %w", op, err)
	}

	defer func() {
		_ = f.Close()
	}()

	for _, n := range notifications {
		_, err := fmt.Fprintf(f, "id:%d recipient:%s subject:%s\n", n.ID, n.Recipient, n.Subject)
		if err != nil {
			return fmt.Errorf("%s: write: %w", op, err)
		}
	}

	return nil
}
