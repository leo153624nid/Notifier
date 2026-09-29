package cached_repo

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"notifier/internal/service"
)

type CachedNotificationRepo struct {
	repo  service.NotificationRepo
	cache *redis.Client
	ttl   time.Duration
}

func NewCachedNotificationRepo(
	repo service.NotificationRepo,
	cache *redis.Client,
	ttl time.Duration,
) *CachedNotificationRepo {
	return &CachedNotificationRepo{
		repo:  repo,
		cache: cache,
		ttl:   ttl,
	}
}

// MARK: - Support & Helpers
func notificationCacheKey(id int) string {
	return fmt.Sprintf("notifier:v1:notification:%d", id)
}

func notificationsListCacheKey(page, size *int) string {
	if page != nil && size != nil {
		return fmt.Sprintf("notifier:v1:notifications:page:%dsize:%d", *page, *size)
	}
	return fmt.Sprintf("notifier:v1:notifications:page:%ssize:%s", "all", "all")
}

func (r *CachedNotificationRepo) invalidateNotificationCache(ctx context.Context, id int) error {
	key := notificationCacheKey(id)
	if err := r.cache.Del(ctx, key).Err(); err != nil {
		return err
	}
	return nil
}
