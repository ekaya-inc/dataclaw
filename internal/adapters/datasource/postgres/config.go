package postgres

import (
	"errors"
	"fmt"

	datasource "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

func FromMap(config map[string]any) (*Config, error) {
	cfg := &Config{
		Host:     datasource.StringValue(config["host"]),
		Port:     datasource.IntValue(config["port"], 5432),
		User:     datasource.StringValue(config["user"]),
		Password: datasource.StringValue(config["password"]),
		Database: datasource.StringValue(datasource.FirstNonNil(config["database"], config["name"])),
		SSLMode:  datasource.StringValue(config["ssl_mode"]),
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}
	if cfg.Host == "" || cfg.Database == "" {
		return nil, errors.New("postgres host and database are required")
	}
	if cfg.Port <= 0 {
		return nil, fmt.Errorf("postgres port must be positive")
	}
	return cfg, nil
}

func Fingerprint(config map[string]any) (string, error) {
	cfg, err := FromMap(config)
	if err != nil {
		return "", err
	}
	return datasource.CanonicalFingerprint(cfg)
}
