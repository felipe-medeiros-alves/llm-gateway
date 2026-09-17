package cache

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/meta/llm-gateway/internal/domain"
	"github.com/redis/go-redis/v9"
)

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type Cache interface {
	GetExact(ctx context.Context, model, prompt string) (string, bool, error)
	SetExact(ctx context.Context, model, prompt, response string, ttl time.Duration) error
	GetSemantic(ctx context.Context, model string, vec []float32, threshold float64) (string, bool, error)
	SetSemantic(ctx context.Context, model, prompt, response string, vec []float32, ttl time.Duration) error
	EnsureIndex(ctx context.Context) error
}

type RedisCache struct {
	rdb       *redis.Client
	embedder  Embedder
	semanticOK bool
}

func NewRedisCache(rdb *redis.Client, embedder Embedder) *RedisCache {
	return &RedisCache{rdb: rdb, embedder: embedder, semanticOK: true}
}

func (c *RedisCache) SemanticEnabled() bool {
	return c.semanticOK
}

func cacheKey(model, prompt string) string {
	h := sha256.Sum256([]byte(domain.NormalizePrompt(model, prompt)))
	return "exact:" + hex.EncodeToString(h[:])
}

func (c *RedisCache) GetExact(ctx context.Context, model, prompt string) (string, bool, error) {
	val, err := c.rdb.Get(ctx, cacheKey(model, prompt)).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return val, true, nil
}

func (c *RedisCache) SetExact(ctx context.Context, model, prompt, response string, ttl time.Duration) error {
	return c.rdb.Set(ctx, cacheKey(model, prompt), response, ttl).Err()
}

func vecToBytes(vec []float32) []byte {
	b := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	return b
}

func (c *RedisCache) EnsureIndex(ctx context.Context) error {
	_, err := c.rdb.Do(ctx, "FT.CREATE", "idx:semantic", "ON", "HASH", "PREFIX", "1", "sem:", "SCHEMA",
		"prompt", "TEXT", "response", "TEXT", "model", "TAG", "embedding", "VECTOR", "HNSW", "6",
		"TYPE", "FLOAT32", "DIM", "384", "DISTANCE_METRIC", "COSINE").Result()
	if err != nil && !strings.Contains(err.Error(), "Index already exists") {
		c.semanticOK = false
		return err
	}
	return nil
}

func (c *RedisCache) GetSemantic(ctx context.Context, model string, vec []float32, threshold float64) (string, bool, error) {
	if !c.semanticOK || len(vec) == 0 {
		return "", false, nil
	}
	query := fmt.Sprintf("@model:{%s}=>[KNN 1 @embedding $vec AS score]", tagQueryEscape(model))
	res, err := c.rdb.Do(ctx, "FT.SEARCH", "idx:semantic", query,
		"PARAMS", "2", "vec", vecToBytes(vec),
		"RETURN", "2", "response", "score",
		"DIALECT", "2").Result()
	if err != nil {
		if strings.Contains(err.Error(), "Unknown index") {
			c.semanticOK = false
			return "", false, nil
		}
		return "", false, err
	}
	if m, ok := res.(map[interface{}]interface{}); ok {
		return parseSemanticMap(m, threshold)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) < 2 {
		return "", false, nil
	}
	n := searchCount(arr[0])
	if n == 0 {
		return "", false, nil
	}
	fields, ok := arr[2].([]interface{})
	if !ok {
		return "", false, nil
	}
	var response string
	var score float64
	for i := 0; i+1 < len(fields); i += 2 {
		key, _ := fields[i].(string)
		switch key {
		case "response":
			response, _ = fields[i+1].(string)
		case "score":
			switch v := fields[i+1].(type) {
			case string:
				fmt.Sscanf(v, "%f", &score)
			case float64:
				score = v
			}
		}
	}
	similarity := 1 - score
	if similarity >= threshold && response != "" {
		return response, true, nil
	}
	return "", false, nil
}

func tagQueryEscape(model string) string {
	return strings.ReplaceAll(model, "-", "\\-")
}

func parseSemanticMap(m map[interface{}]interface{}, threshold float64) (string, bool, error) {
	results, ok := m["results"].([]interface{})
	if !ok || len(results) == 0 {
		return "", false, nil
	}
	first, ok := results[0].(map[interface{}]interface{})
	if !ok {
		return "", false, nil
	}
	attrs, ok := first["extra_attributes"].(map[interface{}]interface{})
	if !ok {
		return "", false, nil
	}
	response, _ := attrs["response"].(string)
	score := parseScore(attrs["score"])
	similarity := 1 - score
	if similarity >= threshold && response != "" {
	 return response, true, nil
	}
	return "", false, nil
}

func parseScore(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case string:
		var f float64
		fmt.Sscanf(t, "%f", &f)
		return f
	default:
		return 1
	}
}

func searchCount(v interface{}) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case string:
		var n int64
		fmt.Sscanf(t, "%d", &n)
		return n
	default:
		return 0
	}
}

func (c *RedisCache) SetSemantic(ctx context.Context, model, prompt, response string, vec []float32, ttl time.Duration) error {
	if !c.semanticOK {
		return nil
	}
	id := uuid.NewString()
	key := "sem:" + id
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, key, map[string]interface{}{
		"prompt":    prompt,
		"response":  response,
		"model":     model,
		"embedding": vecToBytes(vec),
	})
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}
