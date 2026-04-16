package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	DefaultBindAddr = "127.0.0.1"
	DefaultPort     = 18790
)

type Config struct {
	BindAddr   string
	Port       int
	DataDir    string
	SQLitePath string
	SecretPath string
	Version    string
}

func Load(version string) (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}

	dataDir := envOrDefault("DATACLAW_DATA_DIR", filepath.Join(home, ".dataclaw"))
	port, err := intEnvOrDefault("DATACLAW_PORT", DefaultPort)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		BindAddr:   normalizeBindAddr(envOrDefault("DATACLAW_BIND_ADDR", DefaultBindAddr)),
		Port:       port,
		DataDir:    dataDir,
		SQLitePath: envOrDefault("DATACLAW_DB_PATH", filepath.Join(dataDir, "dataclaw.sqlite")),
		SecretPath: envOrDefault("DATACLAW_SECRET_PATH", filepath.Join(dataDir, "secret.key")),
		Version:    version,
	}
	return cfg, nil
}

func normalizeBindAddr(bindAddr string) string {
	bindAddr = strings.TrimSpace(bindAddr)
	switch bindAddr {
	case "", "127.0.0.1", "localhost":
		return DefaultBindAddr
	default:
		return DefaultBindAddr
	}
}

func (c *Config) BaseURL(port int) string {
	return (&url.URL{Scheme: "http", Host: fmt.Sprintf("%s:%d", c.BindAddr, port)}).String()
}

func (c *Config) UIBaseURL(port int) string {
	host := c.BindAddr
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return (&url.URL{Scheme: "http", Host: fmt.Sprintf("%s:%d", host, port)}).String()
}

func envOrDefault(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}

func intEnvOrDefault(key string, fallback int) (int, error) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}
