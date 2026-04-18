package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type MCPToolEventType string

const (
	MCPToolEventTypeCall  MCPToolEventType = "tool_call"
	MCPToolEventTypeError MCPToolEventType = "tool_error"
)

type MCPToolEvent struct {
	ID            string           `json:"id"`
	AgentID       *string          `json:"agent_id"`
	AgentName     string           `json:"agent_name"`
	ToolName      string           `json:"tool_name"`
	EventType     MCPToolEventType `json:"event_type"`
	WasSuccessful bool             `json:"was_successful"`
	DurationMs    int              `json:"duration_ms"`
	RequestParams map[string]any   `json:"request_params"`
	ResultSummary map[string]any   `json:"result_summary"`
	ErrorMessage  string           `json:"error_message"`
	QueryName     string           `json:"query_name"`
	SQLText       string           `json:"sql_text"`
	CreatedAt     time.Time        `json:"created_at"`
}

type MCPToolEventSummary struct {
	ID            string           `json:"id"`
	AgentID       *string          `json:"agent_id"`
	AgentName     string           `json:"agent_name"`
	ToolName      string           `json:"tool_name"`
	EventType     MCPToolEventType `json:"event_type"`
	WasSuccessful bool             `json:"was_successful"`
	DurationMs    int              `json:"duration_ms"`
	HasDetails    bool             `json:"has_details"`
	CreatedAt     time.Time        `json:"created_at"`
}

type ListMCPToolEventOptions struct {
	Since     *time.Time
	EventType MCPToolEventType
	ToolName  string
	AgentName string
	Limit     int
	Offset    int
}

type MCPToolEventPage struct {
	Items  []*MCPToolEventSummary `json:"items"`
	Total  int                    `json:"total"`
	Limit  int                    `json:"limit"`
	Offset int                    `json:"offset"`
}

const mcpToolEventSummaryColumns = `id, agent_id, agent_name, tool_name, event_type, was_successful, duration_ms, created_at, ` +
	`CASE WHEN request_params_json != '{}' OR result_summary_json != '{}' OR error_message != '' OR query_name != '' OR sql_text != '' THEN 1 ELSE 0 END AS has_details`

const mcpToolEventDetailColumns = `id, agent_id, agent_name, tool_name, event_type, was_successful, duration_ms, request_params_json, result_summary_json, error_message, query_name, sql_text, created_at`

func (s *Store) RecordMCPToolEvent(ctx context.Context, event *MCPToolEvent, updateLastUsedAt *time.Time) error {
	if event == nil {
		return errors.New("mcp tool event is required")
	}
	event.AgentName = strings.TrimSpace(event.AgentName)
	event.ToolName = strings.TrimSpace(event.ToolName)
	event.ErrorMessage = strings.TrimSpace(event.ErrorMessage)
	event.QueryName = strings.TrimSpace(event.QueryName)
	event.SQLText = strings.TrimSpace(event.SQLText)
	if event.AgentName == "" {
		return errors.New("agent name is required")
	}
	if event.ToolName == "" {
		return errors.New("tool name is required")
	}
	switch event.EventType {
	case MCPToolEventTypeCall, MCPToolEventTypeError:
	default:
		return fmt.Errorf("unsupported mcp event type: %s", event.EventType)
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	} else {
		event.CreatedAt = event.CreatedAt.UTC()
	}
	if event.RequestParams == nil {
		event.RequestParams = map[string]any{}
	}
	if event.ResultSummary == nil {
		event.ResultSummary = map[string]any{}
	}
	requestParamsJSON, err := marshalJSONObject(event.RequestParams)
	if err != nil {
		return fmt.Errorf("marshal request params: %w", err)
	}
	resultSummaryJSON, err := marshalJSONObject(event.ResultSummary)
	if err != nil {
		return fmt.Errorf("marshal result summary: %w", err)
	}
	if updateLastUsedAt != nil && (event.AgentID == nil || strings.TrimSpace(*event.AgentID) == "") {
		return errors.New("agent id is required to update last_used_at")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO mcp_tool_events(
			id, agent_id, agent_name, tool_name, event_type, was_successful, duration_ms, request_params_json, result_summary_json, error_message, query_name, sql_text, created_at
		) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.ID, nullableString(event.AgentID), event.AgentName, event.ToolName, string(event.EventType), boolToInt(event.WasSuccessful), event.DurationMs, requestParamsJSON, resultSummaryJSON, event.ErrorMessage, event.QueryName, event.SQLText, event.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return err
	}

	if updateLastUsedAt != nil {
		_, err = tx.ExecContext(ctx, `UPDATE agents SET last_used_at = ? WHERE id = ?`, updateLastUsedAt.UTC().Format(time.RFC3339), *event.AgentID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *Store) ListMCPToolEvents(ctx context.Context, options ListMCPToolEventOptions) (*MCPToolEventPage, error) {
	options = normalizeMCPToolEventOptions(options)
	whereClause, args := buildMCPToolEventWhereClause(options)

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mcp_tool_events`+whereClause, args...).Scan(&total); err != nil {
		return nil, err
	}

	queryArgs := append(append([]any(nil), args...), options.Limit, options.Offset)
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+mcpToolEventSummaryColumns+`
		FROM mcp_tool_events`+whereClause+`
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*MCPToolEventSummary, 0, options.Limit)
	for rows.Next() {
		event, err := scanMCPToolEventSummary(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &MCPToolEventPage{Items: items, Total: total, Limit: options.Limit, Offset: options.Offset}, nil
}

func (s *Store) GetMCPToolEvent(ctx context.Context, id string) (*MCPToolEvent, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("event id is required")
	}
	row := s.db.QueryRowContext(ctx, `SELECT `+mcpToolEventDetailColumns+` FROM mcp_tool_events WHERE id = ?`, id)
	event, err := scanMCPToolEvent(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return event, nil
}

func normalizeMCPToolEventOptions(options ListMCPToolEventOptions) ListMCPToolEventOptions {
	options.ToolName = strings.TrimSpace(options.ToolName)
	options.AgentName = strings.TrimSpace(options.AgentName)
	if options.Limit <= 0 {
		options.Limit = 50
	}
	if options.Limit > 100 {
		options.Limit = 100
	}
	if options.Offset < 0 {
		options.Offset = 0
	}
	if options.Since != nil {
		since := options.Since.UTC()
		options.Since = &since
	}
	return options
}

func buildMCPToolEventWhereClause(options ListMCPToolEventOptions) (string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)
	if options.Since != nil {
		clauses = append(clauses, `created_at >= ?`)
		args = append(args, options.Since.Format(time.RFC3339))
	}
	if options.EventType != "" {
		clauses = append(clauses, `event_type = ?`)
		args = append(args, string(options.EventType))
	}
	if options.ToolName != "" {
		clauses = append(clauses, `LOWER(tool_name) LIKE ?`)
		args = append(args, "%"+strings.ToLower(options.ToolName)+"%")
	}
	if options.AgentName != "" {
		clauses = append(clauses, `LOWER(agent_name) LIKE ?`)
		args = append(args, "%"+strings.ToLower(options.AgentName)+"%")
	}
	if len(clauses) == 0 {
		return "", args
	}
	return ` WHERE ` + strings.Join(clauses, ` AND `), args
}

func scanMCPToolEventSummary(scanner interface{ Scan(dest ...any) error }) (*MCPToolEventSummary, error) {
	var summary MCPToolEventSummary
	var agentID sql.NullString
	var wasSuccessful, hasDetails int
	var createdAt string
	if err := scanner.Scan(&summary.ID, &agentID, &summary.AgentName, &summary.ToolName, &summary.EventType, &wasSuccessful, &summary.DurationMs, &createdAt, &hasDetails); err != nil {
		return nil, err
	}
	summary.WasSuccessful = wasSuccessful == 1
	summary.HasDetails = hasDetails == 1
	if agentID.Valid && strings.TrimSpace(agentID.String) != "" {
		value := agentID.String
		summary.AgentID = &value
	}
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	summary.CreatedAt = parsed
	return &summary, nil
}

func scanMCPToolEvent(scanner interface{ Scan(dest ...any) error }) (*MCPToolEvent, error) {
	var event MCPToolEvent
	var agentID sql.NullString
	var wasSuccessful int
	var requestParamsJSON, resultSummaryJSON, createdAt string
	if err := scanner.Scan(&event.ID, &agentID, &event.AgentName, &event.ToolName, &event.EventType, &wasSuccessful, &event.DurationMs, &requestParamsJSON, &resultSummaryJSON, &event.ErrorMessage, &event.QueryName, &event.SQLText, &createdAt); err != nil {
		return nil, err
	}
	event.WasSuccessful = wasSuccessful == 1
	if agentID.Valid && strings.TrimSpace(agentID.String) != "" {
		value := agentID.String
		event.AgentID = &value
	}
	requestParams, err := unmarshalJSONObject(requestParamsJSON)
	if err != nil {
		return nil, fmt.Errorf("decode request params: %w", err)
	}
	resultSummary, err := unmarshalJSONObject(resultSummaryJSON)
	if err != nil {
		return nil, fmt.Errorf("decode result summary: %w", err)
	}
	event.RequestParams = requestParams
	event.ResultSummary = resultSummary
	event.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	return &event, nil
}

func marshalJSONObject(value map[string]any) (string, error) {
	if value == nil {
		value = map[string]any{}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func unmarshalJSONObject(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, err
	}
	if decoded == nil {
		return map[string]any{}, nil
	}
	return decoded, nil
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}
