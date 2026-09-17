package domain

import "strings"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

type ChatResponse struct {
	ID       string
	Model    string
	Content  string
	Provider string
	Cached   bool
}

type StreamChunk struct {
	Delta    string
	Done     bool
	Provider string
	Cached   bool
}

func LastUserContent(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return ""
}

func NormalizePrompt(model, content string) string {
	return strings.ToLower(strings.TrimSpace(model)) + "\n" + strings.ToLower(strings.TrimSpace(content))
}
