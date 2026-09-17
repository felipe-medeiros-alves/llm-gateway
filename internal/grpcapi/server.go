package grpcapi

import (
	gatewayv1 "github.com/meta/llm-gateway/gen/gateway/v1"
	"github.com/meta/llm-gateway/internal/config"
	"github.com/meta/llm-gateway/internal/domain"
	"github.com/meta/llm-gateway/internal/router"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	gatewayv1.UnimplementedGatewayServiceServer
	router *router.Router
	keys   map[string]config.GatewayKey
}

func Register(s *grpc.Server, r *router.Router, gatewayKeys []config.GatewayKey) {
	index := make(map[string]config.GatewayKey, len(gatewayKeys))
	for _, k := range gatewayKeys {
		index[k.Key] = k
	}
	gatewayv1.RegisterGatewayServiceServer(s, &Server{router: r, keys: index})
}

func (s *Server) ChatCompletion(req *gatewayv1.ChatRequest, stream gatewayv1.GatewayService_ChatCompletionServer) error {
	if _, ok := s.keys[req.GetApiKey()]; !ok {
		return status.Error(codes.Unauthenticated, "invalid api key")
	}
	dreq := domain.ChatRequest{
		Model:     req.GetModel(),
		MaxTokens: int(req.GetMaxTokens()),
		Stream:    true,
	}
	for _, m := range req.GetMessages() {
		dreq.Messages = append(dreq.Messages, domain.Message{Role: m.GetRole(), Content: m.GetContent()})
	}
	chunks, errs := s.router.ChatStream(stream.Context(), dreq)
	for c := range chunks {
		if err := stream.Send(&gatewayv1.ChatChunk{
			Delta:    c.Delta,
			Done:     c.Done,
			Provider: c.Provider,
			Cached:   c.Cached,
		}); err != nil {
			return err
		}
		if c.Done {
			return nil
		}
	}
	if err := <-errs; err != nil {
		return status.Errorf(codes.Unavailable, "%v", err)
	}
	return nil
}
