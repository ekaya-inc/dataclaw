package core

import (
	"context"
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

	_, err := service.TestDraftQuery(context.Background(), "SELECT * FROM orders WHERE total > {{min_total}}", []models.QueryParameter{{Name: "min_total", Type: "decimal", Default: 0.0}}, false, 25)
	if err != nil {
		t.Fatalf("TestDraftQuery: %v", err)
	}
	if gotQuery != "SELECT * FROM orders WHERE total > {{min_total}}" {
		t.Fatalf("expected unprepared SQL, got %q", gotQuery)
	}
	if len(gotParams) != 1 || gotParams[0].Name != "min_total" {
		t.Fatalf("expected query parameter definitions, got %#v", gotParams)
	}
	if gotValues != nil {
		t.Fatalf("expected nil execution values for draft test, got %#v", gotValues)
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
			executeMutatingQuery: func(_ context.Context, _ string, _ []models.QueryParameter, values map[string]any, limit int) (*QueryResult, error) {
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
			executeMutatingQuery: func(_ context.Context, sqlQuery string, _ []models.QueryParameter, _ map[string]any, limit int) (*QueryResult, error) {
				gotQuery = sqlQuery
				gotLimit = limit
				return &QueryResult{}, nil
			},
		}, nil
	}

	_, err := service.TestDraftQuery(context.Background(), "DELETE FROM contracts WHERE id = {{id}} RETURNING id", []models.QueryParameter{{Name: "id", Type: "uuid", Required: true}}, true, 400)
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
