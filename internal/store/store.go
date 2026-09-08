package store

import (
	"notifier/internal/notification"
)

type Notification = notification.Notification

type Store interface {
	Save(n Notification) (int, error)
	GetAll() ([]Notification, error)
	GetById(id int) (Notification, error)
	UpdateStatus(id int, status string) error
}
