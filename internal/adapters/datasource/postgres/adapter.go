package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
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
	if err := a.db.QueryRowContext(ctx, "SELECT current_database()").Scan(&current); err != nil {
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
	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db, nil
}

func buildConnectionString(cfg *Config) string {
	return fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		url.QueryEscape(cfg.User),
		url.QueryEscape(cfg.Password),
		cfg.Host,
		cfg.Port,
		url.QueryEscape(cfg.Database),
		url.QueryEscape(cfg.SSLMode),
	)
}
