package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/internal/domain"
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

func (r *PostgresRepository) Save(ctx context.Context, n domain.Notification) (int, error) {
	const op = "PostgresRepository.Save"

	var id int
	err := r.db.QueryRow(
		ctx,
		`INSERT INTO notifications (recipient, subject, body, channel, is_urgent, status)
		VALUES ($1, $2, $3, $4, $5, 'pending')
		RETURNING id`,
		n.Recipient, n.Subject, n.Body, n.Channel, n.IsUrgent,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("%s: scan: %w", op, err)
	}

	return id, nil
}

func (r *PostgresRepository) GetAll(ctx context.Context) ([]domain.Notification, error) {
	const op = "PostgresRepository.GetAll"

	rows, err := r.db.Query(
		ctx,
		`SELECT 
		id, recipient, subject, body, channel, is_urgent, status 
		FROM notifications ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	result, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Notification, error) {
		var n domain.Notification
		scanErr := row.Scan(&n.ID, &n.Recipient, &n.Subject, &n.Body, &n.Channel, &n.IsUrgent, &n.Status)
		return n, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("%s: collect: %w", op, err)
	}

	return result, nil
}

func (r *PostgresRepository) GetList(ctx context.Context, page int, size int) ([]domain.Notification, error) {
	const op = "PostgresRepository.GetList"

	rows, err := r.db.Query(
		ctx,
		`SELECT
		id, recipient, subject, body, channel, is_urgent, status
		FROM notifications ORDER BY id LIMIT $1 OFFSET $2`,
		size,
		(page-1)*size,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	result, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Notification, error) {
		var n domain.Notification
		scanErr := row.Scan(&n.ID, &n.Recipient, &n.Subject, &n.Body, &n.Channel, &n.IsUrgent, &n.Status)
		return n, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("%s: collect: %w", op, err)
	}

	return result, nil
}

func (r *PostgresRepository) GetById(ctx context.Context, id int) (domain.Notification, error) {
	const op = "PostgresRepository.GetById"

	var n domain.Notification
	err := r.db.QueryRow(
		ctx,
		`SELECT 
		id, recipient, subject, body, channel, is_urgent, status 
		FROM notifications WHERE id=$1`,
		id,
	).Scan(&n.ID, &n.Recipient, &n.Subject, &n.Body, &n.Channel, &n.IsUrgent, &n.Status)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Notification{}, fmt.Errorf("%s: scan: %w", op, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: scan: %w", op, err)
	}

	return n, nil
}

func (r *PostgresRepository) UpdateStatus(ctx context.Context, id int, status string) error {
	const op = "PostgresRepository.UpdateStatus"

	tag, err := r.db.Exec(
		ctx,
		`UPDATE notifications SET status=$1 WHERE id=$2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, domain.ErrNotFound)
	}

	return nil
}
