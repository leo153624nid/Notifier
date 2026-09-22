package repository

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"

	"notifier/internal/domain"
)

type MemoryRepository struct {
	notifications map[int]domain.Notification
	nextID        int
	mu            sync.Mutex
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		notifications: make(map[int]domain.Notification),
		nextID:        0,
	}
}

// MARK: - `NotificationRepo` interface implementation
func (r *MemoryRepository) Save(_ context.Context, n domain.Notification) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	n.ID = r.nextID
	n.Status = "pending"
	r.notifications[n.ID] = n

	return n.ID, nil
}

func (r *MemoryRepository) GetAll(_ context.Context) ([]domain.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]domain.Notification, 0, len(r.notifications))

	for _, n := range r.notifications {
		result = append(result, n)
	}

	return result, nil
}

func (r *MemoryRepository) GetList(ctx context.Context, page int, size int) ([]domain.Notification, error) {
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
		return []domain.Notification{}, nil
	}

	slices.SortFunc(all, func(a, b domain.Notification) int {
		return cmp.Compare(a.ID, b.ID)
	})
	lastIndex := min(firstIndex+(size-1), len(all)-1)
	all = all[firstIndex : lastIndex+1]

	result := make([]domain.Notification, len(all), size)
	copy(result, all)

	return result, nil
}

func (r *MemoryRepository) GetById(_ context.Context, id int) (domain.Notification, error) {
	const op = "MemoryRepository.GetById"

	r.mu.Lock()
	defer r.mu.Unlock()

	n, ok := r.notifications[id]
	if !ok {
		err := fmt.Errorf("%s: %w", op, domain.ErrNotFound)
		return domain.Notification{}, err
	}

	return n, nil
}

func (r *MemoryRepository) DeleteById(_ context.Context, id int) error {
	const op = "MemoryRepository.DeleteById"

	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.notifications[id]
	if !ok {
		return fmt.Errorf("%s: %w", op, domain.ErrNotFound)
	}

	delete(r.notifications, id)

	return nil
}

func (r *MemoryRepository) UpdateStatus(_ context.Context, id int, status string) error {
	const op = "MemoryRepository.UpdateStatus"

	r.mu.Lock()
	defer r.mu.Unlock()

	n, ok := r.notifications[id]
	if !ok {
		return fmt.Errorf("%s: %w", op, domain.ErrNotFound)
	}

	n.Status = status
	r.notifications[id] = n

	return nil
}
