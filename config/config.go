// Package config provides configuration loading for the Yao Registry server.
// Configuration can be set via environment variables or CLI flags, with flags
// taking precedence over environment variables.
package config

import (
	"os"
	"strconv"
)

// Config holds all configurable parameters for the registry server.
type Config struct {
	DBPath   string // SQLite database file path
	DataPath string // Package file storage root directory
	Host     string // Listen IP address
	Port     int    // Listen port
	AuthFile string // Authentication file path for push credentials
	MaxSize  int64  // Maximum package size in MB
}

// DefaultConfig returns a Config populated with default values.
func DefaultConfig() *Config {
	return &Config{
		DBPath:   "./data/registry.db",
		DataPath: "./data/storage",
		Host:     "0.0.0.0",
		Port:     8080,
		AuthFile: "./data/.auth",
		MaxSize:  512,
	}
}

// LoadFromEnv loads configuration from REGISTRY_* environment variables,
// falling back to the provided defaults for any unset variables.
func LoadFromEnv(defaults *Config) *Config {
	if defaults == nil {
		defaults = DefaultConfig()
	}
	cfg := *defaults

	if v := os.Getenv("REGISTRY_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("REGISTRY_DATA_PATH"); v != "" {
		cfg.DataPath = v
	}
	if v := os.Getenv("REGISTRY_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("REGISTRY_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cfg.Port = port
		}
	}
	if v := os.Getenv("REGISTRY_AUTH_FILE"); v != "" {
		cfg.AuthFile = v
	}
	if v := os.Getenv("REGISTRY_MAX_SIZE"); v != "" {
		if size, err := strconv.ParseInt(v, 10, 64); err == nil && size > 0 {
			cfg.MaxSize = size
		}
	}
	return &cfg
}

// Addr returns the listen address in "host:port" format.
func (c *Config) Addr() string {
	return c.Host + ":" + strconv.Itoa(c.Port)
}

// MaxSizeBytes returns the maximum package size in bytes.
func (c *Config) MaxSizeBytes() int64 {
	return c.MaxSize * 1024 * 1024
}
