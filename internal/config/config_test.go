package config_test

import (
	"testing"

	"github.com/meta/llm-gateway/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoadParsesGatewayKeys(t *testing.T) {
	t.Setenv("GATEWAY_KEYS", "abc:tenant-a:30,def:tenant-b:60")
	t.Setenv("MODEL_ALIASES", "fast=openai:gpt-4o-mini,vllm:llama3")
	cfg := config.Load()
	require.Len(t, cfg.GatewayKeys, 2)
	require.Equal(t, "abc", cfg.GatewayKeys[0].Key)
	require.Equal(t, 30, cfg.GatewayKeys[0].RPM)
	require.Equal(t, []config.ProviderTarget{
		{Provider: "openai", Model: "gpt-4o-mini"},
		{Provider: "vllm", Model: "llama3"},
	}, cfg.ModelAliases["fast"])
}

func TestLoadDefaultsHTTPAddr(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	cfg := config.Load()
	require.Equal(t, ":8080", cfg.HTTPAddr)
}
