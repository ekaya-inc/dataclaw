package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/ekaya-inc/dataclaw/internal/core"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

func TestListMCPEventsReturnsPaginatedFilteredResults(t *testing.T) {
	api := newTestAPI(t)
	ctx := t.Context()

	agent, err := api.service.CreateAgent(ctx, core.AgentInput{Name: "Marketing bot"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	agentID := agent.ID

	now := time.Now().UTC()
	seedMCPEvent(t, api, &storepkg.MCPToolEvent{
		ID:            "evt_old",
		AgentID:       stringPtrHTTP(agentID),
		AgentName:     "Marketing bot",
		ToolName:      "query",
		EventType:     storepkg.MCPToolEventTypeCall,
		WasSuccessful: true,
		DurationMs:    9,
		RequestParams: map[string]any{"sql": "SELECT 1"},
		ResultSummary: map[string]any{"row_count": 1},
		CreatedAt:     now.Add(-48 * time.Hour),
	})
	seedMCPEvent(t, api, &storepkg.MCPToolEvent{
		ID:            "evt_error",
		AgentID:       stringPtrHTTP(agentID),
		AgentName:     "Marketing bot",
		ToolName:      "execute_query",
		EventType:     storepkg.MCPToolEventTypeError,
		WasSuccessful: false,
		DurationMs:    18,
		RequestParams: map[string]any{"query_id": "query_1"},
		ResultSummary: map[string]any{},
		ErrorMessage:  "permission denied",
		CreatedAt:     now.Add(-2 * time.Hour),
	})
	seedMCPEvent(t, api, &storepkg.MCPToolEvent{
		ID:            "evt_recent",
		AgentID:       stringPtrHTTP(agentID),
		AgentName:     "Operations bot",
		ToolName:      "execute_query",
		EventType:     storepkg.MCPToolEventTypeCall,
		WasSuccessful: true,
		DurationMs:    12,
		RequestParams: map[string]any{"query_id": "query_2"},
		ResultSummary: map[string]any{"row_count": 2},
		CreatedAt:     now.Add(-30 * time.Minute),
	})

	rec := performJSONRequest(t, api, http.MethodGet, "/api/mcp-events?range=24h&event_type=tool_error&tool_name=execute&agent_name=marketing&limit=1&offset=0", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	data := decodeData(t, rec)
	if got := data["total"]; got != float64(1) {
		t.Fatalf("expected total=1, got %#v", got)
	}
	if got := data["limit"]; got != float64(1) {
		t.Fatalf("expected limit=1, got %#v", got)
	}
	if got := data["offset"]; got != float64(0) {
		t.Fatalf("expected offset=0, got %#v", got)
	}
	items, ok := data["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 result item, got %#v", data["items"])
	}
	item := items[0].(map[string]any)
	if item["id"] != "evt_error" {
		t.Fatalf("expected evt_error, got %#v", item["id"])
	}
	if item["agent_name"] != "Marketing bot" {
		t.Fatalf("expected Marketing bot, got %#v", item["agent_name"])
	}
	if item["tool_name"] != "execute_query" {
		t.Fatalf("expected execute_query, got %#v", item["tool_name"])
	}
}

func TestListMCPEventsReturnsStableEmptyPayload(t *testing.T) {
	api := newTestAPI(t)
	rec := performJSONRequest(t, api, http.MethodGet, "/api/mcp-events?agent_name=missing&limit=25&offset=50", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	data := decodeData(t, rec)
	items, ok := data["items"].([]any)
	if !ok {
		t.Fatalf("expected items slice, got %#v", data["items"])
	}
	if len(items) != 0 {
		t.Fatalf("expected empty items, got %#v", items)
	}
	if data["total"] != float64(0) || data["limit"] != float64(25) || data["offset"] != float64(50) {
		t.Fatalf("unexpected pagination payload: %#v", data)
	}
}

func seedMCPEvent(t *testing.T, api *API, event *storepkg.MCPToolEvent) {
	t.Helper()
	if err := api.service.RecordAgentToolEvent(t.Context(), event, false); err != nil {
		t.Fatalf("RecordMCPToolEvent(%s): %v", event.ID, err)
	}
}

func stringPtrHTTP(value string) *string {
	return &value
}
