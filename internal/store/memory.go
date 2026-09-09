package store

import (
	"fmt"
	"sync"

	"notifier/internal/notification"
)

type MemoryStore struct {
	notifications map[int]notification.Notification
	nextID        int
	mu            sync.Mutex
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		notifications: make(map[int]notification.Notification),
		nextID:        0,
	}
}

func (s *MemoryStore) Save(n notification.Notification) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	n.ID = s.nextID
	n.Status = "pending"
	s.notifications[n.ID] = n

	return n.ID, nil
}

func (s *MemoryStore) GetAll() ([]notification.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]notification.Notification, 0, len(s.notifications))

	for _, n := range s.notifications {
		result = append(result, n)
	}

	return result, nil
}

func (s *MemoryStore) GetById(id int) (notification.Notification, error) {
	const op = "MemoryStore.GetById"

	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.notifications[id]
	if !ok {
		err := fmt.Errorf("%s: %w", op, notification.ErrNotFound)
		return notification.Notification{}, err
	}

	return n, nil
}

func (s *MemoryStore) UpdateStatus(id int, status string) error {
	const op = "MemoryStore.UpdateStatus"

	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.notifications[id]
	if !ok {
		return fmt.Errorf("%s: %w", op, notification.ErrNotFound)
	}

	n.Status = status
	s.notifications[id] = n

	return nil
}
