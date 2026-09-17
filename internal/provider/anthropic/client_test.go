package anthropic_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/meta/llm-gateway/internal/domain"
	"github.com/meta/llm-gateway/internal/provider/anthropic"
	"github.com/stretchr/testify/require"
)

func TestAnthropicChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/messages", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"claude says hi"}]}`))
	}))
	defer srv.Close()
	p := anthropic.NewAnthropic("sk-ant", srv.URL, srv.Client())
	resp, err := p.Chat(context.Background(), domain.ChatRequest{
		Model: "claude-3-5-haiku-20241022",
		Messages: []domain.Message{
			{Role: "system", Content: "be brief"},
			{Role: "user", Content: "hello"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "claude says hi", resp.Content)
}

func TestAnthropicStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"a "}}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"b"}}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"message_stop"}` + "\n\n"))
	}))
	defer srv.Close()
	p := anthropic.NewAnthropic("sk-ant", srv.URL, srv.Client())
	chunks, errs := p.ChatStream(context.Background(), domain.ChatRequest{
		Model: "claude", Messages: []domain.Message{{Role: "user", Content: "hi"}}, Stream: true,
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
	require.Equal(t, "a b", sb.String())
}
