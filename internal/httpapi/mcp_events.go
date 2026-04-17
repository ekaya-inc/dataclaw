package httpapi

import (
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
