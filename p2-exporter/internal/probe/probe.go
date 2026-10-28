package probe

import (
	"context"
	"io"
	"net/http"
	"sync"
	"time"
)

type Target struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type Options struct {
	Client      *http.Client
	Concurrency int
	Timeout     time.Duration
}
type Result struct {
	Name       string
	URL        string
	Success    bool
	StatusCode int
	Duration   time.Duration
	Err        error
}

func Probe(ctx context.Context, target Target, client *http.Client) Result {
	req, err := http.NewRequestWithContext(ctx, "GET", target.URL, nil)
	if err != nil {
		return Result{
			Name:    target.Name,
			URL:     target.URL,
			Success: false,
			Err:     err,
		}
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return Result{
			Name:     target.Name,
			URL:      target.URL,
			Success:  false,
			Duration: duration,
			Err:      err,
		}
	}
	defer resp.Body.Close()

	// read out resp.Body for reuse conn nexttime
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	return Result{
		Name:       target.Name,
		URL:        target.URL,
		Success:    resp.StatusCode != 429 && resp.StatusCode < 500,
		StatusCode: resp.StatusCode,
		Duration:   duration,
	}
}
func ProbeOnce(ctx context.Context, targets []Target, opts Options) []Result {
	// workers share the single client
	var wg sync.WaitGroup
	jobs := make(chan Target, len(targets))
	results := make(chan Result, len(targets))
	responses := make([]Result, 0, len(targets))

	for _, target := range targets {
		jobs <- target
	}

	for w := 1; w <= opts.Concurrency; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
					probeCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
					results <- Probe(probeCtx, job, opts.Client)
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

	for r := range results {
		responses = append(responses, r)
	}

	return responses
}
