package router

import (
	"context"
	"strings"
	"time"

	"github.com/meta/llm-gateway/internal/cache"
	"github.com/meta/llm-gateway/internal/config"
	"github.com/meta/llm-gateway/internal/domain"
	"github.com/meta/llm-gateway/internal/provider"
)

type Router struct {
	cache      cache.Cache
	aliases    map[string][]config.ProviderTarget
	providers  map[string]provider.Provider
	embedder   cache.Embedder
	threshold  float64
	ttl        time.Duration
}

func New(
	c cache.Cache,
	aliases map[string][]config.ProviderTarget,
	providers map[string]provider.Provider,
	embedder cache.Embedder,
	threshold float64,
	ttl time.Duration,
) *Router {
	return &Router{
		cache: c, aliases: aliases, providers: providers,
		embedder: embedder, threshold: threshold, ttl: ttl,
	}
}

func (r *Router) Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error) {
	prompt := domain.LastUserContent(req.Messages)
	targets := r.resolveTargets(req.Model)

	if val, hit, _ := r.cache.GetExact(ctx, req.Model, prompt); hit {
		return domain.ChatResponse{Model: req.Model, Content: val, Cached: true}, nil
	}

	vec, _ := r.embedder.Embed(ctx, prompt)
	if val, hit, _ := r.cache.GetSemantic(ctx, req.Model, vec, r.threshold); hit {
		return domain.ChatResponse{Model: req.Model, Content: val, Cached: true}, nil
	}

	var lastErr error
	for _, target := range targets {
		p := r.providers[target.Provider]
		if p == nil {
			continue
		}
		sub := req
		sub.Model = target.Model
		resp, err := p.Chat(ctx, sub)
		if err == nil {
			resp.Model = req.Model
			_ = r.cache.SetExact(ctx, req.Model, prompt, resp.Content, r.ttl)
			_ = r.cache.SetSemantic(ctx, req.Model, prompt, resp.Content, vec, r.ttl)
			return resp, nil
		}
		lastErr = err
		if !provider.IsRetryable(err) {
			return domain.ChatResponse{}, err
		}
	}
	if lastErr != nil {
		return domain.ChatResponse{}, lastErr
	}
	return domain.ChatResponse{}, errNoProviders
}

var errNoProviders = &noProvidersError{}

type noProvidersError struct{}

func (e *noProvidersError) Error() string { return "no providers configured for model" }

func (r *Router) ChatStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	out := make(chan domain.StreamChunk, 16)
	errs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errs)
		prompt := domain.LastUserContent(req.Messages)
		if val, hit, _ := r.cache.GetExact(ctx, req.Model, prompt); hit {
			out <- domain.StreamChunk{Delta: val, Provider: "cache", Cached: true}
			out <- domain.StreamChunk{Done: true, Cached: true}
			return
		}
		vec, _ := r.embedder.Embed(ctx, prompt)
		if val, hit, _ := r.cache.GetSemantic(ctx, req.Model, vec, r.threshold); hit {
			out <- domain.StreamChunk{Delta: val, Provider: "cache", Cached: true}
			out <- domain.StreamChunk{Done: true, Cached: true}
			return
		}

		targets := r.resolveTargets(req.Model)
		var lastErr error
		for _, target := range targets {
			p := r.providers[target.Provider]
			if p == nil {
				continue
			}
			sub := req
			sub.Model = target.Model
			sub.Stream = true
			chunks, pErrs := p.ChatStream(ctx, sub)
			var acc strings.Builder
			var providerName string
			streamOK := true
			for c := range chunks {
				if c.Done {
					break
				}
				providerName = c.Provider
				acc.WriteString(c.Delta)
				out <- domain.StreamChunk{Delta: c.Delta, Provider: c.Provider}
			}
			select {
			case err := <-pErrs:
				if err != nil {
					lastErr = err
					if provider.IsRetryable(err) {
						streamOK = false
					} else {
						errs <- err
						return
					}
				}
			default:
			}
			if streamOK && acc.Len() > 0 {
				content := acc.String()
				_ = r.cache.SetExact(ctx, req.Model, prompt, content, r.ttl)
				_ = r.cache.SetSemantic(ctx, req.Model, prompt, content, vec, r.ttl)
				out <- domain.StreamChunk{Done: true, Provider: providerName}
				return
			}
		}
		if lastErr != nil {
			errs <- lastErr
		} else {
			errs <- errNoProviders
		}
	}()
	return out, errs
}

func (r *Router) resolveTargets(model string) []config.ProviderTarget {
	if t, ok := r.aliases[model]; ok && len(t) > 0 {
		return t
	}
	return []config.ProviderTarget{{Provider: "openai", Model: model}}
}
