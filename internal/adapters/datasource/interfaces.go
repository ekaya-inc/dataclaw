package datasource

import (
	"context"

	"github.com/ekaya-inc/dataclaw/pkg/models"
)

type ConnectionTester interface {
	TestConnection(ctx context.Context) error
	Close() error
}

type DatasourceInfo struct {
	DatabaseName string         `json:"database_name,omitempty"`
	SchemaName   string         `json:"schema_name,omitempty"`
	CurrentUser  string         `json:"current_user,omitempty"`
	Version      string         `json:"version,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
}

type DatasourceIntrospector interface {
	GetDatasourceInfo(ctx context.Context) (*DatasourceInfo, error)
	Close() error
}

type SchemaExplorer interface {
	ExploreSchema(ctx context.Context, request SchemaExploreRequest) (*SchemaExploreResult, error)
	Close() error
}

type QueryExecutor interface {
	Query(ctx context.Context, sqlQuery string, options QueryOptions) (*QueryResult, error)
	QueryWithParameters(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, options QueryOptions) (*QueryResult, error)
	ExecuteDMLQuery(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, options QueryOptions) (*QueryResult, error)
	Execute(ctx context.Context, sqlQuery string, options QueryOptions) (*ExecuteResult, error)
	CountRows(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any) (*CountResult, error)
	Close() error
}

type AdapterCapabilities struct {
	SupportsArrayParameters bool `json:"supports_array_parameters"`
	SupportsSchemaExplore   bool `json:"supports_schema_explore"`
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
	Info                          AdapterInfo
	ConnectionTesterFactory       func(ctx context.Context, config map[string]any) (ConnectionTester, error)
	DatasourceIntrospectorFactory func(ctx context.Context, config map[string]any) (DatasourceIntrospector, error)
	SchemaExplorerFactory         func(ctx context.Context, config map[string]any) (SchemaExplorer, error)
	QueryExecutorFactory          func(ctx context.Context, config map[string]any) (QueryExecutor, error)
	ConfigFingerprint             func(config map[string]any) (string, error)
}

type SchemaExplorerFactory interface {
	NewSchemaExplorer(ctx context.Context, dsType string, config map[string]any) (SchemaExplorer, error)
}

type Factory interface {
	NewConnectionTester(ctx context.Context, dsType string, config map[string]any) (ConnectionTester, error)
	NewDatasourceIntrospector(ctx context.Context, dsType string, config map[string]any) (DatasourceIntrospector, error)
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

type QueryOptions struct {
	Limit                int  `json:"limit,omitempty"`
	Offset               int  `json:"offset,omitempty"`
	OffsetAlreadyApplied bool `json:"-"`
}

type QueryResult struct {
	Columns    []QueryColumn    `json:"columns"`
	Rows       []map[string]any `json:"rows"`
	RowCount   int              `json:"row_count"`
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
	HasMore    bool             `json:"has_more"`
	NextOffset int              `json:"next_offset,omitempty"`
}

type ExecuteResult struct {
	Columns      []QueryColumn    `json:"columns"`
	Rows         []map[string]any `json:"rows"`
	RowCount     int              `json:"row_count"`
	RowsAffected int64            `json:"rows_affected"`
	Limit        int              `json:"limit,omitempty"`
	Offset       int              `json:"offset,omitempty"`
	HasMore      bool             `json:"has_more,omitempty"`
	NextOffset   int              `json:"next_offset,omitempty"`
}

type CountResult struct {
	RowCount int64 `json:"row_count"`
	Exact    bool  `json:"exact"`
}
