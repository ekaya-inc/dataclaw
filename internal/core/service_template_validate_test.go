package core

import (
	"context"
	"strings"
	"testing"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/adapters/datasource/mssql"
	"github.com/ekaya-inc/dataclaw/internal/store"
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
