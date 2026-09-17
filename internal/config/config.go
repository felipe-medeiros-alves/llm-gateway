package config

import (
	"os"
	"strconv"
	"strings"
)

type ProviderTarget struct {
	Provider string
	Model    string
}

type GatewayKey struct {
	Key    string
	Tenant string
	RPM    int
}

type Config struct {
	HTTPAddr          string
	GRPCAddr          string
	RedisURL          string
	GatewayKeys       []GatewayKey
	ModelAliases      map[string][]ProviderTarget
	CacheTTLSeconds   int
	SemanticThreshold float64
	Embedder          string
	LogLevel          string
	OpenAIAPIKey      string
	AnthropicAPIKey   string
	OpenAIBaseURL     string
	AnthropicBaseURL  string
	VLLMBaseURL       string
}

func Load() Config {
	return Config{
		HTTPAddr:          getenv("HTTP_ADDR", ":8080"),
		GRPCAddr:          getenv("GRPC_ADDR", ":9090"),
		RedisURL:          getenv("REDIS_URL", "redis://localhost:6379/0"),
		GatewayKeys:       parseGatewayKeys(getenv("GATEWAY_KEYS", "dev-key:default:60")),
		ModelAliases:      parseModelAliases(getenv("MODEL_ALIASES", "")),
		CacheTTLSeconds:   positiveInt("CACHE_TTL_SECONDS", 3600),
		SemanticThreshold: parseFloat("SEMANTIC_THRESHOLD", 0.92),
		Embedder:          getenv("EMBEDDER", "hash"),
		LogLevel:          getenv("LOG_LEVEL", "info"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		AnthropicAPIKey:   os.Getenv("ANTHROPIC_API_KEY"),
		OpenAIBaseURL:     getenv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		AnthropicBaseURL:  getenv("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
		VLLMBaseURL:       getenv("VLLM_BASE_URL", "http://localhost:8000"),
	}
}

func parseGatewayKeys(raw string) []GatewayKey {
	var out []GatewayKey
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Split(part, ":")
		if len(fields) < 3 {
			continue
		}
		rpm, _ := strconv.Atoi(fields[2])
		if rpm <= 0 {
			rpm = 60
		}
		out = append(out, GatewayKey{Key: fields[0], Tenant: fields[1], RPM: rpm})
	}
	return out
}

func parseModelAliases(raw string) map[string][]ProviderTarget {
	out := make(map[string][]ProviderTarget)
	for _, aliasDef := range strings.Split(raw, ";") {
		aliasDef = strings.TrimSpace(aliasDef)
		if aliasDef == "" {
			continue
		}
		parts := strings.SplitN(aliasDef, "=", 2)
		if len(parts) != 2 {
			continue
		}
		var targets []ProviderTarget
		for _, target := range strings.Split(parts[1], ",") {
			target = strings.TrimSpace(target)
			pm := strings.SplitN(target, ":", 2)
			if len(pm) != 2 {
				continue
			}
			targets = append(targets, ProviderTarget{Provider: pm[0], Model: pm[1]})
		}
		out[parts[0]] = targets
	}
	return out
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func positiveInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func parseFloat(key string, fallback float64) float64 {
	v, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil {
		return fallback
	}
	return v
}
