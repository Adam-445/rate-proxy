package config

import (
	"strings"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		checkConfig func(*testing.T, *Config)
	}{
		{
			name: "Valid config with all defaults",
			input: `{
				"frontend": {
					"port": ":8080",
					"rate_limit": {"capacity": 10, "rate": 1}
				},
				"backend": {
					"servers": [{"address": "localhost:8081"}]
				}
			}`,
			wantErr: false,
			checkConfig: func(t *testing.T, c *Config) {
				if c.Backend.Algorithm != "round-robin" {
					t.Errorf("expected default algorithm 'round-robin', got %s", c.Backend.Algorithm)
				}
				if c.Storage.Type != "memory" {
					t.Errorf("expected default storage 'memory', got %s", c.Storage.Type)
				}
				if c.Backend.HealthCheck.IntervalSeconds != 5 {
					t.Errorf("expected default interval 5, got %d", c.Backend.HealthCheck.IntervalSeconds)
				}
			},
		},
		{
			name: "Valid Redis config defaults",
			input: `{
				"storage": {"type": "redis"},
				"frontend": {"port": ":8080", "rate_limit": {"capacity": 10, "rate": 1}},
				"backend": {"servers": [{"address": "localhost:8081"}]}
			}`,
			wantErr: false,
			checkConfig: func(t *testing.T, c *Config) {
				if c.Storage.Redis.Address != "localhost:6379" {
					t.Errorf("expected default redis address, got %s", c.Storage.Redis.Address)
				}
			},
		},
		{
			name:    "Invalid JSON",
			input:   `{ "frontend": { "port": :8080 } }`, // Missing quotes
			wantErr: true,
		},
		{
			name: "Validation error: No servers",
			input: `{
				"frontend": {"port": ":8080", "rate_limit": {"capacity": 10, "rate": 1}},
				"backend": {"servers": []}
			}`,
			wantErr: true,
		},
		{
			name: "Validation error: Zero rate limit",
			input: `{
				"frontend": {"port": ":8080", "rate_limit": {"capacity": 0, "rate": 1}},
				"backend": {"servers": [{"address": "localhost:8081"}]}
			}`,
			wantErr: true,
		},
		{
			name: "Validation error: Unknown algorithm",
			input: `{
				"frontend": {"port": ":8080", "rate_limit": {"capacity": 10, "rate": 1}},
				"backend": {
					"algorithm": "chaos-theory",
					"servers": [{"address": "localhost:8081"}]
				}
			}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			cfg, err := LoadConfig(r)

			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && tt.checkConfig != nil {
				tt.checkConfig(t, cfg)
			}
		})
	}
}
