package audit

import (
	"fmt"
	"os"
	"sync"

	"notifier/internal/notification"
)

type Logger struct {
	path string
	mu   sync.Mutex
}

func NewLogger(path string) *Logger {
	return &Logger{path: path}
}

func (l *Logger) Write(notifications []notification.Notification) error {
	const op = "audit.Logger.Write"

	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Create(l.path)
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
