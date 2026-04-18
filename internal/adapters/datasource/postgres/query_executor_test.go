package postgres

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
	want := "SELECT * FROM (SELECT * FROM accounts) AS _limited LIMIT 100"
	if got != want {
		t.Fatalf("unexpected wrapped query:\n got: %q\nwant: %q", got, want)
	}
}

func TestSupportsExecuteStatementAllowsDDLAndDML(t *testing.T) {
	tests := map[string]bool{
		"CREATE TABLE scratch_execute (id integer)":        true,
		"ALTER TABLE scratch_execute ADD COLUMN body text": true,
		"DROP TABLE scratch_execute":                       true,
		"INSERT INTO scratch_execute (id) VALUES (1)":      true,
		"DELETE FROM scratch_execute WHERE id = 1":         true,
		"SELECT * FROM scratch_execute":                    false,
	}

	for sqlQuery, want := range tests {
		if got := datasource.SupportsExecuteStatement(sqlQuery); got != want {
			t.Fatalf("SupportsExecuteStatement(%q) = %v, want %v", sqlQuery, got, want)
		}
	}
}

func TestIsReturningStatementDetectsReturningClause(t *testing.T) {
	if !isReturningStatement("INSERT INTO scratch_execute (id) VALUES (1) RETURNING id") {
		t.Fatal("expected RETURNING clause to be detected")
	}
	if isReturningStatement("CREATE TABLE scratch_execute (id integer)") {
		t.Fatal("expected DDL not to be treated as returning")
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

	driverName := fmt.Sprintf("postgres-execute-test-%d", atomic.AddUint64(&executeDriverCounter, 1))
	sql.Register(driverName, &executeDriver{state: state})

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestExecuteUsesQueryForReturningStatements(t *testing.T) {
	state := &executeDriverState{
		columns:      []string{"id"},
		columnTypes:  []string{"INT4"},
		rows:         [][]driver.Value{{int64(7)}},
		rowsAffected: 99,
	}
	executor := &QueryExecutor{adapter: &Adapter{db: newExecuteTestDB(t, state)}}

	result, err := executor.Execute(context.Background(), "INSERT INTO scratch_execute (id) VALUES (7) RETURNING id", 25)
	if err != nil {
		t.Fatalf("Execute RETURNING: %v", err)
	}
	if len(state.querySQLs) != 1 || len(state.execSQLs) != 0 {
		t.Fatalf("expected QueryContext-only path, got queries=%v execs=%v", state.querySQLs, state.execSQLs)
	}
	if result.RowCount != 1 || result.RowsAffected != 1 {
		t.Fatalf("expected 1 returned row / 1 rows_affected, got %#v", result)
	}
	if len(result.Rows) != 1 || result.Rows[0]["id"] != int64(7) {
		t.Fatalf("unexpected returned rows: %#v", result.Rows)
	}
}

func TestExecuteUsesExecForNonReturningStatements(t *testing.T) {
	state := &executeDriverState{rowsAffected: 2}
	executor := &QueryExecutor{adapter: &Adapter{db: newExecuteTestDB(t, state)}}

	result, err := executor.Execute(context.Background(), "DELETE FROM scratch_execute WHERE id > 10", 25)
	if err != nil {
		t.Fatalf("Execute non-returning DML: %v", err)
	}
	if len(state.execSQLs) != 1 || len(state.querySQLs) != 0 {
		t.Fatalf("expected ExecContext-only path, got queries=%v execs=%v", state.querySQLs, state.execSQLs)
	}
	if result.RowCount != 0 || result.RowsAffected != 2 {
		t.Fatalf("unexpected non-returning result: %#v", result)
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
