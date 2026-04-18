package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/ekaya-inc/dataclaw/migrations"
	"github.com/ekaya-inc/dataclaw/pkg/models"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "dataclaw.sqlite"), migrations.FS)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	return store
}

func TestStorePersistsDatasourceAndQueries(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defer store.Close()

	ds := &Datasource{
		Name:     "Primary",
		Type:     "postgres",
		Provider: "postgres",
		Config: map[string]any{
			"host": "db.example.com",
			"port": 5432,
			"name": "analytics",
		},
	}
	if err := store.SaveDatasource(ctx, ds); err != nil {
		t.Fatalf("save datasource: %v", err)
	}

	loadedDS, err := store.GetDatasource(ctx)
	if err != nil {
		t.Fatalf("get datasource: %v", err)
	}
	if loadedDS == nil || loadedDS.Name != "Primary" || loadedDS.Type != "postgres" {
		t.Fatalf("unexpected datasource: %#v", loadedDS)
	}

	query := &ApprovedQuery{
		DatasourceID:          loadedDS.ID,
		NaturalLanguagePrompt: "Connectivity check",
		AdditionalContext:     "Probe for a live database connection.",
		SQLQuery:              "SELECT true AS connected",
		Parameters:            []models.QueryParameter{},
		OutputColumns:         []models.OutputColumn{{Name: "connected", Type: "boolean"}},
	}
	if err := store.CreateQuery(ctx, query); err != nil {
		t.Fatalf("create query: %v", err)
	}

	queries, err := store.ListQueries(ctx)
	if err != nil {
		t.Fatalf("list queries: %v", err)
	}
	if len(queries) != 1 || queries[0].NaturalLanguagePrompt != "Connectivity check" {
		t.Fatalf("unexpected queries: %#v", queries)
	}
	if len(queries[0].OutputColumns) != 1 || queries[0].OutputColumns[0].Name != "connected" {
		t.Fatalf("expected output columns to round-trip, got %#v", queries[0].OutputColumns)
	}

}

func TestStoreUpgradesLegacyAgentsTableWithManageApprovedQueriesColumn(t *testing.T) {
	ctx := context.Background()
	sqlitePath := filepath.Join(t.TempDir(), "dataclaw.sqlite")

	db, err := sql.Open("sqlite", sqlitePath)
	if err != nil {
		t.Fatalf("open legacy sqlite: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		);
		CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			api_key_encrypted TEXT NOT NULL,
			can_query INTEGER NOT NULL DEFAULT 0,
			can_execute INTEGER NOT NULL DEFAULT 0,
			approved_query_scope TEXT NOT NULL DEFAULT 'none' CHECK (approved_query_scope IN ('none', 'all', 'selected')),
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			last_used_at TEXT
		);
		INSERT INTO agents(id, name, api_key_encrypted, can_query, can_execute, approved_query_scope, created_at, updated_at)
		VALUES('agent_1', 'Legacy agent', 'encrypted', 0, 0, 'none', '2026-04-18T00:00:00Z', '2026-04-18T00:00:00Z');
		INSERT INTO schema_migrations(version, applied_at) VALUES(1, '2026-04-18T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy sqlite: %v", err)
	}

	store, err := Open(ctx, sqlitePath, migrations.FS)
	if err != nil {
		t.Fatalf("Open(upgrade): %v", err)
	}
	defer store.Close()

	hasManagerColumn, err := tableHasColumn(ctx, store.DB(), "agents", "can_manage_approved_queries")
	if err != nil {
		t.Fatalf("query agents table columns after upgrade: %v", err)
	}
	if !hasManagerColumn {
		t.Fatal("expected upgraded schema to include can_manage_approved_queries column")
	}

	var managerFlag int
	if err := store.DB().QueryRowContext(ctx, `SELECT can_manage_approved_queries FROM agents WHERE id = 'agent_1'`).Scan(&managerFlag); err != nil {
		t.Fatalf("query upgraded agent manager flag: %v", err)
	}
	if managerFlag != 0 {
		t.Fatalf("expected upgraded legacy rows to default manager flag to 0, got %d", managerFlag)
	}

	var applied int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = 2`).Scan(&applied); err != nil {
		t.Fatalf("query schema_migrations version 2: %v", err)
	}
	if applied != 1 {
		t.Fatalf("expected migration version 2 to be recorded, got count=%d", applied)
	}
}

func TestSaveDatasourceUpdatePreservesApprovedQueries(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defer store.Close()

	ds := &Datasource{
		Name:     "Primary",
		Type:     "postgres",
		Provider: "postgres",
		Config: map[string]any{
			"host": "db.example.com",
			"port": 5432,
			"name": "analytics",
		},
	}
	if err := store.SaveDatasource(ctx, ds); err != nil {
		t.Fatalf("save datasource: %v", err)
	}

	if err := store.CreateQuery(ctx, &ApprovedQuery{
		DatasourceID:          ds.ID,
		NaturalLanguagePrompt: "Connectivity check",
		SQLQuery:              "SELECT true AS connected",
	}); err != nil {
		t.Fatalf("create query: %v", err)
	}

	ds.Name = "Primary warehouse"
	if err := store.SaveDatasource(ctx, ds); err != nil {
		t.Fatalf("rename datasource: %v", err)
	}

	loaded, err := store.GetDatasource(ctx)
	if err != nil {
		t.Fatalf("get datasource: %v", err)
	}
	if loaded == nil || loaded.Name != "Primary warehouse" {
		t.Fatalf("unexpected datasource after rename: %#v", loaded)
	}

	queries, err := store.ListQueries(ctx)
	if err != nil {
		t.Fatalf("list queries: %v", err)
	}
	if len(queries) != 1 {
		t.Fatalf("expected query to survive datasource rename, got %#v", queries)
	}
}

func tableHasColumn(ctx context.Context, db *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+tableName+`)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			typeName   string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultVal, &pk); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}
	return false, rows.Err()
}
