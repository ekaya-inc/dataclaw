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
			},
		},
		ConnectionTesterFactory: func(ctx context.Context, config map[string]any) (datasource.ConnectionTester, error) {
			return NewAdapter(ctx, config)
		},
		DatasourceIntrospectorFactory: func(ctx context.Context, config map[string]any) (datasource.DatasourceIntrospector, error) {
			return NewDatasourceIntrospector(ctx, config)
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
