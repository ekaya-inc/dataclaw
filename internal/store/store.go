package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/ekaya-inc/dataclaw/pkg/models"
)

type Datasource struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Type            string         `json:"type"`
	Provider        string         `json:"provider,omitempty"`
	Config          map[string]any `json:"config,omitempty"`
	ConfigEncrypted string         `json:"-"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type ApprovedQuery struct {
	ID                    string                  `json:"id"`
	DatasourceID          string                  `json:"datasource_id"`
	NaturalLanguagePrompt string                  `json:"natural_language_prompt"`
	AdditionalContext     string                  `json:"additional_context"`
	SQLQuery              string                  `json:"sql_query"`
	AllowsModification    bool                    `json:"allows_modification"`
	Parameters            []models.QueryParameter `json:"parameters"`
	OutputColumns         []models.OutputColumn   `json:"output_columns"`
	Constraints           string                  `json:"constraints"`
	CreatedAt             time.Time               `json:"created_at"`
	UpdatedAt             time.Time               `json:"updated_at"`
}

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, sqlitePath string, migrationFS embed.FS) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir sqlite dir: %w", err)
	}
	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON;`); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	store := &Store{db: db}
	if err := store.runMigrations(ctx, migrationFS); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) DB() *sql.DB  { return s.db }

func (s *Store) runMigrations(ctx context.Context, migrationFS embed.FS) error {
	entries, err := fs.ReadDir(migrationFS, ".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		version, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
		if err != nil {
			return fmt.Errorf("parse migration version %s: %w", name, err)
		}
		var count int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&count); err != nil {
			return err
		}
		applied := 0
		if count > 0 {
			if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, version).Scan(&applied); err != nil {
				return err
			}
		}
		if applied > 0 {
			continue
		}
		content, err := migrationFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO schema_migrations(version, applied_at) VALUES(?, ?)`, version, time.Now().UTC().Format(time.RFC3339)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) GetDatasource(ctx context.Context) (*Datasource, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, type, provider, config_encrypted, created_at, updated_at FROM datasources ORDER BY created_at DESC LIMIT 1`)
	var ds Datasource
	var configRaw string
	var createdAt, updatedAt string
	if err := row.Scan(&ds.ID, &ds.Name, &ds.Type, &ds.Provider, &configRaw, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	ds.ConfigEncrypted = configRaw
	var config map[string]any
	if err := json.Unmarshal([]byte(configRaw), &config); err == nil {
		ds.Config = config
	}
	ds.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	ds.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &ds, nil
}

func (s *Store) SaveDatasource(ctx context.Context, ds *Datasource) error {
	if ds.ID == "" {
		ds.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if ds.CreatedAt.IsZero() {
		ds.CreatedAt = now
	}
	ds.UpdatedAt = now
	configRaw := ds.ConfigEncrypted
	if configRaw == "" {
		cfg, err := json.Marshal(ds.Config)
		if err != nil {
			return err
		}
		configRaw = string(cfg)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO datasources(id, name, type, provider, config_encrypted, created_at, updated_at)
		VALUES(?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			type = excluded.type,
			provider = excluded.provider,
			config_encrypted = excluded.config_encrypted,
			updated_at = excluded.updated_at
	`, ds.ID, ds.Name, ds.Type, ds.Provider, configRaw, ds.CreatedAt.Format(time.RFC3339), ds.UpdatedAt.Format(time.RFC3339))
	return err
}

func (s *Store) DeleteDatasource(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM datasources`)
	return err
}

const approvedQueryColumns = `id, datasource_id, natural_language_prompt, additional_context, sql_query, allows_modification, parameters_json, output_columns_json, constraints, created_at, updated_at`

func (s *Store) ListQueries(ctx context.Context) ([]*ApprovedQuery, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+approvedQueryColumns+` FROM approved_queries ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var queries []*ApprovedQuery
	for rows.Next() {
		q, err := scanQuery(rows)
		if err != nil {
			return nil, err
		}
		queries = append(queries, q)
	}
	return queries, rows.Err()
}

func (s *Store) GetQuery(ctx context.Context, id string) (*ApprovedQuery, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+approvedQueryColumns+` FROM approved_queries WHERE id = ?`, id)
	q, err := scanQuery(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return q, err
}

func (s *Store) CreateQuery(ctx context.Context, q *ApprovedQuery) error {
	if q.ID == "" {
		q.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	q.CreatedAt = now
	q.UpdatedAt = now
	return s.upsertQuery(ctx, q, true)
}

func (s *Store) UpdateQuery(ctx context.Context, q *ApprovedQuery) error {
	q.UpdatedAt = time.Now().UTC()
	return s.upsertQuery(ctx, q, false)
}

func (s *Store) upsertQuery(ctx context.Context, q *ApprovedQuery, create bool) error {
	params, err := json.Marshal(q.Parameters)
	if err != nil {
		return err
	}
	if q.OutputColumns == nil {
		q.OutputColumns = []models.OutputColumn{}
	}
	outputs, err := json.Marshal(q.OutputColumns)
	if err != nil {
		return err
	}
	if create {
		_, err = s.db.ExecContext(ctx, `INSERT INTO approved_queries(id, datasource_id, natural_language_prompt, additional_context, sql_query, allows_modification, parameters_json, output_columns_json, constraints, created_at, updated_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			q.ID, q.DatasourceID, q.NaturalLanguagePrompt, q.AdditionalContext, q.SQLQuery, boolToInt(q.AllowsModification), string(params), string(outputs), q.Constraints, q.CreatedAt.Format(time.RFC3339), q.UpdatedAt.Format(time.RFC3339))
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE approved_queries SET datasource_id = ?, natural_language_prompt = ?, additional_context = ?, sql_query = ?, allows_modification = ?, parameters_json = ?, output_columns_json = ?, constraints = ?, updated_at = ? WHERE id = ?`,
		q.DatasourceID, q.NaturalLanguagePrompt, q.AdditionalContext, q.SQLQuery, boolToInt(q.AllowsModification), string(params), string(outputs), q.Constraints, q.UpdatedAt.Format(time.RFC3339), q.ID)
	return err
}

func (s *Store) DeleteQuery(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM approved_queries WHERE id = ?`, id)
	return err
}

func scanQuery(scanner interface{ Scan(dest ...any) error }) (*ApprovedQuery, error) {
	var q ApprovedQuery
	var paramsRaw, outputsRaw, createdAt, updatedAt string
	var allowsModification int
	if err := scanner.Scan(&q.ID, &q.DatasourceID, &q.NaturalLanguagePrompt, &q.AdditionalContext, &q.SQLQuery, &allowsModification, &paramsRaw, &outputsRaw, &q.Constraints, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(paramsRaw), &q.Parameters)
	if q.Parameters == nil {
		q.Parameters = []models.QueryParameter{}
	}
	_ = json.Unmarshal([]byte(outputsRaw), &q.OutputColumns)
	if q.OutputColumns == nil {
		q.OutputColumns = []models.OutputColumn{}
	}
	q.AllowsModification = allowsModification == 1
	q.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	q.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &q, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
