package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/meta/llm-gateway/internal/domain"
	"github.com/meta/llm-gateway/internal/provider"
)

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewOpenAI(apiKey, baseURL string, httpClient *http.Client) provider.Provider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{apiKey: apiKey, baseURL: strings.TrimSuffix(baseURL, "/"), httpClient: httpClient}
}

func (c *Client) Name() string { return "openai" }

type chatBody struct {
	Model       string           `json:"model"`
	Messages    []domain.Message `json:"messages"`
	Stream      bool             `json:"stream,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (c *Client) Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error) {
	body := chatBody{
		Model: req.Model, Messages: req.Messages, Stream: false,
		MaxTokens: req.MaxTokens, Temperature: req.Temperature,
	}
	b, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return domain.ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.ChatResponse{}, provider.RetryableError{Err: err}
	}
	defer resp.Body.Close()
	if providerRetry(resp.StatusCode) {
		return domain.ChatResponse{}, provider.RetryableError{Status: resp.StatusCode, Err: fmt.Errorf("openai status %d", resp.StatusCode)}
	}
	if resp.StatusCode >= 400 {
		return domain.ChatResponse{}, fmt.Errorf("openai status %d", resp.StatusCode)
	}
	var parsed chatResp
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return domain.ChatResponse{}, err
	}
	content := ""
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
	}
	return domain.ChatResponse{Model: req.Model, Content: content, Provider: "openai"}, nil
}

func providerRetry(code int) bool {
	return code == 429 || code >= 500
}

func (c *Client) ChatStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	chunks := make(chan domain.StreamChunk, 8)
	errs := make(chan error, 1)
	go func() {
		defer close(chunks)
		defer close(errs)
		body := chatBody{
			Model: req.Model, Messages: req.Messages, Stream: true,
			MaxTokens: req.MaxTokens, Temperature: req.Temperature,
		}
		b, _ := json.Marshal(body)
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
		if err != nil {
			errs <- err
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
		}
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errs <- provider.RetryableError{Err: err}
			return
		}
		defer resp.Body.Close()
		if providerRetry(resp.StatusCode) {
			errs <- provider.RetryableError{Status: resp.StatusCode, Err: fmt.Errorf("openai status %d", resp.StatusCode)}
			return
		}
		if resp.StatusCode >= 400 {
			errs <- fmt.Errorf("openai status %d", resp.StatusCode)
			return
		}
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				chunks <- domain.StreamChunk{Done: true, Provider: "openai"}
				return
			}
			var parsed chatResp
			if err := json.Unmarshal([]byte(data), &parsed); err != nil {
				continue
			}
			if len(parsed.Choices) > 0 && parsed.Choices[0].Delta.Content != "" {
				chunks <- domain.StreamChunk{Delta: parsed.Choices[0].Delta.Content, Provider: "openai"}
			}
		}
		if err := sc.Err(); err != nil && err != io.EOF {
			errs <- err
		}
		chunks <- domain.StreamChunk{Done: true, Provider: "openai"}
	}()
	return chunks, errs
}
