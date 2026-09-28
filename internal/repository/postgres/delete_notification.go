package postgres

import (
	"context"
	"fmt"

	core_errors "notifier/internal/core/errors"
)

func (r *Repository) DeleteById(
	ctx context.Context,
	id int,
) error {
	const op = "PostgresRepository.DeleteById"

	result, err := r.pool.Exec(
		ctx,
		`DELETE 
		FROM notifications WHERE id=$1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("%s: delete: %w", op, err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("%s: delete: %w", op, core_errors.ErrNotFound)
	}

	return nil
}
