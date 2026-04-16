package core

import (
	"context"
	"strings"
	"testing"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/pkg/models"
)

func TestValidateRequiredParameters(t *testing.T) {
	tests := []struct {
		name          string
		paramDefs     []models.QueryParameter
		supplied      map[string]any
		expectedError string
	}{
		{
			name: "all required parameters supplied",
			paramDefs: []models.QueryParameter{
				{Name: "customer_id", Type: "uuid", Required: true},
				{Name: "limit", Type: "integer", Required: false, Default: 100},
			},
			supplied: map[string]any{"customer_id": "550e8400-e29b-41d4-a716-446655440000"},
		},
		{
			name: "missing required parameter",
			paramDefs: []models.QueryParameter{
				{Name: "customer_id", Type: "uuid", Required: true},
			},
			supplied:      map[string]any{},
			expectedError: "required parameter 'customer_id' is missing",
		},
		{
			name: "blank required string is treated as missing",
			paramDefs: []models.QueryParameter{
				{Name: "customer_id", Type: "string", Required: true},
			},
			supplied:      map[string]any{"customer_id": "   "},
			expectedError: "required parameter 'customer_id' is missing",
		},
		{
			name: "required parameter can fall back to default",
			paramDefs: []models.QueryParameter{
				{Name: "customer_id", Type: "uuid", Required: true, Default: "550e8400-e29b-41d4-a716-446655440000"},
			},
			supplied: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRequiredParameters(tt.paramDefs, tt.supplied)
			if tt.expectedError == "" && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.expectedError != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expectedError)
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %v", tt.expectedError, err)
				}
			}
		})
	}
}

func TestPrepareExecutionParameterValues(t *testing.T) {
	tests := []struct {
		name          string
		paramDefs     []models.QueryParameter
		supplied      map[string]any
		expected      map[string]any
		expectedError string
	}{
		{
			name: "coerces scalar values and fills defaults",
			paramDefs: []models.QueryParameter{
				{Name: "customer_id", Type: "uuid", Required: true},
				{Name: "limit", Type: "integer", Required: false, Default: 100},
				{Name: "active", Type: "boolean", Required: false, Default: "true"},
			},
			supplied: map[string]any{
				"customer_id": "550e8400-e29b-41d4-a716-446655440000",
			},
			expected: map[string]any{
				"customer_id": "550e8400-e29b-41d4-a716-446655440000",
				"limit":       int64(100),
				"active":      true,
			},
		},
		{
			name: "supports string arrays from comma separated strings",
			paramDefs: []models.QueryParameter{
				{Name: "statuses", Type: "string[]", Required: true},
			},
			supplied: map[string]any{"statuses": "pending, active , archived"},
			expected: map[string]any{"statuses": []string{"pending", "active", "archived"}},
		},
		{
			name: "supports integer arrays from json values",
			paramDefs: []models.QueryParameter{
				{Name: "ids", Type: "integer[]", Required: true},
			},
			supplied: map[string]any{"ids": []any{float64(1), "2", int64(3)}},
			expected: map[string]any{"ids": []int64{1, 2, 3}},
		},
		{
			name: "rejects unknown parameters",
			paramDefs: []models.QueryParameter{
				{Name: "known", Type: "string", Required: true},
			},
			supplied:      map[string]any{"known": "ok", "extra": "nope"},
			expectedError: "unknown parameter 'extra'",
		},
		{
			name: "rejects invalid uuid",
			paramDefs: []models.QueryParameter{
				{Name: "customer_id", Type: "uuid", Required: true},
			},
			supplied:      map[string]any{"customer_id": "not-a-uuid"},
			expectedError: "invalid UUID format",
		},
		{
			name: "rejects non integral floats for integer parameters",
			paramDefs: []models.QueryParameter{
				{Name: "limit", Type: "integer", Required: true},
			},
			supplied:      map[string]any{"limit": 3.14},
			expectedError: "cannot convert non-integer number",
		},
		{
			name: "coerces default arrays from strings",
			paramDefs: []models.QueryParameter{
				{Name: "statuses", Type: "string[]", Required: true, Default: "pending,active"},
			},
			supplied: map[string]any{},
			expected: map[string]any{"statuses": []string{"pending", "active"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := prepareExecutionParameterValues(tt.paramDefs, tt.supplied)
			if tt.expectedError == "" && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.expectedError != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expectedError)
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %v", tt.expectedError, err)
				}
				return
			}
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d values, got %d: %#v", len(tt.expected), len(got), got)
			}
			for key, want := range tt.expected {
				if gotValue := got[key]; !valuesEqual(gotValue, want) {
					t.Fatalf("expected %q to be %#v, got %#v", key, want, gotValue)
				}
			}
		})
	}
}

func TestExecuteStoredQueryRejectsDisabledQueries(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	ctx := context.Background()
	query, err := service.CreateQuery(ctx, &store.ApprovedQuery{
		Name:        "Disabled query",
		Description: "Should not execute",
		SQLQuery:    "SELECT true AS connected",
		IsEnabled:   false,
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}

	_, err = service.ExecuteStoredQuery(ctx, query.ID, nil, 100)
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("expected disabled-query error, got %v", err)
	}
}

func TestExecuteStoredQueryRejectsInjectionBeforeDatasourceCall(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	ctx := context.Background()
	query, err := service.CreateQuery(ctx, &store.ApprovedQuery{
		Name:        "Account lookup",
		Description: "Find one account",
		SQLQuery:    "SELECT * FROM accounts WHERE id = {{account_id}}",
		Parameters: []models.QueryParameter{
			{Name: "account_id", Type: "string", Required: true},
		},
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}

	_, err = service.ExecuteStoredQuery(ctx, query.ID, map[string]any{"account_id": "'; DROP TABLE users--"}, 100)
	if err == nil || !strings.Contains(err.Error(), "SQL injection") {
		t.Fatalf("expected injection error, got %v", err)
	}
}

func TestExecuteStoredQueryRejectsSQLServerArrayParameters(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()
	seedDatasource(t, service, "mssql")

	ctx := context.Background()
	query, err := service.CreateQuery(ctx, &store.ApprovedQuery{
		Name:        "Account lookup",
		Description: "Find a set of accounts",
		SQLQuery:    "SELECT * FROM accounts WHERE id IN ({{account_ids}})",
		Parameters: []models.QueryParameter{
			{Name: "account_ids", Type: "integer[]", Required: true},
		},
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}

	_, err = service.ExecuteStoredQuery(ctx, query.ID, map[string]any{"account_ids": "1,2,3"}, 100)
	if err == nil || !strings.Contains(err.Error(), "SQL Server") {
		t.Fatalf("expected SQL Server array-parameter error, got %v", err)
	}
}

func TestCreateQueryRejectsMutatingApprovedSQL(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()
	seedDatasource(t, service, "postgres")

	_, err := service.CreateQuery(context.Background(), &store.ApprovedQuery{
		Name:        "Mutating query",
		Description: "Should be rejected",
		SQLQuery:    "UPDATE accounts SET disabled = true",
		IsEnabled:   true,
	})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("expected read-only validation error, got %v", err)
	}
}

func seedDatasource(t *testing.T, service *Service, dsType string) {
	t.Helper()

	service.adapters.(*fakeAdapterFactory).newTester = func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error) {
		return fakeConnectionTester{}, nil
	}
	_, err := service.UpsertDatasource(context.Background(), &store.Datasource{
		Name:     "Primary",
		Type:     dsType,
		Provider: dsType,
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
}

func valuesEqual(left any, right any) bool {
	switch expected := right.(type) {
	case []string:
		actual, ok := left.([]string)
		if !ok || len(actual) != len(expected) {
			return false
		}
		for i := range expected {
			if actual[i] != expected[i] {
				return false
			}
		}
		return true
	case []int64:
		actual, ok := left.([]int64)
		if !ok || len(actual) != len(expected) {
			return false
		}
		for i := range expected {
			if actual[i] != expected[i] {
				return false
			}
		}
		return true
	default:
		return left == right
	}
}
