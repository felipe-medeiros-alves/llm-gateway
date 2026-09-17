package vllm_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/meta/llm-gateway/internal/domain"
	"github.com/meta/llm-gateway/internal/provider/vllm"
	"github.com/stretchr/testify/require"
)

func TestVLLMChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"local llm"}}]}`))
	}))
	defer srv.Close()
	p := vllm.NewVLLM(srv.URL, srv.Client())
	resp, err := p.Chat(context.Background(), domain.ChatRequest{
		Model: "llama", Messages: []domain.Message{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	require.Equal(t, "local llm", resp.Content)
	require.Equal(t, "vllm", resp.Provider)
	require.Equal(t, "vllm", p.Name())
}
