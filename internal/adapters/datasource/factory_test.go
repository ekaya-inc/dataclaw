package datasource

import (
	"context"
	"strings"
	"testing"

	"github.com/ekaya-inc/dataclaw/pkg/models"
)

type testConnectionTester struct{}

type testQueryExecutor struct{}

func (testConnectionTester) TestConnection(context.Context) error { return nil }
func (testConnectionTester) Close() error                         { return nil }

func (testQueryExecutor) Query(context.Context, string, int) (*QueryResult, error) {
	return &QueryResult{}, nil
}
func (testQueryExecutor) QueryWithParameters(context.Context, string, []models.QueryParameter, map[string]any, int) (*QueryResult, error) {
	return &QueryResult{}, nil
}
func (testQueryExecutor) ExecuteMutatingQuery(context.Context, string, []models.QueryParameter, map[string]any, int) (*QueryResult, error) {
	return &QueryResult{}, nil
}
func (testQueryExecutor) Close() error { return nil }

func TestFactoryResolvesRegisteredAdapters(t *testing.T) {
	registry := NewRegistry()
	registry.Register(Registration{
		Info: AdapterInfo{Type: "example", DisplayName: "Example"},
		ConnectionTesterFactory: func(context.Context, map[string]any) (ConnectionTester, error) {
			return testConnectionTester{}, nil
		},
		QueryExecutorFactory: func(context.Context, map[string]any) (QueryExecutor, error) {
			return testQueryExecutor{}, nil
		},
	})

	factory := NewFactory(registry)
	tester, err := factory.NewConnectionTester(context.Background(), "example", nil)
	if err != nil {
		t.Fatalf("NewConnectionTester: %v", err)
	}
	if _, ok := tester.(testConnectionTester); !ok {
		t.Fatalf("unexpected tester type: %T", tester)
	}

	executor, err := factory.NewQueryExecutor(context.Background(), "example", nil)
	if err != nil {
		t.Fatalf("NewQueryExecutor: %v", err)
	}
	if _, ok := executor.(testQueryExecutor); !ok {
		t.Fatalf("unexpected executor type: %T", executor)
	}
}

func TestFactoryRejectsUnsupportedType(t *testing.T) {
	factory := NewFactory(NewRegistry())

	_, err := factory.NewConnectionTester(context.Background(), "missing", nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported datasource type") {
		t.Fatalf("expected unsupported type error, got %v", err)
	}
	if factory.SupportsType("missing") {
		t.Fatal("expected missing adapter type to be unsupported")
	}
}

func TestFactoryListTypesReturnsSortedTypes(t *testing.T) {
	registry := NewRegistry()
	registry.Register(Registration{Info: AdapterInfo{Type: "mssql"}})
	registry.Register(Registration{Info: AdapterInfo{Type: "postgres"}})

	got := NewFactory(registry).ListTypes()
	if len(got) != 2 {
		t.Fatalf("expected 2 adapter types, got %#v", got)
	}
	if got[0].Type != "mssql" || got[1].Type != "postgres" {
		t.Fatalf("expected sorted adapter types, got %#v", got)
	}
}
