package main

import (
	"fmt"
	"os"
)

func WriteAuditLog(path string, notifications map[int]Notification) error {
	const op = "WriteAuditLog"

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("%s: create: %w", op, err)
	}

	defer f.Close()

	for _, n := range notifications {
		_, err := fmt.Fprintf(f, "id:%d recipient:%s subject:%s\n", n.ID, n.Recipient, n.Subject)
		if err != nil {
			return fmt.Errorf("%s: write: %w", op, err)
		}
	}

	return nil
}
