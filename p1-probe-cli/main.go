package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Response struct {
	URL        string        `json:"url,omitempty"`
	StatusCode int           `json:"status_code,omitempty"`
	Latency    time.Duration `json:"latency,omitempty"`
	Err        string        `json:"err,omitempty"`
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

	resq, err := http.Get(url)
	latency := time.Since(start)
	if err != nil {
		return Response{
			URL:        url,
			StatusCode: 0,
			Latency:    time.Duration(latency) * time.Millisecond,
			Err:        err.Error(),
		}
	}
	defer resq.Body.Close()

	io.Copy(io.Discard, io.LimitReader(resq.Body, 4096))

	return Response{
		URL:        url,
		StatusCode: resq.StatusCode,
		Latency:    time.Duration(latency) * time.Millisecond,
		Err:        "",
	}

}
