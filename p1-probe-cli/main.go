package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Latency time.Duration

type Response struct {
	URL        string  `json:"url,omitempty"`
	StatusCode int     `json:"status_code,omitempty"`
	Latency    Latency `json:"latency,omitempty"`
	Err        string  `json:"err,omitempty"`
}

func main() {
	urls := []string{
		"https://httpbin.org/get",
		"https://jsonplaceholder.typicode.com/posts/1",
		"https://youcantfindme.com",
		"https://go.dev/doc/",
		"https://httpbin.org/delay/2",
	}

	var respSet []Response

	for _, url := range urls {
		respSet = append(respSet, probe(url))
	}

	data, err := json.Marshal(respSet)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(data))
}

func probe(url string) Response {
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

func (d Latency) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}
