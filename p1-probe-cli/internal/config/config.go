package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Target struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

type Config struct {
	Targets []Target `yaml:"targets"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("fail fast")
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if len(cfg.Targets) == 0 {
		return nil, errors.New("config: no targets defined")
	}

	return &cfg, nil
}
