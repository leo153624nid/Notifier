package store

import (
	"cmp"
	"context"
	"fmt"
	"slices"
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

func (s *MemoryStore) Save(_ context.Context, n notification.Notification) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextID++
	n.ID = s.nextID
	n.Status = "pending"
	s.notifications[n.ID] = n

	return n.ID, nil
}

func (s *MemoryStore) GetAll(_ context.Context) ([]notification.Notification, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]notification.Notification, 0, len(s.notifications))

	for _, n := range s.notifications {
		result = append(result, n)
	}

	return result, nil
}

func (s *MemoryStore) GetList(ctx context.Context, page int, size int) ([]notification.Notification, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	all, err := s.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	firstIndex := (page - 1) * size
	if firstIndex > len(all)-1 {
		return []notification.Notification{}, nil
	}

	slices.SortFunc(all, func(a, b notification.Notification) int {
		return cmp.Compare(a.ID, b.ID)
	})
	lastIndex := min(firstIndex+(size-1), len(all)-1)
	all = all[firstIndex : lastIndex+1]

	result := make([]notification.Notification, len(all), size)
	copy(result, all)

	return result, nil
}

func (s *MemoryStore) GetById(_ context.Context, id int) (notification.Notification, error) {
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

func (s *MemoryStore) UpdateStatus(_ context.Context, id int, status string) error {
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
