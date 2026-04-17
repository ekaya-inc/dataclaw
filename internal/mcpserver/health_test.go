package mcpserver

import (
	"context"
	"errors"
	"strings"
	"testing"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/core"
	"github.com/ekaya-inc/dataclaw/pkg/models"
)

type fakeMCPAdapterFactory struct {
	supported map[string]bool
	newTester func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error)
	newQuery  func(context.Context, string, map[string]any) (dsadapter.QueryExecutor, error)
	typeInfo  map[string]dsadapter.AdapterInfo
}

type fakeMCPConnectionTester struct {
	test func(context.Context) error
}

type fakeMCPQueryExecutor struct{}

func newFakeMCPAdapterFactory() *fakeMCPAdapterFactory {
	return &fakeMCPAdapterFactory{
		supported: map[string]bool{"postgres": true},
		newTester: func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error) {
			return fakeMCPConnectionTester{}, nil
		},
		newQuery: func(context.Context, string, map[string]any) (dsadapter.QueryExecutor, error) {
			return fakeMCPQueryExecutor{}, nil
		},
		typeInfo: map[string]dsadapter.AdapterInfo{
			"postgres": {
				Type:        "postgres",
				DisplayName: "PostgreSQL",
				SQLDialect:  "PostgreSQL",
			},
		},
	}
}

func (f *fakeMCPAdapterFactory) NewConnectionTester(ctx context.Context, dsType string, config map[string]any) (dsadapter.ConnectionTester, error) {
	if !f.SupportsType(dsType) {
		return nil, errors.New("unsupported datasource type: " + dsType)
	}
	return f.newTester(ctx, dsType, config)
}

func (f *fakeMCPAdapterFactory) NewQueryExecutor(ctx context.Context, dsType string, config map[string]any) (dsadapter.QueryExecutor, error) {
	if !f.SupportsType(dsType) {
		return nil, errors.New("unsupported datasource type: " + dsType)
	}
	return f.newQuery(ctx, dsType, config)
}

func (f *fakeMCPAdapterFactory) ConfigFingerprint(dsType string, config map[string]any) (string, error) {
	if !f.SupportsType(dsType) {
		return "", errors.New("unsupported datasource type: " + dsType)
	}
	return dsadapter.CanonicalFingerprint(config)
}

func (f *fakeMCPAdapterFactory) ListTypes() []dsadapter.AdapterInfo {
	types := make([]dsadapter.AdapterInfo, 0, len(f.typeInfo))
	for dsType, info := range f.typeInfo {
		if f.supported[dsType] {
			types = append(types, info)
		}
	}
	return types
}

func (f *fakeMCPAdapterFactory) TypeInfo(dsType string) (dsadapter.AdapterInfo, bool) {
	info, ok := f.typeInfo[dsType]
	return info, ok
}

func (f *fakeMCPAdapterFactory) SupportsType(dsType string) bool {
	return f != nil && f.supported[dsType]
}

func (f fakeMCPConnectionTester) TestConnection(ctx context.Context) error {
	if f.test != nil {
		return f.test(ctx)
	}
	return nil
}

func (f fakeMCPConnectionTester) Close() error { return nil }

func (fakeMCPQueryExecutor) Query(context.Context, string, int) (*dsadapter.QueryResult, error) {
	return nil, errors.New("unexpected Query call")
}

func (fakeMCPQueryExecutor) QueryWithParameters(context.Context, string, []models.QueryParameter, map[string]any, int) (*dsadapter.QueryResult, error) {
	return nil, errors.New("unexpected QueryWithParameters call")
}

func (fakeMCPQueryExecutor) ExecuteMutatingQuery(context.Context, string, []models.QueryParameter, map[string]any, int) (*dsadapter.QueryResult, error) {
	return nil, errors.New("unexpected ExecuteMutatingQuery call")
}

func (fakeMCPQueryExecutor) Close() error { return nil }

func TestZeroPermissionAgentGetsHealthOnly(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newTestMCPClient(t)

	agentView, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	agent, err := service.AuthenticateAgent(ctx, agentView.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent: %v", err)
	}

	if got, want := listToolNames(t, withAuthorizedAgent(ctx, agent), mcpClient), []string{"health"}; !equalStrings(got, want) {
		t.Fatalf("unexpected zero-permission tool list: got %v want %v", got, want)
	}
}

func TestHealthToolReportsDatasourceNotConfigured(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newTestMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), false)

	agentView, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	agent, err := service.AuthenticateAgent(ctx, agentView.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent: %v", err)
	}

	payload := callToolJSON(t, withAuthorizedAgent(ctx, agent), mcpClient, "health", nil)
	if got := requireString(t, payload, "engine"); got != "healthy" {
		t.Fatalf("expected healthy engine, got %q", got)
	}
	if got := requireString(t, payload, "version"); got != "test" {
		t.Fatalf("expected version test, got %q", got)
	}
	datasource := asMap(t, payload["datasource"])
	if got := requireString(t, datasource, "status"); got != "not_configured" {
		t.Fatalf("expected datasource status not_configured, got %q", got)
	}
	if got := requireString(t, datasource, "error"); got != "no datasource configured" {
		t.Fatalf("expected no datasource configured error, got %q", got)
	}
}

func TestHealthToolReportsDatasourceConnected(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newTestMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), true)

	agentView, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	agent, err := service.AuthenticateAgent(ctx, agentView.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent: %v", err)
	}

	payload := callToolJSON(t, withAuthorizedAgent(ctx, agent), mcpClient, "health", nil)
	datasource := asMap(t, payload["datasource"])
	if got := requireString(t, datasource, "name"); got != "Primary" {
		t.Fatalf("expected datasource name Primary, got %q", got)
	}
	if got := requireString(t, datasource, "type"); got != "postgres" {
		t.Fatalf("expected datasource type postgres, got %q", got)
	}
	if got := requireString(t, datasource, "status"); got != "connected" {
		t.Fatalf("expected datasource status connected, got %q", got)
	}
}

func TestHealthToolReportsDatasourceConnectionError(t *testing.T) {
	ctx := context.Background()
	factory := newFakeMCPAdapterFactory()
	factory.newTester = func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error) {
		return fakeMCPConnectionTester{test: func(context.Context) error {
			return errors.New("connection refused")
		}}, nil
	}
	mcpClient, service := newTestMCPClientWithFactoryAndDatasource(t, factory, true)

	agentView, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	agent, err := service.AuthenticateAgent(ctx, agentView.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent: %v", err)
	}

	payload := callToolJSON(t, withAuthorizedAgent(ctx, agent), mcpClient, "health", nil)
	datasource := asMap(t, payload["datasource"])
	if got := requireString(t, datasource, "name"); got != "Primary" {
		t.Fatalf("expected datasource name Primary, got %q", got)
	}
	if got := requireString(t, datasource, "type"); got != "postgres" {
		t.Fatalf("expected datasource type postgres, got %q", got)
	}
	if got := requireString(t, datasource, "status"); got != "error" {
		t.Fatalf("expected datasource status error, got %q", got)
	}
	if got := requireString(t, datasource, "error"); got != "connection refused" {
		t.Fatalf("expected connection refused error, got %q", got)
	}
}

func TestHealthToolReportsDatasourceTimeout(t *testing.T) {
	ctx := context.Background()
	factory := newFakeMCPAdapterFactory()
	factory.newTester = func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error) {
		return fakeMCPConnectionTester{test: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}}, nil
	}
	mcpClient, service := newTestMCPClientWithFactoryAndDatasource(t, factory, true)

	agentView, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	agent, err := service.AuthenticateAgent(ctx, agentView.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent: %v", err)
	}

	payload := callToolJSON(t, withAuthorizedAgent(ctx, agent), mcpClient, "health", nil)
	datasource := asMap(t, payload["datasource"])
	if got := requireString(t, datasource, "status"); got != "error" {
		t.Fatalf("expected datasource status error, got %q", got)
	}
	if got := requireString(t, datasource, "name"); got != "Primary" {
		t.Fatalf("expected datasource name Primary, got %q", got)
	}
	if got := requireString(t, datasource, "type"); got != "postgres" {
		t.Fatalf("expected datasource type postgres, got %q", got)
	}
	if got := requireString(t, datasource, "error"); !strings.Contains(got, "timed out") {
		t.Fatalf("expected timeout error text, got %q", got)
	}
}

func TestHealthToolCallDoesNotUpdateLastUsedAt(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), true)

	agentView, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	callToolJSONWithHeader(t, ctx, mcpClient, "health", nil, agentView.APIKey)

	agent, err := service.GetAgent(ctx, agentView.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if agent.LastUsedAt != nil {
		t.Fatalf("expected health call not to update last_used_at, got %#v", agent.LastUsedAt)
	}
}

var _ dsadapter.Factory = (*fakeMCPAdapterFactory)(nil)
var _ dsadapter.ConnectionTester = fakeMCPConnectionTester{}
var _ dsadapter.QueryExecutor = fakeMCPQueryExecutor{}
