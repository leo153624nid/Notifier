package cached_repo

import (
	"context"
	"fmt"
	"uuid"

	"go.uber.org/zap"

	"notifier/internal/core/domain"
	core_logger "notifier/internal/core/logger"
)

func (r *CachedNotificationRepo) CreateIdempotent(
	ctx context.Context,
	consumer string,
	eventID uuid.UUID,
	n domain.Notification,
) (domain.Notification, error) {
	const op = "CachedNotificationRepo.CreateIdempotent"

	logger := core_logger.FromContext(ctx)

	created, err := r.repo.CreateIdempotent(ctx, consumer, eventID, n)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("%s: save: %w", op, err)
	}

	if err := r.invalidateNotificationCache(ctx, created.ID); err != nil {
		logger.Warn(
			"invalidate cache",
			zap.String("op", op),
			zap.Error(err),
		)
	}

	return created, nil
}
