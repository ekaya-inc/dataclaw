package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ekaya-inc/dataclaw/internal/core"
)

const healthDatasourceTimeout = 2 * time.Second

type healthResult struct {
	Engine     string            `json:"engine"`
	Version    string            `json:"version"`
	Datasource *datasourceHealth `json:"datasource,omitempty"`
}

type datasourceHealth struct {
	Name   string `json:"name,omitempty"`
	Type   string `json:"type,omitempty"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func registerHealthTool(srv *server.MCPServer, version string, service *core.Service) {
	tool := mcp.NewTool(
		"health",
		mcp.WithDescription("Returns server health status, version, and datasource connectivity"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	srv.AddTool(tool, func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if _, err := requireAgent(ctx); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		body, err := json.Marshal(healthResult{
			Engine:     "healthy",
			Version:    version,
			Datasource: checkDatasourceHealth(ctx, service),
		})
		if err != nil {
			return nil, fmt.Errorf("marshal health response: %w", err)
		}
		return mcp.NewToolResultText(string(body)), nil
	})
}

func checkDatasourceHealth(ctx context.Context, service *core.Service) *datasourceHealth {
	ds, err := service.GetDatasource(ctx)
	if err != nil {
		return &datasourceHealth{
			Status: "error",
			Error:  fmt.Sprintf("failed to load datasource: %v", err),
		}
	}
	if ds == nil {
		return &datasourceHealth{
			Status: "not_configured",
			Error:  "no datasource configured",
		}
	}

	result := &datasourceHealth{
		Name: ds.Name,
		Type: ds.Type,
	}

	testCtx, cancel := context.WithTimeout(ctx, healthDatasourceTimeout)
	defer cancel()

	if err := service.TestDatasource(testCtx, ds); err != nil {
		result.Status = "error"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(testCtx.Err(), context.DeadlineExceeded) {
			result.Error = fmt.Sprintf("datasource health check timed out after %s", healthDatasourceTimeout)
			return result
		}
		result.Error = err.Error()
		return result
	}

	result.Status = "connected"
	return result
}
