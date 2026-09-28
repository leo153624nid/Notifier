package cached_repo

import (
	"context"

	"go.uber.org/zap"

	core_logger "notifier/internal/core/logger"
)

func (r *CachedNotificationRepo) DeleteById(
	ctx context.Context,
	id int,
) error {
	const op = "CachedNotificationRepo.DeleteById"

	logger := core_logger.FromContext(ctx)

	if err := r.repo.DeleteById(ctx, id); err != nil {
		return err
	}

	if cacheErr := r.invalidateNotificationCache(ctx, id); cacheErr != nil {
		logger.Warn(
			"invalidate cache failed",
			zap.String("op", op),
			zap.Error(cacheErr),
		)
	}

	return nil
}
