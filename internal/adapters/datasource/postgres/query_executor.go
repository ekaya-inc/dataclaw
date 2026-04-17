package postgres

import (
	"context"
	"fmt"

	datasource "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/pkg/models"
)

type QueryExecutor struct {
	adapter *Adapter
}

func NewQueryExecutor(ctx context.Context, config map[string]any) (*QueryExecutor, error) {
	adapter, err := NewAdapter(ctx, config)
	if err != nil {
		return nil, err
	}
	return &QueryExecutor{adapter: adapter}, nil
}

func (e *QueryExecutor) Query(ctx context.Context, sqlQuery string, limit int) (*datasource.QueryResult, error) {
	wrapped := wrapQuery(sqlQuery, limit)
	return datasource.ExecuteQueryRows(ctx, e.adapter.db, wrapped, nil, limit)
}

func (e *QueryExecutor) QueryWithParameters(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*datasource.QueryResult, error) {
	prepared, params, err := datasource.PrepareReadOnlyParameterizedQuery(sqlQuery, paramDefs, values)
	if err != nil {
		return nil, err
	}
	wrapped := wrapQuery(prepared, limit)
	return datasource.ExecuteQueryRows(ctx, e.adapter.db, wrapped, params, limit)
}

func (e *QueryExecutor) ExecuteMutatingQuery(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*datasource.QueryResult, error) {
	prepared, params, err := datasource.PrepareMutatingParameterizedQuery(sqlQuery, paramDefs, values)
	if err != nil {
		return nil, err
	}
	return datasource.ExecuteQueryRows(ctx, e.adapter.db, prepared, params, limit)
}

func (e *QueryExecutor) Close() error {
	if e == nil || e.adapter == nil {
		return nil
	}
	return e.adapter.Close()
}

func wrapQuery(sqlQuery string, limit int) string {
	return fmt.Sprintf("SELECT * FROM (%s) AS _limited LIMIT %d", sqlQuery, datasource.NormalizeLimit(limit))
}
