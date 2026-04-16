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
		DatasourceID: loadedDS.ID,
		Name:         "Connectivity",
		Description:  "Connectivity check",
		SQLQuery:     "SELECT true AS connected",
		Parameters:   []models.QueryParameter{},
		IsEnabled:    true,
	}
	if err := store.CreateQuery(ctx, query); err != nil {
		t.Fatalf("create query: %v", err)
	}

	queries, err := store.ListQueries(ctx)
	if err != nil {
		t.Fatalf("list queries: %v", err)
	}
	if len(queries) != 1 || queries[0].Name != "Connectivity" {
		t.Fatalf("unexpected queries: %#v", queries)
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
