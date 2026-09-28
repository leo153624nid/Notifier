package postgres

import (
	"context"
	"fmt"

	core_errors "notifier/internal/core/errors"
)

func (r *Repository) UpdateStatus(
	ctx context.Context,
	id int,
	status string,
) error {
	const op = "PostgresRepository.UpdateStatus"

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
