package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	_ "github.com/microsoft/go-mssqldb"
)

type Adapter struct {
	config *Config
	db     *sql.DB
}

func NewAdapter(ctx context.Context, config map[string]any) (*Adapter, error) {
	cfg, err := FromMap(config)
	if err != nil {
		return nil, err
	}
	db, err := openDB(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Adapter{config: cfg, db: db}, nil
}

func (a *Adapter) TestConnection(ctx context.Context) error {
	var current string
	if err := a.db.QueryRowContext(ctx, "SELECT DB_NAME()").Scan(&current); err != nil {
		return err
	}
	if !strings.EqualFold(current, a.config.Database) {
		return fmt.Errorf("connected to wrong database: expected %q but got %q", a.config.Database, current)
	}
	return nil
}

func (a *Adapter) Close() error {
	if a == nil || a.db == nil {
		return nil
	}
	return a.db.Close()
}

func openDB(ctx context.Context, cfg *Config) (*sql.DB, error) {
	connStr := buildConnectionString(cfg)
	db, err := sql.Open("sqlserver", connStr)
	if err != nil {
		return nil, fmt.Errorf("open sqlserver: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlserver: %w", err)
	}
	return db, nil
}

func buildConnectionString(cfg *Config) string {
	query := url.Values{}
	query.Set("database", cfg.Database)
	query.Set("encrypt", strconv.FormatBool(cfg.Encrypt))
	if cfg.TrustServerCertificate {
		query.Set("TrustServerCertificate", "true")
	}
	if cfg.ConnectionTimeout > 0 {
		query.Set("connection timeout", fmt.Sprintf("%d", cfg.ConnectionTimeout))
	}
	return fmt.Sprintf(
		"sqlserver://%s:%s@%s:%d?%s",
		url.QueryEscape(cfg.Username),
		url.QueryEscape(cfg.Password),
		cfg.Host,
		cfg.Port,
		query.Encode(),
	)
}
