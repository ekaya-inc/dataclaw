package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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

func New(version string, service *core.Service) *Server {
	mcpServer := server.NewMCPServer("dataclaw", version, server.WithToolCapabilities(true))
	registerQueryTool(mcpServer, service)
	registerListQueriesTool(mcpServer, service)
	registerCreateQueryTool(mcpServer, service)
	registerUpdateQueryTool(mcpServer, service)
	registerDeleteQueryTool(mcpServer, service)
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
		body, _ := json.Marshal(map[string]any{"queries": queries})
		return mcp.NewToolResultText(string(body)), nil
	})
}

func registerCreateQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("create_query",
		mcp.WithDescription("Create an approved query directly in DataClaw."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Human-readable query name")),
		mcp.WithString("description", mcp.Description("What the query is for")),
		mcp.WithString("sql", mcp.Required(), mcp.Description("SQL query text with optional {{parameters}} placeholders")),
		mcp.WithArray("parameters", mcp.Description("Optional parameter definitions"), mcp.Items(map[string]any{"type": "object"})),
		mcp.WithBoolean("is_enabled", mcp.Description("Whether the query is enabled")),
	)
	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]any)
		parameters, err := parseParameters(args["parameters"])
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		name, _ := args["name"].(string)
		description, _ := args["description"].(string)
		sqlQuery, _ := args["sql"].(string)
		enabled := true
		if raw, ok := args["is_enabled"].(bool); ok {
			enabled = raw
		}
		query, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{Name: name, Description: description, SQLQuery: sqlQuery, Parameters: parameters, IsEnabled: enabled})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, _ := json.Marshal(map[string]any{"query": query})
		return mcp.NewToolResultText(string(body)), nil
	})
}

func registerUpdateQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("update_query",
		mcp.WithDescription("Update an approved query directly in DataClaw."),
		mcp.WithString("query_id", mcp.Required(), mcp.Description("Query ID")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Human-readable query name")),
		mcp.WithString("description", mcp.Description("What the query is for")),
		mcp.WithString("sql", mcp.Required(), mcp.Description("SQL query text with optional {{parameters}} placeholders")),
		mcp.WithArray("parameters", mcp.Description("Optional parameter definitions"), mcp.Items(map[string]any{"type": "object"})),
		mcp.WithBoolean("is_enabled", mcp.Description("Whether the query is enabled")),
	)
	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args, _ := req.Params.Arguments.(map[string]any)
		id, _ := args["query_id"].(string)
		parameters, err := parseParameters(args["parameters"])
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		name, _ := args["name"].(string)
		description, _ := args["description"].(string)
		sqlQuery, _ := args["sql"].(string)
		enabled := true
		if raw, ok := args["is_enabled"].(bool); ok {
			enabled = raw
		}
		query, err := service.UpdateQuery(ctx, id, &storepkg.ApprovedQuery{Name: name, Description: description, SQLQuery: sqlQuery, Parameters: parameters, IsEnabled: enabled})
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, _ := json.Marshal(map[string]any{"query": query})
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
