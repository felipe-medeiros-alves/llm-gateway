package grpcapi_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	gatewayv1 "github.com/meta/llm-gateway/gen/gateway/v1"
	"github.com/meta/llm-gateway/internal/config"
	"github.com/meta/llm-gateway/internal/domain"
	"github.com/meta/llm-gateway/internal/grpcapi"
	"github.com/meta/llm-gateway/internal/provider"
	"github.com/meta/llm-gateway/internal/router"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

type stubProvider struct {
	text string
}

func (s stubProvider) Name() string { return "openai" }
func (s stubProvider) Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error) {
	return domain.ChatResponse{Content: s.text, Provider: "openai"}, nil
}
func (s stubProvider) ChatStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	ch := make(chan domain.StreamChunk, 3)
	errs := make(chan error, 1)
	ch <- domain.StreamChunk{Delta: "hel", Provider: "openai"}
	ch <- domain.StreamChunk{Delta: "lo", Provider: "openai"}
	ch <- domain.StreamChunk{Done: true, Provider: "openai"}
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

func TestGRPCChatCompletionStream(t *testing.T) {
	lis := bufconn.Listen(bufSize)
	rtr := router.New(
		noopCache{},
		nil,
		map[string]provider.Provider{"openai": stubProvider{text: "hello"}},
		stubEmbedder{},
		0.92,
		time.Minute,
	)
	s := grpc.NewServer()
	grpcapi.Register(s, rtr, []config.GatewayKey{{Key: "k1", Tenant: "t", RPM: 60}})
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(func() { s.Stop() })

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	client := gatewayv1.NewGatewayServiceClient(conn)
	stream, err := client.ChatCompletion(context.Background(), &gatewayv1.ChatRequest{
		ApiKey: "k1", Model: "gpt-4o-mini",
		Messages: []*gatewayv1.ChatMessage{{Role: "user", Content: "hi"}},
	})
	require.NoError(t, err)
	var sb string
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		sb += chunk.GetDelta()
		if chunk.GetDone() {
			break
		}
	}
	require.Equal(t, "hello", sb)
}
