package main

import (
	"fmt"
	"notifier/internal/notification"
	"os"
)

func WriteAuditLog(path string, notifications []notification.Notification) error {
	const op = "WriteAuditLog"

	f, err := os.Create(path)
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
