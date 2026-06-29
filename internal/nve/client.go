// Package nve is a client for the NVE HydAPI (Norwegian Water Resources and
// Energy Directorate) hydrological time-series API: https://hydapi.nve.no
package nve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is a thin HTTP client for the NVE HydAPI.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient overrides the default HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// NewClient builds a HydAPI client. The API key is sent in the X-API-Key header.
func NewClient(baseURL, apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Stations returns the list of stations. If activeOnly is true, only currently
// active stations are returned.
func (c *Client) Stations(ctx context.Context, activeOnly bool) ([]Station, error) {
	q := url.Values{}
	if activeOnly {
		q.Set("Active", "1")
	}
	return doList[Station](ctx, c, "/Stations", q)
}

// ObservationsParams selects which observations to fetch.
type ObservationsParams struct {
	StationID      string
	Parameter      int32
	ResolutionTime int32  // minutes: 0 = instantaneous, 60 = hourly, 1440 = daily
	ReferenceTime  string // ISO-8601 duration (e.g. "P1D") or interval ("start/end")
}

// Observations fetches observations for a station/parameter within a reference window.
// One Series is returned per matching parameter series.
func (c *Client) Observations(ctx context.Context, p ObservationsParams) ([]Series, error) {
	q := url.Values{}
	q.Set("StationId", p.StationID)
	q.Set("Parameter", strconv.Itoa(int(p.Parameter)))
	q.Set("ResolutionTime", strconv.Itoa(int(p.ResolutionTime)))
	if p.ReferenceTime != "" {
		q.Set("ReferenceTime", p.ReferenceTime)
	}
	return doList[Series](ctx, c, "/Observations", q)
}

func doList[T any](ctx context.Context, c *Client, path string, q url.Values) ([]T, error) {
	u := c.baseURL + path
	if enc := q.Encode(); enc != "" {
		u += "?" + enc
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("nve: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nve: %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nve: %s: unexpected status %s", path, resp.Status)
	}

	var env envelope[T]
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("nve: %s: decode: %w", path, err)
	}
	return env.Data, nil
}
