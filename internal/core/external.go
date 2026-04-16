package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	mssql "github.com/microsoft/go-mssqldb"

	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/pkg/models"
	sqltmpl "github.com/ekaya-inc/dataclaw/pkg/sql"
)

type QueryColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type QueryResult struct {
	Columns  []QueryColumn    `json:"columns"`
	Rows     []map[string]any `json:"rows"`
	RowCount int              `json:"row_count"`
}

type datasourceExecutor struct {
	mu          sync.Mutex
	fingerprint string
	db          *sql.DB
}

func (e *datasourceExecutor) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.db != nil {
		err := e.db.Close()
		e.db = nil
		e.fingerprint = ""
		return err
	}
	return nil
}

func (e *datasourceExecutor) open(ctx context.Context, ds *storepkg.Datasource) (*sql.DB, error) {
	if ds == nil {
		return nil, errors.New("no datasource configured")
	}
	fingerprint := ds.Type + ":" + ds.UpdatedAt.UTC().Format(time.RFC3339Nano)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.db != nil && e.fingerprint == fingerprint {
		return e.db, nil
	}
	if e.db != nil {
		_ = e.db.Close()
		e.db = nil
	}
	db, err := openDatasourceDB(ctx, ds)
	if err != nil {
		return nil, err
	}
	e.db = db
	e.fingerprint = fingerprint
	return e.db, nil
}

func openDatasourceDB(ctx context.Context, ds *storepkg.Datasource) (*sql.DB, error) {
	switch ds.Type {
	case "postgres":
		connStr, err := buildPostgresConnString(ds.Config)
		if err != nil {
			return nil, err
		}
		db, err := sql.Open("pgx", connStr)
		if err != nil {
			return nil, fmt.Errorf("open postgres: %w", err)
		}
		db.SetMaxOpenConns(5)
		db.SetConnMaxIdleTime(5 * time.Minute)
		pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := db.PingContext(pingCtx); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("ping postgres: %w", err)
		}
		return db, nil
	case "mssql":
		connStr, err := buildMSSQLConnString(ds.Config)
		if err != nil {
			return nil, err
		}
		db, err := sql.Open("sqlserver", connStr)
		if err != nil {
			return nil, fmt.Errorf("open sqlserver: %w", err)
		}
		db.SetMaxOpenConns(5)
		db.SetConnMaxIdleTime(5 * time.Minute)
		pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := db.PingContext(pingCtx); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("ping sqlserver: %w", err)
		}
		return db, nil
	default:
		return nil, fmt.Errorf("unsupported datasource type: %s", ds.Type)
	}
}

func buildPostgresConnString(cfg map[string]any) (string, error) {
	host := stringValue(cfg["host"])
	port := intValue(cfg["port"], 5432)
	user := stringValue(cfg["user"])
	password := stringValue(cfg["password"])
	database := stringValue(firstNonNil(cfg["database"], cfg["name"]))
	sslMode := stringValue(cfg["ssl_mode"])
	if sslMode == "" {
		sslMode = "disable"
	}
	if host == "" || database == "" {
		return "", errors.New("postgres host and database are required")
	}
	return fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		url.QueryEscape(user),
		url.QueryEscape(password),
		host,
		port,
		url.QueryEscape(database),
		url.QueryEscape(sslMode),
	), nil
}

func buildMSSQLConnString(cfg map[string]any) (string, error) {
	host := stringValue(cfg["host"])
	port := intValue(cfg["port"], 1433)
	user := stringValue(firstNonNil(cfg["username"], cfg["user"]))
	password := stringValue(cfg["password"])
	database := stringValue(firstNonNil(cfg["database"], cfg["name"]))
	if host == "" || database == "" || user == "" {
		return "", errors.New("sql server host, database, and user are required")
	}
	query := url.Values{}
	query.Set("database", database)
	query.Set("encrypt", boolString(cfg["encrypt"], false))
	if boolValue(cfg["trust_server_certificate"], false) {
		query.Set("TrustServerCertificate", "true")
	}
	if timeout := intValue(cfg["connection_timeout"], 0); timeout > 0 {
		query.Set("connection timeout", strconv.Itoa(timeout))
	}
	return fmt.Sprintf("sqlserver://%s:%s@%s:%d?%s",
		url.QueryEscape(user),
		url.QueryEscape(password),
		host,
		port,
		query.Encode(),
	), nil
}

func testDatasourceConnection(ctx context.Context, ds *storepkg.Datasource) error {
	db, err := openDatasourceDB(ctx, ds)
	if err != nil {
		return err
	}
	defer db.Close()
	switch ds.Type {
	case "postgres":
		var current string
		if err := db.QueryRowContext(ctx, "SELECT current_database()").Scan(&current); err != nil {
			return err
		}
		expected := stringValue(firstNonNil(ds.Config["database"], ds.Config["name"]))
		if !strings.EqualFold(current, expected) {
			return fmt.Errorf("connected to wrong database: expected %q but got %q", expected, current)
		}
	case "mssql":
		var current string
		if err := db.QueryRowContext(ctx, "SELECT DB_NAME()").Scan(&current); err != nil {
			return err
		}
		expected := stringValue(firstNonNil(ds.Config["database"], ds.Config["name"]))
		if !strings.EqualFold(current, expected) {
			return fmt.Errorf("connected to wrong database: expected %q but got %q", expected, current)
		}
	}
	return nil
}

func validateReadOnlySQL(sqlQuery string) (string, error) {
	result := sqltmpl.ValidateAndNormalize(sqlQuery)
	if result.Error != nil {
		return "", result.Error
	}
	normalized := result.NormalizedSQL
	tokens := tokenizeSQL(normalized)
	if len(tokens) == 0 {
		return "", errors.New("sql is required")
	}
	first := tokens[0].Text
	if first != "SELECT" && first != "WITH" {
		return "", errors.New("only read-only SELECT or WITH statements are allowed")
	}
	if containsMutatingKeyword(tokens) {
		return "", errors.New("only read-only SELECT or WITH statements are allowed")
	}
	switch first {
	case "SELECT":
		if hasSelectInto(tokens, 0) {
			return "", errors.New("SELECT INTO is not allowed in read-only queries")
		}
	case "WITH":
		mainStart, err := withMainStatementStart(tokens)
		if err != nil {
			return "", err
		}
		if mainStart >= len(tokens) || tokens[mainStart].Text != "SELECT" {
			return "", errors.New("only read-only SELECT or WITH statements are allowed")
		}
		if hasSelectInto(tokens[mainStart:], 0) {
			return "", errors.New("SELECT INTO is not allowed in read-only queries")
		}
	}
	return normalized, nil
}

type sqlToken struct {
	Text  string
	Depth int
}

func tokenizeSQL(sqlQuery string) []sqlToken {
	tokens := make([]sqlToken, 0, 32)
	depth := 0
	for i := 0; i < len(sqlQuery); {
		switch {
		case isSQLWhitespace(sqlQuery[i]):
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
		case sqlQuery[i] == '$':
			next, ok := skipDollarQuotedString(sqlQuery, i)
			if ok {
				i = next
				continue
			}
			i++
		case isSQLWordStart(sqlQuery[i]):
			start := i
			i++
			for i < len(sqlQuery) && isSQLWordPart(sqlQuery[i]) {
				i++
			}
			tokens = append(tokens, sqlToken{Text: strings.ToUpper(sqlQuery[start:i]), Depth: depth})
		case sqlQuery[i] == '(':
			tokens = append(tokens, sqlToken{Text: "(", Depth: depth})
			depth++
			i++
		case sqlQuery[i] == ')':
			if depth > 0 {
				depth--
			}
			tokens = append(tokens, sqlToken{Text: ")", Depth: depth})
			i++
		case sqlQuery[i] == ',':
			tokens = append(tokens, sqlToken{Text: ",", Depth: depth})
			i++
		default:
			i++
		}
	}
	return tokens
}

func containsMutatingKeyword(tokens []sqlToken) bool {
	for _, token := range tokens {
		switch token.Text {
		case "INSERT", "UPDATE", "DELETE", "MERGE", "ALTER", "CREATE", "DROP", "TRUNCATE":
			return true
		}
	}
	return false
}

func withMainStatementStart(tokens []sqlToken) (int, error) {
	if len(tokens) == 0 || tokens[0].Text != "WITH" {
		return -1, errors.New("only read-only SELECT or WITH statements are allowed")
	}
	i := 1
	if i < len(tokens) && tokens[i].Depth == 0 && tokens[i].Text == "RECURSIVE" {
		i++
	}
	for {
		if i >= len(tokens) || tokens[i].Depth != 0 || !isSQLIdentifierToken(tokens[i].Text) {
			return -1, errors.New("only read-only SELECT or WITH statements are allowed")
		}
		i++
		if i < len(tokens) && tokens[i].Depth == 0 && tokens[i].Text == "(" {
			var err error
			i, err = skipTokenGroup(tokens, i)
			if err != nil {
				return -1, errors.New("only read-only SELECT or WITH statements are allowed")
			}
		}
		if i >= len(tokens) || tokens[i].Depth != 0 || tokens[i].Text != "AS" {
			return -1, errors.New("only read-only SELECT or WITH statements are allowed")
		}
		i++
		if i < len(tokens) && tokens[i].Depth == 0 && tokens[i].Text == "NOT" {
			i++
			if i >= len(tokens) || tokens[i].Depth != 0 || tokens[i].Text != "MATERIALIZED" {
				return -1, errors.New("only read-only SELECT or WITH statements are allowed")
			}
			i++
		} else if i < len(tokens) && tokens[i].Depth == 0 && tokens[i].Text == "MATERIALIZED" {
			i++
		}
		if i >= len(tokens) || tokens[i].Depth != 0 || tokens[i].Text != "(" {
			return -1, errors.New("only read-only SELECT or WITH statements are allowed")
		}
		var err error
		i, err = skipTokenGroup(tokens, i)
		if err != nil {
			return -1, errors.New("only read-only SELECT or WITH statements are allowed")
		}
		if i >= len(tokens) {
			return -1, errors.New("only read-only SELECT or WITH statements are allowed")
		}
		if tokens[i].Depth == 0 && tokens[i].Text == "," {
			i++
			continue
		}
		return i, nil
	}
}

func hasSelectInto(tokens []sqlToken, depth int) bool {
	seenSelect := false
	for _, token := range tokens {
		if token.Depth != depth {
			continue
		}
		switch token.Text {
		case "SELECT":
			seenSelect = true
		case "INTO":
			if seenSelect {
				return true
			}
		case "FROM":
			if seenSelect {
				return false
			}
		}
	}
	return false
}

func skipTokenGroup(tokens []sqlToken, start int) (int, error) {
	if start >= len(tokens) || tokens[start].Text != "(" {
		return -1, io.ErrUnexpectedEOF
	}
	targetDepth := tokens[start].Depth
	for i := start + 1; i < len(tokens); i++ {
		if tokens[i].Text == ")" && tokens[i].Depth == targetDepth {
			return i + 1, nil
		}
	}
	return -1, io.ErrUnexpectedEOF
}

func isSQLWhitespace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r' || char == '\f'
}

func isSQLWordStart(char byte) bool {
	return char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isSQLWordPart(char byte) bool {
	return isSQLWordStart(char) || char == '$' || (char >= '0' && char <= '9')
}

func isSQLIdentifierToken(token string) bool {
	switch token {
	case "AS", "SELECT", "WITH", "INSERT", "UPDATE", "DELETE", "MERGE":
		return false
	default:
		return token != "" && token != "," && token != "(" && token != ")"
	}
}

func skipLineComment(sqlQuery string, start int) int {
	for start < len(sqlQuery) && sqlQuery[start] != '\n' {
		start++
	}
	return start
}

func skipBlockComment(sqlQuery string, start int) int {
	for start+1 < len(sqlQuery) {
		if sqlQuery[start] == '*' && sqlQuery[start+1] == '/' {
			return start + 2
		}
		start++
	}
	return len(sqlQuery)
}

func skipSingleQuotedString(sqlQuery string, start int) int {
	for start < len(sqlQuery) {
		if sqlQuery[start] == '\'' {
			if start+1 < len(sqlQuery) && sqlQuery[start+1] == '\'' {
				start += 2
				continue
			}
			return start + 1
		}
		start++
	}
	return len(sqlQuery)
}

func skipDelimitedIdentifier(sqlQuery string, start int, delimiter byte) int {
	for start < len(sqlQuery) {
		if sqlQuery[start] == delimiter {
			if start+1 < len(sqlQuery) && sqlQuery[start+1] == delimiter {
				start += 2
				continue
			}
			return start + 1
		}
		start++
	}
	return len(sqlQuery)
}

func skipBracketIdentifier(sqlQuery string, start int) int {
	for start < len(sqlQuery) {
		if sqlQuery[start] == ']' {
			if start+1 < len(sqlQuery) && sqlQuery[start+1] == ']' {
				start += 2
				continue
			}
			return start + 1
		}
		start++
	}
	return len(sqlQuery)
}

func skipDollarQuotedString(sqlQuery string, start int) (int, bool) {
	end := start + 1
	for end < len(sqlQuery) && ((sqlQuery[end] >= 'A' && sqlQuery[end] <= 'Z') || (sqlQuery[end] >= 'a' && sqlQuery[end] <= 'z') || (sqlQuery[end] >= '0' && sqlQuery[end] <= '9') || sqlQuery[end] == '_') {
		end++
	}
	if end >= len(sqlQuery) || sqlQuery[end] != '$' {
		return start, false
	}
	delimiter := sqlQuery[start : end+1]
	closeIdx := strings.Index(sqlQuery[end+1:], delimiter)
	if closeIdx < 0 {
		return len(sqlQuery), true
	}
	return end + 1 + closeIdx + len(delimiter), true
}

func validateStoredSQL(sqlQuery string, params []models.QueryParameter) (string, error) {
	result := sqltmpl.ValidateAndNormalize(sqlQuery)
	if result.Error != nil {
		return "", result.Error
	}
	normalized := result.NormalizedSQL
	if normalized == "" {
		return "", errors.New("sql is required")
	}
	if err := sqltmpl.ValidateParameterDefinitions(normalized, params); err != nil {
		return "", err
	}
	if problems := sqltmpl.FindParametersInStringLiterals(normalized); len(problems) > 0 {
		return "", fmt.Errorf("parameters inside string literals are not allowed: %s", strings.Join(problems, ", "))
	}
	return normalized, nil
}

func resolveSQLAndArgs(dsType, sqlQuery string, params []models.QueryParameter, values map[string]any) (string, []any, error) {
	prepared, args, err := prepareParameterizedQuery(sqlQuery, params, values)
	if err != nil {
		return "", nil, err
	}
	if dsType == "mssql" {
		prepared, args = convertParamsToMSSQL(prepared, args)
	}
	return prepared, args, nil
}

func prepareParameterizedQuery(sqlQuery string, params []models.QueryParameter, values map[string]any) (string, []any, error) {
	normalized, err := validateStoredSQL(sqlQuery, params)
	if err != nil {
		return "", nil, err
	}
	return sqltmpl.SubstituteParameters(normalized, params, values)
}

func prepareReadOnlyParameterizedQuery(dsType, sqlQuery string, params []models.QueryParameter, values map[string]any) (string, []any, error) {
	prepared, args, err := prepareParameterizedQuery(sqlQuery, params, values)
	if err != nil {
		return "", nil, err
	}
	readOnly, err := validateReadOnlySQL(prepared)
	if err != nil {
		return "", nil, err
	}
	if dsType == "mssql" {
		readOnly, args = convertParamsToMSSQL(readOnly, args)
	}
	return readOnly, args, nil
}

func convertParamsToMSSQL(query string, args []any) (string, []any) {
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

func executeQueryRows(ctx context.Context, db *sql.DB, query string, args []any, limit int) (*QueryResult, error) {
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
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	result := &QueryResult{Columns: columns, Rows: make([]map[string]any, 0)}
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

func stringValue(v any) string {
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

func intValue(v any, fallback int) int {
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

func boolValue(v any, fallback bool) bool {
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

func boolString(v any, fallback bool) string {
	if boolValue(v, fallback) {
		return "true"
	}
	return "false"
}

func firstNonNil(values ...any) any {
	for _, v := range values {
		if v != nil && stringValue(v) != "" {
			return v
		}
	}
	return nil
}
