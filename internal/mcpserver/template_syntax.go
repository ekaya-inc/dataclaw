package mcpserver

import (
	"context"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/core"
)

const (
	templateSyntaxHintsTTL     = 30 * time.Second
	templateSyntaxHintsTimeout = 2 * time.Second
)

var approvedQueryTemplateToolNames = []string{"validate_query", "create_query", "update_query"}

var activeDatasourceSQLToolProperties = map[string][]string{
	"query":      {"sql"},
	"execute":    {"sql"},
	"count_rows": {"sql"},
}

type templateSyntaxHintsCache struct {
	mu      sync.RWMutex
	entries map[templateSyntaxHintsCacheKey]templateSyntaxHintsCacheEntry
}

type templateSyntaxHintsCacheKey struct {
	ID        string
	UpdatedAt time.Time
}

type templateSyntaxHintsCacheEntry struct {
	suffix   string
	storedAt time.Time
}

func newTemplateSyntaxHintsCache() *templateSyntaxHintsCache {
	return &templateSyntaxHintsCache{
		entries: make(map[templateSyntaxHintsCacheKey]templateSyntaxHintsCacheEntry),
	}
}

func (c *templateSyntaxHintsCache) getFresh(key templateSyntaxHintsCacheKey) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok || time.Since(entry.storedAt) > templateSyntaxHintsTTL {
		return "", false
	}
	return entry.suffix, true
}

func (c *templateSyntaxHintsCache) getAny(key templateSyntaxHintsCacheKey) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok {
		return "", false
	}
	return entry.suffix, true
}

func (c *templateSyntaxHintsCache) set(key templateSyntaxHintsCacheKey, suffix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = templateSyntaxHintsCacheEntry{
		suffix:   suffix,
		storedAt: time.Now(),
	}
}

func enrichApprovedQueryToolDescriptions(ctx context.Context, service *core.Service, cache *templateSyntaxHintsCache, tools []mcp.Tool) []mcp.Tool {
	indices := indexToolsByName(tools, approvedQueryTemplateToolNames)
	if len(indices) == 0 {
		return tools
	}

	suffix := resolveTemplateSyntaxHintSuffix(ctx, service, cache)
	if suffix == "" {
		return tools
	}

	for _, idx := range indices {
		appendDescriptionSuffix(&tools[idx], suffix)
		appendPropertyDescriptionSuffix(&tools[idx], "sql_query", suffix)
	}
	return tools
}

func enrichActiveDatasourceSQLToolDescriptions(ctx context.Context, service *core.Service, tools []mcp.Tool) []mcp.Tool {
	suffix := resolveActiveDatasourceSQLGuidanceSuffix(ctx, service)
	if suffix == "" {
		return tools
	}

	for idx := range tools {
		propertyNames, ok := activeDatasourceSQLToolProperties[tools[idx].Name]
		if !ok {
			continue
		}
		appendDescriptionSuffix(&tools[idx], suffix)
		for _, propertyName := range propertyNames {
			appendPropertyDescriptionSuffix(&tools[idx], propertyName, suffix)
		}
	}
	return tools
}

func resolveTemplateSyntaxHintSuffix(ctx context.Context, service *core.Service, cache *templateSyntaxHintsCache) string {
	ds, err := service.GetDatasource(ctx)
	if err != nil || ds == nil {
		return ""
	}

	key := templateSyntaxHintsCacheKey{ID: ds.ID, UpdatedAt: ds.UpdatedAt.UTC()}
	if suffix, ok := cache.getFresh(key); ok {
		return suffix
	}

	refreshCtx, cancel := context.WithTimeout(ctx, templateSyntaxHintsTimeout)
	defer cancel()

	hints, err := service.ActiveDatasourceTemplateSyntaxHints(refreshCtx)
	if err != nil || hints == nil || hints.IsZero() {
		if suffix, ok := cache.getAny(key); ok {
			return suffix
		}
		return ""
	}

	dialect := ""
	if info, ok := service.DatasourceTypeInfo(ds.Type); ok {
		dialect = info.SQLDialect
	}
	suffix := renderTemplateSyntaxHintSuffix(dialect, *hints)
	cache.set(key, suffix)
	return suffix
}

func resolveActiveDatasourceSQLGuidanceSuffix(ctx context.Context, service *core.Service) string {
	ds, err := service.GetDatasource(ctx)
	if err != nil || ds == nil {
		return ""
	}

	dialect := ""
	if info, ok := service.DatasourceTypeInfo(ds.Type); ok {
		dialect = info.SQLDialect
	}
	return renderActiveDatasourceSQLGuidanceSuffix(dialect)
}

func renderTemplateSyntaxHintSuffix(dialect string, hints dsadapter.TemplateSyntaxHints) string {
	if hints.IsZero() {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(" For the active datasource")
	if dialect != "" {
		sb.WriteString(" (")
		sb.WriteString(dialect)
		sb.WriteString(")")
	}
	sb.WriteString(", avoid these dialect-native tokens in approved query templates: ")

	parts := make([]string, 0, len(hints.PlaceholderAntiExamples)+len(hints.PaginationAntiExamples))
	for _, example := range hints.PlaceholderAntiExamples {
		parts = append(parts, quoteToken(example))
	}
	for _, example := range hints.PaginationAntiExamples {
		parts = append(parts, quoteToken(example))
	}
	sb.WriteString(strings.Join(parts, ", "))
	sb.WriteString(".")

	if hints.Notes != "" {
		sb.WriteString(" ")
		sb.WriteString(hints.Notes)
	}
	return sb.String()
}

func renderActiveDatasourceSQLGuidanceSuffix(dialect string) string {
	var sb strings.Builder
	sb.WriteString(" Active datasource")
	if dialect != "" {
		sb.WriteString(" dialect: ")
		sb.WriteString(dialect)
		sb.WriteString("; write SQL for that dialect.")
	} else {
		sb.WriteString(": write SQL for the configured datasource dialect.")
	}
	sb.WriteString(" Call get_datasource_information, and explore_schema when available, first when you need the dialect, schema, or table shape.")
	return sb.String()
}

func quoteToken(token string) string {
	return "`" + token + "`"
}

func indexToolsByName(tools []mcp.Tool, names []string) []int {
	want := make(map[string]struct{}, len(names))
	for _, name := range names {
		want[name] = struct{}{}
	}
	indices := make([]int, 0, len(names))
	for i, tool := range tools {
		if _, ok := want[tool.Name]; ok {
			indices = append(indices, i)
		}
	}
	return indices
}

func appendDescriptionSuffix(tool *mcp.Tool, suffix string) {
	tool.Description = tool.Description + suffix
}

func appendPropertyDescriptionSuffix(tool *mcp.Tool, propertyName string, suffix string) {
	property, ok := tool.InputSchema.Properties[propertyName].(map[string]any)
	if !ok {
		return
	}
	description, ok := property["description"].(string)
	if !ok {
		return
	}
	properties := maps.Clone(tool.InputSchema.Properties)
	property = maps.Clone(property)
	property["description"] = description + suffix
	properties[propertyName] = property
	tool.InputSchema.Properties = properties
}
