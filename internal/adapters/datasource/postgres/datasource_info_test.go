package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
)

type datasourceInfoDriverState struct {
	querySQLs   []string
	queryErr    error
	closeCalls  int32
	columns     []string
	columnTypes []string
	rows        [][]driver.Value
}

type datasourceInfoDriver struct {
	state *datasourceInfoDriverState
}

func (d *datasourceInfoDriver) Open(string) (driver.Conn, error) {
	return &datasourceInfoConn{state: d.state}, nil
}

type datasourceInfoConn struct {
	state *datasourceInfoDriverState
}

func (c *datasourceInfoConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("not implemented")
}
func (c *datasourceInfoConn) Begin() (driver.Tx, error) { return nil, fmt.Errorf("not implemented") }
func (c *datasourceInfoConn) Close() error {
	atomic.AddInt32(&c.state.closeCalls, 1)
	return nil
}

func (c *datasourceInfoConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.state.queryErr != nil {
		return nil, c.state.queryErr
	}
	c.state.querySQLs = append(c.state.querySQLs, query)
	return &datasourceInfoRows{
		columns:     c.state.columns,
		columnTypes: c.state.columnTypes,
		rows:        c.state.rows,
	}, nil
}

type datasourceInfoRows struct {
	columns     []string
	columnTypes []string
	rows        [][]driver.Value
	index       int
}

func (r *datasourceInfoRows) Columns() []string { return r.columns }
func (r *datasourceInfoRows) Close() error      { return nil }
func (r *datasourceInfoRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func (r *datasourceInfoRows) ColumnTypeDatabaseTypeName(index int) string {
	if index >= 0 && index < len(r.columnTypes) {
		return r.columnTypes[index]
	}
	return ""
}

var datasourceInfoDriverCounter uint64

func newDatasourceInfoTestDB(t *testing.T, state *datasourceInfoDriverState) *sql.DB {
	t.Helper()

	driverName := fmt.Sprintf("postgres-datasource-info-test-%d", atomic.AddUint64(&datasourceInfoDriverCounter, 1))
	sql.Register(driverName, &datasourceInfoDriver{state: state})

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	return db
}

func TestDatasourceIntrospectorReturnsRuntimeInformation(t *testing.T) {
	state := &datasourceInfoDriverState{
		columns:     []string{"database_name", "schema_name", "current_user", "version"},
		columnTypes: []string{"TEXT", "TEXT", "TEXT", "TEXT"},
		rows: [][]driver.Value{{
			"warehouse",
			"public",
			"analyst",
			"PostgreSQL 16.2 on x86_64-pc-linux-gnu",
		}},
	}
	introspector := &DatasourceIntrospector{adapter: &Adapter{db: newDatasourceInfoTestDB(t, state)}}

	info, err := introspector.GetDatasourceInfo(context.Background())
	if err != nil {
		t.Fatalf("GetDatasourceInfo: %v", err)
	}
	if info.DatabaseName != "warehouse" || info.SchemaName != "public" || info.CurrentUser != "analyst" {
		t.Fatalf("unexpected datasource info: %#v", info)
	}
	if info.Version == "" {
		t.Fatalf("expected version to be populated, got %#v", info)
	}
	if len(state.querySQLs) != 1 || state.querySQLs[0] != `SELECT current_database(), current_schema(), current_user, version()` {
		t.Fatalf("unexpected introspection query history: %#v", state.querySQLs)
	}

	if err := introspector.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if atomic.LoadInt32(&state.closeCalls) == 0 {
		t.Fatal("expected adapter close to close the backing DB connection")
	}
}

func TestDatasourceIntrospectorSurfacesQueryErrors(t *testing.T) {
	state := &datasourceInfoDriverState{
		queryErr: errors.New("metadata unavailable"),
	}
	introspector := &DatasourceIntrospector{adapter: &Adapter{db: newDatasourceInfoTestDB(t, state)}}

	if _, err := introspector.GetDatasourceInfo(context.Background()); err == nil || err.Error() != "metadata unavailable" {
		t.Fatalf("expected metadata error, got %v", err)
	}
	_ = introspector.Close()
}
