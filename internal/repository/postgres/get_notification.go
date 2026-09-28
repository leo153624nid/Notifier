package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"notifier/internal/core/domain"
	core_errors "notifier/internal/core/errors"
)

func (r *Repository) GetById(
	ctx context.Context,
	id int,
) (domain.Notification, error) {
	const op = "PostgresRepository.GetById"

	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout())
	defer cancel()

	var model NotificationModel
	err := r.pool.QueryRow(
		ctx,
		`SELECT 
		id, recipient, subject, body, channel, is_urgent, status 
		FROM notifications WHERE id=$1;`,
		id,
	).Scan(
		&model.ID,
		&model.Recipient,
		&model.Subject,
		&model.Body,
		&model.Channel,
		&model.IsUrgent,
		&model.Status,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Notification{}, fmt.Errorf("%s: scan: %w", op, core_errors.ErrNotFound)
	}
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: scan: %w", op, err)
	}

	return model.toDomain(), nil
}
