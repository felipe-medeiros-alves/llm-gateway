package vllm

import (
	"context"
	"net/http"
	"strings"

	"github.com/meta/llm-gateway/internal/domain"
	"github.com/meta/llm-gateway/internal/provider"
	"github.com/meta/llm-gateway/internal/provider/openai"
)

type wrapper struct {
	inner provider.Provider
}

func NewVLLM(baseURL string, httpClient *http.Client) provider.Provider {
	inner := openai.NewOpenAI("", strings.TrimSuffix(baseURL, "/")+"/v1", httpClient)
	return wrapper{inner: inner}
}

func (v wrapper) Name() string { return "vllm" }

func (v wrapper) Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error) {
	resp, err := v.inner.Chat(ctx, req)
	resp.Provider = "vllm"
	return resp, err
}

func (v wrapper) ChatStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	chunks, errs := v.inner.ChatStream(ctx, req)
	out := make(chan domain.StreamChunk, 8)
	go func() {
		defer close(out)
		for c := range chunks {
			c.Provider = "vllm"
			out <- c
		}
	}()
	return out, errs
}
