package datasource

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	mssql "github.com/microsoft/go-mssqldb"
)

const (
	DefaultQueryLimit = 100
	MaxQueryLimit     = 1000
)

func NormalizeLimit(limit int) int {
	if limit <= 0 || limit > MaxQueryLimit {
		return DefaultQueryLimit
	}
	return limit
}

func ExecuteQueryRows(ctx context.Context, db *sql.DB, query string, args []any, limit int) (*QueryResult, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	columns := make([]QueryColumn, len(colNames))
	for i, name := range colNames {
		columns[i] = QueryColumn{Name: name, Type: normalizeColumnType(colTypes[i].DatabaseTypeName())}
	}

	result := &QueryResult{Columns: columns, Rows: make([]map[string]any, 0)}
	limit = NormalizeLimit(limit)
	for rows.Next() {
		if len(result.Rows) >= limit {
			break
		}
		values := make([]any, len(colNames))
		ptrs := make([]any, len(colNames))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		rowMap := make(map[string]any, len(colNames))
		for i, name := range colNames {
			rowMap[name] = normalizeValue(values[i])
		}
		result.Rows = append(result.Rows, rowMap)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result.RowCount = len(result.Rows)
	return result, nil
}

func ExecuteReturningRows(ctx context.Context, db *sql.DB, query string, args []any, limit int) (*ExecuteResult, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	columns := make([]QueryColumn, len(colNames))
	for i, name := range colNames {
		columns[i] = QueryColumn{Name: name, Type: normalizeColumnType(colTypes[i].DatabaseTypeName())}
	}

	result := &ExecuteResult{
		Columns: columns,
		Rows:    make([]map[string]any, 0),
	}
	limit = NormalizeLimit(limit)
	for rows.Next() {
		values := make([]any, len(colNames))
		ptrs := make([]any, len(colNames))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		rowMap := make(map[string]any, len(colNames))
		for i, name := range colNames {
			rowMap[name] = normalizeValue(values[i])
		}
		if len(result.Rows) < limit {
			result.Rows = append(result.Rows, rowMap)
		}
		result.RowsAffected++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result.RowCount = len(result.Rows)
	return result, nil
}

func ExecuteStatement(ctx context.Context, db *sql.DB, query string, args []any) (*ExecuteResult, error) {
	execResult, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := execResult.RowsAffected()
	if err != nil {
		return nil, err
	}
	return &ExecuteResult{
		Columns:      []QueryColumn{},
		Rows:         []map[string]any{},
		RowCount:     0,
		RowsAffected: rowsAffected,
	}, nil
}

func normalizeValue(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	case time.Time:
		return t.UTC().Format(time.RFC3339Nano)
	case mssql.DateTimeOffset:
		return t
	default:
		return v
	}
}

func normalizeColumnType(name string) string {
	upper := strings.ToUpper(name)
	switch upper {
	case "VARCHAR", "NVARCHAR", "TEXT", "UUID", "UNIQUEIDENTIFIER", "CHAR", "NCHAR":
		return "string"
	case "INT", "INT4", "INT8", "BIGINT", "SMALLINT", "TINYINT":
		return "integer"
	case "DECIMAL", "NUMERIC", "FLOAT", "DOUBLE", "REAL", "MONEY":
		return "decimal"
	case "BOOL", "BOOLEAN", "BIT":
		return "boolean"
	case "DATE":
		return "date"
	case "TIMESTAMP", "TIMESTAMPTZ", "DATETIME", "DATETIME2", "SMALLDATETIME":
		return "timestamp"
	default:
		if upper == "" {
			return "unknown"
		}
		return strings.ToLower(upper)
	}
}

func StringValue(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func IntValue(v any, fallback int) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return i
		}
	}
	return fallback
}

func BoolValue(v any, fallback bool) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(t))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func BoolString(v any, fallback bool) string {
	if BoolValue(v, fallback) {
		return "true"
	}
	return "false"
}

func FirstNonNil(values ...any) any {
	for _, v := range values {
		if v != nil && StringValue(v) != "" {
			return v
		}
	}
	return nil
}

func CanonicalFingerprint(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
