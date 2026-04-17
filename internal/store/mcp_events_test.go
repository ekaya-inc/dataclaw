package store

import (
	"context"
	"testing"
	"time"
)

func TestFreshSchemaIncludesMCPToolEventsTableAndIndexes(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defer store.Close()

	assertSchemaObjectCount(t, ctx, store, "table", "mcp_tool_events", 1)
	assertSchemaObjectCount(t, ctx, store, "index", "idx_mcp_tool_events_created_at", 1)
	assertSchemaObjectCount(t, ctx, store, "index", "idx_mcp_tool_events_agent_name_created_at", 1)
	assertSchemaObjectCount(t, ctx, store, "index", "idx_mcp_tool_events_tool_name_created_at", 1)
	assertSchemaObjectCount(t, ctx, store, "index", "idx_mcp_tool_events_event_type_created_at", 1)
}

func TestListMCPToolEventsSupportsFiltersPaginationAndSnapshots(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defer store.Close()

	agent := &Agent{Name: "Marketing bot", APIKeyEncrypted: "encrypted-key", ApprovedQueryScope: ApprovedQueryScopeNone}
	if err := store.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	base := time.Date(2026, 4, 17, 17, 0, 0, 0, time.UTC)
	recordedAt := base.Add(2 * time.Minute)
	if err := store.RecordMCPToolEvent(ctx, &MCPToolEvent{
		ID:            "evt_query",
		AgentID:       stringPtr(agent.ID),
		AgentName:     agent.Name,
		ToolName:      "query",
		EventType:     MCPToolEventTypeCall,
		WasSuccessful: true,
		DurationMs:    12,
		RequestParams: map[string]any{"sql": "SELECT * FROM campaigns"},
		ResultSummary: map[string]any{"row_count": 3},
		CreatedAt:     recordedAt,
	}, &recordedAt); err != nil {
		t.Fatalf("RecordMCPToolEvent(query): %v", err)
	}
	storedAgent, err := store.GetAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetAgent(after query event): %v", err)
	}
	if storedAgent.LastUsedAt == nil || !storedAgent.LastUsedAt.Equal(recordedAt) {
		t.Fatalf("expected last_used_at=%s, got %#v", recordedAt.Format(time.RFC3339), storedAgent.LastUsedAt)
	}

	storedAgent.Name = "Renamed marketing bot"
	if err := store.UpdateAgent(ctx, storedAgent); err != nil {
		t.Fatalf("UpdateAgent(rename): %v", err)
	}

	if err := store.RecordMCPToolEvent(ctx, &MCPToolEvent{
		ID:            "evt_execute",
		AgentID:       stringPtr(agent.ID),
		AgentName:     "Renamed marketing bot",
		ToolName:      "execute_query",
		EventType:     MCPToolEventTypeError,
		WasSuccessful: false,
		DurationMs:    44,
		RequestParams: map[string]any{"query_id": "query_1"},
		ResultSummary: map[string]any{},
		ErrorMessage:  "permission denied",
		CreatedAt:     base.Add(3 * time.Minute),
	}, nil); err != nil {
		t.Fatalf("RecordMCPToolEvent(execute_query): %v", err)
	}

	if err := store.RecordMCPToolEvent(ctx, &MCPToolEvent{
		ID:            "evt_list",
		AgentID:       stringPtr(agent.ID),
		AgentName:     "Support bot",
		ToolName:      "list_queries",
		EventType:     MCPToolEventTypeCall,
		WasSuccessful: true,
		DurationMs:    6,
		RequestParams: map[string]any{},
		ResultSummary: map[string]any{"count": 2},
		CreatedAt:     base.Add(4 * time.Minute),
	}, nil); err != nil {
		t.Fatalf("RecordMCPToolEvent(list_queries): %v", err)
	}

	page, err := store.ListMCPToolEvents(ctx, ListMCPToolEventOptions{Limit: 2})
	if err != nil {
		t.Fatalf("ListMCPToolEvents(page): %v", err)
	}
	if page.Total != 3 {
		t.Fatalf("expected total=3, got %d", page.Total)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 paginated items, got %d", len(page.Items))
	}
	if page.Items[0].ID != "evt_list" || page.Items[1].ID != "evt_execute" {
		t.Fatalf("unexpected page order: %#v", []string{page.Items[0].ID, page.Items[1].ID})
	}

	errorPage, err := store.ListMCPToolEvents(ctx, ListMCPToolEventOptions{EventType: MCPToolEventTypeError, Limit: 10})
	if err != nil {
		t.Fatalf("ListMCPToolEvents(error filter): %v", err)
	}
	if errorPage.Total != 1 || len(errorPage.Items) != 1 || errorPage.Items[0].ID != "evt_execute" {
		t.Fatalf("unexpected error-filter results: %#v", errorPage.Items)
	}

	toolPage, err := store.ListMCPToolEvents(ctx, ListMCPToolEventOptions{ToolName: "execute", Limit: 10})
	if err != nil {
		t.Fatalf("ListMCPToolEvents(tool filter): %v", err)
	}
	if toolPage.Total != 1 || len(toolPage.Items) != 1 || toolPage.Items[0].ToolName != "execute_query" {
		t.Fatalf("unexpected tool-filter results: %#v", toolPage.Items)
	}

	agentPage, err := store.ListMCPToolEvents(ctx, ListMCPToolEventOptions{AgentName: "marketing", Limit: 10})
	if err != nil {
		t.Fatalf("ListMCPToolEvents(agent filter): %v", err)
	}
	if agentPage.Total != 2 {
		t.Fatalf("expected 2 marketing events, got %d", agentPage.Total)
	}
	if agentPage.Items[1].AgentName != "Marketing bot" {
		t.Fatalf("expected original agent snapshot to persist, got %#v", agentPage.Items[1].AgentName)
	}

	since := base.Add(3*time.Minute - time.Second)
	rangePage, err := store.ListMCPToolEvents(ctx, ListMCPToolEventOptions{Since: &since, Limit: 10})
	if err != nil {
		t.Fatalf("ListMCPToolEvents(range filter): %v", err)
	}
	if rangePage.Total != 2 {
		t.Fatalf("expected 2 ranged events, got %d", rangePage.Total)
	}
}

func assertSchemaObjectCount(t *testing.T, ctx context.Context, store *Store, objectType, name string, want int) {
	t.Helper()
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = ? AND name = ?`, objectType, name).Scan(&count); err != nil {
		t.Fatalf("query %s %s presence: %v", objectType, name, err)
	}
	if count != want {
		t.Fatalf("expected %s %s count=%d, got %d", objectType, name, want, count)
	}
}

func stringPtr(value string) *string {
	return &value
}
