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

	ObservationsStored = promauto.NewCounter(prometheus.CounterOpts{
		Name: "observations_stored_total",
		Help: "Total number of observation rows stored.",
	})

	AnomaliesDetected = promauto.NewCounter(prometheus.CounterOpts{
		Name: "anomalies_detected_total",
		Help: "Total number of anomalies detected.",
	})

	AlertsDispatched = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "alerts_dispatched_total",
		Help: "Total number of alerts dispatched, by sink and outcome.",
	}, []string{"sink", "outcome"})

	// The route label is the chi route template, not the raw path, to keep
	// cardinality bounded.
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests served by the API.",
	}, []string{"method", "route", "status"})

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

func ObserveHTTP(method, route string, status int, d time.Duration) {
	statusCode := strconv.Itoa(status)
	HTTPRequests.WithLabelValues(method, route, statusCode).Inc()
	HTTPRequestDuration.WithLabelValues(method, route, statusCode).Observe(d.Seconds())
}
