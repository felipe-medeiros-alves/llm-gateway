package anthropic

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

func NewAnthropic(apiKey, baseURL string, httpClient *http.Client) provider.Provider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{apiKey: apiKey, baseURL: strings.TrimSuffix(baseURL, "/"), httpClient: httpClient}
}

func (c *Client) Name() string { return "anthropic" }

func (c *Client) Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error) {
	body, err := buildBody(req, false)
	if err != nil {
		return domain.ChatResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return domain.ChatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return domain.ChatResponse{}, provider.RetryableError{Err: err}
	}
	defer resp.Body.Close()
	if retryStatus(resp.StatusCode) {
		return domain.ChatResponse{}, provider.RetryableError{Status: resp.StatusCode, Err: fmt.Errorf("anthropic status %d", resp.StatusCode)}
	}
	if resp.StatusCode >= 400 {
		return domain.ChatResponse{}, fmt.Errorf("anthropic status %d", resp.StatusCode)
	}
	var parsed struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return domain.ChatResponse{}, err
	}
	text := ""
	if len(parsed.Content) > 0 {
		text = parsed.Content[0].Text
	}
	return domain.ChatResponse{Model: req.Model, Content: text, Provider: "anthropic"}, nil
}

func buildBody(req domain.ChatRequest, stream bool) ([]byte, error) {
	var systemParts []string
	var messages []map[string]string
	for _, m := range req.Messages {
		if m.Role == "system" {
			systemParts = append(systemParts, m.Content)
			continue
		}
		role := m.Role
		if role == "assistant" {
			role = "assistant"
		}
		if role != "user" && role != "assistant" {
			role = "user"
		}
		messages = append(messages, map[string]string{"role": role, "content": m.Content})
	}
	payload := map[string]interface{}{
		"model":      req.Model,
		"max_tokens": maxTokens(req.MaxTokens),
		"messages":   messages,
		"stream":     stream,
	}
	if len(systemParts) > 0 {
		payload["system"] = strings.Join(systemParts, "\n")
	}
	return json.Marshal(payload)
}

func maxTokens(n int) int {
	if n <= 0 {
		return 1024
	}
	return n
}

func retryStatus(code int) bool {
	return code == 429 || code >= 500
}

func (c *Client) ChatStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error) {
	chunks := make(chan domain.StreamChunk, 8)
	errs := make(chan error, 1)
	go func() {
		defer close(chunks)
		defer close(errs)
		body, err := buildBody(req, true)
		if err != nil {
			errs <- err
			return
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(body))
		if err != nil {
			errs <- err
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", c.apiKey)
		httpReq.Header.Set("anthropic-version", "2023-06-01")
		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			errs <- provider.RetryableError{Err: err}
			return
		}
		defer resp.Body.Close()
		if retryStatus(resp.StatusCode) {
			errs <- provider.RetryableError{Status: resp.StatusCode, Err: fmt.Errorf("anthropic status %d", resp.StatusCode)}
			return
		}
		if resp.StatusCode >= 400 {
			errs <- fmt.Errorf("anthropic status %d", resp.StatusCode)
			return
		}
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			line := sc.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			var ev struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal([]byte(data), &ev); err != nil {
				continue
			}
			if ev.Type == "content_block_delta" && ev.Delta.Text != "" {
				chunks <- domain.StreamChunk{Delta: ev.Delta.Text, Provider: "anthropic"}
			}
			if ev.Type == "message_stop" {
				chunks <- domain.StreamChunk{Done: true, Provider: "anthropic"}
				return
			}
		}
		if err := sc.Err(); err != nil && err != io.EOF {
			errs <- err
		}
		chunks <- domain.StreamChunk{Done: true, Provider: "anthropic"}
	}()
	return chunks, errs
}
