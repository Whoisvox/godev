package config

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Whoisvox/godev/probe-exporter/internal/probe"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Internal time.Duration  `yaml:"internal"`
	Targets  []probe.Target `yaml:"targets"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	slog.Info("config loaded",
		"internal", cfg.Internal,
		"targets", cfg.Targets,
	)
	return &cfg, nil
}
