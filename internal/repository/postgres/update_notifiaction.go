package postgres

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	core_errors "notifier/internal/core/errors"
	core_logger "notifier/internal/core/logger"
)

func (r *Repository) UpdateStatus(
	ctx context.Context,
	id int,
	status string,
) error {
	const op = "PostgresRepository.UpdateStatus"

	logger := core_logger.FromContext(ctx)
	logger.Debug(
		"want update status",
		zap.String("status", status),
		zap.Int("id", id),
	)

	tag, err := r.pool.Exec(
		ctx,
		`UPDATE notifications SET status=$1 WHERE id=$2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, core_errors.ErrNotFound)
	}

	return nil
}
