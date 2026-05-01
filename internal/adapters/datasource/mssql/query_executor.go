package mssql

import (
	"context"
	"database/sql"
	"errors"
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

func (e *QueryExecutor) Query(ctx context.Context, sqlQuery string, options datasource.QueryOptions) (*datasource.QueryResult, error) {
	prepared, err := prepareQuery(sqlQuery, options)
	if err != nil {
		return nil, err
	}
	return datasource.ExecuteQueryRows(ctx, e.adapter.db, prepared, nil, queryRowsOptions(options))
}

func (e *QueryExecutor) QueryWithParameters(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, options datasource.QueryOptions) (*datasource.QueryResult, error) {
	preparedSQL, params, err := datasource.PrepareReadOnlyParameterizedQuery(sqlQuery, paramDefs, values)
	if err != nil {
		return nil, err
	}
	convertedQuery, namedArgs := convertParams(preparedSQL, params)
	finalQuery, err := prepareQuery(convertedQuery, options)
	if err != nil {
		return nil, err
	}
	return datasource.ExecuteQueryRows(ctx, e.adapter.db, finalQuery, namedArgs, queryRowsOptions(options))
}

func (e *QueryExecutor) ExecuteDMLQuery(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, options datasource.QueryOptions) (*datasource.QueryResult, error) {
	preparedSQL, params, err := datasource.PrepareDMLParameterizedQuery(sqlQuery, paramDefs, values)
	if err != nil {
		return nil, err
	}
	convertedQuery, namedArgs := convertParams(preparedSQL, params)
	return datasource.ExecuteQueryRows(ctx, e.adapter.db, convertedQuery, namedArgs, options)
}

func (e *QueryExecutor) Execute(ctx context.Context, sqlQuery string, options datasource.QueryOptions) (*datasource.ExecuteResult, error) {
	if !datasource.SupportsExecuteStatement(sqlQuery) {
		return nil, datasource.ErrExecuteStatementType
	}
	if isOutputStatement(sqlQuery) {
		return datasource.ExecuteReturningRows(ctx, e.adapter.db, sqlQuery, nil, options)
	}
	return datasource.ExecuteStatement(ctx, e.adapter.db, sqlQuery, nil)
}

func (e *QueryExecutor) CountRows(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any) (*datasource.CountResult, error) {
	query := sqlQuery
	var args []any
	if len(paramDefs) > 0 {
		preparedSQL, params, err := datasource.PrepareReadOnlyParameterizedQuery(sqlQuery, paramDefs, values)
		if err != nil {
			return nil, err
		}
		query, args = convertParams(preparedSQL, params)
	}
	countSQL, err := prepareCountQuery(query)
	if err != nil {
		return nil, err
	}
	return datasource.ExecuteCountRows(ctx, e.adapter.db, countSQL, args)
}

func (e *QueryExecutor) Close() error {
	if e == nil || e.adapter == nil {
		return nil
	}
	return e.adapter.Close()
}

func wrapQuery(sqlQuery string, options datasource.QueryOptions) string {
	return fmt.Sprintf("SELECT TOP (%d) * FROM (%s) AS _limited", datasource.FetchLimit(options), sqlQuery)
}

func prepareQuery(sqlQuery string, options datasource.QueryOptions) (string, error) {
	callerProvidedLimit := options.Limit > 0
	callerProvidedOffset := options.Offset != 0
	options = datasource.NormalizeQueryOptions(options)
	if hasTopLevelKeyword(sqlQuery, "OFFSET") || hasTopLevelKeyword(sqlQuery, "FETCH") {
		if callerProvidedLimit || callerProvidedOffset {
			return "", errors.New("sql server query already defines OFFSET/FETCH; remove it to use adapter pagination")
		}
		return sqlQuery, nil
	}
	if hasTopLevelKeyword(sqlQuery, "TOP") {
		if callerProvidedLimit || callerProvidedOffset {
			return "", errors.New("sql server query with TOP cannot also use adapter pagination")
		}
		return sqlQuery, nil
	}
	if hasTopLevelOrderBy(sqlQuery) {
		return appendOffsetFetch(sqlQuery, options), nil
	}
	if options.Offset > 0 {
		return "", errors.New("sql server offset pagination requires a top-level ORDER BY clause")
	}
	if startsWithKeyword(sqlQuery, "WITH") {
		return "", errors.New("sql server CTE queries require a top-level ORDER BY clause for adapter pagination")
	}
	return wrapQuery(sqlQuery, options), nil
}

func prepareCountQuery(sqlQuery string) (string, error) {
	if startsWithKeyword(sqlQuery, "WITH") {
		return "", errors.New("sql server count_rows does not support CTE queries")
	}
	if hasTopLevelKeyword(sqlQuery, "OFFSET") || hasTopLevelKeyword(sqlQuery, "FETCH") {
		return "", errors.New("sql server count_rows does not support queries that already define OFFSET/FETCH")
	}
	counted := trimTrailingSemicolon(sqlQuery)
	if hasTopLevelOrderBy(counted) && !hasTopLevelKeyword(counted, "TOP") {
		counted = strings.TrimSpace(counted[:topLevelOrderByIndex(counted)])
	}
	if counted == "" {
		return "", errors.New("sql is required")
	}
	return fmt.Sprintf("SELECT COUNT(*) AS row_count FROM (%s) AS _counted", counted), nil
}

func appendOffsetFetch(sqlQuery string, options datasource.QueryOptions) string {
	base := trimTrailingSemicolon(sqlQuery)
	return fmt.Sprintf("%s OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", base, options.Offset, options.Limit+1)
}

func trimTrailingSemicolon(sqlQuery string) string {
	return strings.TrimRight(strings.TrimSpace(sqlQuery), "; \t\r\n")
}

func queryRowsOptions(options datasource.QueryOptions) datasource.QueryOptions {
	options = datasource.NormalizeQueryOptions(options)
	options.OffsetAlreadyApplied = true
	return options
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

type topLevelToken struct {
	value string
	start int
	end   int
}

func scanTopLevelKeywords(sqlQuery string) []string {
	tokens := scanTopLevelTokens(sqlQuery)
	keywords := make([]string, 0, len(tokens))
	for _, token := range tokens {
		keywords = append(keywords, token.value)
	}
	return keywords
}

func hasTopLevelKeyword(sqlQuery string, keyword string) bool {
	keyword = strings.ToUpper(keyword)
	for _, token := range scanTopLevelTokens(sqlQuery) {
		if token.value == keyword {
			return true
		}
	}
	return false
}

func hasTopLevelOrderBy(sqlQuery string) bool {
	return topLevelOrderByIndex(sqlQuery) >= 0
}

func topLevelOrderByIndex(sqlQuery string) int {
	tokens := scanTopLevelTokens(sqlQuery)
	for i := 0; i+1 < len(tokens); i++ {
		if tokens[i].value == "ORDER" && tokens[i+1].value == "BY" {
			return tokens[i].start
		}
	}
	return -1
}

func scanTopLevelTokens(sqlQuery string) []topLevelToken {
	tokens := make([]topLevelToken, 0, 16)
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
				tokens = append(tokens, topLevelToken{value: strings.ToUpper(sqlQuery[start:i]), start: start, end: i})
			}
		}
	}
	return tokens
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
