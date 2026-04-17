package mssql

import (
	"errors"
	"fmt"

	datasource "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
)

type Config struct {
	Host                   string
	Port                   int
	Username               string
	Password               string
	Database               string
	Encrypt                bool
	TrustServerCertificate bool
	ConnectionTimeout      int
}

func FromMap(config map[string]any) (*Config, error) {
	cfg := &Config{
		Host:                   datasource.StringValue(config["host"]),
		Port:                   datasource.IntValue(config["port"], 1433),
		Username:               datasource.StringValue(datasource.FirstNonNil(config["username"], config["user"])),
		Password:               datasource.StringValue(config["password"]),
		Database:               datasource.StringValue(datasource.FirstNonNil(config["database"], config["name"])),
		Encrypt:                datasource.BoolValue(config["encrypt"], false),
		TrustServerCertificate: datasource.BoolValue(config["trust_server_certificate"], false),
		ConnectionTimeout:      datasource.IntValue(config["connection_timeout"], 0),
	}
	if cfg.Host == "" || cfg.Database == "" || cfg.Username == "" {
		return nil, errors.New("sql server host, database, and user are required")
	}
	if cfg.Port <= 0 {
		return nil, fmt.Errorf("sql server port must be positive")
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
