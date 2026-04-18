package core

import (
	"context"
	"errors"
	"testing"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/pkg/models"
)

func TestTestDatasourceUsesAdapterFactory(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()

	calls := 0
	var gotType string
	var gotConfig map[string]any
	factory.newTester = func(_ context.Context, dsType string, config map[string]any) (dsadapter.ConnectionTester, error) {
		gotType = dsType
		gotConfig = config
		return fakeConnectionTester{test: func(context.Context) error {
			calls++
			return nil
		}}, nil
	}

	err := service.TestDatasource(context.Background(), &store.Datasource{
		Type:   "postgres",
		Config: map[string]any{"host": "db.example.com", "database": "warehouse"},
	})
	if err != nil {
		t.Fatalf("TestDatasource: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected tester to run once, ran %d times", calls)
	}
	if gotType != "postgres" {
		t.Fatalf("expected postgres adapter, got %q", gotType)
	}
	if gotConfig["host"] != "db.example.com" {
		t.Fatalf("expected config to reach adapter, got %#v", gotConfig)
	}
}

func TestGetDatasourceInformationUsesAdapterIntrospector(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	var gotType string
	var gotConfig map[string]any
	factory.newIntrospector = func(_ context.Context, dsType string, config map[string]any) (dsadapter.DatasourceIntrospector, error) {
		gotType = dsType
		gotConfig = config
		return fakeDatasourceIntrospector{
			info: &dsadapter.DatasourceInfo{
				DatabaseName: "warehouse",
				SchemaName:   "public",
				CurrentUser:  "analyst",
				Version:      "PostgreSQL 16.0",
			},
		}, nil
	}

	info, err := service.GetDatasourceInformation(context.Background())
	if err != nil {
		t.Fatalf("GetDatasourceInformation: %v", err)
	}
	if gotType != "postgres" {
		t.Fatalf("expected postgres adapter, got %q", gotType)
	}
	if gotConfig["database"] != "warehouse" {
		t.Fatalf("expected datasource config to reach introspector, got %#v", gotConfig)
	}
	if info.Name != "Primary" || info.Type != "postgres" || info.SQLDialect != "PostgreSQL" {
		t.Fatalf("expected composed datasource metadata, got %#v", info)
	}
	if info.DatabaseName != "warehouse" || info.SchemaName != "public" || info.CurrentUser != "analyst" || info.Version != "PostgreSQL 16.0" {
		t.Fatalf("expected runtime datasource info, got %#v", info)
	}
}

func TestGetDatasourceInformationFailsWithoutDatasource(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()

	if _, err := service.GetDatasourceInformation(context.Background()); !errors.Is(err, ErrNoDatasourceConfigured) {
		t.Fatalf("expected ErrNoDatasourceConfigured, got %v", err)
	}
}

func TestGetDatasourceInformationReturnsPartialMetadataWhenIntrospectionFails(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	factory.newIntrospector = func(context.Context, string, map[string]any) (dsadapter.DatasourceIntrospector, error) {
		return fakeDatasourceIntrospector{err: errors.New("metadata unavailable")}, nil
	}

	info, err := service.GetDatasourceInformation(context.Background())
	if err == nil || err.Error() != "metadata unavailable" {
		t.Fatalf("expected metadata unavailable error, got %v", err)
	}
	if info == nil {
		t.Fatal("expected partial datasource information on introspection error")
	}
	if info.Name != "Primary" || info.Type != "postgres" || info.SQLDialect != "PostgreSQL" {
		t.Fatalf("expected partial metadata to preserve datasource identity, got %#v", info)
	}
	if info.DatabaseName != "" || info.CurrentUser != "" || info.Version != "" {
		t.Fatalf("expected runtime fields to stay empty on introspection failure, got %#v", info)
	}
}

func TestGetDatasourceInformationReturnsPartialMetadataWhenIntrospectorCreationFails(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	factory.newIntrospector = func(context.Context, string, map[string]any) (dsadapter.DatasourceIntrospector, error) {
		return nil, errors.New("unsupported metadata query")
	}

	info, err := service.GetDatasourceInformation(context.Background())
	if err == nil || err.Error() != "unsupported metadata query" {
		t.Fatalf("expected unsupported metadata query error, got %v", err)
	}
	if info == nil {
		t.Fatal("expected partial datasource information on introspector creation error")
	}
	if info.Name != "Primary" || info.Type != "postgres" || info.SQLDialect != "PostgreSQL" {
		t.Fatalf("expected partial metadata to preserve datasource identity, got %#v", info)
	}
}

func TestTestRawQueryUsesAdapterExecutor(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	var gotQuery string
	var gotLimit int
	factory.newExecutor = func(_ context.Context, dsType string, config map[string]any) (dsadapter.QueryExecutor, error) {
		if dsType != "postgres" {
			t.Fatalf("expected postgres adapter, got %q", dsType)
		}
		return fakeQueryExecutor{query: func(_ context.Context, sqlQuery string, limit int) (*QueryResult, error) {
			gotQuery = sqlQuery
			gotLimit = limit
			return &QueryResult{}, nil
		}}, nil
	}

	if _, err := service.TestRawQuery(context.Background(), "select 1", 25); err != nil {
		t.Fatalf("TestRawQuery: %v", err)
	}
	wantQuery, err := validateReadOnlySQL("select 1")
	if err != nil {
		t.Fatalf("validateReadOnlySQL: %v", err)
	}
	if gotQuery != wantQuery {
		t.Fatalf("expected validated read-only SQL %q, got %q", wantQuery, gotQuery)
	}
	if gotLimit != 25 {
		t.Fatalf("expected limit 25, got %d", gotLimit)
	}
}

func TestTestDraftQueryUsesPreparedParameters(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	var gotQuery string
	var gotParams []models.QueryParameter
	var gotValues map[string]any
	var gotLimit int
	factory.newExecutor = func(_ context.Context, dsType string, config map[string]any) (dsadapter.QueryExecutor, error) {
		return fakeQueryExecutor{queryWithParameters: func(_ context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*QueryResult, error) {
			gotQuery = sqlQuery
			gotParams = paramDefs
			gotValues = values
			gotLimit = limit
			return &QueryResult{}, nil
		}}, nil
	}

	_, err := service.TestDraftQuery(context.Background(), "SELECT * FROM orders WHERE total > {{min_total}}", []models.QueryParameter{{Name: "min_total", Type: "decimal", Default: 0.0}}, map[string]any{"min_total": 42.5}, false, 25)
	if err != nil {
		t.Fatalf("TestDraftQuery: %v", err)
	}
	if gotQuery != "SELECT * FROM orders WHERE total > {{min_total}}" {
		t.Fatalf("expected unprepared SQL, got %q", gotQuery)
	}
	if len(gotParams) != 1 || gotParams[0].Name != "min_total" {
		t.Fatalf("expected query parameter definitions, got %#v", gotParams)
	}
	if gotValues["min_total"] != 42.5 {
		t.Fatalf("expected caller-supplied min_total to flow through, got %#v", gotValues)
	}
	if gotLimit != 25 {
		t.Fatalf("expected limit 25, got %d", gotLimit)
	}
}

func TestExecuteStoredQueryUsesPreparedParameters(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	ctx := context.Background()
	query, err := service.CreateQuery(ctx, &store.ApprovedQuery{
		NaturalLanguagePrompt: "Find one account by id",
		AdditionalContext:     "Lookup a single account row for a given id.",
		SQLQuery:              "SELECT * FROM accounts WHERE id = {{account_id}}",
		Parameters:            []models.QueryParameter{{Name: "account_id", Type: "uuid", Required: true}},
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}

	var gotQuery string
	var gotParams []models.QueryParameter
	var gotValues map[string]any
	var gotLimit int
	factory.newExecutor = func(_ context.Context, dsType string, config map[string]any) (dsadapter.QueryExecutor, error) {
		return fakeQueryExecutor{queryWithParameters: func(_ context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*QueryResult, error) {
			gotQuery = sqlQuery
			gotParams = paramDefs
			gotValues = values
			gotLimit = limit
			return &QueryResult{}, nil
		}}, nil
	}

	_, err = service.ExecuteStoredQuery(ctx, query.ID, map[string]any{"account_id": "550e8400-e29b-41d4-a716-446655440000"}, 50)
	if err != nil {
		t.Fatalf("ExecuteStoredQuery: %v", err)
	}
	if gotQuery != "SELECT * FROM accounts WHERE id = {{account_id}}" {
		t.Fatalf("expected unprepared SQL, got %q", gotQuery)
	}
	if len(gotParams) != 1 || gotParams[0].Name != "account_id" {
		t.Fatalf("expected query parameter definitions, got %#v", gotParams)
	}
	if gotValues["account_id"] != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatalf("expected query values, got %#v", gotValues)
	}
	if gotLimit != 50 {
		t.Fatalf("expected limit 50, got %d", gotLimit)
	}
}

func TestExecuteStoredQueryForwardsLimitForMutatingQueries(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	ctx := context.Background()
	query, err := service.CreateQuery(ctx, &store.ApprovedQuery{
		NaturalLanguagePrompt: "Retire a batch of marketing contracts",
		SQLQuery:              "DELETE FROM contracts WHERE owner = {{owner}} RETURNING id",
		AllowsModification:    true,
		Parameters:            []models.QueryParameter{{Name: "owner", Type: "string", Required: true}},
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}

	var gotLimit int
	var gotValues map[string]any
	factory.newExecutor = func(_ context.Context, _ string, _ map[string]any) (dsadapter.QueryExecutor, error) {
		return fakeQueryExecutor{
			executeDMLQuery: func(_ context.Context, _ string, _ []models.QueryParameter, values map[string]any, limit int) (*QueryResult, error) {
				gotValues = values
				gotLimit = limit
				return &QueryResult{}, nil
			},
		}, nil
	}

	_, err = service.ExecuteStoredQuery(ctx, query.ID, map[string]any{"owner": "marketing"}, 250)
	if err != nil {
		t.Fatalf("ExecuteStoredQuery: %v", err)
	}
	if gotLimit != 250 {
		t.Fatalf("expected caller limit 250 to flow through, got %d", gotLimit)
	}
	if gotValues["owner"] != "marketing" {
		t.Fatalf("expected execution values, got %#v", gotValues)
	}
}

func TestTestDraftQueryForwardsLimitForMutatingQueries(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	var gotLimit int
	var gotQuery string
	factory.newExecutor = func(_ context.Context, _ string, _ map[string]any) (dsadapter.QueryExecutor, error) {
		return fakeQueryExecutor{
			executeDMLQuery: func(_ context.Context, sqlQuery string, _ []models.QueryParameter, _ map[string]any, limit int) (*QueryResult, error) {
				gotQuery = sqlQuery
				gotLimit = limit
				return &QueryResult{}, nil
			},
		}, nil
	}

	_, err := service.TestDraftQuery(context.Background(), "DELETE FROM contracts WHERE id = {{id}} RETURNING id", []models.QueryParameter{{Name: "id", Type: "uuid", Required: true}}, map[string]any{"id": "550e8400-e29b-41d4-a716-446655440000"}, true, 400)
	if err != nil {
		t.Fatalf("TestDraftQuery: %v", err)
	}
	if gotQuery != "DELETE FROM contracts WHERE id = {{id}} RETURNING id" {
		t.Fatalf("expected unprepared SQL, got %q", gotQuery)
	}
	if gotLimit != 400 {
		t.Fatalf("expected caller limit 400 to flow through, got %d", gotLimit)
	}
}

func TestExecuteRawStatementUsesAdapterExecuteForDDL(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	var gotQuery string
	var gotLimit int
	factory.newExecutor = func(_ context.Context, dsType string, config map[string]any) (dsadapter.QueryExecutor, error) {
		if dsType != "postgres" {
			t.Fatalf("expected postgres adapter, got %q", dsType)
		}
		return fakeQueryExecutor{
			execute: func(_ context.Context, sqlQuery string, limit int) (*ExecuteResult, error) {
				gotQuery = sqlQuery
				gotLimit = limit
				return &ExecuteResult{}, nil
			},
		}, nil
	}

	if _, err := service.ExecuteRawStatement(context.Background(), "CREATE TABLE scratch_execute (id integer);", 75); err != nil {
		t.Fatalf("ExecuteRawStatement: %v", err)
	}
	if gotQuery != "CREATE TABLE scratch_execute (id integer)" {
		t.Fatalf("expected normalized DDL, got %q", gotQuery)
	}
	if gotLimit != 75 {
		t.Fatalf("expected limit 75, got %d", gotLimit)
	}
}

func TestExecuteRawStatementAllowsProceduralDDL(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	var gotQuery string
	factory.newExecutor = func(_ context.Context, dsType string, config map[string]any) (dsadapter.QueryExecutor, error) {
		if dsType != "postgres" {
			t.Fatalf("expected postgres adapter, got %q", dsType)
		}
		return fakeQueryExecutor{
			execute: func(_ context.Context, sqlQuery string, limit int) (*ExecuteResult, error) {
				gotQuery = sqlQuery
				return &ExecuteResult{}, nil
			},
		}, nil
	}

	sqlQuery := `CREATE OR REPLACE FUNCTION scratch_execute()
RETURNS integer
LANGUAGE plpgsql
AS $fn$
BEGIN
	RETURN 1;
END;
$fn$;`

	if _, err := service.ExecuteRawStatement(context.Background(), sqlQuery, 75); err != nil {
		t.Fatalf("ExecuteRawStatement: %v", err)
	}
	if gotQuery != `CREATE OR REPLACE FUNCTION scratch_execute()
RETURNS integer
LANGUAGE plpgsql
AS $fn$
BEGIN
	RETURN 1;
END;
$fn$` {
		t.Fatalf("expected procedural DDL to be preserved, got %q", gotQuery)
	}
}

func TestExecuteRawStatementRejectsMultipleStatements(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	if _, err := service.ExecuteRawStatement(context.Background(), "CREATE TABLE a (id integer); DROP TABLE a", 10); err == nil {
		t.Fatal("expected multiple statements to be rejected")
	}
}

func TestExecuteRawStatementRejectsReadOnlySQL(t *testing.T) {
	factory := newFakeAdapterFactory()
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	called := false
	factory.newExecutor = func(_ context.Context, _ string, _ map[string]any) (dsadapter.QueryExecutor, error) {
		return fakeQueryExecutor{
			execute: func(_ context.Context, _ string, _ int) (*ExecuteResult, error) {
				called = true
				return nil, errors.New("execute only accepts single-statement DDL or DML")
			},
		}, nil
	}

	if _, err := service.ExecuteRawStatement(context.Background(), "SELECT 1", 10); err == nil {
		t.Fatal("expected read-only SQL to be rejected by raw execute")
	}
	if !called {
		t.Fatal("expected adapter Execute to enforce raw execute statement type")
	}
}
