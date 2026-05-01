package datasource

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
)

type helperDriverState struct {
	columns     []string
	columnTypes []string
	rows        [][]driver.Value
	querySQLs   []string
}

type helperDriver struct {
	state *helperDriverState
}

func (d *helperDriver) Open(string) (driver.Conn, error) {
	return &helperConn{state: d.state}, nil
}

type helperConn struct {
	state *helperDriverState
}

func (c *helperConn) Prepare(string) (driver.Stmt, error) { return nil, fmt.Errorf("not implemented") }
func (c *helperConn) Close() error                        { return nil }
func (c *helperConn) Begin() (driver.Tx, error)           { return nil, fmt.Errorf("not implemented") }

func (c *helperConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.state.querySQLs = append(c.state.querySQLs, query)
	return &helperRows{columns: c.state.columns, columnTypes: c.state.columnTypes, rows: c.state.rows}, nil
}

type helperRows struct {
	columns     []string
	columnTypes []string
	rows        [][]driver.Value
	index       int
}

func (r *helperRows) Columns() []string { return r.columns }
func (r *helperRows) Close() error      { return nil }
func (r *helperRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}
func (r *helperRows) ColumnTypeDatabaseTypeName(index int) string {
	if index >= 0 && index < len(r.columnTypes) {
		return r.columnTypes[index]
	}
	return ""
}

var helperDriverCounter uint64

func newHelperTestDB(t *testing.T, state *helperDriverState) *sql.DB {
	t.Helper()
	driverName := fmt.Sprintf("datasource-helper-test-%d", atomic.AddUint64(&helperDriverCounter, 1))
	sql.Register(driverName, &helperDriver{state: state})
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestExecuteQueryRowsFetchesSentinelAndReportsNextOffset(t *testing.T) {
	state := &helperDriverState{
		columns:     []string{"id"},
		columnTypes: []string{"INT"},
		rows:        [][]driver.Value{{int64(1)}, {int64(2)}, {int64(3)}},
	}
	result, err := ExecuteQueryRows(context.Background(), newHelperTestDB(t, state), "SELECT id FROM accounts", nil, QueryOptions{Limit: 2, Offset: 10, OffsetAlreadyApplied: true})
	if err != nil {
		t.Fatalf("ExecuteQueryRows: %v", err)
	}
	if result.RowCount != 2 || len(result.Rows) != 2 {
		t.Fatalf("expected 2 returned rows, got %#v", result)
	}
	if !result.HasMore || result.NextOffset != 12 {
		t.Fatalf("expected has_more with next_offset=12, got %#v", result)
	}
	if result.Limit != 2 || result.Offset != 10 {
		t.Fatalf("expected result metadata to preserve limit/offset, got %#v", result)
	}
}

func TestExecuteReturningRowsPaginatesRowsButCountsAllAffected(t *testing.T) {
	state := &helperDriverState{
		columns:     []string{"id"},
		columnTypes: []string{"INT"},
		rows:        [][]driver.Value{{int64(1)}, {int64(2)}, {int64(3)}, {int64(4)}},
	}
	result, err := ExecuteReturningRows(context.Background(), newHelperTestDB(t, state), "DELETE FROM accounts RETURNING id", nil, QueryOptions{Limit: 2})
	if err != nil {
		t.Fatalf("ExecuteReturningRows: %v", err)
	}
	if result.RowCount != 2 || len(result.Rows) != 2 {
		t.Fatalf("expected 2 returned rows, got %#v", result)
	}
	if result.RowsAffected != 4 {
		t.Fatalf("expected rows_affected=4, got %#v", result)
	}
	if !result.HasMore || result.NextOffset != 2 {
		t.Fatalf("expected has_more with next_offset=2, got %#v", result)
	}
}

func TestExecuteCountRowsReturnsExactCount(t *testing.T) {
	state := &helperDriverState{
		columns:     []string{"row_count"},
		columnTypes: []string{"INT"},
		rows:        [][]driver.Value{{int64(42)}},
	}
	result, err := ExecuteCountRows(context.Background(), newHelperTestDB(t, state), "SELECT COUNT(*) FROM accounts", nil)
	if err != nil {
		t.Fatalf("ExecuteCountRows: %v", err)
	}
	if result.RowCount != 42 || !result.Exact {
		t.Fatalf("expected exact row_count=42, got %#v", result)
	}
}
