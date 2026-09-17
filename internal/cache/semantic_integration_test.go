//go:build integration

package cache_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/meta/llm-gateway/internal/cache"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestSemanticCacheRoundTrip(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379/0"
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	ctx := context.Background()
	require.NoError(t, rdb.Ping(ctx).Err())

	c := cache.NewRedisCache(rdb, cache.NewHashEmbedder(384))
	require.NoError(t, c.EnsureIndex(ctx))

	model := "test-model"
	prompt := "integration semantic prompt unique " + time.Now().Format(time.RFC3339Nano)
	emb := cache.NewHashEmbedder(384)
	v, err := emb.Embed(ctx, prompt)
	require.NoError(t, err)

	require.NoError(t, c.SetSemantic(ctx, model, prompt, "semantic answer", v, 5*time.Minute))

	got, hit, err := c.GetSemantic(ctx, model, v, 0.99)
	require.NoError(t, err)
	require.True(t, hit)
	require.Equal(t, "semantic answer", got)
}
