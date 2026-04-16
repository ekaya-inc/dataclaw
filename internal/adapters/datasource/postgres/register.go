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
			},
		},
		ConnectionTesterFactory: func(ctx context.Context, config map[string]any) (datasource.ConnectionTester, error) {
			return NewAdapter(ctx, config)
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
