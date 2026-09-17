package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"authservice/internal/domain"
)

type PostgresRepository struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

func NewPostgresRepository(db *pgxpool.Pool, logger *slog.Logger) *PostgresRepository {
	return &PostgresRepository{
		db:     db,
		logger: logger,
	}
}

func (r *PostgresRepository) Create(ctx context.Context, u domain.User) (uuid.UUID, error) {
	const op = "PostgresRepository.Create"

	var id uuid.UUID
	err := r.db.QueryRow(
		ctx,
		`INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id`,
		u.Email, u.PasswordHash,
	).Scan(&id)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("%s: scan: %w", op, err)
	}

	return id, nil
}

func (r *PostgresRepository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	const op = "PostgresRepository.GetByEmail"

	var u domain.User
	err := r.db.QueryRow(
		ctx,
		`SELECT 
		id, email, password_hash, created_at
		FROM users WHERE email=$1`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, fmt.Errorf("%s: scan: %w", op, domain.ErrInvalidCredentials)
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("%s: scan: %w", op, err)
	}

	return u, nil
}
