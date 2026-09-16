package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"notifier/internal/domain"
)

type CachedNotificationRepo struct {
	repo   NotificationRepo
	redis  *redis.Client
	logger *slog.Logger
	ttl    time.Duration
}

func NewCachedNotificationRepo(
	repo NotificationRepo,
	redis *redis.Client,
	logger *slog.Logger,
	ttl time.Duration,
) *CachedNotificationRepo {
	return &CachedNotificationRepo{
		repo:   repo,
		redis:  redis,
		logger: logger,
		ttl:    ttl,
	}
}

// MARK: - `NotificationRepo` interface implementation
func (r *CachedNotificationRepo) Save(ctx context.Context, n domain.Notification) (int, error) {
	const op = "CachedNotificationRepo.Save"

	id, err := r.repo.Save(ctx, n)
	if err != nil {
		return 0, fmt.Errorf("%s: save: %w", op, err)
	}

	if cacheErr := r.invalidateNotificationCache(ctx, id); cacheErr != nil {
		r.logger.Warn("%s: invalidate: %w", op, cacheErr)
	}

	return id, nil
}

func (r *CachedNotificationRepo) GetAll(ctx context.Context) ([]domain.Notification, error) {
	return r.repo.GetAll(ctx)
}

func (r *CachedNotificationRepo) GetList(ctx context.Context, page int, size int) ([]domain.Notification, error) {
	const op = "CachedNotificationRepo.GetList"

	return r.repo.GetList(ctx, page, size)
}

func (r *CachedNotificationRepo) GetById(ctx context.Context, id int) (domain.Notification, error) {
	const op = "CachedNotificationRepo.GetById"

	key := notificationCacheKey(id)

	cached, err := r.redis.Get(ctx, key).Bytes()
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
		setErr := r.redis.Set(ctx, key, data, r.ttl).Err()
		if setErr != nil {
			r.logger.Error("%s: set to cache: %w", op, setErr)
		}
	}

	return n, nil
}

func (r *CachedNotificationRepo) UpdateStatus(ctx context.Context, id int, status string) error {
	const op = "CachedNotificationRepo.UpdateStatus"

	if err := r.repo.UpdateStatus(ctx, id, status); err != nil {
		return err
	}

	if cacheErr := r.invalidateNotificationCache(ctx, id); cacheErr != nil {
		r.logger.Warn("%s: invalidate: %w", op, cacheErr)
	}

	return nil
}

// MARK: - Support & Helpers
func notificationCacheKey(id int) string {
	return fmt.Sprintf("notification:%d", id)
}

func (r *CachedNotificationRepo) invalidateNotificationCache(ctx context.Context, id int) error {
	key := notificationCacheKey(id)
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		return err
	}
	return nil
}
