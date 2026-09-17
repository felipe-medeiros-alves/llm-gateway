package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/meta/llm-gateway/internal/config"
)

type ctxKey int

const principalKey ctxKey = 1

type Principal struct {
	Key    string
	Tenant string
	RPM    int
}

func Middleware(keys []config.GatewayKey) func(http.Handler) http.Handler {
	index := make(map[string]config.GatewayKey, len(keys))
	for _, k := range keys {
		index[k.Key] = k
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, `{"error":"missing bearer token"}`, http.StatusUnauthorized)
				return
			}
			token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			k, ok := index[token]
			if !ok {
				http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), principalKey, Principal{Key: k.Key, Tenant: k.Tenant, RPM: k.RPM})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey).(Principal)
	return p, ok
}
