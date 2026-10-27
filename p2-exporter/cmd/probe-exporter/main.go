package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Whoisvox/godev/probe-exporter/internal/config"
	"github.com/Whoisvox/godev/probe-exporter/internal/exporter"
	"github.com/Whoisvox/godev/probe-exporter/internal/probe"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var addr = flag.String("listen", ":9115", "metrics listen Addr(:Port)")
	var configFile = flag.String("config", "targets.yaml", "path of config file")
	var concurrency = flag.Int("concurrency", 3, "probe concurrency")
	var timeout = flag.Duration("timeout", 3*time.Second, "each request timeout(3s, 500ms)")
	flag.Parse()
	if *concurrency < 1 {
		slog.Error("concurrency invalid(> 0)", "concurrency", *concurrency)
		os.Exit(1)
	}
	opts := probe.Options{
		Client:      &http.Client{},
		Concurrency: *concurrency,
		Timeout:     *timeout,
	}

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	metrics := exporter.NewMetrics()

	go probeLoop(ctx, cfg, metrics, opts)

	// main goroutine start up HTTP server
	http.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{Addr: *addr}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			slog.Error("bind used", "err", err)
			os.Exit(1)
		}
	}()
	<-ctx.Done()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown failed", "err", err)
		slog.Error("force shutdown ...")
		os.Exit(1)
	}
}

func probeLoop(ctx context.Context, cfg *config.Config, m *exporter.Metrics, opts probe.Options) {
	responses := probe.ProbeOnce(ctx, cfg.Targets, opts)
	m.Update(responses)
	ticker := time.NewTicker(cfg.Internal)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			responses = probe.ProbeOnce(ctx, cfg.Targets, opts)
			m.Update(responses)
			slog.Info("results", "responses", responses)
		}
	}
}
