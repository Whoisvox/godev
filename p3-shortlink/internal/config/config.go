package config

import (
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DSN               string
	Addr              string
	ReadHeaderTimeout int
	BaseURL           string
}

func Load() *Config {
	cfg := &Config{
		DSN:               getEnv("DB_DSN", ""),
		Addr:              getEnv("ADDR", ":8080"),
		ReadHeaderTimeout: getIntEnv("READ_HEADER_TIMEOUT", 5),
		BaseURL:           getEnv("BASE_URL", "http://localhost:8080"),
	}

	// ---- validate attributes ----
	if cfg.DSN == "" {
		slog.Error("DB_DSN must not be empty")
		os.Exit(1)
	}
	if cfg.ReadHeaderTimeout <= 0 {
		slog.Error("READ_HEADER_TIMEOUT must be > 0")
		os.Exit(1)
	}
	if !strings.Contains(cfg.Addr, ":") {
		slog.Error("ADDR must be in host:port format", "got", cfg.Addr)
		os.Exit(1)
	}

	_, portStr, _ := net.SplitHostPort(cfg.Addr)
	if _, err := strconv.Atoi(portStr); err != nil {
		slog.Error("ADDR port must be numeric", "got", portStr)
		os.Exit(1)
	}

	return cfg

}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		i, err := strconv.Atoi(v)
		if err != nil {
			slog.Error("invalid int env", "key", key, "val", v)
			os.Exit(1)
		}
		return i
	}
	return fallback
}
