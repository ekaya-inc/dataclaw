package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ekaya-inc/dataclaw/internal/security"
	"github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/migrations"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "dataclaw.sqlite"), migrations.FS)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	secret, err := security.LoadOrCreateSecret(filepath.Join(t.TempDir(), "secret.key"))
	if err != nil {
		t.Fatalf("load secret: %v", err)
	}
	return New(st, secret, "test", func() string { return "http://127.0.0.1:18790" })
}

func TestEnsureOpenClawKeyIsStableUntilRotation(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()

	ctx := context.Background()
	first, err := service.EnsureOpenClawKey(ctx)
	if err != nil {
		t.Fatalf("EnsureOpenClawKey first call: %v", err)
	}
	second, err := service.EnsureOpenClawKey(ctx)
	if err != nil {
		t.Fatalf("EnsureOpenClawKey second call: %v", err)
	}
	if first.APIKey == "" || second.APIKey == "" {
		t.Fatal("expected non-empty api keys")
	}
	if first.APIKey != second.APIKey {
		t.Fatalf("expected key to remain stable, got %q and %q", first.APIKey, second.APIKey)
	}

	rotated, err := service.RotateOpenClawKey(ctx)
	if err != nil {
		t.Fatalf("RotateOpenClawKey: %v", err)
	}
	if rotated.APIKey == first.APIKey {
		t.Fatal("expected rotated key to change")
	}

	ok, err := service.ValidateOpenClawKey(ctx, rotated.APIKey)
	if err != nil {
		t.Fatalf("ValidateOpenClawKey: %v", err)
	}
	if !ok {
		t.Fatal("expected rotated key to validate")
	}
}

func TestDatasourceConfigIsEncryptedAtRest(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()

	ds := &store.Datasource{
		Name: "Primary",
		Type: "postgres",
		Config: map[string]any{
			"host":     "db.example.com",
			"database": "appdb",
			"user":     "alice",
			"password": "super-secret-password",
		},
	}
	if err := service.encryptDatasource(ds); err != nil {
		t.Fatalf("encryptDatasource: %v", err)
	}
	if ds.ConfigEncrypted == "" {
		t.Fatal("expected encrypted datasource config")
	}
	if err := service.store.SaveDatasource(context.Background(), ds); err != nil {
		t.Fatalf("SaveDatasource: %v", err)
	}

	var raw string
	if err := service.store.DB().QueryRowContext(context.Background(), `SELECT config_encrypted FROM datasources LIMIT 1`).Scan(&raw); err != nil {
		t.Fatalf("query raw datasource config: %v", err)
	}
	for _, forbidden := range []string{"db.example.com", "appdb", "alice", "super-secret-password"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("expected stored config to hide %q, got %q", forbidden, raw)
		}
	}
}

func TestValidateQuerySQLReadOnly(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()

	if _, err := service.ValidateQuerySQL("SELECT 1", nil, true); err != nil {
		t.Fatalf("expected SELECT to be valid: %v", err)
	}
	if _, err := service.ValidateQuerySQL("WITH sample AS (SELECT 1 AS value) SELECT value FROM sample", nil, true); err != nil {
		t.Fatalf("expected read-only CTE to be valid: %v", err)
	}
	if _, err := service.ValidateQuerySQL("UPDATE users SET admin = true", nil, true); err == nil {
		t.Fatal("expected UPDATE to be rejected for read-only validation")
	}
	if _, err := service.ValidateQuerySQL("WITH gone AS (DELETE FROM users RETURNING id) SELECT * FROM gone", nil, true); err == nil {
		t.Fatal("expected mutating CTE to be rejected for read-only validation")
	}
	if _, err := service.ValidateQuerySQL("SELECT * INTO archive_users FROM users", nil, true); err == nil {
		t.Fatal("expected SELECT INTO to be rejected for read-only validation")
	}
}

func TestUpsertDatasourceReturnsDecryptedConfig(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()

	service.tester = func(context.Context, *store.Datasource) error { return nil }

	ds, err := service.UpsertDatasource(context.Background(), &store.Datasource{
		Name: "Primary",
		Type: "postgres",
		Config: map[string]any{
			"host":     "db.example.com",
			"database": "warehouse",
			"user":     "analyst",
			"password": "secret",
		},
	})
	if err != nil {
		t.Fatalf("UpsertDatasource: %v", err)
	}
	if got := ds.Config["host"]; got != "db.example.com" {
		t.Fatalf("expected decrypted host in response, got %#v", got)
	}
	if got := ds.Config["password"]; got != "secret" {
		t.Fatalf("expected decrypted password in response, got %#v", got)
	}
}

func TestUpsertDatasourceRenamePreservesQueries(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()

	testCalls := 0
	service.tester = func(context.Context, *store.Datasource) error {
		testCalls++
		return nil
	}

	ctx := context.Background()
	ds, err := service.UpsertDatasource(ctx, &store.Datasource{
		Name:     "Primary",
		Type:     "postgres",
		Provider: "postgres",
		Config: map[string]any{
			"host":     "db.example.com",
			"database": "warehouse",
			"user":     "analyst",
			"password": "secret",
		},
	})
	if err != nil {
		t.Fatalf("create datasource: %v", err)
	}
	if testCalls != 1 {
		t.Fatalf("expected tester to run once on create, ran %d times", testCalls)
	}

	if _, err := service.CreateQuery(ctx, &store.ApprovedQuery{
		Name:      "Connectivity",
		SQLQuery:  "SELECT true AS connected",
		IsEnabled: true,
	}); err != nil {
		t.Fatalf("create query: %v", err)
	}

	renamed, err := service.UpsertDatasource(ctx, &store.Datasource{
		Name:     "Primary warehouse",
		Type:     "postgres",
		Provider: "postgres",
		Config: map[string]any{
			"host":     "db.example.com",
			"database": "warehouse",
			"user":     "analyst",
			"password": "secret",
		},
	})
	if err != nil {
		t.Fatalf("rename datasource: %v", err)
	}
	if renamed.Name != "Primary warehouse" {
		t.Fatalf("expected renamed datasource, got %#v", renamed)
	}
	if renamed.ID != ds.ID {
		t.Fatalf("expected rename to preserve datasource id, got %q want %q", renamed.ID, ds.ID)
	}
	if testCalls != 1 {
		t.Fatalf("expected rename to skip tester, ran %d times", testCalls)
	}

	queries, err := service.ListQueries(ctx)
	if err != nil {
		t.Fatalf("list queries: %v", err)
	}
	if len(queries) != 1 {
		t.Fatalf("expected query to survive rename, got %#v", queries)
	}
	if queries[0].DatasourceID != ds.ID {
		t.Fatalf("expected query datasource id to remain %q, got %q", ds.ID, queries[0].DatasourceID)
	}
}

func TestUpsertDatasourceRejectsConnectionChanges(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()

	service.tester = func(context.Context, *store.Datasource) error { return nil }

	ctx := context.Background()
	if _, err := service.UpsertDatasource(ctx, &store.Datasource{
		Name:     "Primary",
		Type:     "postgres",
		Provider: "postgres",
		Config: map[string]any{
			"host":     "db.example.com",
			"database": "warehouse",
			"user":     "analyst",
			"password": "secret",
		},
	}); err != nil {
		t.Fatalf("create datasource: %v", err)
	}

	_, err := service.UpsertDatasource(ctx, &store.Datasource{
		Name:     "Primary",
		Type:     "postgres",
		Provider: "postgres",
		Config: map[string]any{
			"host":     "db.example.com",
			"database": "reporting",
			"user":     "analyst",
			"password": "secret",
		},
	})
	if err == nil {
		t.Fatal("expected update to reject connection changes")
	}
	if !strings.Contains(err.Error(), "remove and recreate") {
		t.Fatalf("unexpected error: %v", err)
	}
}
