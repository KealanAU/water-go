// Package config loads environment-based configuration for both binaries,
// reading a local .env file when present.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	NVEBaseURL string
	NVEAPIKey  string

	NVEMaxRetries int
	NVERateLimit  float64 // requests per second (<= 0 disables rate limiting)
	NVETimeout    time.Duration

	DatabaseURL string

	PollInterval time.Duration
	StationIDs   []string
	// DiscoverStations is true when STATION_IDS is empty or "all": the poller
	// then auto-discovers active stations (bounded by MaxStations) instead of
	// using an explicit list.
	DiscoverStations bool
	MaxStations      int
	Parameters       []int32
	// Lookback is how far back each poll requests observations. The NVE API needs
	// an explicit start/end interval, which the poller builds from now-Lookback..now.
	Lookback       time.Duration
	ResolutionTime int32

	APIAddr            string
	APIDefaultPageSize int
	APIMaxPageSize     int
	// Optional API auth and per-client throttling. Empty APIKeys disables auth;
	// APIRateLimit <= 0 disables API throttling.
	APIKeys      []string
	APIRateLimit float64
	APIRateBurst int

	// Ingester only; the API serves /metrics on its own listener.
	MetricsAddr string

	AnomalyThreshold float64 // |z-score| at/above which a point is an anomaly
	AnomalyWindow    int     // number of recent points used for rolling stats
	AlertWebhookURL  string  // optional: POST alert JSON here (empty disables)
}

func Load() (*Config, error) {
	// Best-effort: a missing .env is not an error (e.g. in containers env vars are injected).
	_ = godotenv.Load()

	stationIDs, discover := stationConfig()

	cfg := &Config{
		NVEBaseURL:    getEnv("NVE_BASE_URL", "https://hydapi.nve.no/api/v1"),
		NVEAPIKey:     os.Getenv("NVE_API_KEY"),
		NVEMaxRetries: getEnvInt("NVE_MAX_RETRIES", 3),
		NVERateLimit:  getEnvFloat("NVE_RATE_LIMIT", 5.0),
		NVETimeout:    getEnvDuration("NVE_TIMEOUT", 30*time.Second),

		DatabaseURL: getEnv("DATABASE_URL", "postgres://water:water@localhost:5432/water?sslmode=disable"),

		PollInterval:     getEnvDuration("POLL_INTERVAL", 5*time.Minute),
		StationIDs:       stationIDs,
		DiscoverStations: discover,
		MaxStations:      getEnvInt("MAX_STATIONS", 25),
		Parameters:       getEnvInts("PARAMETERS", []int32{1000, 1001, 1003}),
		Lookback:         getEnvDuration("LOOKBACK", 24*time.Hour),
		ResolutionTime:   int32(getEnvInt("RESOLUTION_TIME", 60)),

		APIAddr:            getEnv("API_ADDR", ":8080"),
		APIDefaultPageSize: getEnvInt("API_DEFAULT_PAGE_SIZE", 500),
		APIMaxPageSize:     getEnvInt("API_MAX_PAGE_SIZE", 5000),
		APIKeys:            getEnvList("API_KEYS", nil),
		APIRateLimit:       getEnvFloat("API_RATE_LIMIT", 10.0),
		APIRateBurst:       getEnvInt("API_RATE_BURST", 20),

		MetricsAddr: getEnv("METRICS_ADDR", ":9090"),

		AnomalyThreshold: getEnvFloat("ANOMALY_THRESHOLD", 3.0),
		AnomalyWindow:    getEnvInt("ANOMALY_WINDOW", 100),
		AlertWebhookURL:  os.Getenv("ALERT_WEBHOOK_URL"),
	}

	// Page sizes and the anomaly window are clamped by their consumers
	// (api.NewServer, pipeline.NewAnomalyDetector); only values nothing else
	// guards are clamped here.
	if cfg.MaxStations < 1 {
		cfg.MaxStations = 1
	}
	if cfg.AnomalyThreshold <= 0 {
		cfg.AnomalyThreshold = 3.0
	}
	return cfg, nil
}

// The ingester needs the key; the API does not, so the check is separate from Load.
func (c *Config) RequireNVEAPIKey() error {
	if c.NVEAPIKey == "" {
		return fmt.Errorf("NVE_API_KEY is required (set it in .env)")
	}
	return nil
}

// An unset STATION_IDS keeps the historical default of a single station; an
// explicit "all" (or a value that trims to empty) opts into auto-discovery.
func stationConfig() (ids []string, discover bool) {
	raw, set := os.LookupEnv("STATION_IDS")
	if !set {
		return []string{"2.32.0"}, false
	}
	if strings.EqualFold(strings.TrimSpace(raw), "all") {
		return nil, true
	}
	list := getEnvList("STATION_IDS", nil)
	if len(list) == 0 {
		return nil, true
	}
	return list, false
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getEnvList(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func getEnvInts(key string, fallback []int32) []int32 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]int32, 0, len(parts))
	for _, p := range parts {
		if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
			out = append(out, int32(n))
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
