package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/ekaya-inc/dataclaw/internal/core"
)

const datasourceInformationToolName = "get_datasource_information"

var (
	datasourceInformationDescription        = "Returns the configured datasource identity and runtime metadata, including name, type, SQL dialect, database name, schema name, current user, and version."
	datasourceInformationDescriptionTTL     = 30 * time.Second
	datasourceInformationDescriptionTimeout = 2 * time.Second
)

type datasourceInformationResult struct {
	Name         string         `json:"name,omitempty"`
	Type         string         `json:"type,omitempty"`
	SQLDialect   string         `json:"sql_dialect,omitempty"`
	Status       string         `json:"status"`
	DatabaseName string         `json:"database_name,omitempty"`
	SchemaName   string         `json:"schema_name,omitempty"`
	CurrentUser  string         `json:"current_user,omitempty"`
	Version      string         `json:"version,omitempty"`
	Error        string         `json:"error,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
}

type datasourceInfoDescriptionCache struct {
	mu      sync.RWMutex
	entries map[datasourceInfoCacheKey]datasourceInfoDescriptionCacheEntry
}

type datasourceInfoCacheKey struct {
	ID        string
	UpdatedAt time.Time
}

type datasourceInfoDescriptionCacheEntry struct {
	description string
	storedAt    time.Time
}

func newDatasourceInfoDescriptionCache() *datasourceInfoDescriptionCache {
	return &datasourceInfoDescriptionCache{
		entries: make(map[datasourceInfoCacheKey]datasourceInfoDescriptionCacheEntry),
	}
}

func registerDatasourceInformationTool(srv *server.MCPServer, service *core.Service) {
	tool := mcp.NewTool(
		datasourceInformationToolName,
		mcp.WithDescription(datasourceInformationDescription),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
	)

	srv.AddTool(tool, func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if _, err := requireAgent(ctx); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		info, err := service.GetDatasourceInformation(ctx)
		switch {
		case err == nil:
			return marshalDatasourceInformationResult(buildDatasourceInformationResult(info, "connected", ""))
		case errors.Is(err, core.ErrNoDatasourceConfigured):
			return marshalDatasourceInformationResult(buildDatasourceInformationResult(nil, "not_configured", err.Error()))
		default:
			return marshalDatasourceInformationResult(buildDatasourceInformationResult(info, "error", err.Error()))
		}
	})
}

func marshalDatasourceInformationResult(payload datasourceInformationResult) (*mcp.CallToolResult, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return mcp.NewToolResultText(string(body)), nil
}

func buildDatasourceInformationResult(info *core.DatasourceInformation, status string, errMsg string) datasourceInformationResult {
	result := datasourceInformationResult{Status: status}
	if info != nil {
		result.Name = info.Name
		result.Type = info.Type
		result.SQLDialect = info.SQLDialect
		result.DatabaseName = info.DatabaseName
		result.SchemaName = info.SchemaName
		result.CurrentUser = info.CurrentUser
		result.Version = info.Version
		if len(info.Extra) > 0 {
			result.Extra = maps.Clone(info.Extra)
		}
	}
	if strings.TrimSpace(errMsg) != "" {
		result.Error = strings.TrimSpace(errMsg)
	}
	return result
}

func enrichDatasourceInformationToolDescriptions(ctx context.Context, service *core.Service, cache *datasourceInfoDescriptionCache, tools []mcp.Tool) []mcp.Tool {
	index := -1
	for i, tool := range tools {
		if tool.Name == datasourceInformationToolName {
			index = i
			break
		}
	}
	if index < 0 {
		return tools
	}

	ds, err := service.GetDatasource(ctx)
	if err != nil || ds == nil {
		tools[index].Description = datasourceInformationDescription
		return tools
	}

	key := datasourceInfoCacheKey{ID: ds.ID, UpdatedAt: ds.UpdatedAt.UTC()}
	if description, ok := cache.getFresh(key); ok {
		tools[index].Description = description
		return tools
	}

	refreshCtx, cancel := context.WithTimeout(ctx, datasourceInformationDescriptionTimeout)
	defer cancel()

	info, err := service.GetDatasourceInformation(refreshCtx)
	if err != nil {
		if description, ok := cache.getAny(key); ok {
			tools[index].Description = description
			return tools
		}
		tools[index].Description = datasourceInformationDescription
		return tools
	}

	description := renderDatasourceInformationDescription(info)
	cache.set(key, description)
	tools[index].Description = description
	return tools
}

func renderDatasourceInformationDescription(info *core.DatasourceInformation) string {
	if info == nil {
		return datasourceInformationDescription
	}

	parts := []string{}
	if info.Name != "" || info.Type != "" || info.SQLDialect != "" {
		details := []string{}
		if info.Name != "" {
			details = append(details, "name="+info.Name)
		}
		if info.Type != "" {
			details = append(details, "type="+info.Type)
		}
		if info.SQLDialect != "" {
			details = append(details, "sql_dialect="+info.SQLDialect)
		}
		parts = append(parts, strings.Join(details, ", "))
	}
	if info.DatabaseName != "" {
		parts = append(parts, "database="+info.DatabaseName)
	}
	if info.SchemaName != "" {
		parts = append(parts, "schema="+info.SchemaName)
	}
	if info.CurrentUser != "" {
		parts = append(parts, "current_user="+info.CurrentUser)
	}
	if info.Version != "" {
		parts = append(parts, "version="+truncateDescriptionValue(info.Version))
	}
	if len(parts) == 0 {
		return datasourceInformationDescription
	}
	return datasourceInformationDescription + " Current datasource: " + strings.Join(parts, "; ") + "."
}

func truncateDescriptionValue(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= 96 {
		return value
	}
	return value[:93] + "..."
}

func (c *datasourceInfoDescriptionCache) getFresh(key datasourceInfoCacheKey) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok || time.Since(entry.storedAt) > datasourceInformationDescriptionTTL {
		return "", false
	}
	return entry.description, true
}

func (c *datasourceInfoDescriptionCache) getAny(key datasourceInfoCacheKey) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}
	return entry.description, true
}

func (c *datasourceInfoDescriptionCache) set(key datasourceInfoCacheKey, description string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = datasourceInfoDescriptionCacheEntry{
		description: description,
		storedAt:    time.Now(),
	}
}
