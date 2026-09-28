package cached_repo

import (
	"context"

	"go.uber.org/zap"
)

func (r *CachedNotificationRepo) DeleteById(
	ctx context.Context,
	id int,
) error {
	const op = "CachedNotificationRepo.DeleteById"

	if err := r.repo.DeleteById(ctx, id); err != nil {
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
