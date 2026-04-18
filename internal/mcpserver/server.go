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

type queryToolRequest struct {
	QueryID               string                  `json:"query_id"`
	NaturalLanguagePrompt string                  `json:"natural_language_prompt"`
	AdditionalContext     string                  `json:"additional_context"`
	SQLQuery              string                  `json:"sql_query"`
	AllowsModification    bool                    `json:"allows_modification"`
	Parameters            []models.QueryParameter `json:"parameters"`
	OutputColumns         []models.OutputColumn   `json:"output_columns"`
	Constraints           string                  `json:"constraints"`
}

func New(version string, service *core.Service) *Server {
	mcpServer := buildMCPServer(version, service)
	return &Server{httpServer: server.NewStreamableHTTPServer(mcpServer, server.WithStateLess(true)), service: service}
}

func buildMCPServer(version string, service *core.Service) *server.MCPServer {
	hooks := &server.Hooks{}
	hooks.AddAfterListTools(func(ctx context.Context, _ any, _ *mcp.ListToolsRequest, result *mcp.ListToolsResult) {
		if result == nil {
			return
		}
		result.Tools = filterToolsForContext(ctx, service, result.Tools)
	})

	mcpServer := server.NewMCPServer("dataclaw", version, server.WithToolCapabilities(true), server.WithHooks(hooks))
	registerHealthTool(mcpServer, version, service)
	registerQueryTool(mcpServer, service)
	registerExecuteTool(mcpServer, service)
	registerListQueriesTool(mcpServer, service)
	registerCreateQueryTool(mcpServer, service)
	registerUpdateQueryTool(mcpServer, service)
	registerDeleteQueryTool(mcpServer, service)
	registerExecuteQueryTool(mcpServer, service)
	return mcpServer
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
	srv.AddTool(tool, trackedToolHandler(service, "query", func(ctx context.Context, agent *storepkg.Agent, req mcp.CallToolRequest) (any, error) {
		if !agent.CanQuery {
			return nil, errors.New("agent is not allowed to use raw query")
		}
		sqlQuery, err := req.RequireString("sql")
		if err != nil {
			return nil, err
		}
		limit := extractLimit(req, 100)
		return service.TestRawQuery(ctx, sqlQuery, limit)
	}))
}

func registerExecuteTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("execute",
		mcp.WithDescription("Execute ad-hoc mutating SQL/DDL/DML against the configured datasource when the authenticated agent has raw execute access."),
		mcp.WithString("sql", mcp.Required(), mcp.Description("Mutating SQL statement to execute")),
		mcp.WithNumber("limit", mcp.Description("Maximum rows to return (default 100, max 1000)")),
	)
	srv.AddTool(tool, trackedToolHandler(service, "execute", func(ctx context.Context, agent *storepkg.Agent, req mcp.CallToolRequest) (any, error) {
		if !agent.CanExecute {
			return nil, errors.New("agent is not allowed to use raw execute")
		}
		sqlQuery, err := req.RequireString("sql")
		if err != nil {
			return nil, err
		}
		limit := extractLimit(req, 100)
		return service.ExecuteRawMutation(ctx, sqlQuery, limit)
	}))
}

func registerListQueriesTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("list_queries", mcp.WithDescription("List approved queries available to or manageable by the authenticated agent."))
	srv.AddTool(tool, trackedToolHandler(service, "list_queries", func(ctx context.Context, agent *storepkg.Agent, _ mcp.CallToolRequest) (any, error) {
		queries, err := service.ListQueriesForAgent(ctx, agent)
		if err != nil {
			return nil, err
		}
		return map[string]any{"queries": normalizeApprovedQueries(queries)}, nil
	}))
}

func registerCreateQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("create_query",
		mcp.WithDescription("Create an approved query in the catalog when the authenticated agent can manage approved queries."),
		mcp.WithString("natural_language_prompt", mcp.Required(), mcp.Description("Human-readable prompt describing when to use the query.")),
		mcp.WithString("additional_context", mcp.Description("Optional usage notes or extra context for the query.")),
		mcp.WithString("sql_query", mcp.Required(), mcp.Description("SQL body for the approved query.")),
		mcp.WithBoolean("allows_modification", mcp.Description("Whether the query intentionally performs mutations instead of read-only access.")),
		mcp.WithArray("parameters", mcp.Description("Optional parameter definitions for the query."), mcp.Items(queryParameterItemSchema())),
		mcp.WithArray("output_columns", mcp.Description("Optional documented output columns."), mcp.Items(outputColumnItemSchema())),
		mcp.WithString("constraints", mcp.Description("Optional business constraints or caveats.")),
	)
	srv.AddTool(tool, trackedToolHandler(service, "create_query", func(ctx context.Context, agent *storepkg.Agent, req mcp.CallToolRequest) (any, error) {
		input, err := parseQueryToolRequest(req)
		if err != nil {
			return nil, err
		}
		query, err := service.CreateQueryForAgent(ctx, agent, approvedQueryFromToolRequest(input))
		if err != nil {
			return nil, err
		}
		return map[string]any{"query": normalizeApprovedQuery(query)}, nil
	}))
}

func registerUpdateQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("update_query",
		mcp.WithDescription("Replace an approved query definition when the authenticated agent can manage approved queries."),
		mcp.WithString("query_id", mcp.Required(), mcp.Description("Approved query ID to replace.")),
		mcp.WithString("natural_language_prompt", mcp.Required(), mcp.Description("Human-readable prompt describing when to use the query.")),
		mcp.WithString("additional_context", mcp.Description("Optional usage notes or extra context for the query.")),
		mcp.WithString("sql_query", mcp.Required(), mcp.Description("Full replacement SQL body for the approved query.")),
		mcp.WithBoolean("allows_modification", mcp.Description("Whether the query intentionally performs mutations instead of read-only access.")),
		mcp.WithArray("parameters", mcp.Description("Full replacement parameter definitions for the query."), mcp.Items(queryParameterItemSchema())),
		mcp.WithArray("output_columns", mcp.Description("Full replacement documented output columns."), mcp.Items(outputColumnItemSchema())),
		mcp.WithString("constraints", mcp.Description("Optional business constraints or caveats.")),
	)
	srv.AddTool(tool, trackedToolHandler(service, "update_query", func(ctx context.Context, agent *storepkg.Agent, req mcp.CallToolRequest) (any, error) {
		queryID, err := req.RequireString("query_id")
		if err != nil {
			return nil, err
		}
		input, err := parseQueryToolRequest(req)
		if err != nil {
			return nil, err
		}
		query, err := service.UpdateQueryForAgent(ctx, agent, queryID, approvedQueryFromToolRequest(input))
		if err != nil {
			return nil, err
		}
		return map[string]any{"query": normalizeApprovedQuery(query)}, nil
	}))
}

func registerDeleteQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("delete_query",
		mcp.WithDescription("Delete an approved query when the authenticated agent can manage approved queries."),
		mcp.WithString("query_id", mcp.Required(), mcp.Description("Approved query ID to delete.")),
	)
	srv.AddTool(tool, trackedToolHandler(service, "delete_query", func(ctx context.Context, agent *storepkg.Agent, req mcp.CallToolRequest) (any, error) {
		queryID, err := req.RequireString("query_id")
		if err != nil {
			return nil, err
		}
		if err := service.DeleteQueryForAgent(ctx, agent, queryID); err != nil {
			return nil, err
		}
		return map[string]any{"deleted": true, "query_id": queryID}, nil
	}))
}

func registerExecuteQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("execute_query",
		mcp.WithDescription("Execute an approved query that the authenticated agent is allowed to access. Parameters are validated and bound before execution."),
		mcp.WithString("query_id", mcp.Required(), mcp.Description("Query ID")),
		mcp.WithObject("parameters", mcp.Description("Parameter values keyed by parameter name")),
		mcp.WithNumber("limit", mcp.Description("Maximum rows to return (default 100, max 1000)")),
	)
	srv.AddTool(tool, trackedToolHandler(service, "execute_query", func(ctx context.Context, agent *storepkg.Agent, req mcp.CallToolRequest) (any, error) {
		queryID, err := req.RequireString("query_id")
		if err != nil {
			return nil, err
		}
		args, _ := req.Params.Arguments.(map[string]any)
		values, err := parseParameterValues(args["parameters"])
		if err != nil {
			return nil, err
		}
		limit := extractLimit(req, 100)
		return service.ExecuteStoredQueryForAgent(ctx, agent, queryID, values, limit)
	}))
}

func filterToolsForContext(ctx context.Context, service *core.Service, tools []mcp.Tool) []mcp.Tool {
	agent, ok := authorizedAgentFromContext(ctx)
	if !ok || agent == nil {
		return []mcp.Tool{}
	}
	hasDatasource, err := service.HasDatasource(ctx)
	if err != nil || !hasDatasource {
		return filterToolsByName(tools, map[string]bool{"health": true})
	}
	allowed := allowedTools(agent)
	return filterToolsByName(tools, allowed)
}

func allowedTools(agent *storepkg.Agent) map[string]bool {
	allowed := make(map[string]bool, 8)
	if agent == nil {
		return allowed
	}
	allowed["health"] = true
	if agent.CanQuery {
		allowed["query"] = true
	}
	if agent.CanExecute {
		allowed["execute"] = true
	}
	if agent.CanManageApprovedQueries {
		allowed["list_queries"] = true
		allowed["create_query"] = true
		allowed["update_query"] = true
		allowed["delete_query"] = true
	}
	if agent.ApprovedQueryScope != storepkg.ApprovedQueryScopeNone {
		allowed["list_queries"] = true
		if agent.ApprovedQueryScope == storepkg.ApprovedQueryScopeAll || len(agent.ApprovedQueryIDs) > 0 {
			allowed["execute_query"] = true
		}
	}
	return allowed
}

func filterToolsByName(tools []mcp.Tool, allowed map[string]bool) []mcp.Tool {
	filtered := make([]mcp.Tool, 0, len(tools))
	for _, tool := range tools {
		if allowed[tool.Name] {
			filtered = append(filtered, tool)
		}
	}
	return filtered
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

func parseQueryToolRequest(req mcp.CallToolRequest) (queryToolRequest, error) {
	args, _ := req.Params.Arguments.(map[string]any)
	if args == nil {
		args = map[string]any{}
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return queryToolRequest{}, errors.New("invalid query arguments")
	}
	var input queryToolRequest
	if err := json.Unmarshal(raw, &input); err != nil {
		return queryToolRequest{}, errors.New("invalid query arguments")
	}
	return input, nil
}

func approvedQueryFromToolRequest(input queryToolRequest) *storepkg.ApprovedQuery {
	parameters := input.Parameters
	if parameters == nil {
		parameters = []models.QueryParameter{}
	}
	outputColumns := input.OutputColumns
	if outputColumns == nil {
		outputColumns = []models.OutputColumn{}
	}
	return &storepkg.ApprovedQuery{
		NaturalLanguagePrompt: input.NaturalLanguagePrompt,
		AdditionalContext:     input.AdditionalContext,
		SQLQuery:              input.SQLQuery,
		AllowsModification:    input.AllowsModification,
		Parameters:            parameters,
		OutputColumns:         outputColumns,
		Constraints:           input.Constraints,
	}
}

func queryParameterItemSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":        map[string]any{"type": "string"},
			"type":        map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"required":    map[string]any{"type": "boolean"},
			"default":     map[string]any{},
			"example":     map[string]any{},
		},
		"additionalProperties": true,
	}
}

func outputColumnItemSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":        map[string]any{"type": "string"},
			"type":        map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
		},
		"additionalProperties": true,
	}
}
