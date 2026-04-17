package mcpserver

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/ekaya-inc/dataclaw/internal/core"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

func TestSuccessfulTrackedToolCallRecordsBoundedSummaryAndUpdatesLastUsedAt(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), true)

	query, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{NaturalLanguagePrompt: "List accounts", SQLQuery: "SELECT * FROM accounts"})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}
	reader, err := service.CreateAgent(ctx, core.AgentInput{
		Name:               "Reader",
		ApprovedQueryScope: storepkg.ApprovedQueryScopeAll,
	})
	if err != nil {
		t.Fatalf("CreateAgent(reader): %v", err)
	}

	payload := callToolJSONWithHeader(t, ctx, mcpClient, "list_queries", nil, reader.APIKey)
	queries := asSlice(t, payload["queries"])
	if len(queries) != 1 {
		t.Fatalf("expected 1 listed query, got %d", len(queries))
	}

	page, err := service.ListMCPToolEvents(ctx, storepkg.ListMCPToolEventOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListMCPToolEvents: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("expected 1 recorded event, got %#v", page)
	}
	event := page.Items[0]
	if event.ToolName != "list_queries" || event.EventType != storepkg.MCPToolEventTypeCall || !event.WasSuccessful {
		t.Fatalf("unexpected success event: %#v", event)
	}
	if event.AgentName != "Reader" {
		t.Fatalf("expected agent snapshot Reader, got %#v", event.AgentName)
	}
	if got := event.ResultSummary["query_count"]; got != float64(1) && got != 1 {
		t.Fatalf("expected query_count=1, got %#v", got)
	}
	if got := event.ResultSummary["query_ids"]; got == nil {
		t.Fatalf("expected query_ids summary, got %#v", event.ResultSummary)
	}
	if _, ok := event.ResultSummary["queries"]; ok {
		t.Fatalf("expected bounded summary without raw queries payload, got %#v", event.ResultSummary)
	}
	if len(event.RequestParams) != 0 {
		t.Fatalf("expected list_queries request summary to stay empty, got %#v", event.RequestParams)
	}
	if got := event.ResultSummary["query_ids"].([]any)[0]; got != query.ID {
		t.Fatalf("expected query id %s, got %#v", query.ID, got)
	}

	agent, err := service.GetAgent(ctx, reader.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if agent.LastUsedAt == nil {
		t.Fatal("expected last_used_at to update after successful tracked tool call")
	}
}

func TestFailedTrackedToolCallRecordsSanitizedErrorRequestWithoutUpdatingLastUsedAt(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), true)

	queryA, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{NaturalLanguagePrompt: "List accounts", SQLQuery: "SELECT * FROM accounts"})
	if err != nil {
		t.Fatalf("CreateQuery(queryA): %v", err)
	}
	queryB, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{NaturalLanguagePrompt: "List contacts", SQLQuery: "SELECT * FROM contacts"})
	if err != nil {
		t.Fatalf("CreateQuery(queryB): %v", err)
	}
	reader, err := service.CreateAgent(ctx, core.AgentInput{
		Name:               "Reader",
		ApprovedQueryScope: storepkg.ApprovedQueryScopeSelected,
		ApprovedQueryIDs:   []string{queryA.ID},
	})
	if err != nil {
		t.Fatalf("CreateAgent(reader): %v", err)
	}

	assertToolErrorWithHeader(
		t,
		ctx,
		mcpClient,
		"execute_query",
		map[string]any{"query_id": queryB.ID, "parameters": map[string]any{"account_id": "secret-value"}},
		reader.APIKey,
		"agent is not allowed to execute this approved query",
	)

	page, err := service.ListMCPToolEvents(ctx, storepkg.ListMCPToolEventOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListMCPToolEvents: %v", err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("expected 1 recorded error event, got %#v", page)
	}
	event := page.Items[0]
	if event.ToolName != "execute_query" || event.EventType != storepkg.MCPToolEventTypeError || event.WasSuccessful {
		t.Fatalf("unexpected error event: %#v", event)
	}
	if event.ErrorMessage != "agent is not allowed to execute this approved query" {
		t.Fatalf("unexpected error message: %#v", event.ErrorMessage)
	}
	if got := event.RequestParams["query_id"]; got != queryB.ID {
		t.Fatalf("expected request summary query_id=%s, got %#v", queryB.ID, got)
	}
	if got := event.RequestParams["parameter_count"]; got != 1 && got != float64(1) {
		t.Fatalf("expected parameter_count=1, got %#v", got)
	}
	if _, ok := event.RequestParams["parameters"]; ok {
		t.Fatalf("expected sanitized request summary without raw parameter payload, got %#v", event.RequestParams)
	}

	agent, err := service.GetAgent(ctx, reader.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if agent.LastUsedAt != nil {
		t.Fatalf("expected failed tracked tool call not to update last_used_at, got %#v", agent.LastUsedAt)
	}
}

func TestSuccessfulTrackedToolCallStillSucceedsWhenEventPersistenceFails(t *testing.T) {
	ctx := context.Background()
	service, st := newTestMCPService(t, newFakeMCPAdapterFactory(), true)

	query, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{NaturalLanguagePrompt: "List accounts", SQLQuery: "SELECT * FROM accounts"})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}
	reader, err := service.CreateAgent(ctx, core.AgentInput{
		Name:               "Reader",
		ApprovedQueryScope: storepkg.ApprovedQueryScopeAll,
	})
	if err != nil {
		t.Fatalf("CreateAgent(reader): %v", err)
	}
	internalAgent, err := service.AuthenticateAgent(ctx, reader.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent(reader): %v", err)
	}

	mcpServer := New("test", service)
	inProcess, err := client.NewInProcessClient(extractMCPServer(t, mcpServer))
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	if err := inProcess.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	var initReq mcp.InitializeRequest
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test-client", Version: "1.0.0"}
	if _, err := inProcess.Initialize(ctx, initReq); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	defer inProcess.Close()
	defer service.Close()
	defer st.Close()

	if _, err := st.DB().ExecContext(ctx, `DROP TABLE mcp_tool_events`); err != nil {
		t.Fatalf("DROP TABLE mcp_tool_events: %v", err)
	}

	payload := callToolJSON(t, withAuthorizedAgent(ctx, internalAgent), inProcess, "list_queries", nil)
	queries := asSlice(t, payload["queries"])
	if len(queries) != 1 {
		t.Fatalf("expected 1 listed query, got %d", len(queries))
	}
	if got := asMap(t, queries[0])["query_id"]; got != query.ID {
		t.Fatalf("expected query_id=%s, got %#v", query.ID, got)
	}

	agent, err := service.GetAgent(ctx, reader.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if agent.LastUsedAt == nil {
		t.Fatal("expected fallback last_used_at update after event persistence failure")
	}
}

func TestHealthAndListToolsDoNotRecordMCPDashboardEvents(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), true)

	observer, err := service.CreateAgent(ctx, core.AgentInput{Name: "Observer"})
	if err != nil {
		t.Fatalf("CreateAgent(observer): %v", err)
	}

	_ = listToolNamesWithHeader(t, ctx, mcpClient, observer.APIKey)
	callToolJSONWithHeader(t, ctx, mcpClient, "health", nil, observer.APIKey)

	page, err := service.ListMCPToolEvents(ctx, storepkg.ListMCPToolEventOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListMCPToolEvents: %v", err)
	}
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("expected no dashboard events for health/tools list, got %#v", page)
	}

	agent, err := service.GetAgent(ctx, observer.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if agent.LastUsedAt != nil {
		t.Fatalf("expected health/tools list to keep last_used_at empty, got %#v", agent.LastUsedAt)
	}
}
