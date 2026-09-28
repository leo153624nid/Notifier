package postgres

import (
	"context"
	"fmt"
	"uuid"

	"go.uber.org/zap"

	"notifier/internal/core/domain"
	core_errors "notifier/internal/core/errors"
)

func (r *PostgresRepository) CreateIdempotent(
	ctx context.Context,
	consumer string,
	eventID uuid.UUID,
	n domain.Notification,
) (domain.Notification, error) {
	const op = "PostgresRepository.CreateIdempotent"

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: begin: %w", op, err)
	}
	defer func() {
		if rollErr := tx.Rollback(ctx); rollErr != nil {
			r.logger.Error(
				"rollback failed",
				zap.String("op", op),
				zap.Error(rollErr),
			)
		}
	}()

	tag, err := tx.Exec(
		ctx,
		`INSERT INTO processed_events 
		(consumer, event_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`,
		consumer, eventID,
	)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: insert processed_events: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return domain.Notification{}, fmt.Errorf("%s: %w", op, core_errors.ErrEventAlreadyProcessed)
	}

	const status = "pending"
	var id int
	err = tx.QueryRow(
		ctx,
		`INSERT INTO notifications (recipient, subject, body, channel, is_urgent, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		n.Recipient, n.Subject, n.Body, n.Channel, n.IsUrgent, status,
	).Scan(&id)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: scan: %w", op, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Notification{}, fmt.Errorf("%s: commit: %w", op, err)
	}

	return domain.NewNotification(
		id,
		n.Recipient,
		n.Subject,
		n.Body,
		n.Channel,
		status,
		n.IsUrgent,
	), nil
}
