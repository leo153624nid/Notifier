package cached_repo

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	core_logger "notifier/internal/core/logger"
	"notifier/internal/service"
)

type CachedNotificationRepo struct {
	repo   service.NotificationRepo
	redis  *redis.Client
	logger *core_logger.Logger
	ttl    time.Duration
}

func NewCachedNotificationRepo(
	repo service.NotificationRepo,
	redis *redis.Client,
	logger *core_logger.Logger,
	ttl time.Duration,
) *CachedNotificationRepo {
	return &CachedNotificationRepo{
		repo:   repo,
		redis:  redis,
		logger: logger,
		ttl:    ttl,
	}
}

// MARK: - Support & Helpers
func notificationCacheKey(id int) string {
	return fmt.Sprintf("notifier:v1:notification:%d", id)
}

func notificationsListCacheKey(page, size int) string {
	return fmt.Sprintf("notifier:v1:notifications:page:%d:size:%d", page, size)
}

func (r *CachedNotificationRepo) invalidateNotificationCache(ctx context.Context, id int) error {
	key := notificationCacheKey(id)
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		return err
	}
	return nil
}
