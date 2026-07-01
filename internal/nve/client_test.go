package nve

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testClient builds a client pointed at srv with retries/rate-limit tuned so
// tests stay fast (high rate limit, no client-side throttling delays).
func testClient(t *testing.T, srv *httptest.Server, maxRetries int) *Client {
	t.Helper()
	return NewClient(srv.URL, "secret-key",
		WithHTTPClient(srv.Client()),
		WithMaxRetries(maxRetries),
		WithRateLimit(1000),
	)
}

func TestStationsSendsAPIKeyAndDecodesEnvelope(t *testing.T) {
	var gotKey, gotAccept, gotActive string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		gotAccept = r.Header.Get("Accept")
		gotActive = r.URL.Query().Get("Active")
		assert.Equal(t, "/Stations", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"stationId":"1.2.3","stationName":"Foo"}]}`))
	}))
	defer srv.Close()

	c := testClient(t, srv, 0)
	stations, err := c.Stations(context.Background(), true)
	require.NoError(t, err)
	require.Len(t, stations, 1)
	assert.Equal(t, "1.2.3", stations[0].StationID)
	assert.Equal(t, "Foo", stations[0].StationName)
	assert.Equal(t, "secret-key", gotKey)
	assert.Equal(t, "application/json", gotAccept)
	assert.Equal(t, "1", gotActive)
}

func TestStationsActiveOnlyFalseOmitsParam(t *testing.T) {
	var hadActive bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadActive = r.URL.Query()["Active"]
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := testClient(t, srv, 0)
	_, err := c.Stations(context.Background(), false)
	require.NoError(t, err)
	assert.False(t, hadActive, "Active param must be omitted when activeOnly is false")
}

func TestObservationsQueryParamsAndDecode(t *testing.T) {
	var q map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/Observations", r.URL.Path)
		q = map[string]string{
			"StationId":      r.URL.Query().Get("StationId"),
			"Parameter":      r.URL.Query().Get("Parameter"),
			"ResolutionTime": r.URL.Query().Get("ResolutionTime"),
			"ReferenceTime":  r.URL.Query().Get("ReferenceTime"),
		}
		_, _ = w.Write([]byte(`{"data":[{"stationId":"1.2.3","parameter":1000,"unit":"m","observations":[{"time":"2024-01-01T00:00:00Z","value":1.5}]}]}`))
	}))
	defer srv.Close()

	c := testClient(t, srv, 0)
	series, err := c.Observations(context.Background(), ObservationsParams{
		StationID:      "1.2.3",
		Parameter:      1000,
		ResolutionTime: 60,
		ReferenceTime:  "P1D",
	})
	require.NoError(t, err)
	require.Len(t, series, 1)
	require.Len(t, series[0].Observations, 1)
	require.NotNil(t, series[0].Observations[0].Value)
	assert.Equal(t, 1.5, *series[0].Observations[0].Value)
	assert.Equal(t, "1.2.3", q["StationId"])
	assert.Equal(t, "1000", q["Parameter"])
	assert.Equal(t, "60", q["ResolutionTime"])
	assert.Equal(t, "P1D", q["ReferenceTime"])
}

func TestObservationsOmitsEmptyReferenceTime(t *testing.T) {
	var hadRef bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hadRef = r.URL.Query()["ReferenceTime"]
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := testClient(t, srv, 0)
	_, err := c.Observations(context.Background(), ObservationsParams{StationID: "x", Parameter: 1, ResolutionTime: 0})
	require.NoError(t, err)
	assert.False(t, hadRef, "ReferenceTime must be omitted when empty")
}

func TestRetryOn500ThenSuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"stationId":"ok"}]}`))
	}))
	defer srv.Close()

	c := testClient(t, srv, 3)
	stations, err := c.Stations(context.Background(), false)
	require.NoError(t, err)
	require.Len(t, stations, 1)
	assert.Equal(t, "ok", stations[0].StationID)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
}

func TestRetryOn429HonorsRetryAfter(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	c := testClient(t, srv, 3)
	start := time.Now()
	_, err := c.Stations(context.Background(), false)
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&calls))
	assert.GreaterOrEqual(t, time.Since(start), 900*time.Millisecond, "should honor Retry-After delay")
}

func TestNoRetryOn400(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := testClient(t, srv, 3)
	_, err := c.Stations(context.Background(), false)
	require.Error(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "4xx must not be retried")
}

func TestNoRetryOn404(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := testClient(t, srv, 3)
	_, err := c.Observations(context.Background(), ObservationsParams{StationID: "x", Parameter: 1})
	require.Error(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls), "404 must not be retried")
}

func TestRetriesExhaustedReturnsError(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := testClient(t, srv, 2)
	_, err := c.Stations(context.Background(), false)
	require.Error(t, err)
	// initial attempt + 2 retries.
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls))
}
