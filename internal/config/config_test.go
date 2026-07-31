package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearEnv unsets every variable Load reads so a test starts from a known state.
// t.Setenv registers a cleanup that restores the original value; the following
// os.Unsetenv makes the variable genuinely absent during the test (important for
// STATION_IDS, whose behavior differs between "unset" and "set but empty").
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"NVE_BASE_URL", "NVE_API_KEY", "NVE_MAX_RETRIES", "NVE_RATE_LIMIT", "NVE_TIMEOUT",
		"DATABASE_URL", "POLL_INTERVAL", "STATION_IDS", "MAX_STATIONS", "PARAMETERS",
		"LOOKBACK", "RESOLUTION_TIME", "API_ADDR", "API_DEFAULT_PAGE_SIZE", "API_MAX_PAGE_SIZE",
		"API_KEYS", "API_RATE_LIMIT", "API_RATE_BURST", "METRICS_ADDR", "ANOMALY_THRESHOLD",
		"ANOMALY_WINDOW", "ALERT_WEBHOOK_URL",
	} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

func TestRequireNVEAPIKey(t *testing.T) {
	clearEnv(t)
	cfg, err := Load()
	require.NoError(t, err)
	assert.Empty(t, cfg.NVEAPIKey)
	err = cfg.RequireNVEAPIKey()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NVE_API_KEY")

	t.Setenv("NVE_API_KEY", "k")
	cfg, err = Load()
	require.NoError(t, err)
	assert.NoError(t, cfg.RequireNVEAPIKey())
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "https://hydapi.nve.no/api/v1", cfg.NVEBaseURL)
	assert.Equal(t, 3, cfg.NVEMaxRetries)
	assert.Equal(t, 5.0, cfg.NVERateLimit)
	assert.Equal(t, 30*time.Second, cfg.NVETimeout)
	assert.Equal(t, 5*time.Minute, cfg.PollInterval)
	assert.Equal(t, []string{"2.32.0"}, cfg.StationIDs)
	assert.False(t, cfg.DiscoverStations)
	assert.Equal(t, 25, cfg.MaxStations)
	assert.Equal(t, []int32{1000, 1001, 1003}, cfg.Parameters)
	assert.Equal(t, 24*time.Hour, cfg.Lookback)
	assert.Equal(t, int32(60), cfg.ResolutionTime)
	assert.Equal(t, ":8080", cfg.APIAddr)
	assert.Equal(t, 500, cfg.APIDefaultPageSize)
	assert.Equal(t, 5000, cfg.APIMaxPageSize)
	assert.Empty(t, cfg.APIKeys)
	assert.Equal(t, 10.0, cfg.APIRateLimit)
	assert.Equal(t, 20, cfg.APIRateBurst)
	assert.Equal(t, 3.0, cfg.AnomalyThreshold)
	assert.Equal(t, 100, cfg.AnomalyWindow)
	assert.Empty(t, cfg.AlertWebhookURL)
}

func TestLoadOverrides(t *testing.T) {
	clearEnv(t)
	t.Setenv("NVE_API_KEY", "k")
	t.Setenv("NVE_BASE_URL", "http://example.test")
	t.Setenv("NVE_MAX_RETRIES", "7")
	t.Setenv("NVE_RATE_LIMIT", "2.5")
	t.Setenv("NVE_TIMEOUT", "15s")
	t.Setenv("POLL_INTERVAL", "1m")
	t.Setenv("STATION_IDS", "a, b ,c")
	t.Setenv("MAX_STATIONS", "10")
	t.Setenv("PARAMETERS", "1000, 2000")
	t.Setenv("LOOKBACK", "48h")
	t.Setenv("RESOLUTION_TIME", "1440")
	t.Setenv("API_KEYS", "first, second")
	t.Setenv("API_RATE_LIMIT", "3.5")
	t.Setenv("API_RATE_BURST", "9")
	t.Setenv("ANOMALY_THRESHOLD", "2.0")
	t.Setenv("ANOMALY_WINDOW", "50")
	t.Setenv("ALERT_WEBHOOK_URL", "http://hook.test")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "k", cfg.NVEAPIKey)
	assert.Equal(t, "http://example.test", cfg.NVEBaseURL)
	assert.Equal(t, 7, cfg.NVEMaxRetries)
	assert.Equal(t, 2.5, cfg.NVERateLimit)
	assert.Equal(t, 15*time.Second, cfg.NVETimeout)
	assert.Equal(t, time.Minute, cfg.PollInterval)
	assert.Equal(t, []string{"a", "b", "c"}, cfg.StationIDs)
	assert.False(t, cfg.DiscoverStations)
	assert.Equal(t, 10, cfg.MaxStations)
	assert.Equal(t, []int32{1000, 2000}, cfg.Parameters)
	assert.Equal(t, 48*time.Hour, cfg.Lookback)
	assert.Equal(t, int32(1440), cfg.ResolutionTime)
	assert.Equal(t, []string{"first", "second"}, cfg.APIKeys)
	assert.Equal(t, 3.5, cfg.APIRateLimit)
	assert.Equal(t, 9, cfg.APIRateBurst)
	assert.Equal(t, 2.0, cfg.AnomalyThreshold)
	assert.Equal(t, 50, cfg.AnomalyWindow)
	assert.Equal(t, "http://hook.test", cfg.AlertWebhookURL)
}

func TestStationDiscoveryMode(t *testing.T) {
	tests := []struct {
		name         string
		set          bool
		value        string
		wantIDs      []string
		wantDiscover bool
	}{
		{name: "unset keeps historical default", set: false, wantIDs: []string{"2.32.0"}, wantDiscover: false},
		{name: "all enables discovery", set: true, value: "all", wantIDs: nil, wantDiscover: true},
		{name: "ALL case-insensitive", set: true, value: "ALL", wantIDs: nil, wantDiscover: true},
		{name: "blank enables discovery", set: true, value: "  ", wantIDs: nil, wantDiscover: true},
		{name: "explicit list", set: true, value: "x,y", wantIDs: []string{"x", "y"}, wantDiscover: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			if tt.set {
				t.Setenv("STATION_IDS", tt.value)
			}
			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.wantIDs, cfg.StationIDs)
			assert.Equal(t, tt.wantDiscover, cfg.DiscoverStations)
		})
	}
}

// Page sizes and the anomaly window are clamped by their consumers, not here.
func TestClamping(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		wantMaxStat int
		wantThresh  float64
	}{
		{
			name:        "max stations floored to 1",
			env:         map[string]string{"MAX_STATIONS": "0"},
			wantMaxStat: 1, wantThresh: 3.0,
		},
		{
			name:        "non-positive threshold reset to default",
			env:         map[string]string{"ANOMALY_THRESHOLD": "-1"},
			wantMaxStat: 25, wantThresh: 3.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			cfg, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.wantMaxStat, cfg.MaxStations)
			assert.Equal(t, tt.wantThresh, cfg.AnomalyThreshold)
		})
	}
}

func TestInvalidValuesFallBackToDefaults(t *testing.T) {
	clearEnv(t)
	t.Setenv("NVE_MAX_RETRIES", "not-an-int")
	t.Setenv("NVE_RATE_LIMIT", "not-a-float")
	t.Setenv("NVE_TIMEOUT", "not-a-duration")
	t.Setenv("API_RATE_LIMIT", "not-a-float")
	t.Setenv("API_RATE_BURST", "not-an-int")
	t.Setenv("PARAMETERS", "1000,bad,2000")

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, 3, cfg.NVEMaxRetries)
	assert.Equal(t, 5.0, cfg.NVERateLimit)
	assert.Equal(t, 30*time.Second, cfg.NVETimeout)
	assert.Equal(t, 10.0, cfg.APIRateLimit)
	assert.Equal(t, 20, cfg.APIRateBurst)
	// getEnvInts skips unparseable entries but keeps valid ones.
	assert.Equal(t, []int32{1000, 2000}, cfg.Parameters)
}
