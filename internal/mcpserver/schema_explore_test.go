package mcpserver

import (
	"context"
	"errors"
	"testing"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/core"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

func TestSchemaExplorationToolReturnsAdapterContractPayload(t *testing.T) {
	ctx := context.Background()
	var gotRequest dsadapter.SchemaExploreRequest
	factory := newFakeMCPAdapterFactory()
	factory.newSchema = func(context.Context, string, map[string]any) (dsadapter.SchemaExplorer, error) {
		return fakeMCPSchemaExplorer{
			explore: func(_ context.Context, request dsadapter.SchemaExploreRequest) (*dsadapter.SchemaExploreResult, error) {
				gotRequest = request
				return fakeMCPSchemaExplorer{}.ExploreSchema(ctx, request)
			},
		}, nil
	}
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, factory, true)

	reader, err := service.CreateAgent(ctx, core.AgentInput{Name: "Reader", CanQuery: true})
	if err != nil {
		t.Fatalf("CreateAgent(reader): %v", err)
	}

	payload := callToolJSONWithHeader(t, ctx, mcpClient, schemaExplorationToolName, map[string]any{
		"schema_name": " public ",
		"object_name": " accounts ",
		"detail_mode": "full",
	}, reader.APIKey)

	if gotRequest.SchemaName != "public" || gotRequest.ObjectName != "accounts" || gotRequest.DetailMode != dsadapter.SchemaDetailModeFull {
		t.Fatalf("expected normalized adapter request, got %#v", gotRequest)
	}
	if got := requireString(t, payload, "detail_mode"); got != "full" {
		t.Fatalf("expected full detail_mode, got %q", got)
	}
	summary := asMap(t, payload["summary"])
	if got := summary["object_count"]; got != float64(1) && got != 1 {
		t.Fatalf("expected object_count=1, got %#v", summary)
	}
	objects := asSlice(t, payload["objects"])
	if len(objects) != 1 {
		t.Fatalf("expected one schema object, got %#v", objects)
	}
	object := asMap(t, objects[0])
	if got := requireString(t, object, "schema_name"); got != "public" {
		t.Fatalf("expected schema_name public, got %q", got)
	}
	if got := requireString(t, object, "name"); got != "accounts" {
		t.Fatalf("expected object accounts, got %q", got)
	}
	if columns := asSlice(t, object["columns"]); len(columns) != 2 {
		t.Fatalf("expected full mode to include columns, got %#v", object["columns"])
	}
}

func TestSchemaExplorationToolRequiresRawQueryAccess(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), true)

	query, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{
		NaturalLanguagePrompt: "List contacts",
		SQLQuery:              "SELECT contact_id FROM contacts",
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}
	consumer, err := service.CreateAgent(ctx, core.AgentInput{
		Name:               "Consumer",
		ApprovedQueryScope: storepkg.ApprovedQueryScopeSelected,
		ApprovedQueryIDs:   []string{query.ID},
	})
	if err != nil {
		t.Fatalf("CreateAgent(consumer): %v", err)
	}

	tools := listToolNamesWithHeader(t, ctx, mcpClient, consumer.APIKey)
	for _, toolName := range tools {
		if toolName == schemaExplorationToolName {
			t.Fatalf("expected approved-query-only agent not to discover %s, got %v", schemaExplorationToolName, tools)
		}
	}
	assertToolErrorWithHeader(t, ctx, mcpClient, schemaExplorationToolName, nil, consumer.APIKey, "agent is not allowed to explore datasource schema")
}

func TestSchemaExplorationToolReportsUnavailableReason(t *testing.T) {
	ctx := context.Background()
	factory := newFakeMCPAdapterFactory()
	factory.newSchema = func(context.Context, string, map[string]any) (dsadapter.SchemaExplorer, error) {
		return nil, errors.New("metadata unavailable")
	}
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, factory, true)

	reader, err := service.CreateAgent(ctx, core.AgentInput{Name: "Reader", CanQuery: true})
	if err != nil {
		t.Fatalf("CreateAgent(reader): %v", err)
	}

	payload := callToolJSONWithHeader(t, ctx, mcpClient, schemaExplorationToolName, map[string]any{"detail_mode": "full"}, reader.APIKey)
	if got := requireString(t, payload, "unavailable_reason"); got != "metadata unavailable" {
		t.Fatalf("expected unavailable_reason from adapter, got %q", got)
	}
	if got := requireString(t, payload, "detail_mode"); got != "full" {
		t.Fatalf("expected requested detail_mode full in unavailable payload, got %q", got)
	}
}
