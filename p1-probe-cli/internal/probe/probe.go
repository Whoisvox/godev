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

func Probe(client *http.Client, target config.Target) Response {
	start := time.Now()

	resp, err := client.Get(target.URL)
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

func All(ctx context.Context, targets []config.Target, concurrency int) []Response {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	var wg sync.WaitGroup
	jobs := make(chan config.Target, len(targets))
	results := make(chan Response, len(targets))
	responses := []Response{}

	for _, target := range targets {
		jobs <- target
	}

	for w := 1; w <= concurrency; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for job := range jobs {
				results <- Probe(client, job)
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
