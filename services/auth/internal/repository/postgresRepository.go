package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"uuid"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return uuid.UUID{}, fmt.Errorf("%s: %w", op, domain.ErrUserExists)
		}

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

func (r *PostgresRepository) Delete(ctx context.Context, userID uuid.UUID) error {
	const op = "PostgresRepository.Delete"

	result, err := r.db.Exec(
		ctx,
		`Delete
		FROM users WHERE id=$1`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("%s: delete: %w", op, err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("%s: delete: %w", op, domain.ErrNotFound)
	}

	return nil
}

func (r *PostgresRepository) CreateRefreshToken(ctx context.Context, rt domain.RefreshToken) error {
	const op = "PostgresRepository.CreateRefreshToken"

	_, err := r.db.Exec(
		ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`,
		rt.ID, rt.UserID, rt.TokenHash, rt.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (r *PostgresRepository) GetRefreshToken(ctx context.Context, tokenHash string) (domain.RefreshToken, error) {
	const op = "PostgresRepository.GetRefreshToken"

	var rt domain.RefreshToken
	err := r.db.QueryRow(
		ctx,
		`SELECT id, user_id, token_hash, expires_at, created_at, revoked_at
		FROM refresh_tokens WHERE token_hash=$1`,
		tokenHash,
	).Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.CreatedAt, &rt.RevokedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RefreshToken{}, fmt.Errorf("%s: scan: %w", op, domain.ErrInvalidToken)
	}
	if err != nil {
		return domain.RefreshToken{}, fmt.Errorf("%s: scan: %w", op, err)
	}

	return rt, nil
}

func (r *PostgresRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	const op = "PostgresRepository.RevokeRefreshToken"

	_, err := r.db.Exec(
		ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash=$1 AND revoked_at IS NULL`,
		tokenHash,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
