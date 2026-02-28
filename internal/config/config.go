// Package config handles loading and parsing of the application configuration file
package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Frontend Frontend `json:"frontend"`
	Backend  Backend  `json:"backend"`
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

func LoadConfig(path string) (config *Config, err error) {
	jsonFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer func() {
		closeErr := jsonFile.Close()
		if err == nil {
			err = closeErr
		}
	}()

	config = &Config{}
	decoder := json.NewDecoder(jsonFile)

	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	return config, nil
}
