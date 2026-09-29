package memory_repo

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"
	"uuid"

	"notifier/internal/core/domain"
	core_errors "notifier/internal/core/errors"
)

type MemoryRepository struct {
	notifications map[int]domain.Notification
	events        map[string]struct{}
	nextID        int
	mu            sync.Mutex
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		notifications: make(map[int]domain.Notification),
		events:        make(map[string]struct{}),
		nextID:        0,
	}
}

// MARK: - `NotificationRepo` interface implementation

func (r *MemoryRepository) CreateIdempotent(
	ctx context.Context,
	consumer string,
	eventID uuid.UUID,
	n domain.Notification,
) (domain.Notification, error) {
	const op = "MemoryRepository.CreateIdempotent"

	r.mu.Lock()
	defer r.mu.Unlock()

	eventKey := consumer + ":" + eventID.String()
	if _, ok := r.events[eventKey]; ok {
		return domain.Notification{}, fmt.Errorf("%s: %w", op, core_errors.ErrEventAlreadyProcessed)
	}
	r.events[eventKey] = struct{}{}

	r.nextID++
	n.ID = r.nextID
	n.Status = "pending"
	r.notifications[n.ID] = n

	return n, nil
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

func (r *MemoryRepository) GetList(
	ctx context.Context,
	page *int,
	size *int,
) ([]domain.Notification, error) {
	var pg int
	var sz int
	if page != nil && size != nil {
		pg = *page
		sz = *size
	} else {
		return nil, core_errors.ErrInvalidArgument
	}
	if pg <= 0 {
		pg = 1
	}
	if sz <= 0 {
		sz = 10
	}
	if sz > 100 {
		sz = 100
	}

	all, err := r.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	firstIndex := (pg - 1) * sz
	if firstIndex > len(all)-1 {
		return []domain.Notification{}, nil
	}

	slices.SortFunc(all, func(a, b domain.Notification) int {
		return cmp.Compare(a.ID, b.ID)
	})
	lastIndex := min(firstIndex+(sz-1), len(all)-1)
	all = all[firstIndex : lastIndex+1]

	result := make([]domain.Notification, len(all), sz)
	copy(result, all)

	return result, nil
}

func (r *MemoryRepository) GetById(_ context.Context, id int) (domain.Notification, error) {
	const op = "MemoryRepository.GetById"

	r.mu.Lock()
	defer r.mu.Unlock()

	n, ok := r.notifications[id]
	if !ok {
		err := fmt.Errorf("%s: %w", op, core_errors.ErrNotFound)
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
		return fmt.Errorf("%s: %w", op, core_errors.ErrNotFound)
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
		return fmt.Errorf("%s: %w", op, core_errors.ErrNotFound)
	}

	n.Status = status
	r.notifications[id] = n

	return nil
}
