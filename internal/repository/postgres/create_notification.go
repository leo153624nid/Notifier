package postgres

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"notifier/internal/core/domain"
	core_errors "notifier/internal/core/errors"
	core_logger "notifier/internal/core/logger"
)

func (r *Repository) CreateIdempotent(
	ctx context.Context,
	consumer string,
	eventID uuid.UUID,
	n domain.Notification,
) (domain.Notification, error) {
	const op = "PostgresRepository.CreateIdempotent"

	logger := core_logger.FromContext(ctx)

	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout())
	defer cancel()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: begin: %w", op, err)
	}
	defer func() {
		if rollErr := tx.Rollback(ctx); rollErr != nil && !errors.Is(rollErr, pgx.ErrTxClosed) {
			logger.Error(
				"rollback failed",
				zap.String("op", op),
				zap.Error(rollErr),
			)
		}
	}()

	tag, err := tx.Exec(
		ctx,
		`INSERT INTO processed_events (consumer, event_id) 
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING;`,
		consumer, eventID,
	)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: insert processed_events: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return domain.Notification{}, fmt.Errorf("%s: %w", op, core_errors.ErrEventAlreadyProcessed)
	}

	var created NotificationModel
	err = tx.QueryRow(
		ctx,
		`INSERT INTO notifications (recipient, subject, body, channel, is_urgent, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, recipient, subject, body, channel, is_urgent, status;`,
		n.Recipient, n.Subject, n.Body, n.Channel, n.IsUrgent, "pending",
	).Scan(
		&created.ID,
		&created.Recipient,
		&created.Subject,
		&created.Body,
		&created.Channel,
		&created.IsUrgent,
		&created.Status,
	)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: scan: %w", op, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Notification{}, fmt.Errorf("%s: commit: %w", op, err)
	}

	return created.toDomain(), nil
}
