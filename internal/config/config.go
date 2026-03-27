// Package config defines the shape of the rate-proxy configuration file.
//
// It deliberately contains no io. Loading is handeled by configr.
package config

import (
	"fmt"
)

type Config struct {
	Frontend Frontend `json:"frontend"`
	Backend  Backend  `json:"backend"`
	Storage  Storage  `json:"storage"`
}

type Frontend struct {
	Port      string    `json:"port"`
	RateLimit RateLimit `json:"rate_limit"`
}

type RateLimit struct {
	Capacity int `json:"capacity"`
	Rate     int `json:"rate"`
}

type Backend struct {
	Algorithm   string      `json:"algorithm"`
	HealthCheck HealthCheck `json:"health_check"`
	Servers     []Server    `json:"servers"`
}

type HealthCheck struct {
	IntervalSeconds int `json:"interval_seconds"`
	TimeoutSeconds  int `json:"timeout_seconds"`
	MaxRetries      int `json:"max_retries"`
}

type Server struct {
	Address string `json:"address"`
}

type Storage struct {
	Type  string `json:"type"`
	Redis Redis  `json:"redis"`
}

type Redis struct {
	Address  string `json:"address"`
	Password string `json:"password"`
	DB       int    `json:"db"`
}

// ApplyDefaults fills in zero-value fields with sensible defaults.
// It is intended to be passed to configr.WithDefaults
func ApplyDefaults(c *Config) {
	if c.Backend.Algorithm == "" {
		c.Backend.Algorithm = "round-robin"
	}
	if c.Storage.Type == "" {
		c.Storage.Type = "memory"
	}
	if c.Storage.Type == "redis" {
		if c.Storage.Redis.Address == "" {
			c.Storage.Redis.Address = "localhost:6379"
		}
	}
	if c.Backend.HealthCheck.IntervalSeconds == 0 {
		c.Backend.HealthCheck.IntervalSeconds = 5
	}
	if c.Backend.HealthCheck.TimeoutSeconds == 0 {
		c.Backend.HealthCheck.TimeoutSeconds = 2
	}
	if c.Backend.HealthCheck.MaxRetries == 0 {
		c.Backend.HealthCheck.MaxRetries = 3
	}
}

// Validate rejects configs that would cause errors / break the server.
// It is intended to be passed to configr.WithValidate
func Validate(c Config) error {
	if len(c.Backend.Servers) == 0 {
		return fmt.Errorf("backend.servers cannot be empty")
	}
	for _, svr := range c.Backend.Servers {
		if svr.Address == "" {
			return fmt.Errorf("no backend.servers.*.address can be empty")
		}
	}
	if c.Frontend.Port == "" {
		return fmt.Errorf("frontend.port cannot be empty")
	}
	if c.Frontend.RateLimit.Capacity <= 0 {
		return fmt.Errorf("frontend.rate_limit.capacity must be greater than 0")
	}
	if c.Frontend.RateLimit.Rate <= 0 {
		return fmt.Errorf("frontend.rate_limit.rate must be greater than 0")
	}
	validAlgorithms := map[string]bool{"round-robin": true, "random-selection": true}
	if !validAlgorithms[c.Backend.Algorithm] {
		return fmt.Errorf("unknown algorithm %q", c.Backend.Algorithm)
	}
	validStorages := map[string]bool{"redis": true, "memory": true}
	if !validStorages[c.Storage.Type] {
		return fmt.Errorf("unknown storage type %q", c.Storage.Type)
	}
	return nil
}
