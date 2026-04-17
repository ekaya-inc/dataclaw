package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
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

type authorizedAgentKey struct{}

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
	hooks := &server.Hooks{}
	hooks.AddAfterListTools(func(ctx context.Context, _ any, _ *mcp.ListToolsRequest, result *mcp.ListToolsResult) {
		if result == nil {
			return
		}
		result.Tools = filterToolsForContext(ctx, service, result.Tools)
	})

	mcpServer := server.NewMCPServer("dataclaw", version, server.WithToolCapabilities(true), server.WithHooks(hooks))
	registerQueryTool(mcpServer, service)
	registerExecuteTool(mcpServer, service)
	registerListQueriesTool(mcpServer, service)
	registerExecuteQueryTool(mcpServer, service)
	return &Server{httpServer: server.NewStreamableHTTPServer(mcpServer, server.WithStateLess(true)), service: service}
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent, err := s.authorize(r)
		if err != nil || agent == nil {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}
		s.httpServer.ServeHTTP(w, r.WithContext(withAuthorizedAgent(r.Context(), agent)))
	})
}

func (s *Server) authorize(r *http.Request) (*storepkg.Agent, error) {
	cred := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(cred), "bearer ") {
		cred = strings.TrimSpace(cred[7:])
	}
	if cred == "" {
		cred = strings.TrimSpace(r.Header.Get("X-API-Key"))
	}
	if cred == "" {
		return nil, errors.New("missing api key")
	}
	return s.service.AuthenticateAgent(r.Context(), cred)
}

func registerQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("query",
		mcp.WithDescription("Execute read-only SQL SELECT statements against the configured datasource when the authenticated agent has raw query access."),
		mcp.WithString("sql", mcp.Required(), mcp.Description("SQL SELECT statement to execute")),
		mcp.WithNumber("limit", mcp.Description("Maximum rows to return (default 100, max 1000)")),
	)
	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		agent, err := requireAgent(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if !agent.CanQuery {
			return mcp.NewToolResultError("agent is not allowed to use raw query"), nil
		}
		sqlQuery, err := req.RequireString("sql")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		limit := extractLimit(req, 100)
		result, err := service.TestRawQuery(ctx, sqlQuery, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := service.RecordAgentToolUse(ctx, agent.ID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(body)), nil
	})
}

func registerExecuteTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("execute",
		mcp.WithDescription("Execute ad-hoc mutating SQL/DDL/DML against the configured datasource when the authenticated agent has raw execute access."),
		mcp.WithString("sql", mcp.Required(), mcp.Description("Mutating SQL statement to execute")),
		mcp.WithNumber("limit", mcp.Description("Maximum rows to return (default 100, max 1000)")),
	)
	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		agent, err := requireAgent(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if !agent.CanExecute {
			return mcp.NewToolResultError("agent is not allowed to use raw execute"), nil
		}
		sqlQuery, err := req.RequireString("sql")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		limit := extractLimit(req, 100)
		result, err := service.ExecuteRawMutation(ctx, sqlQuery, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := service.RecordAgentToolUse(ctx, agent.ID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(body)), nil
	})
}

func registerListQueriesTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("list_queries", mcp.WithDescription("List approved queries available to the authenticated agent."))
	srv.AddTool(tool, func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		agent, err := requireAgent(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		queries, err := service.ListQueriesForAgent(ctx, agent)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := service.RecordAgentToolUse(ctx, agent.ID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, _ := json.Marshal(map[string]any{"queries": normalizeApprovedQueries(queries)})
		return mcp.NewToolResultText(string(body)), nil
	})
}

func registerExecuteQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("execute_query",
		mcp.WithDescription("Execute an approved query that the authenticated agent is allowed to access. Parameters are validated and bound before execution."),
		mcp.WithString("query_id", mcp.Required(), mcp.Description("Query ID")),
		mcp.WithObject("parameters", mcp.Description("Parameter values keyed by parameter name")),
		mcp.WithNumber("limit", mcp.Description("Maximum rows to return (default 100, max 1000)")),
	)
	srv.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		agent, err := requireAgent(ctx)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		queryID, err := req.RequireString("query_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		args, _ := req.Params.Arguments.(map[string]any)
		values, err := parseParameterValues(args["parameters"])
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		limit := extractLimit(req, 100)
		result, err := service.ExecuteStoredQueryForAgent(ctx, agent, queryID, values, limit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := service.RecordAgentToolUse(ctx, agent.ID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		body, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(body)), nil
	})
}

func filterToolsForContext(ctx context.Context, service *core.Service, tools []mcp.Tool) []mcp.Tool {
	agent, ok := authorizedAgentFromContext(ctx)
	if !ok || agent == nil {
		return []mcp.Tool{}
	}
	hasDatasource, err := service.HasDatasource(ctx)
	if err != nil || !hasDatasource {
		return []mcp.Tool{}
	}
	allowed := allowedTools(agent)
	filtered := make([]mcp.Tool, 0, len(tools))
	for _, tool := range tools {
		if allowed[tool.Name] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
}

func allowedTools(agent *storepkg.Agent) map[string]bool {
	allowed := make(map[string]bool, 4)
	if agent == nil {
		return allowed
	}
	if agent.CanQuery {
		allowed["query"] = true
	}
	if agent.CanExecute {
		allowed["execute"] = true
	}
	if agent.ApprovedQueryScope != storepkg.ApprovedQueryScopeNone {
		allowed["list_queries"] = true
		if agent.ApprovedQueryScope == storepkg.ApprovedQueryScopeAll || len(agent.ApprovedQueryIDs) > 0 {
			allowed["execute_query"] = true
		}
	}
	return allowed
}

func requireAgent(ctx context.Context) (*storepkg.Agent, error) {
	agent, ok := authorizedAgentFromContext(ctx)
	if !ok || agent == nil {
		return nil, errors.New("unauthorized")
	}
	return agent, nil
}

func withAuthorizedAgent(ctx context.Context, agent *storepkg.Agent) context.Context {
	return context.WithValue(ctx, authorizedAgentKey{}, agent)
}

func authorizedAgentFromContext(ctx context.Context) (*storepkg.Agent, bool) {
	agent, ok := ctx.Value(authorizedAgentKey{}).(*storepkg.Agent)
	return agent, ok
}

func extractLimit(req mcp.CallToolRequest, fallback int) int {
	limit := fallback
	if args, ok := req.Params.Arguments.(map[string]any); ok {
		if raw, ok := args["limit"].(float64); ok {
			limit = int(raw)
		}
	}
	return limit
}

func parseParameterValues(raw any) (map[string]any, error) {
	if raw == nil {
		return nil, nil
	}
	values, ok := raw.(map[string]any)
	if !ok {
		return nil, errors.New("parameters must be an object")
	}
	return values, nil
}

func normalizeApprovedQueries(queries []*storepkg.ApprovedQuery) []approvedQueryResponse {
	result := make([]approvedQueryResponse, 0, len(queries))
	for _, query := range queries {
		if query == nil {
			continue
		}
		result = append(result, normalizeApprovedQuery(query))
	}
	return result
}

func normalizeApprovedQuery(query *storepkg.ApprovedQuery) approvedQueryResponse {
	return approvedQueryResponse{
		QueryID:               query.ID,
		DatasourceID:          query.DatasourceID,
		NaturalLanguagePrompt: query.NaturalLanguagePrompt,
		AdditionalContext:     query.AdditionalContext,
		SQLQuery:              query.SQLQuery,
		AllowsModification:    query.AllowsModification,
		Parameters:            query.Parameters,
		OutputColumns:         query.OutputColumns,
		Constraints:           query.Constraints,
		CreatedAt:             query.CreatedAt,
		UpdatedAt:             query.UpdatedAt,
	}
}
