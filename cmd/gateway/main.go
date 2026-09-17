package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/meta/llm-gateway/internal/cache"
	"github.com/meta/llm-gateway/internal/config"
	"github.com/meta/llm-gateway/internal/grpcapi"
	"github.com/meta/llm-gateway/internal/httpapi"
	"github.com/meta/llm-gateway/internal/provider"
	"github.com/meta/llm-gateway/internal/provider/anthropic"
	"github.com/meta/llm-gateway/internal/provider/openai"
	"github.com/meta/llm-gateway/internal/provider/vllm"
	"github.com/meta/llm-gateway/internal/ratelimit"
	"github.com/meta/llm-gateway/internal/router"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()
	setupLogger(cfg.LogLevel)

	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis url: %v", err)
	}
	rdb := redis.NewClient(opt)
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis ping: %v", err)
	}

	embedder := cache.NewHashEmbedder(384)
	cacheClient := cache.NewRedisCache(rdb, embedder)
	if err := cacheClient.EnsureIndex(ctx); err != nil {
		slog.Warn("semantic cache disabled", "err", err)
	}

	httpClient := &http.Client{Timeout: 120 * time.Second}
	providers := map[string]provider.Provider{
		"openai":    openai.NewOpenAI(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL, httpClient),
		"anthropic": anthropic.NewAnthropic(cfg.AnthropicAPIKey, cfg.AnthropicBaseURL, httpClient),
		"vllm":      vllm.NewVLLM(cfg.VLLMBaseURL, httpClient),
	}

	rtr := router.New(
		cacheClient,
		cfg.ModelAliases,
		providers,
		embedder,
		cfg.SemanticThreshold,
		time.Duration(cfg.CacheTTLSeconds)*time.Second,
	)
	lim := ratelimit.NewRedisLimiter(rdb)

	httpHandler := httpapi.NewServer(rtr, lim, cfg.GatewayKeys)
	go func() {
		slog.Info("http listening", "addr", cfg.HTTPAddr)
		if err := http.ListenAndServe(cfg.HTTPAddr, httpHandler); err != nil {
			log.Fatal(err)
		}
	}()

	grpcSrv := grpc.NewServer()
	grpcapi.Register(grpcSrv, rtr, cfg.GatewayKeys)
	lis, err := net.Listen("tcp", normalizeTCPAddr(cfg.GRPCAddr))
	if err != nil {
		log.Fatal(err)
	}
	slog.Info("grpc listening", "addr", cfg.GRPCAddr)
	log.Fatal(grpcSrv.Serve(lis))
}

func setupLogger(level string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})))
}

func normalizeTCPAddr(addr string) string {
	if strings.HasPrefix(addr, ":") {
		return "0.0.0.0" + addr
	}
	return addr
}
