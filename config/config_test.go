package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DBPath != "./data/registry.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "./data/registry.db")
	}
	if cfg.DataPath != "./data/storage" {
		t.Errorf("DataPath = %q, want %q", cfg.DataPath, "./data/storage")
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want %q", cfg.Host, "0.0.0.0")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want %d", cfg.Port, 8080)
	}
	if cfg.AuthFile != "./data/.auth" {
		t.Errorf("AuthFile = %q, want %q", cfg.AuthFile, "./data/.auth")
	}
	if cfg.MaxSize != 512 {
		t.Errorf("MaxSize = %d, want %d", cfg.MaxSize, 512)
	}
}

func TestLoadFromEnv_Defaults(t *testing.T) {
	clearEnv(t)
	cfg := LoadFromEnv(nil)
	if cfg.DBPath != "./data/registry.db" {
		t.Errorf("DBPath = %q, want default", cfg.DBPath)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want default 8080", cfg.Port)
	}
}

func TestLoadFromEnv_Override(t *testing.T) {
	clearEnv(t)
	t.Setenv("REGISTRY_DB_PATH", "/tmp/test.db")
	t.Setenv("REGISTRY_DATA_PATH", "/tmp/storage")
	t.Setenv("REGISTRY_HOST", "127.0.0.1")
	t.Setenv("REGISTRY_PORT", "9000")
	t.Setenv("REGISTRY_AUTH_FILE", "/etc/.auth")
	t.Setenv("REGISTRY_MAX_SIZE", "1024")

	cfg := LoadFromEnv(nil)
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/tmp/test.db")
	}
	if cfg.DataPath != "/tmp/storage" {
		t.Errorf("DataPath = %q, want %q", cfg.DataPath, "/tmp/storage")
	}
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want %d", cfg.Port, 9000)
	}
	if cfg.AuthFile != "/etc/.auth" {
		t.Errorf("AuthFile = %q, want %q", cfg.AuthFile, "/etc/.auth")
	}
	if cfg.MaxSize != 1024 {
		t.Errorf("MaxSize = %d, want %d", cfg.MaxSize, 1024)
	}
}

func TestLoadFromEnv_InvalidPort(t *testing.T) {
	clearEnv(t)
	t.Setenv("REGISTRY_PORT", "not-a-number")
	cfg := LoadFromEnv(nil)
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want default 8080 for invalid input", cfg.Port)
	}
}

func TestLoadFromEnv_InvalidMaxSize(t *testing.T) {
	clearEnv(t)
	t.Setenv("REGISTRY_MAX_SIZE", "-1")
	cfg := LoadFromEnv(nil)
	if cfg.MaxSize != 512 {
		t.Errorf("MaxSize = %d, want default 512 for invalid input", cfg.MaxSize)
	}
}

func TestLoadFromEnv_ZeroPort(t *testing.T) {
	clearEnv(t)
	t.Setenv("REGISTRY_PORT", "0")
	cfg := LoadFromEnv(nil)
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want default 8080 for zero", cfg.Port)
	}
}

func TestAddr(t *testing.T) {
	cfg := &Config{Host: "127.0.0.1", Port: 9000}
	got := cfg.Addr()
	if got != "127.0.0.1:9000" {
		t.Errorf("Addr() = %q, want %q", got, "127.0.0.1:9000")
	}
}

func TestMaxSizeBytes(t *testing.T) {
	cfg := &Config{MaxSize: 512}
	got := cfg.MaxSizeBytes()
	want := int64(512 * 1024 * 1024)
	if got != want {
		t.Errorf("MaxSizeBytes() = %d, want %d", got, want)
	}
}

func TestLoadFromEnv_PartialOverride(t *testing.T) {
	clearEnv(t)
	t.Setenv("REGISTRY_HOST", "192.168.1.1")
	cfg := LoadFromEnv(nil)
	if cfg.Host != "192.168.1.1" {
		t.Errorf("Host = %q, want %q", cfg.Host, "192.168.1.1")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want default 8080", cfg.Port)
	}
	if cfg.DBPath != "./data/registry.db" {
		t.Errorf("DBPath = %q, want default", cfg.DBPath)
	}
}

func TestLoadFromEnv_CustomDefaults(t *testing.T) {
	clearEnv(t)
	defaults := &Config{
		DBPath:   "/custom/db.sqlite",
		DataPath: "/custom/storage",
		Host:     "10.0.0.1",
		Port:     3000,
		AuthFile: "/custom/.auth",
		MaxSize:  256,
	}
	cfg := LoadFromEnv(defaults)
	if cfg.DBPath != "/custom/db.sqlite" {
		t.Errorf("DBPath = %q, want custom default", cfg.DBPath)
	}
	if cfg.Port != 3000 {
		t.Errorf("Port = %d, want custom default 3000", cfg.Port)
	}
}

func clearEnv(t *testing.T) {
	t.Helper()
	envs := []string{
		"REGISTRY_DB_PATH", "REGISTRY_DATA_PATH", "REGISTRY_HOST",
		"REGISTRY_PORT", "REGISTRY_AUTH_FILE", "REGISTRY_MAX_SIZE",
	}
	for _, e := range envs {
		os.Unsetenv(e)
	}
}
