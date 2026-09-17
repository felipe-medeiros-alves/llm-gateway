package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_requests_total",
		Help: "Total chat completion requests",
	}, []string{"provider", "status", "cached"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gateway_request_duration_seconds",
		Help:    "Chat completion request latency",
		Buckets: prometheus.DefBuckets,
	}, []string{"provider"})

	RateLimitHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "gateway_rate_limit_hits_total",
		Help: "Rate limit rejections",
	})
)

func ObserveChat(provider, status string, cached bool, d time.Duration) {
	cachedLabel := "false"
	if cached {
		cachedLabel = "true"
	}
	RequestsTotal.WithLabelValues(provider, status, cachedLabel).Inc()
	if provider != "" {
		RequestDuration.WithLabelValues(provider).Observe(d.Seconds())
	}
}

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		_ = start
		_ = rw.status
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func FormatCached(cached bool) string {
	return strconv.FormatBool(cached)
}
