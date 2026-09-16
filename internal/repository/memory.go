package repository

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"

	"notifier/internal/notification"
)

type MemoryRepository struct {
	notifications map[int]notification.Notification
	nextID        int
	mu            sync.Mutex
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		notifications: make(map[int]notification.Notification),
		nextID:        0,
	}
}

func (r *MemoryRepository) Save(_ context.Context, n notification.Notification) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	n.ID = r.nextID
	n.Status = "pending"
	r.notifications[n.ID] = n

	return n.ID, nil
}

func (r *MemoryRepository) GetAll(_ context.Context) ([]notification.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]notification.Notification, 0, len(r.notifications))

	for _, n := range r.notifications {
		result = append(result, n)
	}

	return result, nil
}

func (r *MemoryRepository) GetList(ctx context.Context, page int, size int) ([]notification.Notification, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	if size > 100 {
		size = 100
	}

	all, err := r.GetAll(ctx)
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

func (r *MemoryRepository) GetById(_ context.Context, id int) (notification.Notification, error) {
	const op = "MemoryRepository.GetById"

	r.mu.Lock()
	defer r.mu.Unlock()

	n, ok := r.notifications[id]
	if !ok {
		err := fmt.Errorf("%s: %w", op, notification.ErrNotFound)
		return notification.Notification{}, err
	}

	return n, nil
}

func (r *MemoryRepository) UpdateStatus(_ context.Context, id int, status string) error {
	const op = "MemoryRepository.UpdateStatus"

	r.mu.Lock()
	defer r.mu.Unlock()

	n, ok := r.notifications[id]
	if !ok {
		return fmt.Errorf("%s: %w", op, notification.ErrNotFound)
	}

	n.Status = status
	r.notifications[id] = n

	return nil
}
