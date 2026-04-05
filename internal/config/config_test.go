package config

import (
	"testing"
)

// TestApplyDefaults verifies that zero-value fields are filled with the
// expected values and that non-zero fields are not overwritten
func TestApplyDefaults(t *testing.T) {
	t.Run("fills all zero-value fields", func(t *testing.T) {
		c := &Config{
			Frontend: Frontend{Port: ":8080", RateLimit: RateLimit{Capacity: 10, Rate: 1}},
			Backend:  Backend{Servers: []Server{{Address: "localhost:8081"}}},
		}
		ApplyDefaults(c)

		if c.Backend.Algorithm != "round-robin" {
			t.Errorf("expected default algorithm %q, got %q", "round-robin", c.Backend.Algorithm)
		}
		if c.Storage.Type != "memory" {
			t.Errorf("expected default storage %q, got %q", "memory", c.Storage.Type)
		}
		if c.Backend.HealthCheck.IntervalSeconds != 5 {
			t.Errorf("expected default interval 5, got %d", c.Backend.HealthCheck.IntervalSeconds)
		}
		if c.Backend.HealthCheck.TimeoutSeconds != 2 {
			t.Errorf("expected default timeout 2, got %d", c.Backend.HealthCheck.TimeoutSeconds)
		}
		if c.Backend.HealthCheck.MaxRetries != 3 {
			t.Errorf("expected default max_retries 3, got %d", c.Backend.HealthCheck.MaxRetries)
		}
	})

	t.Run("does not overwrite explicitly set values", func(t *testing.T) {
		c := &Config{
			Backend: Backend{
				Algorithm: "random-selection",
				HealthCheck: HealthCheck{
					IntervalSeconds: 10,
					TimeoutSeconds:  5,
					MaxRetries:      1,
				},
			},
			Storage: Storage{Type: "redis"},
		}
		ApplyDefaults(c)

		if c.Backend.Algorithm != "random-selection" {
			t.Errorf("ApplyDefaults overwrote explicit algorithm")
		}
		if c.Backend.HealthCheck.IntervalSeconds != 10 {
			t.Errorf("ApplyDefaults overwrote explicit interval")
		}
	})

	t.Run("fills redis address default when storage is redis", func(t *testing.T) {
		c := &Config{Storage: Storage{Type: "redis"}}
		ApplyDefaults(c)

		if c.Storage.Redis.Address != "localhost:6379" {
			t.Errorf("expected default redis address, got %q", c.Storage.Redis.Address)
		}
	})

	t.Run("does not set redis address default when storage is not redis", func(t *testing.T) {
		c := &Config{Storage: Storage{Type: "memory"}}
		ApplyDefaults(c)

		if c.Storage.Redis.Address != "" {
			t.Errorf("expected empty redis address for memory storage, got %q", c.Storage.Redis.Address)
		}
	})
}

// TestValidate verifies that valid configs pass and that each invalid
// condition is caught individually
func TestValidate(t *testing.T) {
	// base is a minimal, valid config used as the starting point
	base := Config{
		Frontend: Frontend{Port: ":8080", RateLimit: RateLimit{Capacity: 10, Rate: 1}},
		Backend: Backend{
			Algorithm: "round-robin",
			Servers:   []Server{{Address: "localhost:8081"}},
		},
		Storage: Storage{Type: "memory"},
	}

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid config passes",
			mutate:  func(c *Config) {},
			wantErr: false,
		},
		{
			name:    "no servers",
			mutate:  func(c *Config) { c.Backend.Servers = nil },
			wantErr: true,
		},
		{
			name:    "server with empty address",
			mutate:  func(c *Config) { c.Backend.Servers = []Server{{Address: ""}} },
			wantErr: true,
		},
		{
			name:    "empty port",
			mutate:  func(c *Config) { c.Frontend.Port = "" },
			wantErr: true,
		},
		{
			name:    "zero capacity",
			mutate:  func(c *Config) { c.Frontend.RateLimit.Capacity = 0 },
			wantErr: true,
		},
		{
			name:    "negative capacity",
			mutate:  func(c *Config) { c.Frontend.RateLimit.Capacity = -1 },
			wantErr: true,
		},
		{
			name:    "zero rate",
			mutate:  func(c *Config) { c.Frontend.RateLimit.Rate = 0 },
			wantErr: true,
		},
		{
			name:    "unknown algorithm",
			mutate:  func(c *Config) { c.Backend.Algorithm = "chaos-theory" },
			wantErr: true,
		},
		{
			name:    "unknown storage type",
			mutate:  func(c *Config) { c.Storage.Type = "cassandra" },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base // copy so mutations don't bleed between subtests
			tt.mutate(&c)
			err := Validate(c)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
