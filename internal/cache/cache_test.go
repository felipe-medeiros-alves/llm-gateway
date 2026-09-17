package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/meta/llm-gateway/internal/cache"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestHashEmbedderDeterministic(t *testing.T) {
	e := cache.NewHashEmbedder(384)
	v1, err := e.Embed(context.Background(), "hello world")
	require.NoError(t, err)
	v2, err := e.Embed(context.Background(), "hello world")
	require.NoError(t, err)
	require.Equal(t, v1, v2)
	require.Len(t, v1, 384)
}

func TestExactCacheRoundTrip(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := cache.NewRedisCache(rdb, cache.NewHashEmbedder(384))
	ctx := context.Background()
	_, hit, err := c.GetExact(ctx, "gpt-4o-mini", "hello")
	require.NoError(t, err)
	require.False(t, hit)
	require.NoError(t, c.SetExact(ctx, "gpt-4o-mini", "hello", "world", time.Minute))
	val, hit, err := c.GetExact(ctx, "gpt-4o-mini", "hello")
	require.NoError(t, err)
	require.True(t, hit)
	require.Equal(t, "world", val)
}
