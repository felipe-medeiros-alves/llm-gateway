package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/meta/llm-gateway/internal/auth"
	"github.com/meta/llm-gateway/internal/config"
	"github.com/stretchr/testify/require"
)

func TestMiddlewareAcceptsValidBearer(t *testing.T) {
	keys := []config.GatewayKey{{Key: "good", Tenant: "t1", RPM: 10}}
	var gotTenant string
	h := auth.Middleware(keys)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFromContext(r.Context())
		require.True(t, ok)
		gotTenant = p.Tenant
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer good")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "t1", gotTenant)
}

func TestMiddlewareRejectsMissingAuth(t *testing.T) {
	h := auth.Middleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestMiddlewareRejectsInvalidKey(t *testing.T) {
	keys := []config.GatewayKey{{Key: "good", Tenant: "t1", RPM: 10}}
	h := auth.Middleware(keys)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer bad")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
