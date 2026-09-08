package store

import (
	"notifier/internal/notification"
)

type Store interface {
	Save(n notification.Notification) (int, error)
	GetAll() ([]notification.Notification, error)
	GetById(id int) (notification.Notification, error)
	UpdateStatus(id int, status string) error
}
