package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"notifier/internal/core/domain"
)

func (r *Repository) GetList(
	ctx context.Context,
	page *int,
	size *int,
) ([]domain.Notification, error) {
	const op = "PostgresRepository.GetList"

	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout())
	defer cancel()

	var limit *int
	var offset *int
	if page != nil && size != nil {
		s := *size
		limit = new(s)
		p := (*page - 1) * (*limit)
		offset = new(p)
	}

	rows, err := r.pool.Query(
		ctx,
		`SELECT
		id, recipient, subject, body, channel, is_urgent, status
		FROM notifications ORDER BY id ASC
		LIMIT $1 OFFSET $2;`,
		limit,
		offset,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	result, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (NotificationModel, error) {
		var m NotificationModel
		scanErr := row.Scan(&m.ID, &m.Recipient, &m.Subject, &m.Body, &m.Channel, &m.IsUrgent, &m.Status)
		return m, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("%s: collect: %w", op, err)
	}

	return listModelsToDomain(result), nil
}

func (r *Repository) GetAll(ctx context.Context) ([]domain.Notification, error) {
	const op = "PostgresRepository.GetAll"

	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout())
	defer cancel()

	rows, err := r.pool.Query(
		ctx,
		`SELECT 
		id, recipient, subject, body, channel, is_urgent, status 
		FROM notifications ORDER BY id;`,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: query: %w", op, err)
	}
	defer rows.Close()

	result, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (NotificationModel, error) {
		var n NotificationModel
		scanErr := row.Scan(&n.ID, &n.Recipient, &n.Subject, &n.Body, &n.Channel, &n.IsUrgent, &n.Status)
		return n, scanErr
	})
	if err != nil {
		return nil, fmt.Errorf("%s: collect: %w", op, err)
	}

	return listModelsToDomain(result), nil
}
