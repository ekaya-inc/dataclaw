package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

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

func TestStorePersistsSingleDatasourceAndQueries(t *testing.T) {
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

	if err := store.SaveOpenClawCredential(ctx, "encrypted-key", time.Now().UTC()); err != nil {
		t.Fatalf("save openclaw credential: %v", err)
	}
	cred, err := store.GetOpenClawCredential(ctx)
	if err != nil {
		t.Fatalf("get openclaw credential: %v", err)
	}
	if cred == nil || cred.APIKey != "encrypted-key" {
		t.Fatalf("unexpected credential: %#v", cred)
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
