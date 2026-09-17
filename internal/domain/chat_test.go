package domain_test

import (
	"testing"

	"github.com/meta/llm-gateway/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestLastUserContent(t *testing.T) {
	msgs := []domain.Message{
		{Role: "system", Content: "s"},
		{Role: "user", Content: " hello "},
	}
	require.Equal(t, " hello ", domain.LastUserContent(msgs))
}

func TestNormalizePrompt(t *testing.T) {
	require.Equal(t, "gpt-4\nhello", domain.NormalizePrompt("GPT-4", "  hello  "))
}
