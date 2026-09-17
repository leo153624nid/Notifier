package repository

import (
	"context"
	"fmt"
	"sync"
	"uuid"

	"authservice/internal/domain"
)

type MemoryRepository struct {
	users map[string]domain.User
	mu    sync.Mutex
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		users: make(map[string]domain.User),
	}
}

// MARK: - `UserRepo` interface implementation
func (r *MemoryRepository) Create(_ context.Context, u domain.User) (uuid.UUID, error) {
	const op = "MemoryRepository.Create"

	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.users[u.Email]
	if ok {
		return uuid.Nil(), fmt.Errorf("%s: %w", op, domain.ErrUserExists)
	}

	id := uuid.New()
	u.ID = id
	r.users[u.Email] = u

	return u.ID, nil
}

func (r *MemoryRepository) GetByEmail(_ context.Context, email string) (domain.User, error) {
	const op = "MemoryRepository.GetByEmail"

	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.users[email]
	if !ok {
		err := fmt.Errorf("%s: %w", op, domain.ErrInvalidCredentials)
		return domain.User{}, err
	}

	return u, nil
}
