package cached_repo

import (
	"context"

	"go.uber.org/zap"
)

func (r *CachedNotificationRepo) UpdateStatus(
	ctx context.Context,
	id int,
	status string,
) error {
	const op = "CachedNotificationRepo.UpdateStatus"

	if err := r.repo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}

	if cacheErr := r.invalidateNotificationCache(ctx, id); cacheErr != nil {
		r.logger.Warn(
			"invalidate cache failed",
			zap.String("op", op),
			zap.Error(cacheErr),
		)
	}

	return nil
}
