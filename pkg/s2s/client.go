// Package s2s is a minimal REST client for service-to-service calls anywhere
// in the constellation. Every service already vendors keeper (for pkg/auth),
// so this is the shared transport for calling keeper, or any other service,
// over HTTP — not just calls to keeper itself.
package s2s

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// envelope mirrors the {data, error, status} shape every service's render
// package encodes responses with.
type envelope struct {
	Data json.RawMessage `json:"data"`
}

// Client is a thin REST client bound to a single base URL.
type Client struct {
	http *http.Client
	base string
}

// New builds a Client. httpClient must carry a non-zero timeout.
func New(httpClient *http.Client, baseURL string) *Client {
	return &Client{http: httpClient, base: strings.TrimRight(baseURL, "/")}
}

// Get issues GET {base}{path}, decodes the {data, ...} envelope, and
// unmarshals its data into out (skipped when out is nil). Callers own
// caching, retries, and fail-open/closed policy — this only does the request
// and envelope unwrap.
func (c *Client) Get(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return fmt.Errorf("s2s: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("s2s: request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("s2s: %s: unexpected status %d", path, resp.StatusCode)
	}

	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return fmt.Errorf("s2s: decode envelope: %w", err)
	}
	if out == nil || len(env.Data) == 0 {
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("s2s: decode data: %w", err)
	}
	return nil
}
