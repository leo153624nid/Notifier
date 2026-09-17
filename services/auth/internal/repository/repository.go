package repository

import (
	"context"
	"uuid"

	"authservice/internal/domain"
)

type UserRepo interface {
	Create(ctx context.Context, u domain.User) (uuid.UUID, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)
}
