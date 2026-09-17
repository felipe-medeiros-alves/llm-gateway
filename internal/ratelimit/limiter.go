package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Limiter interface {
	Allow(ctx context.Context, tenantKey string, limit int) (bool, error)
}

type RedisLimiter struct {
	rdb *redis.Client
}

func NewRedisLimiter(rdb *redis.Client) *RedisLimiter {
	return &RedisLimiter{rdb: rdb}
}

func (l *RedisLimiter) Allow(ctx context.Context, tenantKey string, limit int) (bool, error) {
	if limit <= 0 {
		limit = 60
	}
	now := time.Now().UTC()
	window := now.Format("200601021504")
	redisKey := fmt.Sprintf("rl:%s:%s", tenantKey, window)
	count, err := l.rdb.Incr(ctx, redisKey).Result()
	if err != nil {
		return false, err
	}
	if count == 1 {
		_ = l.rdb.Expire(ctx, redisKey, 90*time.Second).Err()
	}
	return count <= int64(limit), nil
}
