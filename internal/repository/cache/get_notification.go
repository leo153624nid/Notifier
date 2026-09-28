package cached_repo

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"

	"notifier/internal/core/domain"
	core_logger "notifier/internal/core/logger"
)

func (r *CachedNotificationRepo) GetById(
	ctx context.Context,
	id int,
) (domain.Notification, error) {
	const op = "CachedNotificationRepo.GetById"

	key := notificationCacheKey(id)
	logger := core_logger.FromContext(ctx)

	cached, err := r.cache.Get(ctx, key).Bytes()
	if err == nil {
		var n domain.Notification
		if jsonErr := json.Unmarshal(cached, &n); jsonErr == nil {
			return n, nil
		}
	}

	n, err := r.repo.GetById(ctx, id)
	if err != nil {
		return domain.Notification{}, err
	}

	if data, marshalErr := json.Marshal(n); marshalErr == nil {
		setErr := r.cache.Set(ctx, key, data, r.ttl).Err()
		if setErr != nil {
			logger.Warn(
				"set to cache",
				zap.String("op", op),
				zap.Error(setErr),
			)
		}
	}

	return n, nil
}
