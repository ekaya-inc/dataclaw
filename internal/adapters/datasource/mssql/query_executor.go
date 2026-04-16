package mssql

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

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
	prepared := prepareQuery(sqlQuery, limit)
	return datasource.ExecuteQueryRows(ctx, e.adapter.db, prepared, nil, limit)
}

func (e *QueryExecutor) QueryWithParameters(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*datasource.QueryResult, error) {
	preparedSQL, params, err := datasource.PrepareReadOnlyParameterizedQuery(sqlQuery, paramDefs, values)
	if err != nil {
		return nil, err
	}
	convertedQuery, namedArgs := convertParams(preparedSQL, params)
	finalQuery := prepareQuery(convertedQuery, limit)
	return datasource.ExecuteQueryRows(ctx, e.adapter.db, finalQuery, namedArgs, limit)
}

func (e *QueryExecutor) Close() error {
	if e == nil || e.adapter == nil {
		return nil
	}
	return e.adapter.Close()
}

func wrapQuery(sqlQuery string, limit int) string {
	return fmt.Sprintf("SELECT TOP (%d) * FROM (%s) AS _limited", datasource.NormalizeLimit(limit), sqlQuery)
}

func prepareQuery(sqlQuery string, limit int) string {
	if startsWithKeyword(sqlQuery, "WITH") {
		return sqlQuery
	}
	return wrapQuery(sqlQuery, limit)
}

func convertParams(query string, args []any) (string, []any) {
	re := regexp.MustCompile(`\$(\d+)`)
	converted := re.ReplaceAllStringFunc(query, func(match string) string {
		num := strings.TrimPrefix(match, "$")
		return "@p" + num
	})
	named := make([]any, len(args))
	for i, arg := range args {
		named[i] = sql.Named(fmt.Sprintf("p%d", i+1), arg)
	}
	return converted, named
}

func startsWithKeyword(query string, keyword string) bool {
	for i := 0; i < len(query); {
		switch {
		case isWhitespace(query[i]) || query[i] == ';':
			i++
		case strings.HasPrefix(query[i:], "--"):
			i = skipLineComment(query, i+2)
		case strings.HasPrefix(query[i:], "/*"):
			i = skipBlockComment(query, i+2)
		default:
			if !isWordStart(query[i]) {
				return false
			}
			start := i
			i++
			for i < len(query) && isWordPart(query[i]) {
				i++
			}
			return strings.EqualFold(query[start:i], keyword)
		}
	}
	return false
}

func isWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '\f'
}

func isWordStart(char byte) bool {
	return char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isWordPart(char byte) bool {
	return isWordStart(char) || (char >= '0' && char <= '9')
}

func skipLineComment(query string, start int) int {
	for start < len(query) && query[start] != '\n' {
		start++
	}
	return start
}

func skipBlockComment(query string, start int) int {
	for start+1 < len(query) {
		if query[start] == '*' && query[start+1] == '/' {
			return start + 2
		}
		start++
	}
	return len(query)
}
