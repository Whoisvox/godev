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
	URL        string  `json:"url,omitempty"`
	StatusCode int     `json:"status_code,omitempty"`
	Latency    Latency `json:"latency,omitempty"`
	Err        string  `json:"err,omitempty"`
}

func Probe(url string) Response {
	start := time.Now()

	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Get(url)
	latency := time.Since(start)
	if err != nil {
		return Response{
			URL:        url,
			StatusCode: 0,
			Latency:    Latency(latency),
			Err:        err.Error(),
		}
	}
	defer resp.Body.Close()

	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	return Response{
		URL:        url,
		StatusCode: resp.StatusCode,
		Latency:    Latency(latency),
	}

}

func All(ctx context.Context, targets []config.Target, concurrency int) []Response {
	var wg sync.WaitGroup
	jobs := make(chan string, len(targets))
	responses := []Response{}

	for _, target := range targets {
		jobs <- target.URL
	}

	for w := 1; w <= concurrency; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for job := range jobs {
				responses = append(responses, Probe(job))
			}
		}(w)
	}

	close(jobs)
	wg.Wait()
	return responses
}

func (d Latency) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}
