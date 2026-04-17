package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ekaya-inc/dataclaw/internal/core"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/pkg/models"
)

type Server struct {
	httpServer *server.StreamableHTTPServer
	service    *core.Service
}

type approvedQueryResponse struct {
	QueryID               string                  `json:"query_id"`
	DatasourceID          string                  `json:"datasource_id,omitempty"`
	NaturalLanguagePrompt string                  `json:"natural_language_prompt"`
	AdditionalContext     string                  `json:"additional_context,omitempty"`
	SQLQuery              string                  `json:"sql_query"`
	AllowsModification    bool                    `json:"allows_modification"`
	Parameters            []models.QueryParameter `json:"parameters"`
	OutputColumns         []models.OutputColumn   `json:"output_columns"`
	Constraints           string                  `json:"constraints,omitempty"`
	CreatedAt             time.Time               `json:"created_at"`
	UpdatedAt             time.Time               `json:"updated_at"`
}

func New(version string, service *core.Service) *Server {
	mcpServer := server.NewMCPServer("dataclaw", version, server.WithToolCapabilities(true))
	registerQueryTool(mcpServer, service)
	registerListQueriesTool(mcpServer, service)
	registerCreateQueryTool(mcpServer, service)
	registerUpdateQueryTool(mcpServer, service)
	registerDeleteQueryTool(mcpServer, service)
	registerExecuteQueryTool(mcpServer, service)
	return &Server{httpServer: server.NewStreamableHTTPServer(mcpServer, server.WithStateLess(true)), service: service}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.authorize(r) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}
		s.httpServer.ServeHTTP(w, r)
	})
}

func (s *Server) authorize(r *http.Request) bool {
	ctx := r.Context()
	cred := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(cred), "bearer ") {
		cred = strings.TrimSpace(cred[7:])
	}
	if cred == "" {
		cred = strings.TrimSpace(r.Header.Get("X-API-Key"))
	}
	if cred == "" {
		return false
	}
	ok, err := s.service.ValidateOpenClawKey(ctx, cred)
	return err == nil && ok
}

func registerQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("query",
		mcp.WithDescription("Execute read-only SQL SELECT statements against the configured datasource."),
		mcp.WithString("sql", mcp.Required(), mcp.Description("SQL SELECT statement to execute")),
		mcp.WithNumber("limit", mcp.Description("Maximum rows to return (default 100, max 1000)")),
	)
	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sqlQuery, err := req.RequireString("sql")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		limit := 100
		if args, ok := req.Params.Arguments.(map[string]any); ok {
			if raw, ok := args["limit"].(float64); ok {
				limit = int(raw)
			}
		}
		result, err := service.TestRawQuery(ctx, sqlQuery, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(body)), nil
	})
}

func registerListQueriesTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("list_queries", mcp.WithDescription("List approved queries stored in DataClaw."))
	srv.AddTool(tool, func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		queries, err := service.ListQueries(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, _ := json.Marshal(map[string]any{"queries": normalizeApprovedQueries(queries)})
		return mcp.NewToolResultText(string(body)), nil
	})
}

func registerCreateQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("create_query",
		mcp.WithDescription("Create an approved query directly in DataClaw."),
		mcp.WithString("natural_language_prompt", mcp.Required(), mcp.Description("Plain-language description of what the agent should run this query for; used to match requests to queries.")),
		mcp.WithString("additional_context", mcp.Description("Extra hints to give the LLM when choosing this query.")),
		mcp.WithString("sql_query", mcp.Required(), mcp.Description("SQL query text with optional {{parameters}} placeholders.")),
		mcp.WithBoolean("allows_modification", mcp.Description("Allow this query to run INSERT/UPDATE/DELETE. Defaults to false (read-only).")),
		mcp.WithArray("parameters", mcp.Description("Parameter definitions accepted by the query."), mcp.Items(map[string]any{"type": "object"})),
		mcp.WithArray("output_columns", mcp.Description("Columns the query is expected to return (documentation only)."), mcp.Items(map[string]any{"type": "object"})),
		mcp.WithString("constraints", mcp.Description("Free-form rules the LLM must respect when using this query.")),
	)
	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]any)
		parameters, err := parseParameters(args["parameters"])
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		outputColumns, err := parseOutputColumns(args["output_columns"])
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prompt, err := req.RequireString("natural_language_prompt")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		additionalContext, _ := stringArg(args, "additional_context")
		sqlQuery, err := req.RequireString("sql_query")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		allowsModification := false
		if raw, ok := args["allows_modification"].(bool); ok {
			allowsModification = raw
		}
		constraints, _ := stringArg(args, "constraints")
		query, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{
			NaturalLanguagePrompt: prompt,
			AdditionalContext:     additionalContext,
			SQLQuery:              sqlQuery,
			AllowsModification:    allowsModification,
			Parameters:            parameters,
			OutputColumns:         outputColumns,
			Constraints:           constraints,
		})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, _ := json.Marshal(map[string]any{"query": normalizeApprovedQuery(query)})
		return mcp.NewToolResultText(string(body)), nil
	})
}

func registerUpdateQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("update_query",
		mcp.WithDescription("Update an approved query directly in DataClaw."),
		mcp.WithString("query_id", mcp.Required(), mcp.Description("Query ID")),
		mcp.WithString("natural_language_prompt", mcp.Required(), mcp.Description("Plain-language description of what the agent should run this query for.")),
		mcp.WithString("additional_context", mcp.Description("Extra hints to give the LLM when choosing this query.")),
		mcp.WithString("sql_query", mcp.Required(), mcp.Description("SQL query text with optional {{parameters}} placeholders.")),
		mcp.WithBoolean("allows_modification", mcp.Description("Allow this query to run INSERT/UPDATE/DELETE.")),
		mcp.WithArray("parameters", mcp.Description("Parameter definitions accepted by the query."), mcp.Items(map[string]any{"type": "object"})),
		mcp.WithArray("output_columns", mcp.Description("Columns the query is expected to return."), mcp.Items(map[string]any{"type": "object"})),
		mcp.WithString("constraints", mcp.Description("Free-form rules the LLM must respect when using this query.")),
	)
	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]any)
		id, err := req.RequireString("query_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		prompt, err := req.RequireString("natural_language_prompt")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		sqlQuery, err := req.RequireString("sql_query")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		existing, err := service.GetQuery(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if existing == nil {
			return mcp.NewToolResultError("query not found"), nil
		}

		merged := *existing
		merged.NaturalLanguagePrompt = prompt
		merged.SQLQuery = sqlQuery

		if _, present := args["additional_context"]; present {
			value, _ := args["additional_context"].(string)
			merged.AdditionalContext = value
		}
		if _, present := args["allows_modification"]; present {
			value, _ := args["allows_modification"].(bool)
			merged.AllowsModification = value
		}
		if _, present := args["parameters"]; present {
			parameters, err := parseParameters(args["parameters"])
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			merged.Parameters = parameters
		}
		if _, present := args["output_columns"]; present {
			outputColumns, err := parseOutputColumns(args["output_columns"])
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			merged.OutputColumns = outputColumns
		}
		if _, present := args["constraints"]; present {
			value, _ := args["constraints"].(string)
			merged.Constraints = value
		}

		query, err := service.UpdateQuery(ctx, id, &merged)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, _ := json.Marshal(map[string]any{"query": normalizeApprovedQuery(query)})
		return mcp.NewToolResultText(string(body)), nil
	})
}

func registerDeleteQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("delete_query",
		mcp.WithDescription("Delete an approved query from DataClaw."),
		mcp.WithString("query_id", mcp.Required(), mcp.Description("Query ID")),
	)
	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		queryID, err := req.RequireString("query_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := service.DeleteQuery(ctx, queryID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(`{"deleted":true}`), nil
	})
}

func registerExecuteQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("execute_query",
		mcp.WithDescription("Execute an enabled approved query stored in DataClaw. Parameters are validated and bound before execution."),
		mcp.WithString("query_id", mcp.Required(), mcp.Description("Query ID")),
		mcp.WithObject("parameters", mcp.Description("Parameter values keyed by parameter name")),
		mcp.WithNumber("limit", mcp.Description("Maximum rows to return (default 100, max 1000)")),
	)
	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]any)
		queryID, err := req.RequireString("query_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		parameters, err := parseParameterValues(args["parameters"])
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		limit := 100
		if raw, ok := args["limit"].(float64); ok {
			limit = int(raw)
		}

		result, err := service.ExecuteStoredQuery(ctx, queryID, parameters, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		queryPrompt := ""
		if query, getErr := service.GetQuery(ctx, queryID); getErr == nil && query != nil {
			queryPrompt = query.NaturalLanguagePrompt
		}

		body, _ := json.Marshal(map[string]any{
			"query_id":                queryID,
			"natural_language_prompt": queryPrompt,
			"columns":                 result.Columns,
			"rows":                    result.Rows,
			"row_count":               result.RowCount,
		})
		return mcp.NewToolResultText(string(body)), nil
	})
}

func parseParameters(raw any) ([]models.QueryParameter, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("parameters must be an array")
	}
	params := make([]models.QueryParameter, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid parameter entry")
		}
		param := models.QueryParameter{}
		if name, ok := m["name"].(string); ok {
			param.Name = name
		}
		if typ, ok := m["type"].(string); ok {
			param.Type = typ
		}
		if desc, ok := m["description"].(string); ok {
			param.Description = desc
		}
		if req, ok := m["required"].(bool); ok {
			param.Required = req
		} else {
			param.Required = true
		}
		if def, ok := m["default"]; ok {
			param.Default = def
		}
		if ex, ok := m["example"]; ok {
			param.Example = ex
		}
		params = append(params, param)
	}
	return params, nil
}

func parseOutputColumns(raw any) ([]models.OutputColumn, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("output_columns must be an array")
	}
	columns := make([]models.OutputColumn, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid output_columns entry")
		}
		col := models.OutputColumn{}
		if name, ok := m["name"].(string); ok {
			col.Name = name
		}
		if typ, ok := m["type"].(string); ok {
			col.Type = typ
		}
		if desc, ok := m["description"].(string); ok {
			col.Description = desc
		}
		columns = append(columns, col)
	}
	return columns, nil
}

func parseParameterValues(raw any) (map[string]any, error) {
	if raw == nil {
		return nil, nil
	}
	values, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("parameters must be an object")
	}
	return values, nil
}

func normalizeApprovedQueries(queries []*storepkg.ApprovedQuery) []approvedQueryResponse {
	normalized := make([]approvedQueryResponse, 0, len(queries))
	for _, query := range queries {
		if query == nil {
			continue
		}
		normalized = append(normalized, normalizeApprovedQuery(query))
	}
	return normalized
}

func normalizeApprovedQuery(query *storepkg.ApprovedQuery) approvedQueryResponse {
	parameters := query.Parameters
	if parameters == nil {
		parameters = []models.QueryParameter{}
	}
	outputs := query.OutputColumns
	if outputs == nil {
		outputs = []models.OutputColumn{}
	}
	return approvedQueryResponse{
		QueryID:               query.ID,
		DatasourceID:          query.DatasourceID,
		NaturalLanguagePrompt: query.NaturalLanguagePrompt,
		AdditionalContext:     query.AdditionalContext,
		SQLQuery:              query.SQLQuery,
		AllowsModification:    query.AllowsModification,
		Parameters:            parameters,
		OutputColumns:         outputs,
		Constraints:           query.Constraints,
		CreatedAt:             query.CreatedAt,
		UpdatedAt:             query.UpdatedAt,
	}
}

func stringArg(args map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		value, ok := args[key].(string)
		if ok && strings.TrimSpace(value) != "" {
			return value, true
		}
	}
	return "", false
}
