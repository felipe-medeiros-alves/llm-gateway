package router_test

import (
	"context"
	"testing"
	"time"

	"github.com/meta/llm-gateway/internal/config"
	"github.com/meta/llm-gateway/internal/domain"
	"github.com/meta/llm-gateway/internal/provider"
	"github.com/meta/llm-gateway/internal/router"
	"github.com/stretchr/testify/require"
)

type fakeCache struct {
	exact string
}

func (f *fakeCache) GetExact(ctx context.Context, model, prompt string) (string, bool, error) {
	if f.exact != "" {
		return f.exact, true, nil
	}
	return "", false, nil
}
func (f *fakeCache) SetExact(ctx context.Context, model, prompt, response string, ttl time.Duration) error {
	return nil
}
func (f *fakeCache) GetSemantic(ctx context.Context, model string, vec []float32, threshold float64) (string, bool, error) {
	return "", false, nil
}
func (f *fakeCache) SetSemantic(ctx context.Context, model, prompt, response string, vec []float32, ttl time.Duration) error {
	return nil
}
func (f *fakeCache) EnsureIndex(ctx context.Context) error { return nil }

type fakeEmbedder struct{}

func (fakeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, 384), nil
}

type panicProvider struct{}

func (panicProvider) Name() string { return "panic" }
func (panicProvider) Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error) {
	panic("should not call provider on cache hit")
}
func (panicProvider) ChatStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	panic("should not call provider on cache hit")
}

func TestChatCacheHitSkipsProviders(t *testing.T) {
	r := router.New(
		&fakeCache{exact: "cached answer"},
		map[string][]config.ProviderTarget{"m": {{Provider: "panic", Model: "x"}}},
		map[string]provider.Provider{"panic": panicProvider{}},
		fakeEmbedder{},
		0.92,
		time.Minute,
	)
	resp, err := r.Chat(context.Background(), domain.ChatRequest{
		Model: "m", Messages: []domain.Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	require.True(t, resp.Cached)
	require.Equal(t, "cached answer", resp.Content)
}

type seqProvider struct {
	name   string
	err    error
	text   string
	called int
}

func (s *seqProvider) Name() string { return s.name }
func (s *seqProvider) Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error) {
	s.called++
	if s.err != nil {
		return domain.ChatResponse{}, s.err
	}
	return domain.ChatResponse{Content: s.text, Provider: s.name}, nil
}
func (s *seqProvider) ChatStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	ch := make(chan domain.StreamChunk)
	errs := make(chan error, 1)
	close(ch)
	close(errs)
	return ch, errs
}

func TestChatFallbackOnRetryable(t *testing.T) {
	first := &seqProvider{name: "a", err: provider.RetryableError{Status: 503}}
	second := &seqProvider{name: "b", text: "fallback ok"}
	r := router.New(
		&fakeCache{},
		map[string][]config.ProviderTarget{"m": {{Provider: "a", Model: "m1"}, {Provider: "b", Model: "m2"}}},
		map[string]provider.Provider{"a": first, "b": second},
		fakeEmbedder{},
		0.92,
		time.Minute,
	)
	resp, err := r.Chat(context.Background(), domain.ChatRequest{
		Model: "m", Messages: []domain.Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	require.Equal(t, "fallback ok", resp.Content)
	require.Equal(t, "b", resp.Provider)
	require.Equal(t, 1, first.called)
	require.Equal(t, 1, second.called)
}

func TestChatNoFallbackOn4xx(t *testing.T) {
	first := &seqProvider{name: "a", err: errNonRetry}
	second := &seqProvider{name: "b", text: "nope"}
	r := router.New(
		&fakeCache{},
		map[string][]config.ProviderTarget{"m": {{Provider: "a", Model: "m1"}, {Provider: "b", Model: "m2"}}},
		map[string]provider.Provider{"a": first, "b": second},
		fakeEmbedder{},
		0.92,
		time.Minute,
	)
	_, err := r.Chat(context.Background(), domain.ChatRequest{
		Model: "m", Messages: []domain.Message{{Role: "user", Content: "hi"}},
	})
	require.Error(t, err)
	require.Equal(t, 0, second.called)
}

type nonRetryErr struct{}

func (nonRetryErr) Error() string { return "bad request" }

var errNonRetry = nonRetryErr{}

func TestIsRetryableFalseForNonRetry(t *testing.T) {
	require.False(t, provider.IsRetryable(errNonRetry))
}
