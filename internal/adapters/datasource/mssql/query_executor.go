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

func (e *QueryExecutor) ExecuteDMLQuery(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*datasource.QueryResult, error) {
	preparedSQL, params, err := datasource.PrepareDMLParameterizedQuery(sqlQuery, paramDefs, values)
	if err != nil {
		return nil, err
	}
	convertedQuery, namedArgs := convertParams(preparedSQL, params)
	return datasource.ExecuteQueryRows(ctx, e.adapter.db, convertedQuery, namedArgs, limit)
}

func (e *QueryExecutor) Execute(ctx context.Context, sqlQuery string, limit int) (*datasource.ExecuteResult, error) {
	if !datasource.SupportsExecuteStatement(sqlQuery) {
		return nil, datasource.ErrExecuteStatementType
	}
	if isOutputStatement(sqlQuery) {
		return datasource.ExecuteReturningRows(ctx, e.adapter.db, sqlQuery, nil, limit)
	}
	return datasource.ExecuteStatement(ctx, e.adapter.db, sqlQuery, nil)
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

func isOutputStatement(sqlQuery string) bool {
	switch datasource.FirstStatementKeyword(sqlQuery) {
	case "INSERT", "UPDATE", "DELETE", "MERGE":
		return hasReturningOutputClause(sqlQuery)
	default:
		return false
	}
}

func hasReturningOutputClause(sqlQuery string) bool {
	keywords := scanTopLevelKeywords(sqlQuery)
	outputIndex := -1
	for i, keyword := range keywords {
		if keyword == "OUTPUT" {
			outputIndex = i
			break
		}
	}
	if outputIndex < 0 {
		return false
	}
	for i := outputIndex + 1; i < len(keywords); i++ {
		switch keywords[i] {
		case "INTO":
			return false
		case "VALUES", "SELECT", "FROM", "WHERE", "WHEN", "OPTION", "EXEC", "EXECUTE":
			return true
		}
	}
	return true
}

func scanTopLevelKeywords(sqlQuery string) []string {
	keywords := make([]string, 0, 16)
	depth := 0
	for i := 0; i < len(sqlQuery); {
		switch {
		case isWhitespace(sqlQuery[i]) || sqlQuery[i] == ';':
			i++
		case strings.HasPrefix(sqlQuery[i:], "--"):
			i = skipLineComment(sqlQuery, i+2)
		case strings.HasPrefix(sqlQuery[i:], "/*"):
			i = skipBlockComment(sqlQuery, i+2)
		case sqlQuery[i] == '\'':
			i = skipSingleQuotedString(sqlQuery, i+1)
		case sqlQuery[i] == '"':
			i = skipDelimitedIdentifier(sqlQuery, i+1, '"')
		case sqlQuery[i] == '[':
			i = skipBracketIdentifier(sqlQuery, i+1)
		case sqlQuery[i] == '`':
			i = skipDelimitedIdentifier(sqlQuery, i+1, '`')
		case sqlQuery[i] == '(':
			depth++
			i++
		case sqlQuery[i] == ')':
			if depth > 0 {
				depth--
			}
			i++
		default:
			if !isWordStart(sqlQuery[i]) {
				i++
				continue
			}
			start := i
			i++
			for i < len(sqlQuery) && isWordPart(sqlQuery[i]) {
				i++
			}
			if depth == 0 {
				keywords = append(keywords, strings.ToUpper(sqlQuery[start:i]))
			}
		}
	}
	return keywords
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

func skipSingleQuotedString(query string, start int) int {
	for start < len(query) {
		if query[start] == '\'' {
			if start+1 < len(query) && query[start+1] == '\'' {
				start += 2
				continue
			}
			if start > 0 && query[start-1] == '\\' {
				start++
				continue
			}
			return start + 1
		}
		start++
	}
	return len(query)
}

func skipDelimitedIdentifier(query string, start int, delimiter byte) int {
	for start < len(query) {
		if query[start] == delimiter {
			if start+1 < len(query) && query[start+1] == delimiter {
				start += 2
				continue
			}
			return start + 1
		}
		start++
	}
	return len(query)
}

func skipBracketIdentifier(query string, start int) int {
	for start < len(query) {
		if query[start] == ']' {
			if start+1 < len(query) && query[start+1] == ']' {
				start += 2
				continue
			}
			return start + 1
		}
		start++
	}
	return len(query)
}
