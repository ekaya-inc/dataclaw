package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	datasource "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
)

type schemaExplorerQuery struct {
	sql  string
	args []driver.NamedValue
}

type schemaExplorerDriverState struct {
	queries    []schemaExplorerQuery
	queryErr   error
	closeCalls int32
	responses  []schemaExplorerResponse
}

type schemaExplorerResponse struct {
	columns []string
	rows    [][]driver.Value
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
	copiedArgs := append([]driver.NamedValue(nil), args...)
	c.state.queries = append(c.state.queries, schemaExplorerQuery{sql: query, args: copiedArgs})
	idx := len(c.state.queries) - 1
	if idx >= len(c.state.responses) {
		return &schemaExplorerRows{}, nil
	}
	response := c.state.responses[idx]
	return &schemaExplorerRows{columns: response.columns, rows: response.rows}, nil
}

type schemaExplorerRows struct {
	columns []string
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

var schemaExplorerDriverCounter uint64

func newSchemaExplorerTestDB(t *testing.T, state *schemaExplorerDriverState) *sql.DB {
	t.Helper()

	driverName := fmt.Sprintf("postgres-schema-explorer-test-%d", atomic.AddUint64(&schemaExplorerDriverCounter, 1))
	sql.Register(driverName, &schemaExplorerDriver{state: state})

	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestRegistrationEnablesSchemaExplorer(t *testing.T) {
	registration := Registration()
	if !registration.Info.Capabilities.SupportsSchemaExplore {
		t.Fatal("expected Postgres adapter to advertise schema exploration support")
	}
	if registration.SchemaExplorerFactory == nil {
		t.Fatal("expected Postgres registration to provide a schema explorer factory")
	}
}

func TestSchemaExplorerCompactReturnsObjectSummaries(t *testing.T) {
	state := &schemaExplorerDriverState{
		responses: []schemaExplorerResponse{{
			columns: []string{"schema_name", "object_name", "object_kind", "column_count"},
			rows: [][]driver.Value{{
				"public",
				"accounts",
				"table",
				int64(3),
			}},
		}},
	}
	explorer := &SchemaExplorer{adapter: &Adapter{db: newSchemaExplorerTestDB(t, state)}}

	result, err := explorer.ExploreSchema(context.Background(), datasource.SchemaExploreRequest{
		SchemaName: " public ",
		ObjectName: " accounts ",
	})
	if err != nil {
		t.Fatalf("ExploreSchema: %v", err)
	}
	if result.DetailMode != datasource.SchemaDetailModeCompact {
		t.Fatalf("expected compact detail mode, got %q", result.DetailMode)
	}
	if result.Summary.SchemaCount != 1 || result.Summary.ObjectCount != 1 || result.Summary.ColumnCount != 3 {
		t.Fatalf("unexpected summary: %#v", result.Summary)
	}
	if len(result.Objects) != 1 {
		t.Fatalf("expected one object, got %#v", result.Objects)
	}
	object := result.Objects[0]
	if object.SchemaName != "public" || object.Name != "accounts" || object.Kind != datasource.SchemaObjectKindTable || object.ColumnCount != 3 {
		t.Fatalf("unexpected schema object: %#v", object)
	}
	if len(object.Columns) != 0 {
		t.Fatalf("compact mode should omit columns, got %#v", object.Columns)
	}
	if !hasLimitation(result.Limitations, "row_counts") {
		t.Fatalf("expected row count limitation, got %#v", result.Limitations)
	}
	if len(state.queries) != 1 {
		t.Fatalf("expected one query, got %#v", state.queries)
	}
	if !strings.Contains(state.queries[0].sql, "pg_catalog.pg_class") || !strings.Contains(state.queries[0].sql, "pg_catalog.pg_namespace") {
		t.Fatalf("expected adapter-local pg_catalog schema query, got %q", state.queries[0].sql)
	}
	assertSchemaExplorerArgs(t, state.queries[0].args, "public", "accounts")
}

func TestSchemaExplorerFullPopulatesColumns(t *testing.T) {
	state := &schemaExplorerDriverState{
		responses: []schemaExplorerResponse{
			{
				columns: []string{"schema_name", "object_name", "object_kind", "column_count"},
				rows: [][]driver.Value{{
					"public",
					"accounts",
					"table",
					int64(2),
				}},
			},
			{
				columns: []string{"schema_name", "object_name", "column_name", "data_type", "nullable", "ordinal_position", "column_default"},
				rows: [][]driver.Value{
					{"public", "accounts", "id", "integer", false, int64(1), "nextval('accounts_id_seq'::regclass)"},
					{"public", "accounts", "email", "text", true, int64(2), nil},
				},
			},
		},
	}
	explorer := &SchemaExplorer{adapter: &Adapter{db: newSchemaExplorerTestDB(t, state)}}

	result, err := explorer.ExploreSchema(context.Background(), datasource.SchemaExploreRequest{DetailMode: datasource.SchemaDetailModeFull})
	if err != nil {
		t.Fatalf("ExploreSchema: %v", err)
	}
	if len(state.queries) != 2 {
		t.Fatalf("expected object and column queries, got %#v", state.queries)
	}
	if result.Summary.ColumnCount != 2 {
		t.Fatalf("expected summary column count from populated columns, got %#v", result.Summary)
	}
	if len(result.Objects) != 1 || len(result.Objects[0].Columns) != 2 {
		t.Fatalf("expected populated columns, got %#v", result.Objects)
	}
	idColumn := result.Objects[0].Columns[0]
	if idColumn.Name != "id" || idColumn.Type != "integer" || idColumn.OrdinalPosition != 1 || idColumn.Nullable == nil || *idColumn.Nullable {
		t.Fatalf("unexpected id column: %#v", idColumn)
	}
	if idColumn.Default == "" {
		t.Fatalf("expected default expression to be populated: %#v", idColumn)
	}
	emailColumn := result.Objects[0].Columns[1]
	if emailColumn.Name != "email" || emailColumn.Nullable == nil || !*emailColumn.Nullable || emailColumn.Default != "" {
		t.Fatalf("unexpected email column: %#v", emailColumn)
	}
}

func TestSchemaExplorerReturnsStructuredUnavailableReasonOnQueryFailure(t *testing.T) {
	state := &schemaExplorerDriverState{queryErr: errors.New("permission denied")}
	explorer := &SchemaExplorer{adapter: &Adapter{db: newSchemaExplorerTestDB(t, state)}}

	result, err := explorer.ExploreSchema(context.Background(), datasource.SchemaExploreRequest{})
	if err != nil {
		t.Fatalf("ExploreSchema should return structured limitations instead of an error, got %v", err)
	}
	if result.UnavailableReason == "" || !strings.Contains(result.UnavailableReason, "permission denied") {
		t.Fatalf("expected structured unavailable reason, got %#v", result)
	}
	if len(result.Objects) != 0 {
		t.Fatalf("expected no objects on unavailable schema query, got %#v", result.Objects)
	}
}

func assertSchemaExplorerArgs(t *testing.T, args []driver.NamedValue, schemaName string, objectName string) {
	t.Helper()
	if len(args) != 2 {
		t.Fatalf("expected two query args, got %#v", args)
	}
	if args[0].Value != schemaName || args[1].Value != objectName {
		t.Fatalf("unexpected query args: %#v", args)
	}
}

func hasLimitation(limitations []datasource.SchemaExploreLimitation, feature string) bool {
	for _, limitation := range limitations {
		if limitation.Feature == feature {
			return true
		}
	}
	return false
}
