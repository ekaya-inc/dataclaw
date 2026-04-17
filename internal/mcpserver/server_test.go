package mcpserver

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgoserver "github.com/mark3labs/mcp-go/server"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	_ "github.com/ekaya-inc/dataclaw/internal/adapters/datasource/mssql"
	_ "github.com/ekaya-inc/dataclaw/internal/adapters/datasource/postgres"
	"github.com/ekaya-inc/dataclaw/internal/core"
	"github.com/ekaya-inc/dataclaw/internal/security"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/migrations"
)

type httpAuthContextKey struct{}

func TestListToolsFiltersByAgentPermissions(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newTestMCPClient(t)

	queryReader, err := service.CreateAgent(ctx, core.AgentInput{
		Name:               "Reader",
		CanQuery:           true,
		ApprovedQueryScope: storepkg.ApprovedQueryScopeAll,
	})
	if err != nil {
		t.Fatalf("CreateAgent(reader): %v", err)
	}
	executor, err := service.CreateAgent(ctx, core.AgentInput{
		Name:       "Executor",
		CanExecute: true,
	})
	if err != nil {
		t.Fatalf("CreateAgent(executor): %v", err)
	}

	readerAgent, err := service.AuthenticateAgent(ctx, queryReader.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent(reader): %v", err)
	}
	executorAgent, err := service.AuthenticateAgent(ctx, executor.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent(executor): %v", err)
	}

	readerTools := listToolNames(t, withAuthorizedAgent(ctx, readerAgent), mcpClient)
	if got, want := readerTools, []string{"execute_query", "health", "list_queries", "query"}; !equalStrings(got, want) {
		t.Fatalf("unexpected reader tools: got %v want %v", got, want)
	}

	executorTools := listToolNames(t, withAuthorizedAgent(ctx, executorAgent), mcpClient)
	if got, want := executorTools, []string{"execute", "health"}; !equalStrings(got, want) {
		t.Fatalf("unexpected executor tools: got %v want %v", got, want)
	}
}

func TestSelectedScopeFiltersListQueriesAndBlocksUnauthorizedExecute(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newTestMCPClient(t)

	queryA, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{NaturalLanguagePrompt: "List accounts", SQLQuery: "SELECT * FROM accounts"})
	if err != nil {
		t.Fatalf("CreateQuery(queryA): %v", err)
	}
	queryB, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{NaturalLanguagePrompt: "List contacts", SQLQuery: "SELECT * FROM contacts"})
	if err != nil {
		t.Fatalf("CreateQuery(queryB): %v", err)
	}
	selected, err := service.CreateAgent(ctx, core.AgentInput{
		Name:               "Selected reader",
		ApprovedQueryScope: storepkg.ApprovedQueryScopeSelected,
		ApprovedQueryIDs:   []string{queryA.ID},
	})
	if err != nil {
		t.Fatalf("CreateAgent(selected): %v", err)
	}
	selectedAgent, err := service.AuthenticateAgent(ctx, selected.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent(selected): %v", err)
	}
	agentCtx := withAuthorizedAgent(ctx, selectedAgent)

	payload := callToolJSON(t, agentCtx, mcpClient, "list_queries", nil)
	queries := asSlice(t, payload["queries"])
	if len(queries) != 1 {
		t.Fatalf("expected 1 accessible query, got %d", len(queries))
	}
	query := asMap(t, queries[0])
	assertQueryFields(t, query, queryA.ID, queryA.SQLQuery)

	assertToolError(t, agentCtx, mcpClient, "execute_query", map[string]any{"query_id": queryB.ID}, "agent is not allowed to execute this approved query")
}

func TestDatasourceDeletionFailsClosedForToolDiscovery(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newTestMCPClient(t)

	agentView, err := service.CreateAgent(ctx, core.AgentInput{
		Name:               "Universal",
		CanQuery:           true,
		CanExecute:         true,
		ApprovedQueryScope: storepkg.ApprovedQueryScopeAll,
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	agent, err := service.AuthenticateAgent(ctx, agentView.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent: %v", err)
	}
	agentCtx := withAuthorizedAgent(ctx, agent)

	before := listToolNames(t, agentCtx, mcpClient)
	if len(before) != 5 {
		t.Fatalf("expected 5 tools before datasource deletion, got %v", before)
	}
	if err := service.DeleteDatasource(ctx); err != nil {
		t.Fatalf("DeleteDatasource: %v", err)
	}

	after := listToolNames(t, agentCtx, mcpClient)
	if got, want := after, []string{"health"}; !equalStrings(got, want) {
		t.Fatalf("expected health-only tools after datasource deletion, got %v want %v", got, want)
	}
	assertToolError(t, agentCtx, mcpClient, "list_queries", nil, "no datasource configured")
}

func TestHTTPHeaderAuthMatrixAndLastUsedAt(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), true)

	queryA, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{NaturalLanguagePrompt: "List accounts", SQLQuery: "SELECT * FROM accounts"})
	if err != nil {
		t.Fatalf("CreateQuery(queryA): %v", err)
	}
	queryB, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{NaturalLanguagePrompt: "List contacts", SQLQuery: "SELECT * FROM contacts"})
	if err != nil {
		t.Fatalf("CreateQuery(queryB): %v", err)
	}

	reader, err := service.CreateAgent(ctx, core.AgentInput{
		Name:               "Reader",
		CanQuery:           true,
		ApprovedQueryScope: storepkg.ApprovedQueryScopeSelected,
		ApprovedQueryIDs:   []string{queryA.ID},
	})
	if err != nil {
		t.Fatalf("CreateAgent(reader): %v", err)
	}
	writer, err := service.CreateAgent(ctx, core.AgentInput{Name: "Writer", CanExecute: true})
	if err != nil {
		t.Fatalf("CreateAgent(writer): %v", err)
	}

	readerTools := listToolNamesWithHeader(t, ctx, mcpClient, reader.APIKey)
	if got, want := readerTools, []string{"execute_query", "health", "list_queries", "query"}; !equalStrings(got, want) {
		t.Fatalf("unexpected reader tools via header auth: got %v want %v", got, want)
	}
	readerAfterList, err := service.GetAgent(ctx, reader.ID)
	if err != nil {
		t.Fatalf("GetAgent(reader after tools/list): %v", err)
	}
	if readerAfterList.LastUsedAt != nil {
		t.Fatalf("expected tools/list not to update last_used_at, got %#v", readerAfterList.LastUsedAt)
	}
	callToolJSONWithHeader(t, ctx, mcpClient, "health", nil, reader.APIKey)
	readerAfterHealth, err := service.GetAgent(ctx, reader.ID)
	if err != nil {
		t.Fatalf("GetAgent(reader after health): %v", err)
	}
	if readerAfterHealth.LastUsedAt != nil {
		t.Fatalf("expected health call not to update last_used_at, got %#v", readerAfterHealth.LastUsedAt)
	}

	payload := callToolJSONWithHeader(t, ctx, mcpClient, "list_queries", nil, reader.APIKey)
	queries := asSlice(t, payload["queries"])
	if len(queries) != 1 {
		t.Fatalf("expected 1 query through header auth, got %d", len(queries))
	}
	readerAfterCall, err := service.GetAgent(ctx, reader.ID)
	if err != nil {
		t.Fatalf("GetAgent(reader after tools/call): %v", err)
	}
	if readerAfterCall.LastUsedAt == nil {
		t.Fatal("expected successful tools/call to update last_used_at")
	}

	writerTools := listToolNamesWithHeader(t, ctx, mcpClient, writer.APIKey)
	if got, want := writerTools, []string{"execute", "health"}; !equalStrings(got, want) {
		t.Fatalf("unexpected writer tools via header auth: got %v want %v", got, want)
	}

	assertToolErrorWithHeader(t, ctx, mcpClient, "execute_query", map[string]any{"query_id": queryB.ID}, reader.APIKey, "agent is not allowed to execute this approved query")
	_, err = mcpClient.ListTools(withHTTPAuth(ctx, "wrong-key"), mcp.ListToolsRequest{})
	if err == nil {
		t.Fatal("expected invalid bearer key to fail list tools")
	}
}

func TestHTTPHealthStaysAvailableWithoutDatasource(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClientWithFactoryAndDatasource(t, newFakeMCPAdapterFactory(), false)

	observer, err := service.CreateAgent(ctx, core.AgentInput{
		Name:               "Observer",
		ApprovedQueryScope: storepkg.ApprovedQueryScopeAll,
	})
	if err != nil {
		t.Fatalf("CreateAgent(observer): %v", err)
	}

	if got, want := listToolNamesWithHeader(t, ctx, mcpClient, observer.APIKey), []string{"health"}; !equalStrings(got, want) {
		t.Fatalf("unexpected observer tools via header auth without datasource: got %v want %v", got, want)
	}

	payload := callToolJSONWithHeader(t, ctx, mcpClient, "health", nil, observer.APIKey)
	if got := requireString(t, payload, "engine"); got != "healthy" {
		t.Fatalf("expected healthy engine, got %q", got)
	}
	if got := requireString(t, payload, "version"); got != "test" {
		t.Fatalf("expected version test, got %q", got)
	}
	datasource := asMap(t, payload["datasource"])
	if got := requireString(t, datasource, "status"); got != "not_configured" {
		t.Fatalf("expected datasource status not_configured, got %q", got)
	}
	if got := requireString(t, datasource, "error"); got != "no datasource configured" {
		t.Fatalf("expected no datasource configured error, got %q", got)
	}

	observerAfterHealth, err := service.GetAgent(ctx, observer.ID)
	if err != nil {
		t.Fatalf("GetAgent(observer after health): %v", err)
	}
	if observerAfterHealth.LastUsedAt != nil {
		t.Fatalf("expected health call not to update last_used_at, got %#v", observerAfterHealth.LastUsedAt)
	}

	assertToolErrorWithHeader(t, ctx, mcpClient, "list_queries", nil, observer.APIKey, "no datasource configured")
}

func TestSelectedScopeLosesExecuteToolWhenMembershipsCascadeAway(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newHTTPMCPClient(t)

	query, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{NaturalLanguagePrompt: "List accounts", SQLQuery: "SELECT * FROM accounts"})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}
	agent, err := service.CreateAgent(ctx, core.AgentInput{
		Name:               "Selected",
		ApprovedQueryScope: storepkg.ApprovedQueryScopeSelected,
		ApprovedQueryIDs:   []string{query.ID},
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	before := listToolNamesWithHeader(t, ctx, mcpClient, agent.APIKey)
	if got, want := before, []string{"execute_query", "health", "list_queries"}; !equalStrings(got, want) {
		t.Fatalf("unexpected tools before membership cascade: got %v want %v", got, want)
	}

	if err := service.DeleteQuery(ctx, query.ID); err != nil {
		t.Fatalf("DeleteQuery: %v", err)
	}

	after := listToolNamesWithHeader(t, ctx, mcpClient, agent.APIKey)
	if got, want := after, []string{"health", "list_queries"}; !equalStrings(got, want) {
		t.Fatalf("unexpected tools after membership cascade: got %v want %v", got, want)
	}
}

func newTestMCPClient(t *testing.T) (*client.Client, *core.Service) {
	return newTestMCPClientWithFactoryAndDatasource(t, dsadapter.NewFactory(dsadapter.DefaultRegistry()), true)
}

func newTestMCPClientWithFactoryAndDatasource(t *testing.T, factory dsadapter.Factory, seedDatasource bool) (*client.Client, *core.Service) {
	t.Helper()

	ctx := context.Background()
	service, st := newTestMCPService(t, factory, seedDatasource)

	mcpServer := New("test", service)
	inProcess, err := client.NewInProcessClient(extractMCPServer(t, mcpServer))
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	if err := inProcess.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var initReq mcp.InitializeRequest
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test-client", Version: "1.0.0"}
	if _, err := inProcess.Initialize(ctx, initReq); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	t.Cleanup(func() {
		inProcess.Close()
		_ = service.Close()
		_ = st.Close()
	})

	return inProcess, service
}

func newHTTPMCPClient(t *testing.T) (*client.Client, *core.Service) {
	return newHTTPMCPClientWithFactoryAndDatasource(t, dsadapter.NewFactory(dsadapter.DefaultRegistry()), true)
}

func newHTTPMCPClientWithFactoryAndDatasource(t *testing.T, factory dsadapter.Factory, seedDatasource bool) (*client.Client, *core.Service) {
	t.Helper()

	ctx := context.Background()
	service, st := newTestMCPService(t, factory, seedDatasource)
	bootstrapAgent, err := service.CreateAgent(ctx, core.AgentInput{Name: "Bootstrap", CanQuery: true})
	if err != nil {
		t.Fatalf("CreateAgent(bootstrap): %v", err)
	}

	httpSrv := httptest.NewServer(New("test", service).Handler())
	mcpClient, err := client.NewStreamableHttpClient(httpSrv.URL, transport.WithHTTPHeaderFunc(func(ctx context.Context) map[string]string {
		token, _ := ctx.Value(httpAuthContextKey{}).(string)
		if token == "" {
			return nil
		}
		return map[string]string{"Authorization": "Bearer " + token}
	}))
	if err != nil {
		t.Fatalf("NewStreamableHttpClient: %v", err)
	}
	if err := mcpClient.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var initReq mcp.InitializeRequest
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test-client", Version: "1.0.0"}
	if _, err := mcpClient.Initialize(withHTTPAuth(ctx, bootstrapAgent.APIKey), initReq); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	t.Cleanup(func() {
		mcpClient.Close()
		httpSrv.Close()
		_ = service.Close()
		_ = st.Close()
	})

	return mcpClient, service
}

func extractMCPServer(t *testing.T, srv *Server) *mcpgoserver.MCPServer {
	t.Helper()
	return buildMCPServer("test", srv.service)
}

func newTestMCPService(t *testing.T, factory dsadapter.Factory, seedDatasource bool) (*core.Service, *storepkg.Store) {
	t.Helper()

	ctx := context.Background()
	st, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "dataclaw.sqlite"), migrations.FS)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	secret, err := security.LoadOrCreateSecret(filepath.Join(t.TempDir(), "secret.key"))
	if err != nil {
		t.Fatalf("load secret: %v", err)
	}
	service := core.New(st, secret, "test", func() string { return "http://127.0.0.1:18790" }, factory)

	if seedDatasource {
		configCiphertext, err := security.EncryptString(secret, `{"host":"db.example.com","database":"warehouse","user":"analyst","password":"secret"}`)
		if err != nil {
			t.Fatalf("encrypt datasource config: %v", err)
		}
		if err := st.SaveDatasource(ctx, &storepkg.Datasource{
			Name:            "Primary",
			Type:            "postgres",
			Provider:        "postgres",
			ConfigEncrypted: configCiphertext,
		}); err != nil {
			t.Fatalf("SaveDatasource: %v", err)
		}
	}

	return service, st
}

func listToolNames(t *testing.T, ctx context.Context, mcpClient *client.Client) []string {
	t.Helper()
	result, err := mcpClient.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := make([]string, 0, len((*result).Tools))
	for _, tool := range (*result).Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func listToolNamesWithHeader(t *testing.T, ctx context.Context, mcpClient *client.Client, apiKey string) []string {
	t.Helper()
	result, err := mcpClient.ListTools(withHTTPAuth(ctx, apiKey), mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools(header): %v", err)
	}
	names := make([]string, 0, len((*result).Tools))
	for _, tool := range (*result).Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func callToolJSON(t *testing.T, ctx context.Context, mcpClient *client.Client, tool string, args map[string]any) map[string]any {
	t.Helper()

	result, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      tool,
			Arguments: args,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", tool, err)
	}
	if result.IsError {
		t.Fatalf("CallTool(%s) returned error: %s", tool, resultText(t, result))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &payload); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	return payload
}

func callToolJSONWithHeader(t *testing.T, ctx context.Context, mcpClient *client.Client, tool string, args map[string]any, apiKey string) map[string]any {
	t.Helper()

	result, err := mcpClient.CallTool(withHTTPAuth(ctx, apiKey), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      tool,
			Arguments: args,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(%s, header): %v", tool, err)
	}
	if result.IsError {
		t.Fatalf("CallTool(%s, header) returned error: %s", tool, resultText(t, result))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(resultText(t, result)), &payload); err != nil {
		t.Fatalf("unmarshal tool result: %v", err)
	}
	return payload
}

func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Content))
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return text.Text
}

func assertQueryFields(t *testing.T, query map[string]any, expectedID, expectedSQL string) {
	t.Helper()

	if got := requireString(t, query, "query_id"); got != expectedID {
		t.Fatalf("expected query_id %q, got %q", expectedID, got)
	}
	if got := requireString(t, query, "sql_query"); got != expectedSQL {
		t.Fatalf("expected sql_query %q, got %q", expectedSQL, got)
	}
}

func assertToolError(t *testing.T, ctx context.Context, mcpClient *client.Client, tool string, args map[string]any, expected string) {
	t.Helper()

	result, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      tool,
			Arguments: args,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", tool, err)
	}
	if !result.IsError {
		t.Fatalf("expected %s to return an error", tool)
	}
	if got := resultText(t, result); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func assertToolErrorWithHeader(t *testing.T, ctx context.Context, mcpClient *client.Client, tool string, args map[string]any, apiKey string, expected string) {
	t.Helper()

	result, err := mcpClient.CallTool(withHTTPAuth(ctx, apiKey), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      tool,
			Arguments: args,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(%s, header): %v", tool, err)
	}
	if !result.IsError {
		t.Fatalf("expected %s to return an error", tool)
	}
	if got := resultText(t, result); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func requireString(t *testing.T, payload map[string]any, key string) string {
	t.Helper()
	value, ok := payload[key].(string)
	if !ok || value == "" {
		t.Fatalf("expected %q to be a non-empty string, got %#v", key, payload[key])
	}
	return value
}

func asSlice(t *testing.T, value any) []any {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("expected []any, got %T", value)
	}
	return items
}

func asMap(t *testing.T, value any) map[string]any {
	t.Helper()
	record, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", value)
	}
	return record
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func withHTTPAuth(ctx context.Context, apiKey string) context.Context {
	return context.WithValue(ctx, httpAuthContextKey{}, apiKey)
}
