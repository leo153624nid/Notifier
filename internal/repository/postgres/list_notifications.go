package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"notifier/internal/core/domain"
)

func (r *Repository) GetList(ctx context.Context, page int, size int) ([]domain.Notification, error) {
	const op = "PostgresRepository.GetList"

	rows, err := r.pool.Query(
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

func (r *Repository) GetAll(ctx context.Context) ([]domain.Notification, error) {
	const op = "PostgresRepository.GetAll"

	rows, err := r.pool.Query(
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
