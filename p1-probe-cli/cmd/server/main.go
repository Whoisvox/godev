package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
)

var n int32

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		c := atomic.AddInt32(&n, 1)
		fmt.Printf("server got request #%d\n", c)
		if c <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})
	err := http.ListenAndServe("127.0.0.1:18081", nil)
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			slog.Info("server is shutting down, waitting for handling the rest jobs...")
			return
		}
		slog.Error("server failed", "err", err)
	}
}
