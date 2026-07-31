package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestAuthMiddlewareDisabledWhenNoKeysConfigured(t *testing.T) {
	s := NewServer(nil, testLogger(), Options{})
	h := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stations", nil))

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestAuthMiddlewareAcceptsBearerAndAPIKeyHeaders(t *testing.T) {
	s := NewServer(nil, testLogger(), Options{APIKeys: []string{"secret"}})
	h := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tc := range []struct {
		name   string
		header string
		value  string
	}{
		{name: "bearer", header: "Authorization", value: "Bearer secret"},
		{name: "api key", header: "X-API-Key", value: "secret"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/stations", nil)
			req.Header.Set(tc.header, tc.value)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusNoContent, rec.Code)
		})
	}
}

func TestRoutesRequireAPIKeyForStationsOnly(t *testing.T) {
	h := NewServer(nil, testLogger(), Options{APIKeys: []string{"secret"}}).Routes()

	health := httptest.NewRecorder()
	h.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	assert.Equal(t, http.StatusOK, health.Code)

	stations := httptest.NewRecorder()
	h.ServeHTTP(stations, httptest.NewRequest(http.MethodGet, "/stations", nil))
	assert.Equal(t, http.StatusUnauthorized, stations.Code)
}

func TestAuthMiddlewareRejectsMissingOrInvalidKey(t *testing.T) {
	s := NewServer(nil, testLogger(), Options{APIKeys: []string{"secret"}})
	h := s.authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tc := range []struct {
		name   string
		header string
		value  string
	}{
		{name: "missing"},
		{name: "wrong bearer", header: "Authorization", value: "Bearer wrong"},
		{name: "wrong api key", header: "X-API-Key", value: "wrong"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/stations", nil)
			if tc.header != "" {
				req.Header.Set(tc.header, tc.value)
			}
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

func TestRateLimiterDisabledWhenRateNonPositive(t *testing.T) {
	limiter := newClientRateLimiter(0, 5)
	assert.Nil(t, limiter)

	h := limiter.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/stations", nil))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRateLimiterLimitsPerClient(t *testing.T) {
	limiter := newClientRateLimiter(1, 1)
	h := limiter.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/stations", nil)
	req1.RemoteAddr = "192.0.2.10:1234"
	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req1)
	assert.Equal(t, http.StatusNoContent, rec1.Code)

	req2 := httptest.NewRequest(http.MethodGet, "/stations", nil)
	req2.RemoteAddr = "192.0.2.10:5678"
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusTooManyRequests, rec2.Code)

	req3 := httptest.NewRequest(http.MethodGet, "/stations", nil)
	req3.RemoteAddr = "198.51.100.7:1234"
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, req3)
	assert.Equal(t, http.StatusNoContent, rec3.Code)
}

func TestClientKeyPrefersForwardedClientIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/stations", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")

	var got string
	h := middleware.ClientIPFromXFF()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = clientKey(r)
	}))
	h.ServeHTTP(httptest.NewRecorder(), req)

	assert.Equal(t, "203.0.113.9", got)
}
