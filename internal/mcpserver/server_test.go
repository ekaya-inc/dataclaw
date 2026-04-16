package mcpserver

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgoserver "github.com/mark3labs/mcp-go/server"

	"github.com/ekaya-inc/dataclaw/internal/core"
	"github.com/ekaya-inc/dataclaw/internal/security"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/migrations"
)

func TestListQueriesReturnsCanonicalFields(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newTestMCPClient(t)

	created, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{
		Name:        "Connectivity Check",
		Description: "Use this for testing connectivity to the datasource",
		SQLQuery:    "SELECT true AS connected",
		IsEnabled:   true,
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}

	payload := callToolJSON(t, ctx, mcpClient, "list_queries", nil)
	queries := asSlice(t, payload["queries"])
	if len(queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(queries))
	}

	query := asMap(t, queries[0])
	assertQueryFields(t, query, created.ID, created.SQLQuery)
}

func TestCreateQueryUsesCanonicalSQLOnly(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newTestMCPClient(t)

	payload := callToolJSON(t, ctx, mcpClient, "create_query", map[string]any{
		"name":        "Connectivity Check",
		"description": "Use this for testing connectivity to the datasource",
		"sql":         "SELECT true AS connected",
		"is_enabled":  true,
	})

	query := asMap(t, payload["query"])
	queryID := requireString(t, query, "query_id")
	assertQueryFields(t, query, queryID, "SELECT true AS connected")

	stored, err := service.GetQuery(ctx, queryID)
	if err != nil {
		t.Fatalf("GetQuery: %v", err)
	}
	if stored == nil {
		t.Fatal("expected created query to be stored")
	}
	if stored.SQLQuery != "SELECT true AS connected" {
		t.Fatalf("expected stored SQL to match canonical input, got %q", stored.SQLQuery)
	}
}

func TestUpdateQueryUsesCanonicalArguments(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newTestMCPClient(t)
	created, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{
		Name:      "Connectivity Check",
		SQLQuery:  "SELECT true AS connected",
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}

	payload := callToolJSON(t, ctx, mcpClient, "update_query", map[string]any{
		"query_id":    created.ID,
		"name":        "Updated Connectivity Check",
		"description": "Updated description",
		"sql":         "SELECT 42 AS answer",
		"is_enabled":  false,
	})

	query := asMap(t, payload["query"])
	assertQueryFields(t, query, created.ID, "SELECT 42 AS answer")
	if enabled, ok := query["is_enabled"].(bool); !ok || enabled {
		t.Fatalf("expected updated query to be disabled, got %#v", query["is_enabled"])
	}
}

func TestDeleteQueryUsesCanonicalIdentifier(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newTestMCPClient(t)
	created, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{
		Name:      "Connectivity Check",
		SQLQuery:  "SELECT true AS connected",
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}

	payload := callToolJSON(t, ctx, mcpClient, "delete_query", map[string]any{"query_id": created.ID})
	if deleted, ok := payload["deleted"].(bool); !ok || !deleted {
		t.Fatalf("expected deleted=true, got %#v", payload["deleted"])
	}

	stored, err := service.GetQuery(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetQuery: %v", err)
	}
	if stored != nil {
		t.Fatalf("expected query %q to be deleted", created.ID)
	}
}

func TestLegacyAliasesAreRejected(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newTestMCPClient(t)

	created, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{
		Name:      "Connectivity Check",
		SQLQuery:  "SELECT true AS connected",
		IsEnabled: false,
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}

	tests := []struct {
		name    string
		tool    string
		args    map[string]any
		message string
	}{
		{
			name:    "create_query rejects sql_query",
			tool:    "create_query",
			args:    map[string]any{"name": "Connectivity Check", "sql_query": "SELECT true AS connected"},
			message: "required argument \"sql\" not found",
		},
		{
			name:    "update_query rejects id and sql_query",
			tool:    "update_query",
			args:    map[string]any{"id": created.ID, "name": "Updated Connectivity Check", "sql_query": "SELECT 42 AS answer"},
			message: "required argument \"query_id\" not found",
		},
		{
			name:    "delete_query rejects id",
			tool:    "delete_query",
			args:    map[string]any{"id": created.ID},
			message: "required argument \"query_id\" not found",
		},
		{
			name:    "execute_query rejects id",
			tool:    "execute_query",
			args:    map[string]any{"id": created.ID},
			message: "required argument \"query_id\" not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertToolError(t, ctx, mcpClient, tt.tool, tt.args, tt.message)
		})
	}
}

func TestExecuteQueryRejectsNonObjectParameters(t *testing.T) {
	ctx := context.Background()
	mcpClient, service := newTestMCPClient(t)

	created, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{
		Name:      "Connectivity Check",
		SQLQuery:  "SELECT true AS connected",
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}

	result, err := mcpClient.CallTool(ctx, mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "execute_query",
			Arguments: map[string]any{"query_id": created.ID, "parameters": []any{"bad"}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool(execute_query): %v", err)
	}
	if !result.IsError {
		t.Fatal("expected execute_query to return an error for invalid parameters")
	}
	if got := resultText(t, result); got != "parameters must be an object" {
		t.Fatalf("expected object-parameters error, got %q", got)
	}
}

func newTestMCPClient(t *testing.T) (*client.Client, *core.Service) {
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
	service := core.New(st, secret, "test", func() string { return "http://127.0.0.1:18790" })

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

	mcpServer := mcpgoserver.NewMCPServer("dataclaw", "test", mcpgoserver.WithToolCapabilities(true))
	registerQueryTool(mcpServer, service)
	registerListQueriesTool(mcpServer, service)
	registerCreateQueryTool(mcpServer, service)
	registerUpdateQueryTool(mcpServer, service)
	registerDeleteQueryTool(mcpServer, service)
	registerExecuteQueryTool(mcpServer, service)

	mcpClient, err := client.NewInProcessClient(mcpServer)
	if err != nil {
		t.Fatalf("NewInProcessClient: %v", err)
	}
	if err := mcpClient.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var initReq mcp.InitializeRequest
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "test-client", Version: "1.0.0"}
	if _, err := mcpClient.Initialize(ctx, initReq); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	t.Cleanup(func() {
		mcpClient.Close()
		_ = service.Close()
		_ = st.Close()
	})

	return mcpClient, service
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
	if got := requireString(t, query, "sql"); got != expectedSQL {
		t.Fatalf("expected sql %q, got %q", expectedSQL, got)
	}
	if _, exists := query["id"]; exists {
		t.Fatalf("did not expect legacy id field in response: %#v", query["id"])
	}
	if _, exists := query["sql_query"]; exists {
		t.Fatalf("did not expect legacy sql_query field in response: %#v", query["sql_query"])
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
