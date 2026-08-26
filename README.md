# llm-gateway
Proxy that sits between client applications and multiple LLM providers (OpenAI, Anthropic, local vLLM). It handles API key management, intelligent rate limiting, semantic caching (using Redis to cache similar prompts and save costs), fallback routing, and streaming token responses via Server-Sent Events (SSE) or gRPC.
