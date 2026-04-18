package mcpserver

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/core"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

func TestDatasourceInformationToolReturnsStructuredPayloadWithoutTrackingUsage(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), true)

	observer, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent(observer): %v", err)
	}

	payload := callToolJSONWithHeader(t, ctx, mcpClient, datasourceInformationToolName, nil, observer.APIKey)
	if got := requireString(t, payload, "status"); got != "connected" {
		t.Fatalf("expected connected status, got %q", got)
	}
	if got := requireString(t, payload, "name"); got != "Primary" {
		t.Fatalf("expected datasource name Primary, got %q", got)
	}
	if got := requireString(t, payload, "type"); got != "postgres" {
		t.Fatalf("expected datasource type postgres, got %q", got)
	}
	if got := requireString(t, payload, "sql_dialect"); got != "PostgreSQL" {
		t.Fatalf("expected SQL dialect PostgreSQL, got %q", got)
	}
	if got := requireString(t, payload, "database_name"); got != "warehouse" {
		t.Fatalf("expected database_name warehouse, got %q", got)
	}
	if got := requireString(t, payload, "schema_name"); got != "public" {
		t.Fatalf("expected schema_name public, got %q", got)
	}
	if got := requireString(t, payload, "current_user"); got != "analyst" {
		t.Fatalf("expected current_user analyst, got %q", got)
	}
	if got := requireString(t, payload, "version"); got != "PostgreSQL 16.0" {
		t.Fatalf("expected version PostgreSQL 16.0, got %q", got)
	}

	agent, err := service.GetAgent(ctx, observer.ID)
	if err != nil {
		t.Fatalf("GetAgent(observer): %v", err)
	}
	if agent.LastUsedAt != nil {
		t.Fatalf("expected datasource info call not to update last_used_at, got %#v", agent.LastUsedAt)
	}

	page, err := service.ListMCPToolEvents(ctx, storepkg.ListMCPToolEventOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListMCPToolEvents: %v", err)
	}
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("expected datasource info call to avoid MCP tool events, got %#v", page)
	}
}

func TestDatasourceInformationToolReturnsNotConfiguredWhenDatasourceMissing(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), false)

	observer, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent(observer): %v", err)
	}

	payload := callToolJSONWithHeader(t, ctx, mcpClient, datasourceInformationToolName, nil, observer.APIKey)
	if got := requireString(t, payload, "status"); got != "not_configured" {
		t.Fatalf("expected not_configured status, got %q", got)
	}
	if got := requireString(t, payload, "error"); got != "no datasource configured" {
		t.Fatalf("expected no datasource configured error, got %q", got)
	}
	if _, ok := payload["name"]; ok {
		t.Fatalf("expected missing datasource payload to omit name, got %#v", payload)
	}
}

func TestDatasourceInformationToolReturnsErrorPayloadOnIntrospectionFailure(t *testing.T) {
	ctx := context.Background()
	factory := newFakeMCPAdapterFactory()
	factory.newIntrospector = func(context.Context, string, map[string]any) (dsadapter.DatasourceIntrospector, error) {
		return fakeMCPDatasourceIntrospector{err: errors.New("metadata unavailable")}, nil
	}
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, factory, true)

	observer, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent(observer): %v", err)
	}

	payload := callToolJSONWithHeader(t, ctx, mcpClient, datasourceInformationToolName, nil, observer.APIKey)
	if got := requireString(t, payload, "status"); got != "error" {
		t.Fatalf("expected error status, got %q", got)
	}
	if got := requireString(t, payload, "error"); got != "metadata unavailable" {
		t.Fatalf("expected metadata unavailable error, got %q", got)
	}
	if got := requireString(t, payload, "name"); got != "Primary" {
		t.Fatalf("expected partial datasource name Primary, got %q", got)
	}
	if got := requireString(t, payload, "sql_dialect"); got != "PostgreSQL" {
		t.Fatalf("expected partial SQL dialect PostgreSQL, got %q", got)
	}
	if _, ok := payload["database_name"]; ok {
		t.Fatalf("expected runtime metadata to be omitted on failure, got %#v", payload)
	}
}

func TestListToolsEnrichesDatasourceInformationDescription(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), true)

	observer, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent(observer): %v", err)
	}

	description := listToolDescriptionWithHeader(t, ctx, mcpClient, observer.APIKey, datasourceInformationToolName)
	for _, want := range []string{"name=Primary", "type=postgres", "sql_dialect=PostgreSQL", "database=warehouse", "schema=public", "current_user=analyst", "version=PostgreSQL 16.0"} {
		if !strings.Contains(description, want) {
			t.Fatalf("expected datasource info description to contain %q, got %q", want, description)
		}
	}
}

func TestListToolsUpdatesDatasourceInformationDescriptionAfterDatasourceChange(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), true)

	observer, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent(observer): %v", err)
	}

	before := listToolDescriptionWithHeader(t, ctx, mcpClient, observer.APIKey, datasourceInformationToolName)
	if !strings.Contains(before, "name=Primary") || !strings.Contains(before, "database=warehouse") {
		t.Fatalf("unexpected initial datasource description: %q", before)
	}

	if err := service.DeleteDatasource(ctx); err != nil {
		t.Fatalf("DeleteDatasource: %v", err)
	}

	_, err = service.UpsertDatasource(ctx, &storepkg.Datasource{
		Name:     "Replica",
		Type:     "postgres",
		Provider: "postgres",
		Config: map[string]any{
			"host":     "db.example.com",
			"database": "analytics",
			"user":     "readonly",
			"password": "secret",
		},
	})
	if err != nil {
		t.Fatalf("UpsertDatasource: %v", err)
	}

	after := listToolDescriptionWithHeader(t, ctx, mcpClient, observer.APIKey, datasourceInformationToolName)
	if !strings.Contains(after, "name=Replica") || !strings.Contains(after, "database=analytics") || !strings.Contains(after, "current_user=readonly") {
		t.Fatalf("expected updated datasource description, got %q", after)
	}
}

func TestListToolsKeepsCachedDatasourceInformationDescriptionAfterRefreshFailure(t *testing.T) {
	ctx := context.Background()
	factory := newFakeMCPAdapterFactory()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, factory, true)

	observer, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent(observer): %v", err)
	}

	warm := listToolDescriptionWithHeader(t, ctx, mcpClient, observer.APIKey, datasourceInformationToolName)

	previousTTL := datasourceInformationDescriptionTTL
	datasourceInformationDescriptionTTL = -time.Second
	t.Cleanup(func() {
		datasourceInformationDescriptionTTL = previousTTL
	})

	factory.newIntrospector = func(context.Context, string, map[string]any) (dsadapter.DatasourceIntrospector, error) {
		return fakeMCPDatasourceIntrospector{err: errors.New("metadata unavailable")}, nil
	}

	got := listToolDescriptionWithHeader(t, ctx, mcpClient, observer.APIKey, datasourceInformationToolName)
	if got != warm {
		t.Fatalf("expected cached description fallback after refresh failure,\n got: %q\nwant: %q", got, warm)
	}
}

func TestListToolsFallsBackToGenericDatasourceInformationDescriptionOnColdFailure(t *testing.T) {
	ctx := context.Background()
	factory := newFakeMCPAdapterFactory()
	factory.newIntrospector = func(context.Context, string, map[string]any) (dsadapter.DatasourceIntrospector, error) {
		return fakeMCPDatasourceIntrospector{err: errors.New("metadata unavailable")}, nil
	}
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, factory, true)

	observer, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent(observer): %v", err)
	}

	description := listToolDescriptionWithHeader(t, ctx, mcpClient, observer.APIKey, datasourceInformationToolName)
	if description != datasourceInformationDescription {
		t.Fatalf("expected generic datasource info description on cold failure,\n got: %q\nwant: %q", description, datasourceInformationDescription)
	}
}

func listToolDescriptionWithHeader(t *testing.T, ctx context.Context, mcpClient *client.Client, apiKey, toolName string) string {
	t.Helper()

	result, err := mcpClient.ListTools(withHTTPAuth(ctx, apiKey), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools(header): %v", err)
	}
	tool := requireToolByName(t, (*result).Tools, toolName)
	return tool.Description
}
