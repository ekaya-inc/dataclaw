package mssql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	datasource "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
)

type schemaExplorerDriverState struct {
	querySQLs   []string
	queryArgs   [][]driver.NamedValue
	objectsRows [][]driver.Value
	columnsRows [][]driver.Value
	columnTypes []string
	queryErr    error
	closeCalls  int32
}

type schemaExplorerDriver struct {
	state *schemaExplorerDriverState
}

func (d *schemaExplorerDriver) Open(string) (driver.Conn, error) {
	return &schemaExplorerConn{state: d.state}, nil
}

type schemaExplorerConn struct {
	state *schemaExplorerDriverState
}

func (c *schemaExplorerConn) Prepare(string) (driver.Stmt, error) {
	return nil, fmt.Errorf("not implemented")
}
func (c *schemaExplorerConn) Begin() (driver.Tx, error) { return nil, fmt.Errorf("not implemented") }
func (c *schemaExplorerConn) Close() error {
	atomic.AddInt32(&c.state.closeCalls, 1)
	return nil
}

func (c *schemaExplorerConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if c.state.queryErr != nil {
		return nil, c.state.queryErr
	}
	c.state.querySQLs = append(c.state.querySQLs, query)
	c.state.queryArgs = append(c.state.queryArgs, append([]driver.NamedValue(nil), args...))
	if strings.Contains(query, "JOIN sys.types") {
		return &schemaExplorerRows{
			columns: []string{"schema_name", "object_name", "column_name", "data_type", "is_nullable", "ordinal_position", "column_default"},
			types:   c.state.columnTypes,
			rows:    c.state.columnsRows,
		}, nil
	}
	return &schemaExplorerRows{
		columns: []string{"schema_name", "object_name", "object_kind", "column_count"},
		types:   c.state.columnTypes,
		rows:    c.state.objectsRows,
	}, nil
}

type schemaExplorerRows struct {
	columns []string
	types   []string
	rows    [][]driver.Value
	index   int
}

func (r *schemaExplorerRows) Columns() []string { return r.columns }
func (r *schemaExplorerRows) Close() error      { return nil }
func (r *schemaExplorerRows) Next(dest []driver.Value) error {
	if r.index >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.index])
	r.index++
	return nil
}

func (r *schemaExplorerRows) ColumnTypeDatabaseTypeName(index int) string {
	if index >= 0 && index < len(r.types) {
		return r.types[index]
	}
	return ""
}

var schemaExplorerDriverCounter uint64

func newSchemaExplorerTestDB(t *testing.T, state *schemaExplorerDriverState) *sql.DB {
	t.Helper()

	driverName := fmt.Sprintf("mssql-schema-explorer-test-%d", atomic.AddUint64(&schemaExplorerDriverCounter, 1))
	sql.Register(driverName, &schemaExplorerDriver{state: state})

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestSchemaExplorerReturnsCompactObjectSummary(t *testing.T) {
	state := &schemaExplorerDriverState{
		objectsRows: [][]driver.Value{
			{"dbo", "accounts", "table", int64(3)},
			{"dbo", "active_accounts", "view", int64(2)},
		},
	}
	explorer := &SchemaExplorer{adapter: &Adapter{db: newSchemaExplorerTestDB(t, state)}}

	result, err := explorer.ExploreSchema(context.Background(), datasource.SchemaExploreRequest{
		SchemaName: " dbo ",
	})
	if err != nil {
		t.Fatalf("ExploreSchema: %v", err)
	}

	if result.DetailMode != datasource.SchemaDetailModeCompact {
		t.Fatalf("expected compact detail mode, got %q", result.DetailMode)
	}
	if result.Summary.SchemaCount != 1 || result.Summary.ObjectCount != 2 || result.Summary.ColumnCount != 5 {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
	if len(result.Objects) != 2 {
		t.Fatalf("expected 2 objects, got %#v", result.Objects)
	}
	if result.Objects[0].Kind != datasource.SchemaObjectKindTable || result.Objects[1].Kind != datasource.SchemaObjectKindView {
		t.Fatalf("unexpected object kinds: %#v", result.Objects)
	}
	if len(result.Objects[0].Columns) != 0 {
		t.Fatalf("compact mode should not include columns: %#v", result.Objects[0].Columns)
	}
	if len(state.querySQLs) != 1 || !strings.Contains(state.querySQLs[0], "FROM sys.objects") {
		t.Fatalf("expected one sys.objects query, got %#v", state.querySQLs)
	}
	assertNamedArg(t, state.queryArgs[0], "schema_name", "dbo")
	assertNamedArg(t, state.queryArgs[0], "object_name", "")
}

func TestSchemaExplorerReturnsFullColumnDetails(t *testing.T) {
	state := &schemaExplorerDriverState{
		objectsRows: [][]driver.Value{
			{"dbo", "accounts", "table", int64(2)},
		},
		columnsRows: [][]driver.Value{
			{"dbo", "accounts", "id", "int", false, int64(1), nil},
			{"dbo", "accounts", "name", "nvarchar(100)", true, int64(2), "(N'')"},
		},
	}
	explorer := &SchemaExplorer{adapter: &Adapter{db: newSchemaExplorerTestDB(t, state)}}

	result, err := explorer.ExploreSchema(context.Background(), datasource.SchemaExploreRequest{
		SchemaName: "dbo",
		ObjectName: "accounts",
		DetailMode: datasource.SchemaDetailModeFull,
	})
	if err != nil {
		t.Fatalf("ExploreSchema full: %v", err)
	}

	if result.DetailMode != datasource.SchemaDetailModeFull {
		t.Fatalf("expected full detail mode, got %q", result.DetailMode)
	}
	if len(result.Objects) != 1 || len(result.Objects[0].Columns) != 2 {
		t.Fatalf("expected one object with two columns, got %#v", result.Objects)
	}
	first := result.Objects[0].Columns[0]
	if first.Name != "id" || first.Type != "int" || first.Nullable == nil || *first.Nullable || first.OrdinalPosition != 1 {
		t.Fatalf("unexpected first column: %#v", first)
	}
	second := result.Objects[0].Columns[1]
	if second.Name != "name" || second.Type != "nvarchar(100)" || second.Nullable == nil || !*second.Nullable || second.Default != "(N'')" {
		t.Fatalf("unexpected second column: %#v", second)
	}
	if len(state.querySQLs) != 2 || !strings.Contains(state.querySQLs[1], "JOIN sys.columns") {
		t.Fatalf("expected object and column queries, got %#v", state.querySQLs)
	}
	assertNamedArg(t, state.queryArgs[1], "schema_name", "dbo")
	assertNamedArg(t, state.queryArgs[1], "object_name", "accounts")
}

func TestSchemaExplorerTruncatesLargeFullRequestsToCompactSummary(t *testing.T) {
	rows := make([][]driver.Value, maxSchemaExploreObjects+1)
	for i := range rows {
		rows[i] = []driver.Value{"dbo", fmt.Sprintf("table_%03d", i), "table", int64(1)}
	}
	state := &schemaExplorerDriverState{objectsRows: rows}
	explorer := &SchemaExplorer{adapter: &Adapter{db: newSchemaExplorerTestDB(t, state)}}

	result, err := explorer.ExploreSchema(context.Background(), datasource.SchemaExploreRequest{
		DetailMode: datasource.SchemaDetailModeFull,
	})
	if err != nil {
		t.Fatalf("ExploreSchema truncated: %v", err)
	}

	if !result.Truncated || result.TruncatedReason == "" {
		t.Fatalf("expected truncated result with reason, got %#v", result)
	}
	if result.DetailMode != datasource.SchemaDetailModeCompact {
		t.Fatalf("expected full request to fall back to compact when truncated, got %q", result.DetailMode)
	}
	if len(result.Objects) != maxSchemaExploreObjects {
		t.Fatalf("expected %d objects, got %d", maxSchemaExploreObjects, len(result.Objects))
	}
	if len(result.Limitations) != 1 || result.Limitations[0].Feature != "full_detail" {
		t.Fatalf("expected full_detail limitation, got %#v", result.Limitations)
	}
	if len(state.querySQLs) != 1 {
		t.Fatalf("truncated full request should not fetch columns, got queries %#v", state.querySQLs)
	}
}

func assertNamedArg(t *testing.T, args []driver.NamedValue, name string, want any) {
	t.Helper()

	for _, arg := range args {
		if arg.Name == name {
			if arg.Value != want {
				t.Fatalf("arg %s = %#v, want %#v", name, arg.Value, want)
			}
			return
		}
	}
	t.Fatalf("missing arg %s in %#v", name, args)
}
