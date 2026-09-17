package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/meta/llm-gateway/internal/config"
	"github.com/meta/llm-gateway/internal/domain"
	"github.com/meta/llm-gateway/internal/httpapi"
	"github.com/meta/llm-gateway/internal/provider"
	"github.com/meta/llm-gateway/internal/ratelimit"
	"github.com/meta/llm-gateway/internal/router"
	"github.com/stretchr/testify/require"
)

type stubLimiter struct {
	allow bool
}

func (stubLimiter) Allow(ctx context.Context, key string, limit int) (bool, error) {
	return true, nil
}

type denyLimiter struct{}

func (denyLimiter) Allow(ctx context.Context, key string, limit int) (bool, error) {
	return false, nil
}

type fakeRouter struct {
	resp domain.ChatResponse
}

func (f fakeRouter) Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error) {
	return f.resp, nil
}

func (f fakeRouter) ChatStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	ch := make(chan domain.StreamChunk, 2)
	errs := make(chan error, 1)
	ch <- domain.StreamChunk{Delta: "tok", Provider: "openai"}
	ch <- domain.StreamChunk{Done: true, Provider: "openai"}
	close(ch)
	close(errs)
	return ch, errs
}

// testServer wires a minimal stack with injectable router behavior via embedding.
type testHarness struct {
	router *router.Router
}

func TestChatCompletionsJSON(t *testing.T) {
	fr := &fakeRouterForHTTP{content: "assistant says hi", provider: "openai"}
	srv := newTestServer(fr, stubLimiter{})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"smart-chat","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer good")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "assistant says hi")
}

func TestRateLimit429(t *testing.T) {
	fr := &fakeRouterForHTTP{content: "x", provider: "openai"}
	srv := newTestServer(fr, denyLimiter{})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer good")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
}

type fakeRouterForHTTP struct {
	content  string
	provider string
}

func (f *fakeRouterForHTTP) buildRealRouter() *router.Router {
	return router.New(
		&noopCache{},
		nil,
		map[string]provider.Provider{"openai": providerStub{text: f.content, name: f.provider}},
		stubEmbedder{},
		0.92,
		time.Minute,
	)
}

type providerStub struct {
	text string
	name string
}

func (p providerStub) Name() string { return p.name }
func (p providerStub) Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error) {
	return domain.ChatResponse{Content: p.text, Provider: p.name, Model: req.Model}, nil
}
func (p providerStub) ChatStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	ch := make(chan domain.StreamChunk, 2)
	errs := make(chan error, 1)
	ch <- domain.StreamChunk{Delta: "a", Provider: p.name}
	ch <- domain.StreamChunk{Done: true, Provider: p.name}
	close(ch)
	close(errs)
	return ch, errs
}

type noopCache struct{}

func (noopCache) GetExact(ctx context.Context, model, prompt string) (string, bool, error) {
	return "", false, nil
}
func (noopCache) SetExact(ctx context.Context, model, prompt, response string, ttl time.Duration) error {
	return nil
}
func (noopCache) GetSemantic(ctx context.Context, model string, vec []float32, threshold float64) (string, bool, error) {
	return "", false, nil
}
func (noopCache) SetSemantic(ctx context.Context, model, prompt, response string, vec []float32, ttl time.Duration) error {
	return nil
}
func (noopCache) EnsureIndex(ctx context.Context) error { return nil }

type stubEmbedder struct{}

func (stubEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, 384), nil
}

func newTestServer(_ *fakeRouterForHTTP, lim ratelimit.Limiter) http.Handler {
	keys := []config.GatewayKey{{Key: "good", Tenant: "t1", RPM: 100}}
	p := providerStub{text: "assistant says hi", name: "openai"}
	rtr := router.New(&noopCache{}, nil, map[string]provider.Provider{"openai": p}, stubEmbedder{}, 0.92, time.Minute)
	return httpapi.NewServer(rtr, lim, keys)
}

func TestStreamSSE(t *testing.T) {
	keys := []config.GatewayKey{{Key: "good", Tenant: "t1", RPM: 100}}
	p := providerStub{text: "x", name: "openai"}
	rtr := router.New(&noopCache{}, nil, map[string]provider.Provider{"openai": p}, stubEmbedder{}, 0.92, time.Minute)
	srv := httpapi.NewServer(rtr, stubLimiter{}, keys)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt","stream":true,"messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer good")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "data:")
	require.Contains(t, rec.Body.String(), "[DONE]")
}

func TestJSONShape(t *testing.T) {
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(`{"choices":[{"message":{"content":"x"}}]}`), &m))
}
