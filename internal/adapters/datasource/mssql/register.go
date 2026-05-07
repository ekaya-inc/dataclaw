package mssql

import (
	"context"

	datasource "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
)

func Registration() datasource.Registration {
	return datasource.Registration{
		Info: datasource.AdapterInfo{
			Type:        "mssql",
			DisplayName: "Microsoft SQL Server",
			Description: "Connect to Microsoft SQL Server",
			Icon:        "mssql",
			SQLDialect:  "MSSQL",
			Capabilities: datasource.AdapterCapabilities{
				SupportsArrayParameters: false,
				SupportsSchemaExplore:   true,
			},
			TemplateSyntaxHints: datasource.TemplateSyntaxHints{
				PlaceholderAntiExamples: []string{"@status", "@name"},
				PaginationAntiExamples:  []string{"TOP 10", "OFFSET 10 ROWS FETCH NEXT 5 ROWS ONLY"},
				Notes:                   "SQL Server named bind markers (@name) and TOP/OFFSET-FETCH pagination clauses are applied by DataClaw at execution time. Use {{parameter_name}} placeholders and the tool's limit/offset arguments instead.",
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
		ConfigFingerprint:         Fingerprint,
		ReadOnlyTemplateValidator: ValidateReadOnlyTemplate,
	}
}

func init() {
	datasource.Register(Registration())
}
