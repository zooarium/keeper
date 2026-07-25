// Package httpclient provides a shared outbound HTTP client with retry and
// circuit-breaker resilience, for every service-to-service or third-party
// call anywhere in the constellation (keeper s2s, captcha verification,
// impersonation revocation checks, etc). Build clients here instead of
// &http.Client{} by hand — a tripped breaker or exhausted retry just
// surfaces as a transport error, so existing caller fail-open/fail-closed
// policy is unaffected.
//
// Retries apply regardless of HTTP method, so only use this for
// idempotent-safe requests (GET, or verify-style POSTs safe to repeat).
package httpclient

import (
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"time"

	"github.com/sony/gobreaker/v2"
)

// Config configures a shared client.
type Config struct {
	// Timeout is the per-request timeout. Required — never zero.
	Timeout time.Duration
	// Name identifies this client's breaker in logs (e.g. "keeper-s2s").
	Name string
}

const maxRetries = 2

var retryBackoffs = []time.Duration{100 * time.Millisecond, 300 * time.Millisecond}

// New builds an *http.Client with retry-on-transient-failure and a circuit
// breaker layered over the default transport.
func New(cfg Config) *http.Client {
	if cfg.Timeout <= 0 {
		panic("httpclient: Config.Timeout must be non-zero")
	}

	breaker := gobreaker.NewCircuitBreaker[*http.Response](gobreaker.Settings{
		Name:    cfg.Name,
		Timeout: 30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			slog.Warn("circuit breaker state change", "client", name, "from", from.String(), "to", to.String())
		},
	})

	return &http.Client{
		Timeout:   cfg.Timeout,
		Transport: &roundTripper{next: http.DefaultTransport, breaker: breaker, name: cfg.Name},
	}
}

type roundTripper struct {
	next    http.RoundTripper
	breaker *gobreaker.CircuitBreaker[*http.Response]
	name    string
}

func (rt *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt.breaker.Execute(func() (*http.Response, error) {
		return rt.doWithRetry(req)
	})
}

// doWithRetry retries transient failures (network errors, 5xx, 429) with
// backoff+jitter. Requests with a non-empty, non-resettable body (no
// GetBody) are tried once — the body would already be drained on retry.
func (rt *roundTripper) doWithRetry(req *http.Request) (*http.Response, error) {
	retries := maxRetries
	if req.Body != nil && req.GetBody == nil {
		retries = 0
	}

	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, fmt.Errorf("httpclient: %s: reset body for retry: %w", rt.name, err)
				}
				req.Body = body
			}
			backoff := retryBackoffs[attempt-1]
			time.Sleep(backoff + time.Duration(rand.Int63n(int64(backoff))))
		}

		resp, err := rt.next.RoundTrip(req)
		if err == nil && resp.StatusCode < http.StatusInternalServerError && resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}
		if err != nil {
			lastErr = err
			continue
		}
		lastErr = fmt.Errorf("httpclient: %s: retryable status %d", rt.name, resp.StatusCode)
		_ = resp.Body.Close()
	}
	return nil, lastErr
}
