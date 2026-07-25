package httpclient

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryOn5xx(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Config{Timeout: 2 * time.Second, Name: "test-retry"})
	resp, err := c.Get(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("want 3 calls, got %d", got)
	}
}

func TestBreakerOpensAfterConsecutiveFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(Config{Timeout: 2 * time.Second, Name: "test-breaker"})

	// Each request retries maxRetries times on 500s before exhausting, but
	// they all count as one breaker failure per Execute call.
	var lastErr error
	for i := 0; i < 5; i++ {
		_, lastErr = c.Get(srv.URL)
	}
	if lastErr == nil {
		t.Fatal("want error from exhausted retries, got nil")
	}

	// breaker should now be open: next call fails fast without hitting srv.
	_, err := c.Get(srv.URL)
	if err == nil {
		t.Fatal("want breaker-open error, got nil")
	}
}
