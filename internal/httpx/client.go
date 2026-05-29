package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	HTTP    *http.Client
	Timeout time.Duration
	Retries int
	Backoff []time.Duration
	Logger  *slog.Logger
}

func New(logger *slog.Logger) *Client {
	return &Client{
		HTTP:    &http.Client{Timeout: 10 * time.Second},
		Timeout: 10 * time.Second,
		Retries: 3,
		Backoff: []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second},
		Logger:  logger,
	}
}

// GetJSON performs GET request with retries and returns the response body bytes.
func (c *Client) GetJSON(ctx context.Context, endpoint string, params url.Values) ([]byte, int, error) {
	full := endpoint
	if len(params) > 0 {
		if _, err := url.Parse(endpoint); err != nil {
			return nil, 0, err
		}
		if !strings.Contains(endpoint, "?") {
			full = endpoint + "?" + params.Encode()
		} else {
			full = endpoint + "&" + params.Encode()
		}
	}

	var lastErr error
	var lastStatus int
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			idx := attempt - 1
			if idx >= len(c.Backoff) {
				idx = len(c.Backoff) - 1
			}
			d := c.Backoff[idx]
			select {
			case <-ctx.Done():
				return nil, lastStatus, ctx.Err()
			case <-time.After(d):
			}
		}
		body, status, err := c.doOnce(ctx, full)
		lastStatus = status
		if err != nil {
			lastErr = err
			if c.Logger != nil {
				c.Logger.Debug("http request error", "endpoint", full, "attempt", attempt, "err", err)
			}
			continue
		}
		if status >= 500 || status == 429 {
			lastErr = fmt.Errorf("http %d from %s", status, full)
			continue
		}
		if status >= 400 {
			return body, status, fmt.Errorf("http %d from %s: %s", status, full, truncate(body, 256))
		}
		if c.Logger != nil {
			c.Logger.Debug("http ok", "endpoint", full, "status", status, "bytes", len(body))
		}
		return body, status, nil
	}
	if lastErr == nil {
		lastErr = errors.New("retries exhausted")
	}
	return nil, lastStatus, lastErr
}

func (c *Client) doOnce(ctx context.Context, full string) ([]byte, int, error) {
	rctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(rctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tickr/0.1.0")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}

