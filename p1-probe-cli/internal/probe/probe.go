package probe

import (
	"context"
	"encoding/json"
	"io"
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
	Err        string  `json:"err,omitempty"`
}

func Probe(ctx context.Context, client *http.Client, target config.Target) Response {
	req, err := http.NewRequestWithContext(ctx, "GET", target.URL, nil)
	if err != nil {
		return Response{
			Name:       target.Name,
			URL:        target.URL,
			StatusCode: 0,
			Latency:    0,
			Err:        err.Error(),
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
			Err:        err.Error(),
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

func All(ctx context.Context, targets []config.Target, concurrency int, timeout time.Duration) []Response {
	client := &http.Client{}
	var wg sync.WaitGroup
	jobs := make(chan config.Target, len(targets))
	results := make(chan Response, len(targets))
	responses := make([]Response, 0, len(targets))

	for _, target := range targets {
		jobs <- target
	}

	for w := 1; w <= concurrency; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
					probeCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
					results <- Probe(probeCtx, client, job)
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

func (d Latency) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}
