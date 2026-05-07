package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

		queryName, sqlText := resolveToolAudit(ctx, service, toolName, req)

		result, callErr := run(ctx, agent, req)
		finishedAt := time.Now().UTC()
		event := &storepkg.MCPToolEvent{
			AgentID:       stringPointer(agent.ID),
			AgentName:     agent.Name,
			ToolName:      toolName,
			DurationMs:    int(time.Since(startedAt).Milliseconds()),
			RequestParams: summarizeToolRequest(toolName, req),
			QueryName:     queryName,
			SQLText:       sqlText,
			CreatedAt:     finishedAt,
		}

		if callErr != nil {
			event.EventType = storepkg.MCPToolEventTypeError
			event.ErrorMessage = callErr.Error()
			event.WasSuccessful = false
			_ = service.RecordAgentToolEvent(ctx, event, false)
			return mcp.NewToolResultError(callErr.Error()), nil
		}

		body, resultFormat, err := renderToolResult(toolName, result, req)
		if err != nil {
			event.EventType = storepkg.MCPToolEventTypeError
			event.ErrorMessage = err.Error()
			event.WasSuccessful = false
			_ = service.RecordAgentToolEvent(ctx, event, false)
			return mcp.NewToolResultError(err.Error()), nil
		}

		event.EventType = storepkg.MCPToolEventTypeCall
		event.WasSuccessful = true
		event.ResultSummary = summarizeToolResult(toolName, result)
		addRenderedResultSummary(event.ResultSummary, resultFormat, body)
		if err := service.RecordAgentToolEvent(ctx, event, true); err != nil {
			_ = service.RecordAgentToolUse(ctx, agent.ID)
		}
		return mcp.NewToolResultText(body), nil
	}
}

func resolveToolAudit(ctx context.Context, service *core.Service, toolName string, req mcp.CallToolRequest) (queryName string, sqlText string) {
	args, _ := req.Params.Arguments.(map[string]any)
	switch toolName {
	case "query", "execute":
		if sqlQuery, ok := args["sql"].(string); ok {
			sqlText = strings.TrimSpace(sqlQuery)
		}
	case "validate_query":
		if sqlQuery, ok := args["sql_query"].(string); ok {
			sqlText = strings.TrimSpace(sqlQuery)
		}
	case "create_query":
		if prompt, ok := args["natural_language_prompt"].(string); ok {
			queryName = strings.TrimSpace(prompt)
		}
		if sqlQuery, ok := args["sql_query"].(string); ok {
			sqlText = strings.TrimSpace(sqlQuery)
		}
	case "update_query":
		if prompt, ok := args["natural_language_prompt"].(string); ok {
			queryName = strings.TrimSpace(prompt)
		}
		if sqlQuery, ok := args["sql_query"].(string); ok {
			sqlText = strings.TrimSpace(sqlQuery)
		}
		if queryName == "" || sqlText == "" {
			if existingName, existingSQL, ok := lookupStoredQuery(ctx, service, args["query_id"]); ok {
				if queryName == "" {
					queryName = existingName
				}
				if sqlText == "" {
					sqlText = existingSQL
				}
			}
		}
	case "execute_query", "delete_query":
		if existingName, existingSQL, ok := lookupStoredQuery(ctx, service, args["query_id"]); ok {
			queryName = existingName
			if toolName == "execute_query" {
				sqlText = existingSQL
			}
		}
	case "count_rows":
		if sqlQuery, ok := args["sql"].(string); ok {
			sqlText = strings.TrimSpace(sqlQuery)
		}
		if existingName, existingSQL, ok := lookupStoredQuery(ctx, service, args["query_id"]); ok {
			queryName = existingName
			sqlText = existingSQL
		}
	}
	return queryName, sqlText
}

func lookupStoredQuery(ctx context.Context, service *core.Service, raw any) (string, string, bool) {
	id, ok := raw.(string)
	if !ok {
		return "", "", false
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return "", "", false
	}
	q, err := service.GetQuery(ctx, id)
	if err != nil || q == nil {
		return "", "", false
	}
	return strings.TrimSpace(q.NaturalLanguagePrompt), strings.TrimSpace(q.SQLQuery), true
}

func summarizeToolRequest(toolName string, req mcp.CallToolRequest) map[string]any {
	args, _ := req.Params.Arguments.(map[string]any)
	summary := map[string]any{}
	if limit, ok := intValue(args["limit"]); ok {
		summary["limit"] = limit
	}
	if offset, ok := intValue(args["offset"]); ok {
		summary["offset"] = offset
	}
	if format, ok := stringValue(args["result_format"]); ok {
		summary["result_format"] = format
	} else if format, ok := stringValue(args["format"]); ok {
		summary["result_format"] = format
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
		addParameterSummary(summary, args["parameters"])
	case "count_rows":
		if sqlQuery, ok := args["sql"].(string); ok {
			if statementType := sqlStatementType(sqlQuery); statementType != "" {
				summary["statement_type"] = statementType
			}
		}
		if queryID, ok := args["query_id"].(string); ok && strings.TrimSpace(queryID) != "" {
			summary["query_id"] = queryID
		}
		addParameterSummary(summary, args["parameters"])
	case "validate_query", "create_query", "update_query":
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
		if toolName != "validate_query" {
			if outputColumnCount, ok := arrayLength(args["output_columns"]); ok {
				summary["output_column_count"] = outputColumnCount
			}
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
	case "validate_query":
		return summarizeValidateQueryResult(value)
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

func summarizeValidateQueryResult(value any) map[string]any {
	payload, err := normalizeToolPayload(value)
	if err != nil {
		return map[string]any{}
	}
	summary := map[string]any{}
	if valid, ok := payload["valid"].(bool); ok {
		summary["valid"] = valid
	}
	if normalized, ok := payload["normalized_sql"].(string); ok {
		if statementType := sqlStatementType(normalized); statementType != "" {
			summary["statement_type"] = statementType
		}
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
	if limit, ok := intValue(payload["limit"]); ok {
		summary["limit"] = limit
	}
	if offset, ok := intValue(payload["offset"]); ok {
		summary["offset"] = offset
	}
	if hasMore, ok := payload["has_more"].(bool); ok {
		summary["has_more"] = hasMore
	}
	if nextOffset, ok := intValue(payload["next_offset"]); ok {
		summary["next_offset"] = nextOffset
	}
	if exact, ok := payload["exact"].(bool); ok {
		summary["exact"] = exact
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
	if queryID, ok := record["query_id"].(string); ok && strings.TrimSpace(queryID) != "" {
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

func addParameterSummary(summary map[string]any, raw any) {
	values, ok := raw.(map[string]any)
	if !ok {
		return
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return
	}
	summary["parameter_count"] = len(keys)
	summary["parameter_keys"] = keys
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

func stringValue(raw any) (string, bool) {
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(strings.ToLower(value))
	return value, value != ""
}

func addRenderedResultSummary(summary map[string]any, format string, body string) {
	if summary == nil {
		return
	}
	summary["result_format"] = format
	summary["response_bytes"] = len(body)
	summary["estimated_tokens"] = (len(body) + 3) / 4
}

func renderToolResult(toolName string, value any, req mcp.CallToolRequest) (body string, format string, err error) {
	format, err = requestedResultFormat(toolName, req)
	if err != nil {
		return "", "", err
	}
	if format == "tsv" {
		if rendered, ok := renderTabularTSV(value); ok {
			return rendered, format, nil
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", "", err
	}
	return string(raw), "json", nil
}

func requestedResultFormat(toolName string, req mcp.CallToolRequest) (string, error) {
	defaultFormat := "json"
	switch toolName {
	case "query", "execute", "execute_query":
		defaultFormat = "tsv"
	}
	args, _ := req.Params.Arguments.(map[string]any)
	if args == nil {
		return defaultFormat, nil
	}
	format, ok := stringValue(args["result_format"])
	if !ok {
		format, ok = stringValue(args["format"])
	}
	if !ok {
		return defaultFormat, nil
	}
	switch format {
	case "json", "tsv":
		return format, nil
	default:
		return "", fmt.Errorf("result_format must be one of json or tsv")
	}
}

func renderTabularTSV(value any) (string, bool) {
	switch result := value.(type) {
	case *core.QueryResult:
		return renderRowsTSV(result.Columns, result.Rows, map[string]any{
			"row_count":   result.RowCount,
			"limit":       result.Limit,
			"offset":      result.Offset,
			"has_more":    result.HasMore,
			"next_offset": result.NextOffset,
		}), true
	case *core.ExecuteResult:
		if len(result.Columns) == 0 && len(result.Rows) == 0 {
			return "", false
		}
		return renderRowsTSV(result.Columns, result.Rows, map[string]any{
			"row_count":     result.RowCount,
			"rows_affected": result.RowsAffected,
			"limit":         result.Limit,
			"offset":        result.Offset,
			"has_more":      result.HasMore,
			"next_offset":   result.NextOffset,
		}), true
	default:
		return "", false
	}
}

func renderRowsTSV(columns []core.QueryColumn, rows []map[string]any, metadata map[string]any) string {
	columnNames := columnNamesForTSV(columns, rows)
	var out bytes.Buffer
	out.WriteString("#")
	keys := make([]string, 0, len(metadata))
	for key, value := range metadata {
		if key == "next_offset" {
			if next, ok := intValue(value); !ok || next == 0 {
				continue
			}
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		out.WriteByte('\t')
		out.WriteString(key)
		out.WriteByte('=')
		out.WriteString(tsvCell(metadata[key]))
	}
	out.WriteByte('\n')
	for i, name := range columnNames {
		if i > 0 {
			out.WriteByte('\t')
		}
		out.WriteString(tsvCell(name))
	}
	out.WriteByte('\n')
	for _, row := range rows {
		for i, name := range columnNames {
			if i > 0 {
				out.WriteByte('\t')
			}
			out.WriteString(tsvCell(row[name]))
		}
		out.WriteByte('\n')
	}
	return out.String()
}

func columnNamesForTSV(columns []core.QueryColumn, rows []map[string]any) []string {
	names := make([]string, 0, len(columns))
	for _, column := range columns {
		name := strings.TrimSpace(column.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) > 0 || len(rows) == 0 {
		return names
	}
	for name := range rows[0] {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func tsvCell(value any) string {
	if value == nil {
		return ""
	}
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case fmt.Stringer:
		text = typed.String()
	case bool:
		if typed {
			text = "true"
		} else {
			text = "false"
		}
	default:
		raw, err := json.Marshal(typed)
		if err == nil {
			text = string(raw)
			if strings.HasPrefix(text, "\"") && strings.HasSuffix(text, "\"") {
				var decoded string
				if json.Unmarshal(raw, &decoded) == nil {
					text = decoded
				}
			}
		} else {
			text = fmt.Sprint(typed)
		}
	}
	text = strings.ReplaceAll(text, "\r", `\r`)
	text = strings.ReplaceAll(text, "\n", `\n`)
	text = strings.ReplaceAll(text, "\t", `\t`)
	return text
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
