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
		SQLText:       "SELECT * FROM campaigns",
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
		QueryName:     "Top campaigns",
		SQLText:       "SELECT id FROM campaigns",
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

	executeSummary := findSummaryByID(t, page.Items, "evt_execute")
	if !executeSummary.HasDetails {
		t.Fatalf("expected evt_execute summary to report has_details=true")
	}
	listSummary := findSummaryByID(t, page.Items, "evt_list")
	if !listSummary.HasDetails {
		t.Fatalf("expected evt_list summary to report has_details=true (result summary populated)")
	}

	fullExecute, err := store.GetMCPToolEvent(ctx, "evt_execute")
	if err != nil || fullExecute == nil {
		t.Fatalf("GetMCPToolEvent(evt_execute): %v, %v", fullExecute, err)
	}
	if fullExecute.ErrorMessage != "permission denied" {
		t.Fatalf("expected permission denied, got %#v", fullExecute.ErrorMessage)
	}
	if fullExecute.QueryName != "Top campaigns" || fullExecute.SQLText != "SELECT id FROM campaigns" {
		t.Fatalf("expected query_name/sql_text round-trip, got %#v / %#v", fullExecute.QueryName, fullExecute.SQLText)
	}
	if got := fullExecute.RequestParams["query_id"]; got != "query_1" {
		t.Fatalf("expected query_id=query_1, got %#v", got)
	}

	missing, err := store.GetMCPToolEvent(ctx, "does-not-exist")
	if err != nil {
		t.Fatalf("GetMCPToolEvent(missing): %v", err)
	}
	if missing != nil {
		t.Fatalf("expected nil for missing event, got %#v", missing)
	}
}

func TestClearMCPToolEventsDeletesAllEventsAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defer store.Close()

	base := time.Date(2026, 4, 17, 17, 0, 0, 0, time.UTC)
	for _, event := range []*MCPToolEvent{
		{
			ID:            "evt_clear_1",
			AgentName:     "Marketing bot",
			ToolName:      "query",
			EventType:     MCPToolEventTypeCall,
			WasSuccessful: true,
			DurationMs:    12,
			RequestParams: map[string]any{"sql": "SELECT 1"},
			ResultSummary: map[string]any{"row_count": 1},
			CreatedAt:     base,
		},
		{
			ID:            "evt_clear_2",
			AgentName:     "Support bot",
			ToolName:      "execute_query",
			EventType:     MCPToolEventTypeError,
			WasSuccessful: false,
			DurationMs:    34,
			ErrorMessage:  "permission denied",
			CreatedAt:     base.Add(time.Minute),
		},
	} {
		if err := store.RecordMCPToolEvent(ctx, event, nil); err != nil {
			t.Fatalf("RecordMCPToolEvent(%s): %v", event.ID, err)
		}
	}

	if err := store.ClearMCPToolEvents(ctx); err != nil {
		t.Fatalf("ClearMCPToolEvents: %v", err)
	}
	if err := store.ClearMCPToolEvents(ctx); err != nil {
		t.Fatalf("ClearMCPToolEvents(idempotent): %v", err)
	}

	page, err := store.ListMCPToolEvents(ctx, ListMCPToolEventOptions{Limit: 10})
	if err != nil {
		t.Fatalf("ListMCPToolEvents(after clear): %v", err)
	}
	if page.Total != 0 || len(page.Items) != 0 {
		t.Fatalf("expected cleared event list, got total=%d items=%#v", page.Total, page.Items)
	}
	for _, id := range []string{"evt_clear_1", "evt_clear_2"} {
		event, err := store.GetMCPToolEvent(ctx, id)
		if err != nil {
			t.Fatalf("GetMCPToolEvent(%s after clear): %v", id, err)
		}
		if event != nil {
			t.Fatalf("expected %s to be deleted, got %#v", id, event)
		}
	}
}

func TestExportMCPToolEventsReturnsAllDetailsOldestFirst(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defer store.Close()

	agent := &Agent{Name: "Marketing bot", APIKeyEncrypted: "encrypted-key", ApprovedQueryScope: ApprovedQueryScopeNone}
	if err := store.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	base := time.Date(2026, 4, 17, 17, 0, 0, 0, time.UTC)
	for _, event := range []*MCPToolEvent{
		{
			ID:            "evt_recent",
			AgentName:     "Operations bot",
			ToolName:      "execute_query",
			EventType:     MCPToolEventTypeCall,
			WasSuccessful: true,
			DurationMs:    33,
			RequestParams: map[string]any{"query_id": "query_recent"},
			ResultSummary: map[string]any{"row_count": "2"},
			CreatedAt:     base.Add(3 * time.Minute),
		},
		{
			ID:            "evt_tie_b",
			AgentID:       stringPtr(agent.ID),
			AgentName:     "Marketing bot",
			ToolName:      "query",
			EventType:     MCPToolEventTypeCall,
			WasSuccessful: true,
			DurationMs:    12,
			RequestParams: map[string]any{"sql": "SELECT * FROM campaigns"},
			ResultSummary: map[string]any{"row_count": "3"},
			QueryName:     "Campaigns",
			SQLText:       "SELECT * FROM campaigns",
			CreatedAt:     base,
		},
		{
			ID:            "evt_tie_a",
			AgentID:       stringPtr(agent.ID),
			AgentName:     "Marketing bot",
			ToolName:      "execute_query",
			EventType:     MCPToolEventTypeError,
			WasSuccessful: false,
			DurationMs:    44,
			RequestParams: map[string]any{"query_id": "query_1"},
			ResultSummary: map[string]any{"status": "blocked"},
			ErrorMessage:  "permission denied",
			QueryName:     "Top campaigns",
			SQLText:       "SELECT id FROM campaigns",
			CreatedAt:     base,
		},
	} {
		if err := store.RecordMCPToolEvent(ctx, event, nil); err != nil {
			t.Fatalf("RecordMCPToolEvent(%s): %v", event.ID, err)
		}
	}

	events, err := store.ExportMCPToolEvents(ctx)
	if err != nil {
		t.Fatalf("ExportMCPToolEvents: %v", err)
	}
	if got, want := len(events), 3; got != want {
		t.Fatalf("expected %d exported events, got %d", want, got)
	}
	gotOrder := []string{events[0].ID, events[1].ID, events[2].ID}
	wantOrder := []string{"evt_tie_a", "evt_tie_b", "evt_recent"}
	for index, want := range wantOrder {
		if gotOrder[index] != want {
			t.Fatalf("unexpected export order: got %#v want %#v", gotOrder, wantOrder)
		}
	}

	first := events[0]
	if first.AgentID == nil || *first.AgentID != agent.ID {
		t.Fatalf("expected agent_id round-trip, got %#v", first.AgentID)
	}
	if first.AgentName != "Marketing bot" || first.ToolName != "execute_query" || first.EventType != MCPToolEventTypeError {
		t.Fatalf("unexpected event identity fields: %#v", first)
	}
	if first.WasSuccessful || first.DurationMs != 44 {
		t.Fatalf("unexpected status/duration fields: success=%t duration=%d", first.WasSuccessful, first.DurationMs)
	}
	if first.RequestParams["query_id"] != "query_1" || first.ResultSummary["status"] != "blocked" {
		t.Fatalf("expected JSON details to round-trip, got params=%#v summary=%#v", first.RequestParams, first.ResultSummary)
	}
	if first.ErrorMessage != "permission denied" || first.QueryName != "Top campaigns" || first.SQLText != "SELECT id FROM campaigns" {
		t.Fatalf("expected text details to round-trip, got error=%q query=%q sql=%q", first.ErrorMessage, first.QueryName, first.SQLText)
	}
	if !first.CreatedAt.Equal(base) {
		t.Fatalf("expected created_at=%s, got %s", base.Format(time.RFC3339), first.CreatedAt.Format(time.RFC3339))
	}
	if events[2].AgentID != nil {
		t.Fatalf("expected nil agent_id for evt_recent, got %#v", events[2].AgentID)
	}
}

func TestExportMCPToolEventsEmpty(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defer store.Close()

	events, err := store.ExportMCPToolEvents(ctx)
	if err != nil {
		t.Fatalf("ExportMCPToolEvents(empty): %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected no exported events, got %#v", events)
	}
}

func findSummaryByID(t *testing.T, items []*MCPToolEventSummary, id string) *MCPToolEventSummary {
	t.Helper()
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("expected summary %q, got %#v", id, items)
	return nil
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
