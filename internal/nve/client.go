// Package nve is a client for the NVE HydAPI (Norwegian Water Resources and
// Energy Directorate) hydrological time-series API: https://hydapi.nve.no
package nve

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	DefaultTimeout        = 30 * time.Second
	DefaultMaxRetries     = 3
	DefaultRateLimit      = 5.0 // requests per second
	DefaultBurst          = 1
	defaultBackoffBase    = 500 * time.Millisecond
	defaultBackoffMaxWait = 30 * time.Second
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client

	maxRetries  int
	backoffBase time.Duration
	backoffMax  time.Duration
	limiter     *rate.Limiter
}

type Option func(*Client)

// WithHTTPClient overrides the default HTTP client (e.g. for tests).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithTimeout sets the per-request HTTP timeout. Ignored if a custom HTTP
// client is also supplied via WithHTTPClient after this option.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if d > 0 {
			c.httpClient.Timeout = d
		}
	}
}

// WithMaxRetries bounds how many additional attempts are made on retryable
// failures (network errors, HTTP 5xx, 429). Zero disables retries.
func WithMaxRetries(n int) Option {
	return func(c *Client) {
		if n >= 0 {
			c.maxRetries = n
		}
	}
}

// WithRateLimit sets the client-side request rate (requests/sec). A value <= 0
// disables rate limiting.
func WithRateLimit(rps float64) Option {
	return func(c *Client) {
		if rps <= 0 {
			c.limiter = nil
			return
		}
		burst := int(math.Ceil(rps))
		if burst < 1 {
			burst = 1
		}
		c.limiter = rate.NewLimiter(rate.Limit(rps), burst)
	}
}

func NewClient(baseURL, apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		apiKey:      apiKey,
		httpClient:  &http.Client{Timeout: DefaultTimeout},
		maxRetries:  DefaultMaxRetries,
		backoffBase: defaultBackoffBase,
		backoffMax:  defaultBackoffMaxWait,
		limiter:     rate.NewLimiter(rate.Limit(DefaultRateLimit), DefaultBurst),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

func (c *Client) Stations(ctx context.Context, activeOnly bool) ([]Station, error) {
	q := url.Values{}
	if activeOnly {
		q.Set("Active", "1")
	}
	return doList[Station](ctx, c, "/Stations", q)
}

type ObservationsParams struct {
	StationID      string
	Parameter      int32
	ResolutionTime int32  // minutes: 0 = instantaneous, 60 = hourly, 1440 = daily
	ReferenceTime  string // ISO-8601 duration (e.g. "P1D") or interval ("start/end")
}

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

	body, err := c.get(ctx, path, u)
	if err != nil {
		return nil, err
	}
	defer func() { _ = body.Close() }()

	var env envelope[T]
	if err := json.NewDecoder(body).Decode(&env); err != nil {
		return nil, fmt.Errorf("nve: %s: decode: %w", path, err)
	}
	return env.Data, nil
}

// get performs a GET with client-side rate limiting and bounded exponential
// backoff (with jitter) on retryable failures. The returned body must be closed
// by the caller. Non-retryable failures return immediately.
func (c *Client) get(ctx context.Context, path, u string) (io.ReadCloser, error) {
	for attempt := 0; ; attempt++ {
		if c.limiter != nil {
			if err := c.limiter.Wait(ctx); err != nil {
				return nil, fmt.Errorf("nve: %s: rate limiter: %w", path, err)
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, fmt.Errorf("nve: build request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-API-Key", c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Network/transport error: retryable unless the context is done.
			if ctx.Err() != nil || attempt >= c.maxRetries {
				return nil, fmt.Errorf("nve: %s: %w", path, err)
			}
			if werr := c.wait(ctx, attempt, 0); werr != nil {
				return nil, werr
			}
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return resp.Body, nil
		}

		// Drain+close the body before retrying so the connection can be reused.
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		if !isRetryable(resp.StatusCode) || attempt >= c.maxRetries {
			return nil, fmt.Errorf("nve: %s: unexpected status %s", path, resp.Status)
		}
		if werr := c.wait(ctx, attempt, retryAfter); werr != nil {
			return nil, werr
		}
	}
}

// wait sleeps before the next attempt. If retryAfter is positive it is honored
// (capped at backoffMax); otherwise exponential backoff with full jitter is used.
func (c *Client) wait(ctx context.Context, attempt int, retryAfter time.Duration) error {
	var d time.Duration
	if retryAfter > 0 {
		d = retryAfter
		if d > c.backoffMax {
			d = c.backoffMax
		}
	} else {
		backoff := float64(c.backoffBase) * math.Pow(2, float64(attempt))
		if backoff > float64(c.backoffMax) {
			backoff = float64(c.backoffMax)
		}
		// Full jitter: sleep a random duration in [0, backoff].
		d = time.Duration(rand.Int63n(int64(backoff) + 1))
	}

	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func isRetryable(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

// parseRetryAfter parses a Retry-After header value, which may be either a
// number of seconds or an HTTP date. Returns 0 if absent or unparseable.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
