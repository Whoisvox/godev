package probe

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Whoisvox/godev/probe-cli/internal/config"
)

type countingRT struct {
	n     int32
	inner http.RoundTripper
}

func (c *countingRT) RoundTrip(r *http.Request) (*http.Response, error) {
	atomic.AddInt32(&c.n, 1)
	return c.inner.RoundTrip(r)
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		resp Response
		want bool
	}{
		{"dns error not retried", Response{err: &net.DNSError{IsNotFound: true}}, false},
		{"context deadline exceed not retried", Response{err: context.DeadlineExceeded}, false},
		{"context canceled not retried", Response{err: context.Canceled}, false},
		{"5xx retried", Response{StatusCode: 503}, true},
		{"429 retried", Response{StatusCode: 429}, true},
		{"404 not retried", Response{StatusCode: 404}, false},
		{"200 not retried", Response{StatusCode: 200}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRetryable(tt.resp); got != tt.want {
				t.Errorf("isRetryable(%+v) = %v, want %v", tt.resp, got, tt.want)
			}
		})
	}
}

func TestProbeRetry_ConvergesAfter503(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close() // release port after test

	ctx := context.Background()
	resp := ProbeWithRetry(
		ctx,
		srv.Client(),
		config.Target{
			Name: "flaky",
			URL:  srv.URL,
		},
		RetryConfig{
			MaxRetries: 3,
			BaseDelay:  time.Millisecond,
		})

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&n); got != 3 {
		t.Errorf("attempts = %d, want 3", got) // assert retry really did
	}
}

func TestProbeRetry_ConvergesAlways503(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close() // release port after test

	ctx := context.Background()
	resp := ProbeWithRetry(
		ctx,
		srv.Client(),
		config.Target{
			Name: "flaky",
			URL:  srv.URL,
		},
		RetryConfig{
			MaxRetries: 3,
			BaseDelay:  time.Millisecond,
		})

	if resp.StatusCode != 503 {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&n); got != 3+1 {
		t.Errorf("attempts = %d, want 4", got) // assert retry really did all
	}
}

func TestProbeRetry_ConvergesAlways404(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close() // release port after test

	ctx := context.Background()
	resp := ProbeWithRetry(
		ctx,
		srv.Client(),
		config.Target{
			Name: "flaky",
			URL:  srv.URL,
		},
		RetryConfig{
			MaxRetries: 3,
			BaseDelay:  time.Millisecond,
		})

	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&n); got != 1 {
		t.Errorf("attempts = %d, want 1", got) // assert once, 404 never retry
	}
}

func TestProbeRetry_DeadlineExceeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close() // release port after test

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	cRT := &countingRT{inner: srv.Client().Transport}
	resp := ProbeWithRetry(
		ctx,
		// srv.Client(),
		&http.Client{Transport: cRT},
		config.Target{
			Name: "flaky",
			URL:  srv.URL,
		},
		RetryConfig{
			MaxRetries: 3,
			BaseDelay:  25 * time.Millisecond,
		})

	if !errors.Is(resp.err, context.DeadlineExceeded) {
		t.Errorf("err = %v, errmsg = %s want DeadlineExceeded", resp.err, resp.ErrMsg)
	}
	if got := atomic.LoadInt32(&cRT.n); got != 1 {
		t.Errorf("attempts = %d, want 1", got) // assert once, ctx.deadline never retry
	}
}

func TestAll_EndToEnd(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close() // release port after test

	ctx := context.Background()
	resps := All(
		ctx,
		[]config.Target{
			{
				Name: "Alice",
				URL:  srv.URL,
			},
			{
				Name: "Bob",
				URL:  srv.URL,
			},
			{
				Name: "Clark",
				URL:  srv.URL,
			},
		},
		ProbeOptions{
			Concurrency: 2,
			Timeout:     3 * time.Second,
			RetryCfg: RetryConfig{
				MaxRetries: 3,
				BaseDelay:  time.Microsecond,
			},
		},
	)
	names := make(map[string]bool, len(resps))
	for _, resp := range resps {
		if resp.StatusCode != http.StatusOK {
			t.Errorf("statuscode = %d, want 200", resp.StatusCode)
		}
		names[resp.Name] = true
	}
	for _, want := range []string{"Alice", "Bob", "Clark"} {
		if !names[want] {
			t.Errorf("missing result for target %q", want)
		}
	}
	if len(resps) != 3 {
		t.Errorf("length of reps = %d, want 3", len(resps))
	}
	if got := atomic.LoadInt32(&n); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestLatencyMarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		d    Latency
		want string
	}{
		{name: "1500ms", d: Latency(1500 * time.Millisecond), want: "\"1.5s\""},
		{name: "0ms", d: Latency(0 * time.Millisecond), want: "\"0s\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.d.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("MarshalJSON() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestProbeRetry_CanceledDuringBackoff(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	resp := ProbeWithRetry(ctx, srv.Client(),
		config.Target{Name: "flaky", URL: srv.URL},
		RetryConfig{MaxRetries: 5, BaseDelay: 500 * time.Millisecond})
	elapsed := time.Since(start)

	// a. resp.err
	if !errors.Is(resp.err, context.Canceled) {
		t.Errorf("resp.err = %v, want %v", resp.err, context.Canceled)
	}

	// b. attempts
	if got := atomic.LoadInt32(&n); got != 1 {
		t.Errorf("attempts = %d, want 1", got)
	}

	// c.elapsed should less than 500ms
	// because it can't wait first backoff 500ms and was canceled after 100ms sleep,
	// so elapsed actually great a little bit than 100ms
	if elapsed >= 500*time.Millisecond {
		t.Errorf("elapsed = %v, want < 500ms", elapsed)
	}
}

func TestAll_NoGoroutineLeak(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	runtime.GC()
	before := runtime.NumGoroutine()

	ctx := context.Background()
	targets := []config.Target{
		{Name: "Alice", URL: srv.URL},
		{Name: "Bob", URL: srv.URL},
	}
	opts := ProbeOptions{
		Concurrency: 2,
		Timeout:     3 * time.Second,
		RetryCfg: RetryConfig{
			MaxRetries: 3,
			BaseDelay:  time.Microsecond,
		},
	}

	All(ctx, targets, opts)

	srv.CloseClientConnections()
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	after := runtime.NumGoroutine()

	// GC自己的goroutine
	if after > before+1 {
		t.Errorf("goroutine leak: before=%d after=%d", before, after)
	}
}
