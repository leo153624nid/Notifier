package cached_repo

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"

	"notifier/internal/core/domain"
	core_logger "notifier/internal/core/logger"
)

func (r *CachedNotificationRepo) GetList(
	ctx context.Context,
	page int,
	size int,
) ([]domain.Notification, error) {
	const op = "CachedNotificationRepo.GetList"

	key := notificationsListCacheKey(page, size)
	logger := core_logger.FromContext(ctx)

	cached, err := r.cache.Get(ctx, key).Bytes()
	if err == nil {
		result := make([]domain.Notification, size)
		if jsonErr := json.Unmarshal(cached, &result); jsonErr == nil {
			logger.Debug("cache used")
			return result, nil
		}
	}

	result, err := r.repo.GetList(ctx, page, size)
	if err != nil {
		return nil, err
	}

	if data, marshalErr := json.Marshal(result); marshalErr == nil {
		setErr := r.cache.Set(ctx, key, data, r.ttl).Err()
		if setErr != nil {
			logger.Warn(
				"set to cache",
				zap.String("op", op),
				zap.Error(setErr),
			)
		}
	}

	return result, nil
}

func (r *CachedNotificationRepo) GetAll(ctx context.Context) ([]domain.Notification, error) {
	return r.repo.GetAll(ctx)
}
