package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ekaya-inc/dataclaw/internal/core"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/pkg/models"
)

type authDiag struct {
	reason    string
	tokenHint string
}

type Server struct {
	httpServer *server.StreamableHTTPServer
	service    *core.Service
}

type authorizedAgentKey struct{}

type approvedQueryResponse struct {
	QueryID               string                  `json:"query_id"`
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

const (
	approvedQueryTemplateExample = "Approved query template rules:\n" +
		"1. Placeholders: use {{parameter_name}} for every caller-supplied value, and declare a matching entry in `parameters` for each. Names match ^[A-Za-z_][A-Za-z0-9_]*$. The same placeholder may appear multiple times.\n" +
		"2. Pagination: do NOT put LIMIT/OFFSET (or other pagination clauses) in sql_query. Callers pass `limit` and `offset` to execute_query and DataClaw appends pagination at execute time.\n" +
		"3. Bind markers: do NOT use datasource-native bind markers in sql_query — {{name}} is bound as a prepared-statement parameter under the hood.\n" +
		"4. Casting: when the declared parameter type already matches the column or operator type, no cast is needed; the value is bound as that type. Add `CAST({{name}} AS <type>)` only when the database needs a different type for an SQL operator (e.g. timestamp math, numeric kind juggling, comparing against a column whose type differs from the parameter).\n" +
		"5. Arrays: use `ANY({{arr}})` rather than `IN(...)`. Array parameter types (string[], integer[]) require an adapter that advertises array support; execute_query rejects them otherwise.\n" +
		"6. Determinism: include a stable, total ORDER BY (e.g. created_at DESC, order_id DESC) for any query callers will paginate.\n" +
		"Example:\n" +
		"SELECT order_id, user_id, status, created_at, num_of_item\n" +
		"FROM orders\n" +
		"WHERE status = {{status}}\n" +
		"  AND created_at >= {{created_after}}\n" +
		"  AND user_id = {{user_id}}\n" +
		"  AND num_of_item >= {{min_items}}\n" +
		"ORDER BY created_at DESC, order_id DESC\n" +
		"CAST examples:\n" +
		"- Timestamp math: `created_at >= CAST({{created_after}} AS timestamp)` when the operator needs a timestamp.\n" +
		"- Numeric precision: `amount >= CAST({{min_amount}} AS numeric(12,2))` when decimal precision matters."
	approvedQueryTemplateReference = "Follow the approved query template rules: use {{parameter_name}} with matching parameters, omit LIMIT/OFFSET, avoid datasource-native bind markers, cast only when an SQL operator needs a different type (for example `CAST({{created_after}} AS timestamp)` for timestamp math or `CAST({{min_amount}} AS numeric(12,2))` for numeric precision), use ANY({{arr}}) for arrays, and include a stable ORDER BY for paginated reads."
	parameterValuesDescription     = "Parameter values keyed by parameter name. Use native JSON values when possible: strings for string, date (YYYY-MM-DD), timestamp (RFC3339), and uuid; numbers or numeric strings for integer and decimal; booleans or parseable boolean strings for boolean; JSON arrays or comma-separated strings for string[] and integer[]. Array parameters require a datasource adapter that supports arrays."
	rawSQLDiscoveryDescription     = " Check the active datasource dialect before writing SQL: tools/list includes it on this argument when available; call get_datasource_information, and explore_schema when available, for datasource metadata and table shape."
)

var supportedQueryParameterTypes = []string{
	"string",
	"integer",
	"decimal",
	"boolean",
	"date",
	"timestamp",
	"uuid",
	"string[]",
	"integer[]",
}

func New(version string, service *core.Service) *Server {
	mcpServer := buildMCPServer(version, service)
	return &Server{httpServer: server.NewStreamableHTTPServer(mcpServer, server.WithStateLess(true)), service: service}
}

func buildMCPServer(version string, service *core.Service) *server.MCPServer {
	descriptionCache := newDatasourceInfoDescriptionCache()
	templateSyntaxCache := newTemplateSyntaxHintsCache()
	hooks := &server.Hooks{}
	hooks.AddAfterListTools(func(ctx context.Context, _ any, _ *mcp.ListToolsRequest, result *mcp.ListToolsResult) {
		if result == nil {
			return
		}
		filtered := filterToolsForContext(ctx, service, result.Tools)
		filtered = enrichDatasourceInformationToolDescriptions(ctx, service, descriptionCache, filtered)
		filtered = enrichApprovedQueryToolDescriptions(ctx, service, templateSyntaxCache, filtered)
		filtered = enrichActiveDatasourceSQLToolDescriptions(ctx, service, filtered)
		result.Tools = filtered
	})

	mcpServer := server.NewMCPServer("dataclaw", version, server.WithToolCapabilities(true), server.WithHooks(hooks))
	registerHealthTool(mcpServer, version, service)
	registerDatasourceInformationTool(mcpServer, service)
	registerSchemaExplorationTool(mcpServer, service)
	registerQueryTool(mcpServer, service)
	registerExecuteTool(mcpServer, service)
	registerListQueriesTool(mcpServer, service)
	registerValidateQueryTool(mcpServer, service)
	registerCreateQueryTool(mcpServer, service)
	registerUpdateQueryTool(mcpServer, service)
	registerDeleteQueryTool(mcpServer, service)
	registerExecuteQueryTool(mcpServer, service)
	registerCountRowsTool(mcpServer, service)
	return mcpServer
}

func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent, diag, err := s.authorize(r)
		if err != nil || agent == nil {
			attrs := []any{
				"reason", diag.reason,
				"method", r.Method,
				"path", r.URL.Path,
				"remote", r.RemoteAddr,
				"user_agent", r.UserAgent(),
			}
			if diag.tokenHint != "" {
				attrs = append(attrs, "token_prefix", diag.tokenHint)
			}
			if err != nil {
				attrs = append(attrs, "error", err.Error())
			}
			slog.Warn("mcp request unauthorized", attrs...)
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}
		s.httpServer.ServeHTTP(w, r.WithContext(withAuthorizedAgent(r.Context(), agent)))
	})
}

func (s *Server) authorize(r *http.Request) (*storepkg.Agent, authDiag, error) {
	cred := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(cred), "bearer ") {
		cred = strings.TrimSpace(cred[7:])
	}
	if cred == "" {
		cred = strings.TrimSpace(r.Header.Get("X-API-Key"))
	}
	if cred == "" {
		return nil, authDiag{reason: "no_credential"}, errors.New("missing api key")
	}
	agent, err := s.service.AuthenticateAgent(r.Context(), cred)
	hint := tokenHint(cred)
	if err != nil {
		return nil, authDiag{reason: "lookup_failed", tokenHint: hint}, err
	}
	if agent == nil {
		return nil, authDiag{reason: "unknown_token", tokenHint: hint}, nil
	}
	return agent, authDiag{}, nil
}

func tokenHint(token string) string {
	const prefixLen = 8
	if len(token) <= prefixLen {
		return token
	}
	return token[:prefixLen] + "..."
}

func registerQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("query",
		mcp.WithDescription("Execute one read-only SQL SELECT or WITH statement against the configured datasource when the authenticated agent has raw query access. Call get_datasource_information, and explore_schema when available, first when you need the SQL dialect, schema, or table shape."),
		mcp.WithString("sql", mcp.Required(), mcp.Description("Single datasource-dialect SQL SELECT or WITH statement to execute. Semicolon-separated batches are rejected."+rawSQLDiscoveryDescription)),
		mcp.WithNumber("limit", mcp.Description("Maximum rows to return (default 100, max 1000)")),
		mcp.WithNumber("offset", mcp.Description("Zero-based row offset for deterministic pagination. Use a stable top-level ORDER BY when offset is greater than zero.")),
		mcp.WithString("result_format", mcp.Description("Response format for tabular results: tsv (default) or json.")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	srv.AddTool(tool, trackedToolHandler(service, "query", func(ctx context.Context, agent *storepkg.Agent, req mcp.CallToolRequest) (any, error) {
		if !agent.CanQuery {
			return nil, errors.New("agent is not allowed to use raw query")
		}
		sqlQuery, err := req.RequireString("sql")
		if err != nil {
			return nil, err
		}
		return service.TestRawQuery(ctx, sqlQuery, extractQueryOptions(req))
	}))
}

func registerExecuteTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("execute",
		mcp.WithDescription("Execute one ad-hoc DDL or DML statement against the configured datasource when the authenticated agent has raw execute access. One statement per call: semicolon-separated batches are rejected, while datasource-valid procedural DDL bodies are allowed when they form one statement. Call get_datasource_information, and explore_schema when available, first when you need the SQL dialect, schema, or table shape."),
		mcp.WithString("sql", mcp.Required(), mcp.Description("Single datasource-dialect DDL or DML statement to execute. One statement per call: semicolon-separated batches are rejected, while datasource-valid procedural DDL bodies are allowed when they form one statement."+rawSQLDiscoveryDescription)),
		mcp.WithNumber("limit", mcp.Description("Maximum returned rows when the statement returns rows (default 100, max 1000)")),
		mcp.WithNumber("offset", mcp.Description("Zero-based returned-row offset when the statement returns rows.")),
		mcp.WithString("result_format", mcp.Description("Response format when the statement returns rows: tsv (default) or json.")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	srv.AddTool(tool, trackedToolHandler(service, "execute", func(ctx context.Context, agent *storepkg.Agent, req mcp.CallToolRequest) (any, error) {
		if !agent.CanExecute {
			return nil, errors.New("agent is not allowed to use raw execute")
		}
		sqlQuery, err := req.RequireString("sql")
		if err != nil {
			return nil, err
		}
		return service.ExecuteRawStatement(ctx, sqlQuery, extractQueryOptions(req))
	}))
}

func registerListQueriesTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("list_queries",
		mcp.WithDescription("List approved queries available to or manageable by the authenticated agent."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	srv.AddTool(tool, trackedToolHandler(service, "list_queries", func(ctx context.Context, agent *storepkg.Agent, _ mcp.CallToolRequest) (any, error) {
		queries, err := service.ListQueriesForAgent(ctx, agent)
		if err != nil {
			return nil, err
		}
		return map[string]any{"queries": normalizeApprovedQueries(queries)}, nil
	}))
}

func registerValidateQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("validate_query",
		mcp.WithDescription("Validate an approved query draft without creating or updating the catalog when the authenticated agent can manage approved queries. "+approvedQueryTemplateReference),
		mcp.WithString("sql_query", mcp.Required(), mcp.Description("SQL body to validate without persistence. "+approvedQueryTemplateReference)),
		mcp.WithBoolean("allows_modification", mcp.Description("Controls validation behavior: false requires a read-only SELECT/WITH query; true requires INSERT, UPDATE, or DELETE.")),
		mcp.WithArray("parameters", mcp.Description("Optional parameter definitions for the draft. Every defined parameter must be used in sql_query, and every {{parameter_name}} placeholder used in sql_query must have a matching definition."), mcp.Items(queryParameterItemSchema())),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	srv.AddTool(tool, trackedToolHandler(service, "validate_query", func(ctx context.Context, agent *storepkg.Agent, req mcp.CallToolRequest) (any, error) {
		input, err := parseQueryToolRequest(req)
		if err != nil {
			return nil, err
		}
		normalized, err := service.ValidateQuerySQLForAgent(ctx, agent, input.SQLQuery, input.Parameters, input.AllowsModification)
		if err != nil {
			return nil, err
		}
		return map[string]any{"valid": true, "normalized_sql": normalized}, nil
	}))
}

func registerCreateQueryTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("create_query",
		mcp.WithDescription("Create an approved query in the catalog when the authenticated agent can manage approved queries. "+approvedQueryTemplateExample),
		mcp.WithString("natural_language_prompt", mcp.Required(), mcp.Description("Human-readable prompt describing when to use the query.")),
		mcp.WithString("additional_context", mcp.Description("Optional usage notes or extra context for the query.")),
		mcp.WithString("sql_query", mcp.Required(), mcp.Description("SQL body for the approved query. "+approvedQueryTemplateReference)),
		mcp.WithBoolean("allows_modification", mcp.Description("Controls validation and execution behavior: false requires a read-only SELECT/WITH query; true requires INSERT, UPDATE, or DELETE and executes through the mutating DML path.")),
		mcp.WithArray("parameters", mcp.Description("Optional parameter definitions for the query. Every defined parameter must be used in sql_query, and every {{parameter_name}} placeholder used in sql_query must have a matching definition."), mcp.Items(queryParameterItemSchema())),
		mcp.WithArray("output_columns", mcp.Description("Optional documented output columns. When present, each entry requires non-empty name and type; description is optional."), mcp.Items(outputColumnItemSchema())),
		mcp.WithString("constraints", mcp.Description("Optional business constraints or caveats.")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
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
		mcp.WithDescription("Replace an approved query definition when the authenticated agent can manage approved queries. "+approvedQueryTemplateReference),
		mcp.WithString("query_id", mcp.Required(), mcp.Description("Approved query ID to replace.")),
		mcp.WithString("natural_language_prompt", mcp.Required(), mcp.Description("Human-readable prompt describing when to use the query.")),
		mcp.WithString("additional_context", mcp.Description("Optional usage notes or extra context for the query.")),
		mcp.WithString("sql_query", mcp.Required(), mcp.Description("Full replacement SQL body for the approved query. "+approvedQueryTemplateReference)),
		mcp.WithBoolean("allows_modification", mcp.Description("Controls validation and execution behavior: false requires a read-only SELECT/WITH query; true requires INSERT, UPDATE, or DELETE and executes through the mutating DML path.")),
		mcp.WithArray("parameters", mcp.Description("Full replacement parameter definitions for the query. Every defined parameter must be used in sql_query, and every {{parameter_name}} placeholder used in sql_query must have a matching definition."), mcp.Items(queryParameterItemSchema())),
		mcp.WithArray("output_columns", mcp.Description("Full replacement documented output columns. When present, each entry requires non-empty name and type; description is optional."), mcp.Items(outputColumnItemSchema())),
		mcp.WithString("constraints", mcp.Description("Optional business constraints or caveats.")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
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
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
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
		mcp.WithDescription("Execute an approved query that the authenticated agent is allowed to access. Inspect list_queries for allows_modification before calling: read-only approved queries run as SELECT/WITH, while allows_modification=true queries run as mutating DML. Pass parameters as an object keyed by parameter name; values are validated, coerced, and bound according to the approved query's parameter definitions before execution. Pagination is controlled by the top-level `limit` and `offset` arguments on this call — do not put them inside `parameters`, and the approved query SQL itself does not include LIMIT/OFFSET."),
		mcp.WithString("query_id", mcp.Required(), mcp.Description("Query ID")),
		mcp.WithObject("parameters", mcp.Description(parameterValuesDescription)),
		mcp.WithNumber("limit", mcp.Description("Maximum rows to return (default 100, max 1000)")),
		mcp.WithNumber("offset", mcp.Description("Zero-based row offset for deterministic pagination. Use a stable top-level ORDER BY when offset is greater than zero.")),
		mcp.WithString("result_format", mcp.Description("Response format for tabular results: tsv (default) or json.")),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	tool.Annotations.ReadOnlyHint = nil
	tool.Annotations.DestructiveHint = nil
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
		return service.ExecuteStoredQueryForAgent(ctx, agent, queryID, values, extractQueryOptions(req))
	}))
}

func registerCountRowsTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool("count_rows",
		mcp.WithDescription("Return an exact row count for one read-only raw SQL SELECT or one accessible read-only approved query. Provide exactly one of sql or query_id."),
		mcp.WithString("sql", mcp.Description("Single datasource-dialect SQL SELECT or WITH statement to count. Requires raw query access. Semicolon-separated batches are rejected."+rawSQLDiscoveryDescription)),
		mcp.WithString("query_id", mcp.Description("Approved query ID to count. Requires access to that approved query.")),
		mcp.WithObject("parameters", mcp.Description(parameterValuesDescription+" Only valid with query_id.")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)
	srv.AddTool(tool, trackedToolHandler(service, "count_rows", func(ctx context.Context, agent *storepkg.Agent, req mcp.CallToolRequest) (any, error) {
		args, _ := req.Params.Arguments.(map[string]any)
		if args == nil {
			args = map[string]any{}
		}
		sqlQuery, _ := args["sql"].(string)
		queryID, _ := args["query_id"].(string)
		sqlQuery = strings.TrimSpace(sqlQuery)
		queryID = strings.TrimSpace(queryID)
		hasSQL := sqlQuery != ""
		hasQueryID := queryID != ""
		if hasSQL == hasQueryID {
			return nil, errors.New("provide exactly one of sql or query_id")
		}
		if hasSQL {
			if _, hasParams := args["parameters"]; hasParams {
				return nil, errors.New("parameters are only supported with query_id")
			}
			if !agent.CanQuery {
				return nil, errors.New("agent is not allowed to use raw query")
			}
			return service.CountRows(ctx, sqlQuery)
		}
		values, err := parseParameterValues(args["parameters"])
		if err != nil {
			return nil, err
		}
		return service.CountStoredQueryForAgent(ctx, agent, queryID, values)
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
	allowed[datasourceInformationToolName] = true
	if agent.CanQuery {
		allowed[schemaExplorationToolName] = true
		allowed["query"] = true
		allowed["count_rows"] = true
	}
	if agent.CanExecute {
		allowed["execute"] = true
	}
	if agent.CanManageApprovedQueries {
		allowed["list_queries"] = true
		allowed["validate_query"] = true
		allowed["create_query"] = true
		allowed["update_query"] = true
		allowed["delete_query"] = true
	}
	if agent.ApprovedQueryScope != storepkg.ApprovedQueryScopeNone {
		allowed["list_queries"] = true
		if agent.ApprovedQueryScope == storepkg.ApprovedQueryScopeAll || len(agent.ApprovedQueryIDs) > 0 {
			allowed["execute_query"] = true
			allowed["count_rows"] = true
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

func extractQueryOptions(req mcp.CallToolRequest) core.QueryOptions {
	options := core.QueryOptions{}
	if args, ok := req.Params.Arguments.(map[string]any); ok {
		if limit, ok := intValue(args["limit"]); ok {
			options.Limit = limit
		}
		if offset, ok := intValue(args["offset"]); ok {
			options.Offset = offset
		}
	}
	return options
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
			"name": map[string]any{
				"type":        "string",
				"description": "Parameter name that appears in sql_query as {{parameter_name}}.",
			},
			"type": map[string]any{
				"type":        "string",
				"enum":        supportedQueryParameterTypes,
				"description": "DataClaw parameter type used for validation and value coercion. Supported values: string, integer, decimal, boolean, date, timestamp, uuid, string[], integer[]. Use SQL casts in sql_query when the datasource needs a more specific SQL type.",
			},
			"description": map[string]any{"type": "string"},
			"required":    map[string]any{"type": "boolean"},
			"default": map[string]any{
				"description": "Optional default value using the same JSON shape accepted by execute_query parameters.",
			},
			"example": map[string]any{
				"description": "Example value using the same JSON shape accepted by execute_query parameters.",
			},
		},
		"additionalProperties": true,
	}
}

func outputColumnItemSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":        map[string]any{"type": "string"},
			"type":        map[string]any{"type": "string", "description": "Required documentation-only result column type label; it is not used for result validation or coercion. Prefer the DataClaw parameter type vocabulary when it fits, or the datasource-native column type when that is clearer."},
			"description": map[string]any{"type": "string"},
		},
		"additionalProperties": true,
	}
}
