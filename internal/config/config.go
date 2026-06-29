package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration, sourced from environment variables.
type Config struct {
	// NVE HydAPI
	NVEBaseURL string
	NVEAPIKey  string

	// Database
	DatabaseURL string

	// Pipeline
	PollInterval time.Duration
	StationIDs   []string
	Parameters   []int32
	// ReferenceTime is the NVE lookback window for each poll, e.g. "P1D" (ISO-8601 duration).
	ReferenceTime  string
	ResolutionTime int32

	// API
	APIAddr string
}

// Load reads configuration from the environment, falling back to a .env file if present.
func Load() (*Config, error) {
	// Best-effort: a missing .env is not an error (e.g. in containers env vars are injected).
	_ = godotenv.Load()

	cfg := &Config{
		NVEBaseURL:     getEnv("NVE_BASE_URL", "https://hydapi.nve.no/api/v1"),
		NVEAPIKey:      os.Getenv("NVE_API_KEY"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://water:water@localhost:5432/water?sslmode=disable"),
		PollInterval:   getEnvDuration("POLL_INTERVAL", 5*time.Minute),
		StationIDs:     getEnvList("STATION_IDS", []string{"2.32.0"}),
		Parameters:     getEnvInts("PARAMETERS", []int32{1000, 1001, 1003}),
		ReferenceTime:  getEnv("REFERENCE_TIME", "P1D"),
		ResolutionTime: int32(getEnvInt("RESOLUTION_TIME", 60)),
		APIAddr:        getEnv("API_ADDR", ":8080"),
	}

	if cfg.NVEAPIKey == "" {
		return nil, fmt.Errorf("NVE_API_KEY is required (set it in .env)")
	}
	return cfg, nil
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
