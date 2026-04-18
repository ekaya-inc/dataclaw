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

func TestManagedQueryCRUDToolCallsRecordSafeSummaries(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), true)

	manager, err := service.CreateAgent(ctx, core.AgentInput{
		Name:                     "Manager",
		CanManageApprovedQueries: true,
	})
	if err != nil {
		t.Fatalf("CreateAgent(manager): %v", err)
	}

	createdPayload := callToolJSONWithHeader(t, ctx, mcpClient, "create_query", map[string]any{
		"natural_language_prompt": "List accounts",
		"sql_query":               "SELECT account_id FROM accounts WHERE account_id = {{account_id}}",
		"parameters": []map[string]any{
			{"name": "account_id", "type": "uuid", "description": "Account identifier", "required": true},
		},
		"output_columns": []map[string]any{
			{"name": "account_id", "type": "uuid", "description": "Account identifier"},
		},
		"constraints": "Only visible to account managers.",
	}, manager.APIKey)
	queryID := requireString(t, asMap(t, createdPayload["query"]), "query_id")

	callToolJSONWithHeader(t, ctx, mcpClient, "update_query", map[string]any{
		"query_id":                queryID,
		"natural_language_prompt": "Rename account",
		"sql_query":               "UPDATE accounts SET account_name = {{account_name}} WHERE account_id = {{account_id}}",
		"allows_modification":     true,
		"parameters": []map[string]any{
			{"name": "account_id", "type": "uuid", "description": "Account identifier", "required": true},
			{"name": "account_name", "type": "string", "description": "New account name", "required": true},
		},
		"output_columns": []map[string]any{
			{"name": "rows_affected", "type": "integer", "description": "Rows changed"},
		},
	}, manager.APIKey)

	callToolJSONWithHeader(t, ctx, mcpClient, "delete_query", map[string]any{"query_id": queryID}, manager.APIKey)

	page, err := service.ListMCPToolEvents(ctx, storepkg.ListMCPToolEventOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListMCPToolEvents: %v", err)
	}
	if page.Total != 3 || len(page.Items) != 3 {
		t.Fatalf("expected 3 recorded CRUD events, got %#v", page)
	}

	createEvent := findToolEvent(t, page.Items, "create_query")
	if got := createEvent.RequestParams["statement_type"]; got != "SELECT" {
		t.Fatalf("expected create_query statement_type=SELECT, got %#v", got)
	}
	if got := createEvent.RequestParams["parameter_count"]; got != 1 && got != float64(1) {
		t.Fatalf("expected create_query parameter_count=1, got %#v", got)
	}
	if _, ok := createEvent.RequestParams["sql_query"]; ok {
		t.Fatalf("expected create_query request summary to omit raw sql, got %#v", createEvent.RequestParams)
	}
	if _, ok := createEvent.RequestParams["parameters"]; ok {
		t.Fatalf("expected create_query request summary to omit raw parameters, got %#v", createEvent.RequestParams)
	}
	if got := createEvent.ResultSummary["query_id"]; got != queryID {
		t.Fatalf("expected create_query result summary query_id=%s, got %#v", queryID, got)
	}
	if got := createEvent.ResultSummary["datasource_present"]; got != true {
		t.Fatalf("expected create_query result summary datasource_present=true, got %#v", got)
	}

	updateEvent := findToolEvent(t, page.Items, "update_query")
	if got := updateEvent.RequestParams["query_id"]; got != queryID {
		t.Fatalf("expected update_query request summary query_id=%s, got %#v", queryID, got)
	}
	if got := updateEvent.RequestParams["statement_type"]; got != "UPDATE" {
		t.Fatalf("expected update_query statement_type=UPDATE, got %#v", got)
	}
	if got := updateEvent.RequestParams["allows_modification"]; got != true {
		t.Fatalf("expected update_query allows_modification=true, got %#v", got)
	}
	if _, ok := updateEvent.RequestParams["sql_query"]; ok {
		t.Fatalf("expected update_query request summary to omit raw sql, got %#v", updateEvent.RequestParams)
	}
	if got := updateEvent.ResultSummary["statement_type"]; got != "UPDATE" {
		t.Fatalf("expected update_query result summary statement_type=UPDATE, got %#v", got)
	}

	deleteEvent := findToolEvent(t, page.Items, "delete_query")
	if got := deleteEvent.RequestParams["query_id"]; got != queryID {
		t.Fatalf("expected delete_query request summary query_id=%s, got %#v", queryID, got)
	}
	if got := deleteEvent.ResultSummary["deleted"]; got != true {
		t.Fatalf("expected delete_query result summary deleted=true, got %#v", got)
	}
	if _, ok := deleteEvent.ResultSummary["sql_query"]; ok {
		t.Fatalf("expected delete_query result summary to omit raw sql, got %#v", deleteEvent.ResultSummary)
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

func findToolEvent(t *testing.T, events []*storepkg.MCPToolEvent, toolName string) *storepkg.MCPToolEvent {
	t.Helper()
	for _, event := range events {
		if event.ToolName == toolName {
			return event
		}
	}
	t.Fatalf("expected tool event %q, got %#v", toolName, events)
	return nil
}
