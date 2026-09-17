package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/meta/llm-gateway/internal/auth"
	"github.com/meta/llm-gateway/internal/config"
	"github.com/meta/llm-gateway/internal/domain"
	"github.com/meta/llm-gateway/internal/observability"
	"github.com/meta/llm-gateway/internal/ratelimit"
	"github.com/meta/llm-gateway/internal/router"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	router *router.Router
	limit  ratelimit.Limiter
	keys   []config.GatewayKey
}

func NewServer(r *router.Router, lim ratelimit.Limiter, keys []config.GatewayKey) http.Handler {
	s := &Server{router: r, limit: lim, keys: keys}
	mux := chi.NewRouter()
	mux.Use(middleware.Recoverer)
	mux.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())
	mux.Group(func(r chi.Router) {
		r.Use(auth.Middleware(keys))
		r.Post("/v1/chat/completions", s.handleChat)
	})
	return mux
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	p, ok := auth.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	allowed, err := s.limit.Allow(r.Context(), p.Tenant, p.RPM)
	if err != nil {
		http.Error(w, `{"error":"rate limit check failed"}`, http.StatusInternalServerError)
		return
	}
	if !allowed {
		observability.RateLimitHits.Inc()
		http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
		return
	}

	var body domain.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if body.Stream {
		s.streamChat(w, r, body, start)
		return
	}
	resp, err := s.router.Chat(r.Context(), body)
	if err != nil {
		observability.ObserveChat("", "error", false, time.Since(start))
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadGateway)
		return
	}
	provider := resp.Provider
	if resp.Cached {
		provider = "cache"
	}
	observability.ObserveChat(provider, "ok", resp.Cached, time.Since(start))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      "gw-" + uuid.NewString(),
		"object":  "chat.completion",
		"model":   resp.Model,
		"choices": []map[string]interface{}{{"index": 0, "message": map[string]string{"role": "assistant", "content": resp.Content}, "finish_reason": "stop"}},
	})
}

func (s *Server) streamChat(w http.ResponseWriter, r *http.Request, body domain.ChatRequest, start time.Time) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	id := "gw-" + uuid.NewString()
	chunks, errs := s.router.ChatStream(r.Context(), body)
	var provider string
	cached := false
	for c := range chunks {
		if c.Provider != "" {
			provider = c.Provider
		}
		if c.Cached {
			cached = true
		}
		if c.Done {
			final := map[string]interface{}{
				"id": id, "object": "chat.completion.chunk",
				"choices": []map[string]interface{}{{"index": 0, "delta": map[string]string{}, "finish_reason": "stop"}},
			}
			_ = WriteSSE(w, final)
			_ = WriteSSEDone(w)
			break
		}
		if c.Delta == "" {
			continue
		}
		chunk := map[string]interface{}{
			"id": id, "object": "chat.completion.chunk",
			"choices": []map[string]interface{}{{"index": 0, "delta": map[string]string{"content": c.Delta}}},
		}
		if err := WriteSSE(w, chunk); err != nil {
			return
		}
	}
	if err := <-errs; err != nil {
		observability.ObserveChat(provider, "error", cached, time.Since(start))
		return
	}
	if provider == "" {
		provider = "cache"
	}
	observability.ObserveChat(provider, "ok", cached, time.Since(start))
}
