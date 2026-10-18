package probe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/Whoisvox/godev/probe-cli/internal/config"
)

type Latency time.Duration

type Response struct {
	Name       string  `json:"name,omitempty"`
	URL        string  `json:"url,omitempty"`
	StatusCode int     `json:"status_code,omitempty"`
	Latency    Latency `json:"latency,omitempty"`
	ErrMsg     string  `json:"err,omitempty"`
	err        error
}

type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
}

type ProbeOptions struct {
	Concurrency int
	Timeout     time.Duration
	RetryCfg    RetryConfig
}

func Probe(ctx context.Context, client *http.Client, target config.Target) Response {
	req, err := http.NewRequestWithContext(ctx, "GET", target.URL, nil)
	if err != nil {
		return Response{
			Name:       target.Name,
			URL:        target.URL,
			StatusCode: 0,
			Latency:    0,
			ErrMsg:     err.Error(),
			err:        err,
		}
	}
	start := time.Now()

	resp, err := client.Do(req)
	latency := time.Since(start)
	if err != nil {
		return Response{
			Name:       target.Name,
			URL:        target.URL,
			StatusCode: 0,
			Latency:    Latency(latency),
			ErrMsg:     err.Error(),
			err:        err,
		}
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	return Response{
		Name:       target.Name,
		URL:        target.URL,
		StatusCode: resp.StatusCode,
		Latency:    Latency(latency),
	}

}

func All(ctx context.Context, targets []config.Target, probeOptions ProbeOptions) []Response {
	client := &http.Client{}
	var wg sync.WaitGroup
	jobs := make(chan config.Target, len(targets))
	results := make(chan Response, len(targets))
	responses := make([]Response, 0, len(targets))

	for _, target := range targets {
		jobs <- target
	}

	for w := 1; w <= probeOptions.Concurrency; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
					probeCtx, cancel := context.WithTimeout(ctx, probeOptions.Timeout)
					results <- ProbeWithRetry(probeCtx, client, job, probeOptions.RetryCfg)
					cancel()
				}
			}
		}(w)
	}

	close(jobs)
	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		responses = append(responses, result)
	}
	return responses
}

func ProbeWithRetry(ctx context.Context, client *http.Client, target config.Target, retryCfg RetryConfig) Response {
	var lastResp Response

	for attempt := 0; attempt <= retryCfg.MaxRetries; attempt++ {
		// if not first try, keep await
		if attempt > 0 {
			backoff := retryCfg.BaseDelay * time.Duration(1<<(attempt-1))

			select {
			case <-ctx.Done():
				lastResp.ErrMsg = ctx.Err().Error()
				lastResp.err = ctx.Err()
				return lastResp
			case <-time.After(backoff):
			}
		}

		// call primary Probe
		lastResp = Probe(ctx, client, target)

		// select if it is retryable
		if !isRetryable(lastResp) {
			return lastResp
		}
	}

	// retry celling
	return lastResp
}

func (d Latency) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func isRetryable(resp Response) bool {
	if resp.err != nil {
		// ctx cancled/deadline, shouldn't retry
		if errors.Is(resp.err, context.Canceled) {
			return false
		}
		if errors.Is(resp.err, context.DeadlineExceeded) {
			return false
		}

		var dnsErr *net.DNSError
		if errors.As(resp.err, &dnsErr) {
			return false
		}
		var netErr net.Error
		if errors.As(resp.err, &netErr) {
			return netErr.Timeout()
		}
		return false
	}

	return resp.StatusCode > 499 || resp.StatusCode == 429
}
