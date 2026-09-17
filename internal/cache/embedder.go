package cache

import (
	"context"
	"crypto/sha256"
	"math"
)

type HashEmbedder struct {
	Dim int
}

func NewHashEmbedder(dim int) *HashEmbedder {
	if dim <= 0 {
		dim = 384
	}
	return &HashEmbedder{Dim: dim}
}

func (h *HashEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	_ = ctx
	sum := sha256.Sum256([]byte(text))
	out := make([]float32, h.Dim)
	for i := 0; i < h.Dim; i++ {
		idx := i % len(sum)
		out[i] = float32(int8(sum[idx])) / 127.0
	}
	norm := float32(0)
	for _, v := range out {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm > 0 {
		for i := range out {
			out[i] /= norm
		}
	}
	return out, nil
}
