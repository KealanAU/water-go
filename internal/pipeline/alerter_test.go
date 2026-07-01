package pipeline

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAlert() Alert {
	return Alert{
		StationID:     "1.2.3",
		Parameter:     1000,
		ParameterName: "Water level",
		Time:          time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Value:         12.5,
		Mean:          5,
		Stddev:        2,
		ZScore:        3.75,
		Threshold:     3,
	}
}

func alertMessage(t *testing.T, a Alert) *message.Message {
	t.Helper()
	payload, err := json.Marshal(a)
	require.NoError(t, err)
	return message.NewMessage(watermill.NewUUID(), payload)
}

func TestAlerterWebhookSuccess(t *testing.T) {
	var got Alert
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewAlerter(slog.New(slog.NewTextHandler(io.Discard, nil)), srv.URL)
	err := a.handle(alertMessage(t, testAlert()))
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
	assert.Equal(t, "1.2.3", got.StationID)
	assert.Equal(t, 3.75, got.ZScore)
}

func TestAlerterWebhookNon2xxIsAcked(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := NewAlerter(slog.New(slog.NewTextHandler(io.Discard, nil)), srv.URL)
	// A broken endpoint must not wedge the pipeline: handle still acks (nil).
	err := a.handle(alertMessage(t, testAlert()))
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestAlerterNoWebhookConfigured(t *testing.T) {
	a := NewAlerter(slog.New(slog.NewTextHandler(io.Discard, nil)), "")
	err := a.handle(alertMessage(t, testAlert()))
	require.NoError(t, err)
}

func TestAlerterPoisonMessageDropped(t *testing.T) {
	a := NewAlerter(slog.New(slog.NewTextHandler(io.Discard, nil)), "")
	msg := message.NewMessage(watermill.NewUUID(), []byte("not json"))
	err := a.handle(msg)
	require.NoError(t, err, "poison messages are acked and dropped, not retried")
}

func TestPostWebhookTransportError(t *testing.T) {
	// Point at a server we immediately close to force a connection error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	a := NewAlerter(slog.New(slog.NewTextHandler(io.Discard, nil)), url)
	err := a.postWebhook(context.Background(), []byte(`{}`))
	require.Error(t, err)
}

func TestPostWebhookNon2xxReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	a := NewAlerter(slog.New(slog.NewTextHandler(io.Discard, nil)), srv.URL)
	err := a.postWebhook(context.Background(), []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
}
