package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Whoisvox/godev/shortlink/internal/config"
	"github.com/Whoisvox/godev/shortlink/internal/server"
	"github.com/Whoisvox/godev/shortlink/internal/store"
)

func main() {
	// 1. 加载配置
	cfg := config.Load()

	// 2. 初始化存储
	st, err := store.NewMySQLStore(cfg.DSN)
	if err != nil {
		slog.Error("open db", "err", err)
		os.Exit(1)
	}
	defer st.Db.Close()

	if err := st.Ping(); err != nil {
		slog.Error("db ping", "err", err)
		os.Exit(1)
	}

	// 3. 创建 server
	srv := server.New(st, cfg.BaseURL)

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: time.Duration(cfg.ReadHeaderTimeout) * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// 4. 优雅退出
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("server starting", "addr", cfg.Addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpSrv.Shutdown(shutdownCtx)
	slog.Info("bye")
}
