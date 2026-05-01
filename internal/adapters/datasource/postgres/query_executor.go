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

func (e *QueryExecutor) Query(ctx context.Context, sqlQuery string, options datasource.QueryOptions) (*datasource.QueryResult, error) {
	wrapped := wrapQuery(sqlQuery, options)
	return datasource.ExecuteQueryRows(ctx, e.adapter.db, wrapped, nil, queryRowsOptions(options))
}

func (e *QueryExecutor) QueryWithParameters(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, options datasource.QueryOptions) (*datasource.QueryResult, error) {
	prepared, params, err := datasource.PrepareReadOnlyParameterizedQuery(sqlQuery, paramDefs, values)
	if err != nil {
		return nil, err
	}
	wrapped := wrapQuery(prepared, options)
	return datasource.ExecuteQueryRows(ctx, e.adapter.db, wrapped, params, queryRowsOptions(options))
}

func (e *QueryExecutor) ExecuteDMLQuery(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, options datasource.QueryOptions) (*datasource.QueryResult, error) {
	prepared, params, err := datasource.PrepareDMLParameterizedQuery(sqlQuery, paramDefs, values)
	if err != nil {
		return nil, err
	}
	return datasource.ExecuteQueryRows(ctx, e.adapter.db, prepared, params, options)
}

func (e *QueryExecutor) Execute(ctx context.Context, sqlQuery string, options datasource.QueryOptions) (*datasource.ExecuteResult, error) {
	if !datasource.SupportsExecuteStatement(sqlQuery) {
		return nil, datasource.ErrExecuteStatementType
	}
	if isReturningStatement(sqlQuery) {
		return datasource.ExecuteReturningRows(ctx, e.adapter.db, sqlQuery, nil, options)
	}
	return datasource.ExecuteStatement(ctx, e.adapter.db, sqlQuery, nil)
}

func (e *QueryExecutor) CountRows(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any) (*datasource.CountResult, error) {
	query := sqlQuery
	var args []any
	if len(paramDefs) > 0 {
		prepared, params, err := datasource.PrepareReadOnlyParameterizedQuery(sqlQuery, paramDefs, values)
		if err != nil {
			return nil, err
		}
		query = prepared
		args = params
	}
	return datasource.ExecuteCountRows(ctx, e.adapter.db, countQuery(query), args)
}

func (e *QueryExecutor) Close() error {
	if e == nil || e.adapter == nil {
		return nil
	}
	return e.adapter.Close()
}

func wrapQuery(sqlQuery string, options datasource.QueryOptions) string {
	options = datasource.NormalizeQueryOptions(options)
	return fmt.Sprintf("SELECT * FROM (%s) AS _limited LIMIT %d OFFSET %d", sqlQuery, options.Limit+1, options.Offset)
}

func queryRowsOptions(options datasource.QueryOptions) datasource.QueryOptions {
	options = datasource.NormalizeQueryOptions(options)
	options.OffsetAlreadyApplied = true
	return options
}

func countQuery(sqlQuery string) string {
	return fmt.Sprintf("SELECT COUNT(*) AS row_count FROM (%s) AS _counted", sqlQuery)
}

func isReturningStatement(sqlQuery string) bool {
	switch datasource.FirstStatementKeyword(sqlQuery) {
	case "INSERT", "UPDATE", "DELETE":
		return datasource.ContainsStatementKeyword(sqlQuery, "RETURNING")
	default:
		return false
	}
}
