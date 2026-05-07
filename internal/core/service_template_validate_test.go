package core

import (
	"context"
	"strings"
	"testing"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/adapters/datasource/mssql"
	"github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/pkg/models"
)

// serviceWithMSSQLValidator wires the real mssql template validator into the
// fake factory and configures an mssql datasource so requireDatasource succeeds.
func serviceWithMSSQLValidator(t *testing.T) *Service {
	t.Helper()
	factory := newFakeAdapterFactory()
	factory.validateReadOnlyTemplate = func(dsType, sqlQuery string) error {
		if dsType != "mssql" {
			return nil
		}
		return mssql.ValidateReadOnlyTemplate(sqlQuery)
	}
	factory.newTester = func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error) {
		return fakeConnectionTester{}, nil
	}
	service := newTestServiceWithFactory(t, factory)
	if _, err := service.UpsertDatasource(context.Background(), &store.Datasource{
		Name:   "Primary",
		Type:   "mssql",
		Config: map[string]any{"host": "db", "database": "contoso", "username": "u", "password": "p"},
	}); err != nil {
		t.Fatalf("UpsertDatasource: %v", err)
	}
	return service
}

func TestValidateQuerySQLRejectsMSSQLAntiPatterns(t *testing.T) {
	service := serviceWithMSSQLValidator(t)
	defer service.store.Close()
	ctx := context.Background()

	rejectedReadOnly := []struct {
		name string
		sql  string
		want string
	}{
		{"top", "SELECT TOP 10 OrderKey FROM dbo.orders ORDER BY OrderKey DESC", "TOP"},
		{"offset_fetch", "SELECT OrderKey FROM dbo.orders ORDER BY OrderKey OFFSET 10 ROWS FETCH NEXT 5 ROWS ONLY", "OFFSET/FETCH"},
		{"limit", "SELECT OrderKey FROM dbo.orders ORDER BY OrderKey LIMIT 10", "LIMIT"},
		{"named_marker", "SELECT OrderKey FROM dbo.orders WHERE CustomerKey = @customer ORDER BY OrderKey", "@customer"},
	}
	for _, tc := range rejectedReadOnly {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.ValidateQuerySQL(ctx, tc.sql, nil, false)
			if err == nil {
				t.Fatalf("expected rejection, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error to mention %q, got %q", tc.want, err.Error())
			}
		})
	}
}

func TestValidateQuerySQLAcceptsCleanReadOnlyTemplate(t *testing.T) {
	service := serviceWithMSSQLValidator(t)
	defer service.store.Close()

	good := "SELECT OrderKey FROM dbo.orders ORDER BY OrderKey"
	if _, err := service.ValidateQuerySQL(context.Background(), good, nil, false); err != nil {
		t.Fatalf("expected acceptance, got %v", err)
	}
}

func TestValidateQuerySQLBypassesReadOnlyCheckForDMLTemplates(t *testing.T) {
	service := serviceWithMSSQLValidator(t)
	defer service.store.Close()

	// `UPDATE TOP (n)` is legal T-SQL. The new read-only check must not
	// fire for mutating approved queries because pagination is never
	// appended on the DML path.
	dml := "UPDATE TOP (5) dbo.t SET status = 'done' WHERE id = 1"
	if _, err := service.ValidateQuerySQL(context.Background(), dml, nil, true); err != nil {
		t.Fatalf("expected DML template to bypass read-only check, got %v", err)
	}
}

func TestValidateQuerySQLRejectsArrayParametersOnMSSQL(t *testing.T) {
	service := serviceWithMSSQLValidator(t)
	defer service.store.Close()
	ctx := context.Background()

	cases := []struct {
		name      string
		paramType string
	}{
		{"integer_array", "integer[]"},
		{"string_array", "string[]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := []models.QueryParameter{{Name: "items", Type: tc.paramType, Required: true}}
			sql := "SELECT OrderKey FROM dbo.orders WHERE OrderKey = ANY({{items}}) ORDER BY OrderKey"
			_, err := service.ValidateQuerySQL(ctx, sql, params, false)
			if err == nil {
				t.Fatalf("expected rejection for %s, got nil", tc.paramType)
			}
			if !strings.Contains(err.Error(), "not supported") || !strings.Contains(err.Error(), `"items"`) {
				t.Fatalf("expected error to identify the array parameter and capability gap, got %q", err.Error())
			}
		})
	}
}

func TestCreateQueryRejectsArrayParametersOnMSSQL(t *testing.T) {
	service := serviceWithMSSQLValidator(t)
	defer service.store.Close()
	ctx := context.Background()

	q := &store.ApprovedQuery{
		NaturalLanguagePrompt: "lookup",
		SQLQuery:              "SELECT OrderKey FROM dbo.orders WHERE OrderKey = ANY({{ids}}) ORDER BY OrderKey",
		Parameters:            []models.QueryParameter{{Name: "ids", Type: "integer[]", Required: true}},
	}
	if _, err := service.CreateQuery(ctx, q); err == nil {
		t.Fatal("expected CreateQuery to reject array parameter on MSSQL")
	}
}

func TestValidateQuerySQLAcceptsArrayParametersOnPostgres(t *testing.T) {
	factory := newFakeAdapterFactory()
	factory.newTester = func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error) {
		return fakeConnectionTester{}, nil
	}
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	ctx := context.Background()
	if _, err := service.UpsertDatasource(ctx, &store.Datasource{
		Name:   "Primary",
		Type:   "postgres",
		Config: map[string]any{"host": "db", "database": "warehouse", "username": "u", "password": "p"},
	}); err != nil {
		t.Fatalf("UpsertDatasource: %v", err)
	}

	params := []models.QueryParameter{{Name: "ids", Type: "integer[]", Required: true}}
	sql := "SELECT id FROM users WHERE id = ANY({{ids}}) ORDER BY id"
	if _, err := service.ValidateQuerySQL(ctx, sql, params, false); err != nil {
		t.Fatalf("expected Postgres to accept array parameter, got %v", err)
	}
}
