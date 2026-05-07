package httpapi

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

const (
	defaultMCPEventsLimit = 50
	maxMCPEventsLimit     = 100
)

var mcpEventCSVHeader = []string{
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

func (a *API) handleListMCPEvents(w http.ResponseWriter, r *http.Request) {
	options, err := parseMCPToolEventOptions(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response{Error: err.Error()})
		return
	}
	page, err := a.service.ListMCPToolEvents(r.Context(), options)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: page})
}

func (a *API) handleGetMCPEvent(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/mcp-events/"), "/"))
	if id == "" {
		writeJSON(w, http.StatusNotFound, response{Error: "event id required"})
		return
	}
	event, err := a.service.GetMCPToolEvent(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	if event == nil {
		writeJSON(w, http.StatusNotFound, response{Error: "event not found"})
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"event": event}})
}

func (a *API) handleDownloadMCPEvents(w http.ResponseWriter, r *http.Request) {
	events, err := a.service.ExportMCPToolEvents(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	var body bytes.Buffer
	if err := writeMCPEventsCSV(&body, events); err != nil {
		writeError(w, err)
		return
	}

	filename := "dataclaw-mcp-events-" + time.Now().UTC().Format("20060102-150405Z") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body.Bytes())
}

func (a *API) handleClearMCPEvents(w http.ResponseWriter, r *http.Request) {
	if err := a.service.ClearMCPToolEvents(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"deleted": true}})
}

func writeMCPEventsCSV(w *bytes.Buffer, events []*storepkg.MCPToolEvent) error {
	writer := csv.NewWriter(w)
	if err := writer.Write(mcpEventCSVHeader); err != nil {
		return err
	}
	for _, event := range events {
		record, err := mcpEventCSVRecord(event)
		if err != nil {
			return err
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

func mcpEventCSVRecord(event *storepkg.MCPToolEvent) ([]string, error) {
	if event == nil {
		return nil, errors.New("mcp event is required")
	}
	requestParams, err := mcpEventJSONCell(event.RequestParams)
	if err != nil {
		return nil, fmt.Errorf("marshal request params for %s: %w", event.ID, err)
	}
	resultSummary, err := mcpEventJSONCell(event.ResultSummary)
	if err != nil {
		return nil, fmt.Errorf("marshal result summary for %s: %w", event.ID, err)
	}

	agentID := ""
	if event.AgentID != nil {
		agentID = strings.TrimSpace(*event.AgentID)
	}
	return []string{
		event.ID,
		agentID,
		event.AgentName,
		event.ToolName,
		string(event.EventType),
		strconv.FormatBool(event.WasSuccessful),
		strconv.Itoa(event.DurationMs),
		requestParams,
		resultSummary,
		event.ErrorMessage,
		event.QueryName,
		event.SQLText,
		event.CreatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func mcpEventJSONCell(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func parseMCPToolEventOptions(r *http.Request) (storepkg.ListMCPToolEventOptions, error) {
	query := r.URL.Query()
	limit, err := parseBoundedInt(query.Get("limit"), defaultMCPEventsLimit, maxMCPEventsLimit)
	if err != nil {
		return storepkg.ListMCPToolEventOptions{}, err
	}
	offset, err := parseOffset(query.Get("offset"))
	if err != nil {
		return storepkg.ListMCPToolEventOptions{}, err
	}
	eventType := strings.TrimSpace(query.Get("event_type"))
	if eventType != "" && eventType != string(storepkg.MCPToolEventTypeCall) && eventType != string(storepkg.MCPToolEventTypeError) {
		return storepkg.ListMCPToolEventOptions{}, fmt.Errorf("event_type must be one of %s or %s", storepkg.MCPToolEventTypeCall, storepkg.MCPToolEventTypeError)
	}
	since, err := parseMCPEventRange(strings.TrimSpace(query.Get("range")))
	if err != nil {
		return storepkg.ListMCPToolEventOptions{}, err
	}
	return storepkg.ListMCPToolEventOptions{
		Since:     since,
		EventType: storepkg.MCPToolEventType(eventType),
		ToolName:  query.Get("tool_name"),
		AgentName: query.Get("agent_name"),
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func parseMCPEventRange(value string) (*time.Time, error) {
	if value == "" || value == "all" {
		return nil, nil
	}
	now := time.Now().UTC()
	switch value {
	case "24h":
		since := now.Add(-24 * time.Hour)
		return &since, nil
	case "7d":
		since := now.Add(-7 * 24 * time.Hour)
		return &since, nil
	case "30d":
		since := now.Add(-30 * 24 * time.Hour)
		return &since, nil
	default:
		return nil, errors.New("range must be one of 24h, 7d, 30d, or all")
	}
}

func parseBoundedInt(raw string, fallback, max int) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value %q", raw)
	}
	if value <= 0 {
		return 0, fmt.Errorf("value must be greater than 0")
	}
	if value > max {
		return max, nil
	}
	return value, nil
}

func parseOffset(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value %q", raw)
	}
	if value < 0 {
		return 0, fmt.Errorf("offset must be 0 or greater")
	}
	return value, nil
}
