package config

import (
	"errors"
	"fmt"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

type Defaults struct {
	Remote         string `toml:"remote"`
	PollingActive  int    `toml:"polling_active"`
	PollingIdle    int    `toml:"polling_idle"`
	ConfirmActions bool   `toml:"confirm_actions"`
	PipelineLimit  int    `toml:"pipeline_limit"`
}

type ProviderConfig struct {
	Type     string `toml:"type"`
	Host     string `toml:"host"`
	Token    string `toml:"token"`
	Username string `toml:"username"`
}

type Config struct {
	Defaults  Defaults         `toml:"defaults"`
	Providers []ProviderConfig `toml:"providers"`
}

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		Defaults: Defaults{
			PollingActive:  5,
			PollingIdle:    60,
			ConfirmActions: true,
			PipelineLimit:  25,
		},
	}
}

// LoadFromPath reads a TOML config file. If the file does not exist, returns defaults.
func LoadFromPath(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading config: %w", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	// Apply defaults for zero values
	if cfg.Defaults.PollingActive == 0 {
		cfg.Defaults.PollingActive = 5
	}
	if cfg.Defaults.PollingIdle == 0 {
		cfg.Defaults.PollingIdle = 60
	}
	if cfg.Defaults.PipelineLimit == 0 {
		cfg.Defaults.PipelineLimit = 25
	}

	if err := cfg.validate(); err != nil {
		return cfg, err
	}

	return cfg, nil
}

// Load reads the config from the default path (~/.config/manifold/config.toml).
func Load() (Config, error) {
	home, err := os.UserConfigDir()
	if err != nil {
		return Default(), nil
	}
	return LoadFromPath(home + "/manifold/config.toml")
}

func (c Config) validate() error {
	seen := make(map[string]bool)
	for _, p := range c.Providers {
		host := p.Host
		if host == "" {
			switch p.Type {
			case "github":
				host = "github.com"
			case "gitlab":
				host = "gitlab.com"
			case "bitbucket":
				host = "bitbucket.org"
			}
		}
		key := p.Type + ":" + host
		if seen[key] {
			return fmt.Errorf("duplicate provider config for %s at %s", p.Type, host)
		}
		seen[key] = true
	}
	return nil
}

// FindProvider looks up a provider config by type and host.
func (c Config) FindProvider(providerType, host string) (ProviderConfig, bool) {
	for _, p := range c.Providers {
		pHost := p.Host
		if pHost == "" {
			switch p.Type {
			case "github":
				pHost = "github.com"
			case "gitlab":
				pHost = "gitlab.com"
			case "bitbucket":
				pHost = "bitbucket.org"
			}
		}
		if p.Type == providerType && pHost == host {
			return p, true
		}
	}
	return ProviderConfig{}, false
}
