package core

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/security"
	"github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/migrations"
	"github.com/ekaya-inc/dataclaw/pkg/models"
)

type fakeAdapterFactory struct {
	supported   map[string]bool
	newTester   func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error)
	newExecutor func(context.Context, string, map[string]any) (dsadapter.QueryExecutor, error)
	fingerprint func(string, map[string]any) (string, error)
	typeInfo    map[string]dsadapter.AdapterInfo
}

type fakeConnectionTester struct {
	test func(context.Context) error
}

type fakeQueryExecutor struct {
	query               func(context.Context, string, int) (*QueryResult, error)
	queryWithParameters func(context.Context, string, []models.QueryParameter, map[string]any, int) (*QueryResult, error)
	executeDMLQuery     func(context.Context, string, []models.QueryParameter, map[string]any, int) (*QueryResult, error)
	execute             func(context.Context, string, int) (*ExecuteResult, error)
}

func newFakeAdapterFactory() *fakeAdapterFactory {
	return &fakeAdapterFactory{
		supported: map[string]bool{
			"postgres": true,
			"mssql":    true,
		},
		newTester: func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error) {
			return fakeConnectionTester{}, nil
		},
		newExecutor: func(context.Context, string, map[string]any) (dsadapter.QueryExecutor, error) {
			return nil, errors.New("unexpected query execution in test")
		},
		fingerprint: func(_ string, config map[string]any) (string, error) {
			return dsadapter.CanonicalFingerprint(config)
		},
		typeInfo: map[string]dsadapter.AdapterInfo{
			"postgres": {
				Type:        "postgres",
				DisplayName: "PostgreSQL",
				SQLDialect:  "PostgreSQL",
				Capabilities: dsadapter.AdapterCapabilities{
					SupportsArrayParameters: true,
				},
			},
			"mssql": {
				Type:        "mssql",
				DisplayName: "Microsoft SQL Server",
				SQLDialect:  "MSSQL",
				Capabilities: dsadapter.AdapterCapabilities{
					SupportsArrayParameters: false,
				},
			},
		},
	}
}

func (f *fakeAdapterFactory) NewConnectionTester(ctx context.Context, dsType string, config map[string]any) (dsadapter.ConnectionTester, error) {
	if !f.SupportsType(dsType) {
		return nil, errors.New("unsupported datasource type: " + dsType)
	}
	return f.newTester(ctx, dsType, config)
}

func (f *fakeAdapterFactory) NewQueryExecutor(ctx context.Context, dsType string, config map[string]any) (dsadapter.QueryExecutor, error) {
	if !f.SupportsType(dsType) {
		return nil, errors.New("unsupported datasource type: " + dsType)
	}
	return f.newExecutor(ctx, dsType, config)
}

func (f *fakeAdapterFactory) ConfigFingerprint(dsType string, config map[string]any) (string, error) {
	if !f.SupportsType(dsType) {
		return "", errors.New("unsupported datasource type: " + dsType)
	}
	return f.fingerprint(dsType, config)
}

func (f *fakeAdapterFactory) ListTypes() []dsadapter.AdapterInfo {
	types := make([]dsadapter.AdapterInfo, 0, len(f.typeInfo))
	for dsType, info := range f.typeInfo {
		if f.supported[dsType] {
			types = append(types, info)
		}
	}
	return types
}

func (f *fakeAdapterFactory) TypeInfo(dsType string) (dsadapter.AdapterInfo, bool) {
	info, ok := f.typeInfo[dsType]
	return info, ok
}

func (f *fakeAdapterFactory) SupportsType(dsType string) bool {
	return f != nil && f.supported[dsType]
}

func (f fakeConnectionTester) TestConnection(ctx context.Context) error {
	if f.test != nil {
		return f.test(ctx)
	}
	return nil
}

func (f fakeConnectionTester) Close() error { return nil }

func (f fakeQueryExecutor) Query(ctx context.Context, sqlQuery string, limit int) (*QueryResult, error) {
	if f.query != nil {
		return f.query(ctx, sqlQuery, limit)
	}
	return nil, errors.New("unexpected Query call")
}

func (f fakeQueryExecutor) QueryWithParameters(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*QueryResult, error) {
	if f.queryWithParameters != nil {
		return f.queryWithParameters(ctx, sqlQuery, paramDefs, values, limit)
	}
	return nil, errors.New("unexpected QueryWithParameters call")
}

func (f fakeQueryExecutor) ExecuteDMLQuery(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*QueryResult, error) {
	if f.executeDMLQuery != nil {
		return f.executeDMLQuery(ctx, sqlQuery, paramDefs, values, limit)
	}
	return nil, errors.New("unexpected ExecuteDMLQuery call")
}

func (f fakeQueryExecutor) Execute(ctx context.Context, sqlQuery string, limit int) (*ExecuteResult, error) {
	if f.execute != nil {
		return f.execute(ctx, sqlQuery, limit)
	}
	return nil, errors.New("unexpected Execute call")
}

func (f fakeQueryExecutor) Close() error { return nil }

func newTestService(t *testing.T) *Service {
	return newTestServiceWithFactory(t, newFakeAdapterFactory())
}

func newTestServiceWithFactory(t *testing.T, factory dsadapter.Factory) *Service {
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
	return New(st, secret, "test", func() string { return "http://127.0.0.1:18790" }, factory)
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

	if _, err := service.ValidateQuerySQL("SELECT 1", nil, false); err != nil {
		t.Fatalf("expected SELECT to be valid: %v", err)
	}
	if _, err := service.ValidateQuerySQL("WITH sample AS (SELECT 1 AS value) SELECT value FROM sample", nil, false); err != nil {
		t.Fatalf("expected read-only CTE to be valid: %v", err)
	}
	if _, err := service.ValidateQuerySQL("UPDATE users SET admin = true", nil, false); err == nil {
		t.Fatal("expected UPDATE to be rejected for read-only validation")
	}
	if _, err := service.ValidateQuerySQL("WITH gone AS (DELETE FROM users RETURNING id) SELECT * FROM gone", nil, false); err == nil {
		t.Fatal("expected mutating CTE to be rejected for read-only validation")
	}
	if _, err := service.ValidateQuerySQL("SELECT * INTO archive_users FROM users", nil, false); err == nil {
		t.Fatal("expected SELECT INTO to be rejected for read-only validation")
	}
	if _, err := service.ValidateQuerySQL("UPDATE users SET admin = true", []models.QueryParameter{}, true); err != nil {
		t.Fatalf("expected mutating validation to accept UPDATE, got %v", err)
	}
	if _, err := service.ValidateQuerySQL("SELECT 1", nil, true); err == nil {
		t.Fatal("expected mutating validation to reject SELECT")
	}
	if _, err := service.ValidateQuerySQL("DROP TABLE users", nil, true); err == nil {
		t.Fatal("expected mutating validation to reject DDL")
	}
}

func TestUpsertDatasourceReturnsDecryptedConfig(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()

	service.adapters.(*fakeAdapterFactory).newTester = func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error) {
		return fakeConnectionTester{}, nil
	}

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
	service.adapters.(*fakeAdapterFactory).newTester = func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error) {
		return fakeConnectionTester{
			test: func(context.Context) error {
				testCalls++
				return nil
			},
		}, nil
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
		NaturalLanguagePrompt: "Connectivity check",
		SQLQuery:              "SELECT true AS connected",
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

	service.adapters.(*fakeAdapterFactory).newTester = func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error) {
		return fakeConnectionTester{}, nil
	}

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
