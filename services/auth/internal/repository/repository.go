package repository

import (
	"context"
	"uuid"

	"authservice/internal/domain"
)

type UserRepo interface {
	Create(ctx context.Context, u domain.User) (uuid.UUID, error)
	GetByEmail(ctx context.Context, email string) (domain.User, error)

	CreateRefreshToken(ctx context.Context, rt domain.RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenHash string) (domain.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
}
