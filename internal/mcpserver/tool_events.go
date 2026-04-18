package mcpserver

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/ekaya-inc/dataclaw/internal/core"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

const (
	maxSummaryColumns  = 8
	maxSummaryQueryIDs = 10
)

func trackedToolHandler(service *core.Service, toolName string, run func(context.Context, *storepkg.Agent, mcp.CallToolRequest) (any, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		startedAt := time.Now()
		agent, err := requireAgent(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		result, callErr := run(ctx, agent, req)
		finishedAt := time.Now().UTC()
		event := &storepkg.MCPToolEvent{
			AgentID:       stringPointer(agent.ID),
			AgentName:     agent.Name,
			ToolName:      toolName,
			DurationMs:    int(time.Since(startedAt).Milliseconds()),
			RequestParams: summarizeToolRequest(toolName, req),
			CreatedAt:     finishedAt,
		}

		if callErr != nil {
			event.EventType = storepkg.MCPToolEventTypeError
			event.ErrorMessage = callErr.Error()
			event.WasSuccessful = false
			_ = service.RecordAgentToolEvent(ctx, event, false)
			return mcp.NewToolResultError(callErr.Error()), nil
		}

		event.EventType = storepkg.MCPToolEventTypeCall
		event.WasSuccessful = true
		event.ResultSummary = summarizeToolResult(toolName, result)
		if err := service.RecordAgentToolEvent(ctx, event, true); err != nil {
			_ = service.RecordAgentToolUse(ctx, agent.ID)
		}

		body, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(body)), nil
	}
}

func summarizeToolRequest(toolName string, req mcp.CallToolRequest) map[string]any {
	args, _ := req.Params.Arguments.(map[string]any)
	summary := map[string]any{}
	if limit, ok := intValue(args["limit"]); ok {
		summary["limit"] = limit
	}

	switch toolName {
	case "query", "execute":
		if sqlQuery, ok := args["sql"].(string); ok {
			if statementType := sqlStatementType(sqlQuery); statementType != "" {
				summary["statement_type"] = statementType
			}
		}
	case "execute_query":
		if queryID, ok := args["query_id"].(string); ok && strings.TrimSpace(queryID) != "" {
			summary["query_id"] = queryID
		}
		if values, ok := args["parameters"].(map[string]any); ok {
			keys := make([]string, 0, len(values))
			for key := range values {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				summary["parameter_count"] = len(keys)
				summary["parameter_keys"] = keys
			}
		}
	case "create_query", "update_query":
		if toolName == "update_query" {
			if queryID, ok := args["query_id"].(string); ok && strings.TrimSpace(queryID) != "" {
				summary["query_id"] = queryID
			}
		}
		if sqlQuery, ok := args["sql_query"].(string); ok {
			if statementType := sqlStatementType(sqlQuery); statementType != "" {
				summary["statement_type"] = statementType
			}
		}
		if allowsModification, ok := args["allows_modification"].(bool); ok {
			summary["allows_modification"] = allowsModification
		}
		if parameterCount, ok := arrayLength(args["parameters"]); ok {
			summary["parameter_count"] = parameterCount
		}
		if outputColumnCount, ok := arrayLength(args["output_columns"]); ok {
			summary["output_column_count"] = outputColumnCount
		}
		if constraints, ok := args["constraints"].(string); ok && strings.TrimSpace(constraints) != "" {
			summary["has_constraints"] = true
		}
	case "delete_query":
		if queryID, ok := args["query_id"].(string); ok && strings.TrimSpace(queryID) != "" {
			summary["query_id"] = queryID
		}
	}

	return summary
}

func summarizeToolResult(toolName string, value any) map[string]any {
	switch toolName {
	case "list_queries":
		return summarizeListQueriesResult(value)
	case "create_query", "update_query":
		return summarizeManagedQueryMutationResult(value)
	case "delete_query":
		return summarizeDeleteQueryResult(value)
	default:
		return summarizeQueryResult(value)
	}
}

func summarizeListQueriesResult(value any) map[string]any {
	payload, err := normalizeToolPayload(value)
	if err != nil {
		return map[string]any{}
	}
	queries, _ := payload["queries"].([]any)
	summary := map[string]any{"query_count": len(queries)}
	ids := make([]string, 0, len(queries))
	for _, item := range queries {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		queryID, _ := record["query_id"].(string)
		queryID = strings.TrimSpace(queryID)
		if queryID == "" {
			continue
		}
		ids = append(ids, queryID)
	}
	if len(ids) > maxSummaryQueryIDs {
		summary["query_ids_truncated"] = true
		ids = ids[:maxSummaryQueryIDs]
	}
	if len(ids) > 0 {
		summary["query_ids"] = ids
	}
	return summary
}

func summarizeManagedQueryMutationResult(value any) map[string]any {
	payload, err := normalizeToolPayload(value)
	if err != nil {
		return map[string]any{}
	}
	record, ok := payload["query"].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return summarizeManagedQueryRecord(record)
}

func summarizeDeleteQueryResult(value any) map[string]any {
	payload, err := normalizeToolPayload(value)
	if err != nil {
		return map[string]any{}
	}
	summary := map[string]any{}
	if queryID, ok := payload["query_id"].(string); ok && strings.TrimSpace(queryID) != "" {
		summary["query_id"] = queryID
	}
	if deleted, ok := payload["deleted"].(bool); ok {
		summary["deleted"] = deleted
	}
	return summary
}

func summarizeQueryResult(value any) map[string]any {
	payload, err := normalizeToolPayload(value)
	if err != nil {
		return map[string]any{}
	}
	summary := map[string]any{}
	if rowCount, ok := intValue(payload["row_count"]); ok {
		summary["row_count"] = rowCount
	} else if rowCount, ok := intValue(payload["rowCount"]); ok {
		summary["row_count"] = rowCount
	} else if rows, ok := payload["rows"].([]any); ok {
		summary["row_count"] = len(rows)
	}
	if columnNames := extractColumnNames(payload["columns"]); len(columnNames) > 0 {
		summary["column_names"] = columnNames
	}
	if rowsAffected, ok := intValue(payload["rows_affected"]); ok {
		summary["rows_affected"] = rowsAffected
	}
	return summary
}

func summarizeManagedQueryRecord(record map[string]any) map[string]any {
	summary := map[string]any{}
	if queryID, ok := record["query_id"].(string); ok && strings.TrimSpace(queryID) != "" {
		summary["query_id"] = queryID
	}
	if sqlQuery, ok := record["sql_query"].(string); ok {
		if statementType := sqlStatementType(sqlQuery); statementType != "" {
			summary["statement_type"] = statementType
		}
	}
	if allowsModification, ok := record["allows_modification"].(bool); ok {
		summary["allows_modification"] = allowsModification
	}
	if parameterCount, ok := arrayLength(record["parameters"]); ok {
		summary["parameter_count"] = parameterCount
	}
	if outputColumnCount, ok := arrayLength(record["output_columns"]); ok {
		summary["output_column_count"] = outputColumnCount
	}
	if constraints, ok := record["constraints"].(string); ok && strings.TrimSpace(constraints) != "" {
		summary["has_constraints"] = true
	}
	if datasourceID, ok := record["datasource_id"].(string); ok && strings.TrimSpace(datasourceID) != "" {
		summary["datasource_present"] = true
	}
	return summary
}

func extractColumnNames(raw any) []string {
	columns, ok := raw.([]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(columns))
	for _, item := range columns {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := record["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		names = append(names, name)
		if len(names) == maxSummaryColumns {
			break
		}
	}
	return names
}

func arrayLength(raw any) (int, bool) {
	values, ok := raw.([]any)
	if !ok {
		return 0, false
	}
	return len(values), true
}

func sqlStatementType(sqlQuery string) string {
	for _, token := range strings.Fields(sqlQuery) {
		trimmed := strings.Trim(token, "();")
		if trimmed == "" {
			continue
		}
		return strings.ToUpper(trimmed)
	}
	return ""
}

func intValue(raw any) (int, bool) {
	switch value := raw.(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	case json.Number:
		parsed, err := value.Int64()
		if err != nil {
			return 0, false
		}
		return int(parsed), true
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func normalizeToolPayload(value any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err == nil && decoded != nil {
		return decoded, nil
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return map[string]any{"value": generic}, nil
}

func stringPointer(value string) *string {
	return &value
}
