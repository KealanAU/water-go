// Package metrics defines the application's Prometheus metrics and exposes a
// promhttp handler. All metrics are registered on the default registry via
// promauto, so importing this package is enough to make them available.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	PollTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "nve_poll_total",
		Help: "Total number of NVE observation fetches attempted.",
	})

	FetchErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "nve_fetch_errors_total",
		Help: "Total number of failed NVE fetches.",
	}, []string{"parameter"})

	RequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "nve_request_duration_seconds",
		Help:    "Duration of NVE API client calls in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"call"})

	// ObservationsStored counts observation rows persisted by the normalizer.
	ObservationsStored = promauto.NewCounter(prometheus.CounterOpts{
		Name: "observations_stored_total",
		Help: "Total number of observation rows stored.",
	})

	// AnomaliesDetected counts anomalies recorded by the detector.
	AnomaliesDetected = promauto.NewCounter(prometheus.CounterOpts{
		Name: "anomalies_detected_total",
		Help: "Total number of anomalies detected.",
	})

	AlertsDispatched = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alerts_dispatched_total",
		Help: "Total number of alerts dispatched, by sink and outcome.",
	}, []string{"sink", "outcome"})

	// HTTPRequests counts API requests by route template and response status.
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests served by the API.",
	}, []string{"method", "route", "status"})

	// HTTPRequestDuration observes API request latency by route template.
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Duration of API HTTP requests in seconds.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route", "status"})

	LastPollTimestamp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pipeline_last_poll_timestamp_seconds",
		Help: "Unix timestamp (seconds) of the last completed poll cycle.",
	})
)

func Handler() http.Handler { return promhttp.Handler() }

func ObserveRequest(call string, start time.Time) {
	RequestDuration.WithLabelValues(call).Observe(time.Since(start).Seconds())
}

func IncFetchError(parameter int32) {
	FetchErrors.WithLabelValues(strconv.Itoa(int(parameter))).Inc()
}

func ObservationsStoredAdd(n int) {
	if n > 0 {
		ObservationsStored.Add(float64(n))
	}
}

func MarkPoll() { LastPollTimestamp.SetToCurrentTime() }

// AlertDispatched records an alert dispatch outcome for a sink (log|webhook).
func AlertDispatched(sink, outcome string) {
	AlertsDispatched.WithLabelValues(sink, outcome).Inc()
}

func ObserveHTTP(method, route string, status int, d time.Duration) {
	statusCode := strconv.Itoa(status)
	HTTPRequests.WithLabelValues(method, route, statusCode).Inc()
	HTTPRequestDuration.WithLabelValues(method, route, statusCode).Observe(d.Seconds())
}
