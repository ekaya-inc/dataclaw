package datasource

import (
	"context"

	"github.com/ekaya-inc/dataclaw/pkg/models"
)

type ConnectionTester interface {
	TestConnection(ctx context.Context) error
	Close() error
}

type QueryExecutor interface {
	Query(ctx context.Context, sqlQuery string, limit int) (*QueryResult, error)
	QueryWithParameters(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*QueryResult, error)
	ExecuteDMLQuery(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*QueryResult, error)
	Execute(ctx context.Context, sqlQuery string, limit int) (*ExecuteResult, error)
	Close() error
}

type AdapterCapabilities struct {
	SupportsArrayParameters bool `json:"supports_array_parameters"`
}

type AdapterInfo struct {
	Type         string              `json:"type"`
	DisplayName  string              `json:"display_name"`
	Description  string              `json:"description,omitempty"`
	Icon         string              `json:"icon,omitempty"`
	SQLDialect   string              `json:"sql_dialect,omitempty"`
	Capabilities AdapterCapabilities `json:"capabilities,omitempty"`
}

type Registration struct {
	Info                    AdapterInfo
	ConnectionTesterFactory func(ctx context.Context, config map[string]any) (ConnectionTester, error)
	QueryExecutorFactory    func(ctx context.Context, config map[string]any) (QueryExecutor, error)
	ConfigFingerprint       func(config map[string]any) (string, error)
}

type Factory interface {
	NewConnectionTester(ctx context.Context, dsType string, config map[string]any) (ConnectionTester, error)
	NewQueryExecutor(ctx context.Context, dsType string, config map[string]any) (QueryExecutor, error)
	ConfigFingerprint(dsType string, config map[string]any) (string, error)
	ListTypes() []AdapterInfo
	TypeInfo(dsType string) (AdapterInfo, bool)
	SupportsType(dsType string) bool
}

type QueryColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type QueryResult struct {
	Columns  []QueryColumn    `json:"columns"`
	Rows     []map[string]any `json:"rows"`
	RowCount int              `json:"row_count"`
}

type ExecuteResult struct {
	Columns      []QueryColumn    `json:"columns"`
	Rows         []map[string]any `json:"rows"`
	RowCount     int              `json:"row_count"`
	RowsAffected int64            `json:"rows_affected"`
}
