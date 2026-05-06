package postgres

import (
	"context"

	datasource "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
)

func Registration() datasource.Registration {
	return datasource.Registration{
		Info: datasource.AdapterInfo{
			Type:        "postgres",
			DisplayName: "PostgreSQL",
			Description: "Connect to PostgreSQL-compatible databases",
			Icon:        "postgres",
			SQLDialect:  "PostgreSQL",
			Capabilities: datasource.AdapterCapabilities{
				SupportsArrayParameters: true,
				SupportsSchemaExplore:   true,
			},
			TemplateSyntaxHints: datasource.TemplateSyntaxHints{
				PlaceholderAntiExamples: []string{"$1", "$2"},
				PaginationAntiExamples:  []string{"LIMIT 10 OFFSET 20"},
				Notes:                   "PostgreSQL native bind markers ($1, $2, ...) and LIMIT/OFFSET pagination clauses are applied by DataClaw at execution time. Use {{parameter_name}} placeholders and the tool's limit/offset arguments instead.",
			},
		},
		ConnectionTesterFactory: func(ctx context.Context, config map[string]any) (datasource.ConnectionTester, error) {
			return NewAdapter(ctx, config)
		},
		DatasourceIntrospectorFactory: func(ctx context.Context, config map[string]any) (datasource.DatasourceIntrospector, error) {
			return NewDatasourceIntrospector(ctx, config)
		},
		SchemaExplorerFactory: func(ctx context.Context, config map[string]any) (datasource.SchemaExplorer, error) {
			return NewSchemaExplorer(ctx, config)
		},
		QueryExecutorFactory: func(ctx context.Context, config map[string]any) (datasource.QueryExecutor, error) {
			return NewQueryExecutor(ctx, config)
		},
		ConfigFingerprint: Fingerprint,
	}
}

func init() {
	datasource.Register(Registration())
}
