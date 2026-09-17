# llm-gateway

High-performance Go proxy between client applications and LLM providers (OpenAI, Anthropic, local vLLM). Handles gateway API keys, per-key RPM rate limiting, exact + semantic caching in Redis Stack, ordered fallback routing, and streaming over OpenAI-compatible SSE or gRPC.

## Architecture

```
Client → HTTP /v1/chat/completions (JSON or SSE)
      → gRPC GatewayService/ChatCompletion (stream)
      → Auth (Bearer gateway key)
      → Redis RPM limiter
      → Router (exact cache → semantic cache → provider chain)
      → OpenAI | Anthropic | vLLM
```

## Quick start

```bash
cp .env.example .env
make up          # redis-stack, mock providers, gateway
make smoke       # curl health, chat, stream, metrics
make test        # unit tests
make integration # semantic cache (requires Redis Stack on REDIS_URL)
```

## Configuration

| Variable | Description |
|----------|-------------|
| `HTTP_ADDR` | HTTP listen address (default `:8080`) |
| `GRPC_ADDR` | gRPC listen address (default `:9090`) |
| `REDIS_URL` | Redis/Redis Stack URL |
| `GATEWAY_KEYS` | Comma-separated `key:tenant:rpm` |
| `MODEL_ALIASES` | Semicolon-separated `alias=provider:model,...` chains |
| `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` | Upstream credentials |
| `OPENAI_BASE_URL` / `ANTHROPIC_BASE_URL` / `VLLM_BASE_URL` | Upstream bases |
| `CACHE_TTL_SECONDS` | Cache TTL (default 3600) |
| `SEMANTIC_THRESHOLD` | Min cosine similarity for semantic hit (default 0.92) |
| `EMBEDDER` | `hash` in v1 (deterministic test embedder) |

Example alias:

```env
MODEL_ALIASES=smart-chat=openai:gpt-4o-mini,anthropic:claude-3-5-haiku-20241022,vllm:meta-llama/Llama-3.1-8B-Instruct
```

Fallback tries the next provider on HTTP 429, 5xx, or network timeout only.

## HTTP example

```bash
curl -s http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer dev-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"smart-chat","messages":[{"role":"user","content":"hello"}]}'
```

Streaming:

```bash
curl -sN http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer dev-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"smart-chat","stream":true,"messages":[{"role":"user","content":"hello"}]}'
```

## gRPC example

```bash
grpcurl -plaintext -d '{
  "api_key": "dev-key",
  "model": "smart-chat",
  "messages": [{"role": "user", "content": "hello"}]
}' localhost:9090 gateway.v1.GatewayService/ChatCompletion
```

## Observability

- `GET /healthz` — liveness
- `GET /metrics` — Prometheus (`gateway_requests_total`, `gateway_request_duration_seconds`, `gateway_rate_limit_hits_total`)

See [contracts/openai-compat.md](contracts/openai-compat.md) for the HTTP contract.

## Development

```bash
go test ./... -count=1
go run ./cmd/gateway   # requires Redis and env vars
```

Protobuf regeneration:

```bash
make proto   # requires protoc + protoc-gen-go + protoc-gen-go-grpc
```

## License

MIT — see [LICENSE](LICENSE).
