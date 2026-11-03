package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Whoisvox/godev/probe-exporter/internal/probe"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Interval time.Duration  `yaml:"interval"`
	Targets  []probe.Target `yaml:"targets"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.UnmarshalStrict(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Interval <= 0 {
		return nil, fmt.Errorf("config: interval must be > 0, got %v", cfg.Interval)
	}
	if len(cfg.Targets) == 0 {
		return nil, errors.New("config: no targets defined")
	}
	for i, t := range cfg.Targets {
		if t.Name == "" || t.URL == "" {
			return nil, fmt.Errorf("config: targets[%d]: name and url required", i)
		}
	}

	slog.Info("config loaded",
		"interval", cfg.Interval,
		"targets", cfg.Targets,
	)
	return &cfg, nil
}
