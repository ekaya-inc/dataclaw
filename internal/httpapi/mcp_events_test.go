package httpapi

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/ekaya-inc/dataclaw/internal/core"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

var expectedMCPEventCSVHeader = []string{
	"id",
	"agent_id",
	"agent_name",
	"tool_name",
	"event_type",
	"was_successful",
	"duration_ms",
	"request_params",
	"result_summary",
	"error_message",
	"query_name",
	"sql_text",
	"created_at",
}

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
	if _, leaks := item["request_params"]; leaks {
		t.Fatalf("expected list response to omit request_params, got %#v", item)
	}
	if _, leaks := item["error_message"]; leaks {
		t.Fatalf("expected list response to omit error_message, got %#v", item)
	}
	if got := item["has_details"]; got != true {
		t.Fatalf("expected has_details=true for event with error message, got %#v", got)
	}
}

func TestGetMCPEventReturnsFullDetails(t *testing.T) {
	api := newTestAPI(t)
	ctx := t.Context()

	agent, err := api.service.CreateAgent(ctx, core.AgentInput{Name: "Marketing bot"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	agentID := agent.ID

	seedMCPEvent(t, api, &storepkg.MCPToolEvent{
		ID:            "evt_full",
		AgentID:       stringPtrHTTP(agentID),
		AgentName:     "Marketing bot",
		ToolName:      "execute_query",
		EventType:     storepkg.MCPToolEventTypeCall,
		WasSuccessful: true,
		DurationMs:    12,
		RequestParams: map[string]any{"query_id": "query_1"},
		ResultSummary: map[string]any{"row_count": 2},
		QueryName:     "Top accounts",
		SQLText:       "SELECT account_id FROM accounts",
		CreatedAt:     time.Now().UTC(),
	})

	rec := performJSONRequest(t, api, http.MethodGet, "/api/mcp-events/evt_full", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	data := decodeData(t, rec)
	event, ok := data["event"].(map[string]any)
	if !ok {
		t.Fatalf("expected event payload, got %#v", data)
	}
	if event["query_name"] != "Top accounts" {
		t.Fatalf("expected query_name round-trip, got %#v", event["query_name"])
	}
	if event["sql_text"] != "SELECT account_id FROM accounts" {
		t.Fatalf("expected sql_text round-trip, got %#v", event["sql_text"])
	}
	requestParams, ok := event["request_params"].(map[string]any)
	if !ok || requestParams["query_id"] != "query_1" {
		t.Fatalf("expected request_params to round-trip, got %#v", event["request_params"])
	}

	missing := performJSONRequest(t, api, http.MethodGet, "/api/mcp-events/does-not-exist", nil)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing event, got %d", missing.Code)
	}
}

func TestDeleteMCPEventsClearsListAndDetails(t *testing.T) {
	api := newTestAPI(t)
	ctx := t.Context()

	agent, err := api.service.CreateAgent(ctx, core.AgentInput{Name: "Marketing bot"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	agentID := agent.ID

	seedMCPEvent(t, api, &storepkg.MCPToolEvent{
		ID:            "evt_delete_1",
		AgentID:       stringPtrHTTP(agentID),
		AgentName:     "Marketing bot",
		ToolName:      "query",
		EventType:     storepkg.MCPToolEventTypeCall,
		WasSuccessful: true,
		DurationMs:    12,
		RequestParams: map[string]any{"sql": "SELECT 1"},
		ResultSummary: map[string]any{"row_count": 1},
		CreatedAt:     time.Now().UTC().Add(-time.Minute),
	})
	seedMCPEvent(t, api, &storepkg.MCPToolEvent{
		ID:            "evt_delete_2",
		AgentID:       stringPtrHTTP(agentID),
		AgentName:     "Marketing bot",
		ToolName:      "execute_query",
		EventType:     storepkg.MCPToolEventTypeError,
		WasSuccessful: false,
		DurationMs:    34,
		ErrorMessage:  "permission denied",
		CreatedAt:     time.Now().UTC(),
	})

	before := performJSONRequest(t, api, http.MethodGet, "/api/mcp-events?limit=10", nil)
	if before.Code != http.StatusOK {
		t.Fatalf("expected pre-delete list status 200, got %d: %s", before.Code, before.Body.String())
	}
	beforeData := decodeData(t, before)
	if beforeData["total"] != float64(2) {
		t.Fatalf("expected total=2 before delete, got %#v", beforeData["total"])
	}

	deleteRec := performJSONRequest(t, api, http.MethodDelete, "/api/mcp-events", nil)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete status 200, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
	deleteData := decodeData(t, deleteRec)
	if deleteData["deleted"] != true {
		t.Fatalf("expected deleted=true, got %#v", deleteData["deleted"])
	}

	idempotent := performJSONRequest(t, api, http.MethodDelete, "/api/mcp-events", nil)
	if idempotent.Code != http.StatusOK {
		t.Fatalf("expected idempotent delete status 200, got %d: %s", idempotent.Code, idempotent.Body.String())
	}

	after := performJSONRequest(t, api, http.MethodGet, "/api/mcp-events?limit=10", nil)
	if after.Code != http.StatusOK {
		t.Fatalf("expected post-delete list status 200, got %d: %s", after.Code, after.Body.String())
	}
	afterData := decodeData(t, after)
	items, ok := afterData["items"].([]any)
	if !ok {
		t.Fatalf("expected items slice after delete, got %#v", afterData["items"])
	}
	if afterData["total"] != float64(0) || len(items) != 0 {
		t.Fatalf("expected cleared list after delete, got %#v", afterData)
	}

	for _, id := range []string{"evt_delete_1", "evt_delete_2"} {
		missing := performJSONRequest(t, api, http.MethodGet, "/api/mcp-events/"+id, nil)
		if missing.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for %s after delete, got %d: %s", id, missing.Code, missing.Body.String())
		}
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

func TestDownloadMCPEventsReturnsAllFieldsOldestFirstCSV(t *testing.T) {
	api := newTestAPI(t)
	ctx := t.Context()

	agent, err := api.service.CreateAgent(ctx, core.AgentInput{Name: "Marketing bot"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	agentID := agent.ID

	base := time.Date(2026, 4, 17, 17, 0, 0, 0, time.UTC)
	for _, event := range []*storepkg.MCPToolEvent{
		{
			ID:            "evt_recent",
			AgentID:       stringPtrHTTP(agentID),
			AgentName:     "Marketing bot",
			ToolName:      "execute_query",
			EventType:     storepkg.MCPToolEventTypeError,
			WasSuccessful: false,
			DurationMs:    18,
			RequestParams: map[string]any{"query_id": "query_1"},
			ResultSummary: map[string]any{"status": "blocked"},
			ErrorMessage:  "permission denied, retry",
			QueryName:     "Top \"accounts\"",
			SQLText:       "SELECT account_id,\nname FROM accounts",
			CreatedAt:     base.Add(2 * time.Hour),
		},
		{
			ID:            "evt_old",
			AgentName:     "Operations bot",
			ToolName:      "query",
			EventType:     storepkg.MCPToolEventTypeCall,
			WasSuccessful: true,
			DurationMs:    9,
			RequestParams: map[string]any{"sql": "SELECT 1"},
			ResultSummary: map[string]any{"row_count": "1"},
			CreatedAt:     base.Add(-48 * time.Hour),
		},
		{
			ID:            "evt_middle",
			AgentID:       stringPtrHTTP(agentID),
			AgentName:     "Marketing bot",
			ToolName:      "list_queries",
			EventType:     storepkg.MCPToolEventTypeCall,
			WasSuccessful: true,
			DurationMs:    12,
			RequestParams: map[string]any{"scope": "all"},
			ResultSummary: map[string]any{"query_count": "2"},
			QueryName:     "Campaigns",
			SQLText:       "SELECT * FROM campaigns",
			CreatedAt:     base,
		},
	} {
		seedMCPEvent(t, api, event)
	}

	rec := performJSONRequest(t, api, http.MethodGet, "/api/mcp-events.csv?range=24h&event_type=tool_error&limit=1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected download status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !regexp.MustCompile(`(?i)^text/csv\b`).MatchString(got) {
		t.Fatalf("expected text/csv content type, got %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); !regexp.MustCompile(`attachment; filename="dataclaw-mcp-events-\d{8}-\d{6}Z\.csv"`).MatchString(got) {
		t.Fatalf("expected attachment filename with UTC Z timestamp, got %q", got)
	}

	rows := readCSVRows(t, rec.Body.Bytes())
	if len(rows) != 4 {
		t.Fatalf("expected header plus 3 events, got %d rows: %#v", len(rows), rows)
	}
	if !reflect.DeepEqual(rows[0], expectedMCPEventCSVHeader) {
		t.Fatalf("unexpected CSV header:\n got %#v\nwant %#v", rows[0], expectedMCPEventCSVHeader)
	}
	if got := []string{rows[1][0], rows[2][0], rows[3][0]}; !reflect.DeepEqual(got, []string{"evt_old", "evt_middle", "evt_recent"}) {
		t.Fatalf("unexpected CSV row order: %#v", got)
	}

	index := csvHeaderIndex(t, rows[0])
	old := rows[1]
	if old[index["agent_id"]] != "" {
		t.Fatalf("expected nil agent_id to serialize as empty cell, got %q", old[index["agent_id"]])
	}
	if old[index["created_at"]] != base.Add(-48*time.Hour).Format(time.RFC3339) {
		t.Fatalf("unexpected old created_at: %q", old[index["created_at"]])
	}
	recent := rows[3]
	if recent[index["agent_id"]] != agentID {
		t.Fatalf("expected agent_id=%q, got %q", agentID, recent[index["agent_id"]])
	}
	if recent[index["was_successful"]] != "false" || recent[index["duration_ms"]] != "18" {
		t.Fatalf("unexpected status/duration cells: success=%q duration=%q", recent[index["was_successful"]], recent[index["duration_ms"]])
	}
	if recent[index["error_message"]] != "permission denied, retry" || recent[index["query_name"]] != `Top "accounts"` {
		t.Fatalf("unexpected text detail cells: error=%q query=%q", recent[index["error_message"]], recent[index["query_name"]])
	}
	if recent[index["sql_text"]] != "SELECT account_id,\nname FROM accounts" {
		t.Fatalf("expected SQL cell to preserve comma/newline, got %q", recent[index["sql_text"]])
	}
	assertJSONCell(t, recent[index["request_params"]], "query_id", "query_1")
	assertJSONCell(t, recent[index["result_summary"]], "status", "blocked")
}

func TestDownloadMCPEventsNamespaceDoesNotStealDetailIDs(t *testing.T) {
	api := newTestAPI(t)
	seedMCPEvent(t, api, &storepkg.MCPToolEvent{
		ID:            "download",
		AgentName:     "Marketing bot",
		ToolName:      "query",
		EventType:     storepkg.MCPToolEventTypeCall,
		WasSuccessful: true,
		DurationMs:    12,
		RequestParams: map[string]any{"sql": "SELECT 1"},
		ResultSummary: map[string]any{"row_count": "1"},
		CreatedAt:     time.Date(2026, 4, 17, 17, 0, 0, 0, time.UTC),
	})

	detailRec := performJSONRequest(t, api, http.MethodGet, "/api/mcp-events/download", nil)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("expected detail status 200, got %d: %s", detailRec.Code, detailRec.Body.String())
	}
	event := decodeData(t, detailRec)["event"].(map[string]any)
	if event["id"] != "download" {
		t.Fatalf("expected detail route to return event id=download, got %#v", event["id"])
	}

	csvRec := performJSONRequest(t, api, http.MethodGet, "/api/mcp-events.csv", nil)
	if csvRec.Code != http.StatusOK {
		t.Fatalf("expected CSV status 200, got %d: %s", csvRec.Code, csvRec.Body.String())
	}
	if got := csvRec.Header().Get("Content-Type"); !regexp.MustCompile(`(?i)^text/csv\b`).MatchString(got) {
		t.Fatalf("expected CSV route to return text/csv, got %q", got)
	}
	rows := readCSVRows(t, csvRec.Body.Bytes())
	if len(rows) != 2 || rows[1][0] != "download" {
		t.Fatalf("expected CSV route to export event id=download, got %#v", rows)
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

func readCSVRows(t *testing.T, raw []byte) [][]string {
	t.Helper()
	rows, err := csv.NewReader(bytes.NewReader(raw)).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v\n%s", err, string(raw))
	}
	return rows
}

func csvHeaderIndex(t *testing.T, header []string) map[string]int {
	t.Helper()
	index := make(map[string]int, len(header))
	for position, name := range header {
		index[name] = position
	}
	for _, name := range expectedMCPEventCSVHeader {
		if _, ok := index[name]; !ok {
			t.Fatalf("missing CSV header %q in %#v", name, header)
		}
	}
	return index
}

func assertJSONCell(t *testing.T, raw, key string, want any) {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("parse JSON cell %q: %v", raw, err)
	}
	if got := decoded[key]; got != want {
		t.Fatalf("expected JSON cell %s=%#v, got %#v in %#v", key, want, got, decoded)
	}
}
