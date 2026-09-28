package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"notifier/internal/core/domain"
	core_errors "notifier/internal/core/errors"
)

func (r *PostgresRepository) GetById(
	ctx context.Context,
	id int,
) (domain.Notification, error) {
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
		return domain.Notification{}, fmt.Errorf("%s: scan: %w", op, core_errors.ErrNotFound)
	}
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: scan: %w", op, err)
	}

	return n, nil
}
