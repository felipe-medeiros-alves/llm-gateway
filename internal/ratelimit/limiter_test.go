package ratelimit_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/meta/llm-gateway/internal/ratelimit"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestAllowRespectsRPM(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	lim := ratelimit.NewRedisLimiter(rdb)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		ok, err := lim.Allow(ctx, "tenant-a", 3)
		require.NoError(t, err)
		require.True(t, ok)
	}
	ok, err := lim.Allow(ctx, "tenant-a", 3)
	require.NoError(t, err)
	require.False(t, ok)
}
