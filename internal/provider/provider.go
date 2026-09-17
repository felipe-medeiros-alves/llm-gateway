package provider

import (
	"context"
	"errors"
	"net"

	"github.com/meta/llm-gateway/internal/domain"
)

type Provider interface {
	Name() string
	Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatResponse, error)
	ChatStream(ctx context.Context, req domain.ChatRequest) (<-chan domain.StreamChunk, <-chan error)
}

type RetryableError struct {
	Status int
	Err    error
}

func (e RetryableError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "retryable upstream error"
}

func (e RetryableError) Unwrap() error {
	return e.Err
}

func IsRetryable(err error) bool {
	var re RetryableError
	if errors.As(err, &re) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

func retryableStatus(status int) bool {
	return status == 429 || status >= 500
}
