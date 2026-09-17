package openai_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/meta/llm-gateway/internal/domain"
	"github.com/meta/llm-gateway/internal/provider/openai"
	"github.com/stretchr/testify/require"
)

func TestOpenAIChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hi there"}}]}`))
	}))
	defer srv.Close()
	p := openai.NewOpenAI("sk-test", srv.URL, srv.Client())
	resp, err := p.Chat(context.Background(), domain.ChatRequest{
		Model: "gpt-4o-mini", Messages: []domain.Message{{Role: "user", Content: "hello"}},
	})
	require.NoError(t, err)
	require.Equal(t, "hi there", resp.Content)
	require.Equal(t, "openai", resp.Provider)
}

func TestOpenAIChatStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	p := openai.NewOpenAI("sk-test", srv.URL, srv.Client())
	chunks, errs := p.ChatStream(context.Background(), domain.ChatRequest{
		Model: "gpt-4o-mini", Messages: []domain.Message{{Role: "user", Content: "hello"}}, Stream: true,
	})
	var sb strings.Builder
	for c := range chunks {
		if c.Done {
			break
		}
		sb.WriteString(c.Delta)
	}
	select {
	case err := <-errs:
		if err != nil {
			require.NoError(t, err)
		}
	default:
	}
	require.Equal(t, "Hello", sb.String())
}
