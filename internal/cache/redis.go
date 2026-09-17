package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(
	addr string,
	password string,
	dialTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	})
}

// MARK: - `Pinger` interface implementation
type Pinger struct {
	client *redis.Client
}

func NewPinger(c *redis.Client) *Pinger {
	return &Pinger{client: c}
}

func (p *Pinger) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
