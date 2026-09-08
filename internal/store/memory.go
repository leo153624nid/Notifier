package store

import (
	"fmt"

	"notifier/internal/notification"
)

type MemoryStore struct {
	notifications map[int]Notification
	nextID        int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		notifications: make(map[int]Notification),
		nextID:        0,
	}
}

func (s *MemoryStore) Save(n Notification) (int, error) {
	s.nextID++
	n.ID = s.nextID
	n.Status = "pending"
	s.notifications[n.ID] = n

	return n.ID, nil
}

func (s *MemoryStore) GetAll() ([]Notification, error) {
	result := make([]Notification, 0, len(s.notifications))

	for _, n := range s.notifications {
		result = append(result, n)
	}

	return result, nil
}

func (s *MemoryStore) GetById(id int) (Notification, error) {
	const op = "MemoryStore.GetById"

	n, ok := s.notifications[id]
	if !ok {
		err := fmt.Errorf("%s: %w", op, notification.ErrNotFound)
		return Notification{}, err
	}

	return n, nil
}

func (s *MemoryStore) UpdateStatus(id int, status string) error {
	const op = "MemoryStore.UpdateStatus"

	n, ok := s.notifications[id]
	if !ok {
		return fmt.Errorf("%s: %w", op, notification.ErrNotFound)
	}

	n.Status = status
	s.notifications[id] = n
	return nil
}
