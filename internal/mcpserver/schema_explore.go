package mcpserver

import (
	"context"
	"errors"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ekaya-inc/dataclaw/internal/core"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

const schemaExplorationToolName = "explore_schema"

const schemaExplorationDescription = "Explore datasource schemas, objects, and columns through the configured datasource adapter. Optional filters: schema_name, object_name, and detail_mode compact or full. Returns summary, objects, limitations, truncated-result hints, and unavailable_reason when metadata is not available."

func registerSchemaExplorationTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool(
		schemaExplorationToolName,
		mcp.WithDescription(schemaExplorationDescription),
		mcp.WithString("schema_name", mcp.Description("Optional schema/catalog namespace filter.")),
		mcp.WithString("object_name", mcp.Description("Optional table, view, or other object name filter.")),
		mcp.WithString("detail_mode", mcp.Description("Schema detail level: compact for object summaries or full for column details."), mcp.Enum("compact", "full")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	srv.AddTool(tool, trackedToolHandler(service, schemaExplorationToolName, func(ctx context.Context, agent *storepkg.Agent, req mcp.CallToolRequest) (any, error) {
		if !agent.CanQuery {
			return nil, errors.New("agent is not allowed to explore datasource schema")
		}
		return service.ExploreDatasourceSchema(ctx, schemaExploreRequestFromTool(req))
	}))
}

func schemaExploreRequestFromTool(req mcp.CallToolRequest) core.SchemaExploreRequest {
	args, _ := req.Params.Arguments.(map[string]any)
	return (core.SchemaExploreRequest{
		SchemaName: trimmedStringArgument(args, "schema_name"),
		ObjectName: trimmedStringArgument(args, "object_name"),
		DetailMode: core.SchemaDetailMode(trimmedStringArgument(args, "detail_mode")),
	}).Normalized()
}

func trimmedStringArgument(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}
