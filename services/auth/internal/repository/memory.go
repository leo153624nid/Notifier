package repository

import (
	"context"
	"fmt"
	"sync"
	"time"
	"uuid"

	"authservice/internal/domain"
)

type MemoryRepository struct {
	users         map[string]domain.User
	refreshTokens map[string]domain.RefreshToken // key: token hash
	mu            sync.Mutex
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		users:         make(map[string]domain.User),
		refreshTokens: make(map[string]domain.RefreshToken),
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

func (r *MemoryRepository) Delete(_ context.Context, userID uuid.UUID) error {
	const op = "MemoryRepository.Delete"

	r.mu.Lock()
	defer r.mu.Unlock()

	var email string
	var found bool
	for _, user := range r.users {
		if user.ID == userID {
			email = user.Email
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("%s: %w", op, domain.ErrNotFound)
	}

	delete(r.users, email)

	return nil
}

func (r *MemoryRepository) CreateRefreshToken(_ context.Context, rt domain.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refreshTokens[rt.TokenHash] = rt

	return nil
}

func (r *MemoryRepository) GetRefreshToken(_ context.Context, tokenHash string) (domain.RefreshToken, error) {
	const op = "MemoryRepository.GetRefreshToken"

	r.mu.Lock()
	defer r.mu.Unlock()

	rt, ok := r.refreshTokens[tokenHash]
	if !ok {
		return domain.RefreshToken{}, fmt.Errorf("%s: %w", op, domain.ErrInvalidToken)
	}

	return rt, nil
}

func (r *MemoryRepository) RevokeRefreshToken(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	rt, ok := r.refreshTokens[tokenHash]
	if !ok {
		return nil // idempotent: уже отозван либо никогда не существовал
	}

	now := time.Now()
	rt.RevokedAt = &now
	r.refreshTokens[tokenHash] = rt

	return nil
}
