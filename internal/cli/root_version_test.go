package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

// rewriteTransport rewrites all requests to the specified target base URL.
type rewriteTransport struct {
	target *url.URL
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = t.target.Scheme
	cloned.URL.Host = t.target.Host
	// Keep path/query from original; our test server ignores path and serves for all routes.
	return http.DefaultTransport.RoundTrip(cloned)
}

func TestFetchLatestRelease_Success(t *testing.T) {
	t.Parallel()

	// Test server returns a fixed tag_name in JSON regardless of path.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v1.2.3"}`))
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	// Override the HTTP client to route requests to our test server.
	old := httpClient
	httpClient = &http.Client{
		Timeout:   time.Second,
		Transport: rewriteTransport{target: u},
	}
	t.Cleanup(func() { httpClient = old })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	tag, err := fetchLatestRelease(ctx)
	if err != nil {
		t.Fatalf("fetchLatestRelease error: %v", err)
	}
	if tag != "v1.2.3" {
		t.Fatalf("got tag %q, want %q", tag, "v1.2.3")
	}
}

func TestFetchLatestRelease_Non200(t *testing.T) {
	t.Parallel()

	var status int32 = 500
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(int(atomic.LoadInt32(&status)))
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	old := httpClient
	httpClient = &http.Client{
		Timeout:   time.Second,
		Transport: rewriteTransport{target: u},
	}
	t.Cleanup(func() { httpClient = old })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := fetchLatestRelease(ctx); err == nil {
		t.Fatalf("expected error for non-200 response, got nil")
	}
}
