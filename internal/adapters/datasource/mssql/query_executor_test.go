package mssql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"sync/atomic"
	"testing"

	datasource "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
)

func TestWrapQueryUsesBoundedDefaultLimit(t *testing.T) {
	got := wrapQuery("SELECT * FROM accounts", 0)
	want := "SELECT TOP (100) * FROM (SELECT * FROM accounts) AS _limited"
	if got != want {
		t.Fatalf("unexpected wrapped query:\n got: %q\nwant: %q", got, want)
	}
}

func TestConvertParamsConvertsPlaceholdersAndNamedArgs(t *testing.T) {
	query, args := convertParams("SELECT * FROM accounts WHERE id = $1 AND region = $2", []any{123, "emea"})

	if query != "SELECT * FROM accounts WHERE id = @p1 AND region = @p2" {
		t.Fatalf("unexpected converted query: %q", query)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %#v", args)
	}

	first, ok := args[0].(sql.NamedArg)
	if !ok {
		t.Fatalf("expected first arg to be sql.NamedArg, got %#v", args[0])
	}
	second, ok := args[1].(sql.NamedArg)
	if !ok {
		t.Fatalf("expected second arg to be sql.NamedArg, got %#v", args[1])
	}
	if first.Name != "p1" || first.Value != 123 {
		t.Fatalf("unexpected first named arg: %#v", first)
	}
	if second.Name != "p2" || second.Value != "emea" {
		t.Fatalf("unexpected second named arg: %#v", second)
	}
}

func TestPrepareQueryLeavesCTEQueriesUnwrapped(t *testing.T) {
	query := "WITH recent_accounts AS (SELECT id FROM accounts) SELECT id FROM recent_accounts"
	if got := prepareQuery(query, 25); got != query {
		t.Fatalf("expected CTE query to remain unwrapped, got %q", got)
	}
}

func TestPrepareQueryLeavesParameterizedCTEQueriesUnwrapped(t *testing.T) {
	query, args := convertParams("/* leading comment */ WITH recent_accounts AS (SELECT $1 AS id) SELECT id FROM recent_accounts WHERE id = $1", []any{123})
	got := prepareQuery(query, 25)
	if got != query {
		t.Fatalf("expected parameterized CTE query to remain unwrapped, got %q", got)
	}
	first, ok := args[0].(sql.NamedArg)
	if !ok {
		t.Fatalf("expected sql.NamedArg, got %#v", args[0])
	}
	if first.Name != "p1" || first.Value != 123 {
		t.Fatalf("unexpected named arg: %#v", first)
	}
}

func TestSupportsExecuteStatementAllowsDDLAndDML(t *testing.T) {
	tests := map[string]bool{
		"CREATE TABLE scratch_execute (id integer)":         true,
		"ALTER TABLE scratch_execute ADD body nvarchar(50)": true,
		"DROP TABLE scratch_execute":                        true,
		"INSERT INTO scratch_execute (id) VALUES (1)":       true,
		"DELETE FROM scratch_execute WHERE id = 1":          true,
		"SELECT * FROM scratch_execute":                     false,
	}

	for sqlQuery, want := range tests {
		if got := datasource.SupportsExecuteStatement(sqlQuery); got != want {
			t.Fatalf("SupportsExecuteStatement(%q) = %v, want %v", sqlQuery, got, want)
		}
	}
}

func TestIsOutputStatementDetectsOutputClause(t *testing.T) {
	if !isOutputStatement("INSERT INTO scratch_execute (id) OUTPUT inserted.id VALUES (1)") {
		t.Fatal("expected OUTPUT clause to be detected")
	}
	if isOutputStatement("INSERT INTO scratch_execute (id) OUTPUT inserted.id INTO @audit VALUES (1)") {
		t.Fatal("expected OUTPUT INTO clause to use non-returning execution")
	}
	if isOutputStatement("CREATE TABLE scratch_execute (id integer)") {
		t.Fatal("expected DDL not to be treated as output-returning")
	}
}

type executeDriverState struct {
	querySQLs    []string
	execSQLs     []string
	columns      []string
	columnTypes  []string
	rows         [][]driver.Value
	rowsAffected int64
}

type executeDriver struct {
	state *executeDriverState
}

func (d *executeDriver) Open(name string) (driver.Conn, error) {
	return &executeConn{state: d.state}, nil
}

type executeConn struct {
	state *executeDriverState
}

func (c *executeConn) Prepare(query string) (driver.Stmt, error) {
	return nil, fmt.Errorf("not implemented")
}
func (c *executeConn) Close() error              { return nil }
func (c *executeConn) Begin() (driver.Tx, error) { return nil, fmt.Errorf("not implemented") }

func (c *executeConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	c.state.execSQLs = append(c.state.execSQLs, query)
	return executeResult(c.state.rowsAffected), nil
}

func (c *executeConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	c.state.querySQLs = append(c.state.querySQLs, query)
	return &executeRows{
		columns:     c.state.columns,
		columnTypes: c.state.columnTypes,
		rows:        c.state.rows,
	}, nil
}

type executeResult int64

func (r executeResult) LastInsertId() (int64, error) { return 0, nil }
func (r executeResult) RowsAffected() (int64, error) { return int64(r), nil }

type executeRows struct {
	columns     []string
	columnTypes []string
	rows        [][]driver.Value
	index       int
}

func (r *executeRows) Columns() []string { return r.columns }
func (r *executeRows) Close() error      { return nil }
func (r *executeRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func (r *executeRows) ColumnTypeDatabaseTypeName(index int) string {
	if index >= 0 && index < len(r.columnTypes) {
		return r.columnTypes[index]
	}
	return ""
}

var executeDriverCounter uint64

func newExecuteTestDB(t *testing.T, state *executeDriverState) *sql.DB {
	t.Helper()

	driverName := fmt.Sprintf("mssql-execute-test-%d", atomic.AddUint64(&executeDriverCounter, 1))
	sql.Register(driverName, &executeDriver{state: state})

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestExecuteUsesQueryForOutputStatements(t *testing.T) {
	state := &executeDriverState{
		columns:      []string{"id"},
		columnTypes:  []string{"INT"},
		rows:         [][]driver.Value{{int64(11)}},
		rowsAffected: 99,
	}
	executor := &QueryExecutor{adapter: &Adapter{db: newExecuteTestDB(t, state)}}

	result, err := executor.Execute(context.Background(), "INSERT INTO scratch_execute (id) OUTPUT inserted.id VALUES (11)", 25)
	if err != nil {
		t.Fatalf("Execute OUTPUT: %v", err)
	}
	if len(state.querySQLs) != 1 || len(state.execSQLs) != 0 {
		t.Fatalf("expected QueryContext-only path, got queries=%v execs=%v", state.querySQLs, state.execSQLs)
	}
	if result.RowCount != 1 || result.RowsAffected != 1 {
		t.Fatalf("expected 1 returned row / 1 rows_affected, got %#v", result)
	}
	if len(result.Rows) != 1 || result.Rows[0]["id"] != int64(11) {
		t.Fatalf("unexpected returned rows: %#v", result.Rows)
	}
}

func TestExecuteUsesExecForNonReturningStatements(t *testing.T) {
	state := &executeDriverState{rowsAffected: 3}
	executor := &QueryExecutor{adapter: &Adapter{db: newExecuteTestDB(t, state)}}

	result, err := executor.Execute(context.Background(), "DELETE FROM scratch_execute WHERE id > 10", 25)
	if err != nil {
		t.Fatalf("Execute non-returning DML: %v", err)
	}
	if len(state.execSQLs) != 1 || len(state.querySQLs) != 0 {
		t.Fatalf("expected ExecContext-only path, got queries=%v execs=%v", state.querySQLs, state.execSQLs)
	}
	if result.RowCount != 0 || result.RowsAffected != 3 {
		t.Fatalf("unexpected non-returning result: %#v", result)
	}
}

func TestExecuteUsesExecForOutputIntoStatements(t *testing.T) {
	state := &executeDriverState{rowsAffected: 2}
	executor := &QueryExecutor{adapter: &Adapter{db: newExecuteTestDB(t, state)}}

	result, err := executor.Execute(context.Background(), "INSERT INTO scratch_execute (id) OUTPUT inserted.id INTO @audit VALUES (11)", 25)
	if err != nil {
		t.Fatalf("Execute OUTPUT INTO: %v", err)
	}
	if len(state.execSQLs) != 1 || len(state.querySQLs) != 0 {
		t.Fatalf("expected ExecContext-only path, got queries=%v execs=%v", state.querySQLs, state.execSQLs)
	}
	if result.RowCount != 0 || result.RowsAffected != 2 {
		t.Fatalf("unexpected OUTPUT INTO result: %#v", result)
	}
}

func TestExecuteUsesExecForDDLStatements(t *testing.T) {
	state := &executeDriverState{rowsAffected: 0}
	executor := &QueryExecutor{adapter: &Adapter{db: newExecuteTestDB(t, state)}}

	result, err := executor.Execute(context.Background(), "CREATE TABLE scratch_execute (id integer)", 25)
	if err != nil {
		t.Fatalf("Execute DDL: %v", err)
	}
	if len(state.execSQLs) != 1 || len(state.querySQLs) != 0 {
		t.Fatalf("expected ExecContext-only DDL path, got queries=%v execs=%v", state.querySQLs, state.execSQLs)
	}
	if result.RowCount != 0 || result.RowsAffected != 0 {
		t.Fatalf("unexpected DDL result: %#v", result)
	}
}
