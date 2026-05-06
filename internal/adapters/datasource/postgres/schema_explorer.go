package postgres

import (
	"context"
	"database/sql"
	"fmt"

	datasource "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
)

const maxSchemaExploreObjects = 200

type SchemaExplorer struct {
	adapter *Adapter
}

func NewSchemaExplorer(ctx context.Context, config map[string]any) (*SchemaExplorer, error) {
	adapter, err := NewAdapter(ctx, config)
	if err != nil {
		return nil, err
	}
	return &SchemaExplorer{adapter: adapter}, nil
}

func (e *SchemaExplorer) ExploreSchema(ctx context.Context, request datasource.SchemaExploreRequest) (*datasource.SchemaExploreResult, error) {
	request = request.Normalized()
	result := &datasource.SchemaExploreResult{
		DetailMode:  request.DetailMode,
		Limitations: postgresSchemaExploreLimitations(),
	}

	objects, truncated, err := e.fetchSchemaObjects(ctx, request)
	if err != nil {
		result.UnavailableReason = err.Error()
		return result, nil
	}
	result.Objects = objects
	result.Summary = summarizeSchemaObjects(objects)
	if truncated {
		result.Truncated = true
		result.TruncatedReason = fmt.Sprintf("schema exploration returns at most %d objects; filter by schema_name or object_name for more detail", maxSchemaExploreObjects)
	}

	if request.DetailMode != datasource.SchemaDetailModeFull || len(objects) == 0 {
		return result, nil
	}
	if truncated {
		result.DetailMode = datasource.SchemaDetailModeCompact
		result.Limitations = append(result.Limitations, datasource.SchemaExploreLimitation{
			Feature:           "full_detail",
			UnavailableReason: "full column detail is omitted when object results are truncated",
		})
		return result, nil
	}
	if err := e.populateSchemaColumns(ctx, request, result.Objects); err != nil {
		result.UnavailableReason = err.Error()
		return result, nil
	}
	result.Summary = summarizeSchemaObjects(result.Objects)
	return result, nil
}

func (e *SchemaExplorer) Close() error {
	if e == nil || e.adapter == nil {
		return nil
	}
	return e.adapter.Close()
}

func (e *SchemaExplorer) fetchSchemaObjects(ctx context.Context, request datasource.SchemaExploreRequest) ([]datasource.SchemaObject, bool, error) {
	rows, err := e.adapter.db.QueryContext(ctx, postgresSchemaObjectsSQL, request.SchemaName, request.ObjectName)
	if err != nil {
		return nil, false, fmt.Errorf("postgres schema objects unavailable: %w", err)
	}
	defer rows.Close()

	objects := []datasource.SchemaObject{}
	truncated := false
	for rows.Next() {
		var object datasource.SchemaObject
		var kind string
		if err := rows.Scan(&object.SchemaName, &object.Name, &kind, &object.ColumnCount); err != nil {
			return nil, false, fmt.Errorf("scan postgres schema object: %w", err)
		}
		object.Kind = datasource.SchemaObjectKind(kind)
		objects = append(objects, object)
		if len(objects) > maxSchemaExploreObjects {
			truncated = true
			objects = objects[:maxSchemaExploreObjects]
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate postgres schema objects: %w", err)
	}
	return objects, truncated, nil
}

func (e *SchemaExplorer) populateSchemaColumns(ctx context.Context, request datasource.SchemaExploreRequest, objects []datasource.SchemaObject) error {
	objectIndex := make(map[string]int, len(objects))
	for i, object := range objects {
		objectIndex[schemaObjectKey(object.SchemaName, object.Name)] = i
	}

	rows, err := e.adapter.db.QueryContext(ctx, postgresSchemaColumnsSQL, request.SchemaName, request.ObjectName)
	if err != nil {
		return fmt.Errorf("postgres schema columns unavailable: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName string
		var objectName string
		var column datasource.SchemaColumn
		var nullable bool
		var defaultValue sql.NullString
		if err := rows.Scan(
			&schemaName,
			&objectName,
			&column.Name,
			&column.Type,
			&nullable,
			&column.OrdinalPosition,
			&defaultValue,
		); err != nil {
			return fmt.Errorf("scan postgres schema column: %w", err)
		}
		column.Nullable = &nullable
		if defaultValue.Valid {
			column.Default = defaultValue.String
		}
		idx, ok := objectIndex[schemaObjectKey(schemaName, objectName)]
		if !ok {
			continue
		}
		objects[idx].Columns = append(objects[idx].Columns, column)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate postgres schema columns: %w", err)
	}
	for i := range objects {
		objects[i].ColumnCount = len(objects[i].Columns)
	}
	return nil
}

func summarizeSchemaObjects(objects []datasource.SchemaObject) datasource.SchemaExploreSummary {
	schemas := map[string]struct{}{}
	columns := 0
	for _, object := range objects {
		if object.SchemaName != "" {
			schemas[object.SchemaName] = struct{}{}
		}
		columns += object.ColumnCount
	}
	return datasource.SchemaExploreSummary{
		SchemaCount: len(schemas),
		ObjectCount: len(objects),
		ColumnCount: columns,
	}
}

func schemaObjectKey(schemaName string, objectName string) string {
	return schemaName + "\x00" + objectName
}

func postgresSchemaExploreLimitations() []datasource.SchemaExploreLimitation {
	return []datasource.SchemaExploreLimitation{
		{
			Feature:           "row_counts",
			UnavailableReason: "row counts are not collected by the PostgreSQL schema explorer MVP",
		},
		{
			Feature:           "keys_indexes_foreign_keys",
			UnavailableReason: "key, index, and foreign-key details are not exposed by the current schema explorer contract",
		},
	}
}

var postgresSchemaObjectsSQL = fmt.Sprintf(`
SELECT
	n.nspname AS schema_name,
	c.relname AS object_name,
	CASE c.relkind
		WHEN 'r' THEN 'table'
		WHEN 'p' THEN 'table'
		WHEN 'v' THEN 'view'
		WHEN 'm' THEN 'materialized_view'
		ELSE 'other'
	END AS object_kind,
	COUNT(a.attnum) FILTER (WHERE a.attnum > 0 AND NOT a.attisdropped) AS column_count
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid
WHERE c.relkind IN ('r', 'p', 'v', 'm', 'f')
	AND n.nspname NOT IN ('pg_catalog', 'information_schema')
	AND ($1 = '' OR n.nspname = $1)
	AND ($2 = '' OR c.relname = $2)
GROUP BY n.nspname, c.relname, c.relkind
ORDER BY n.nspname, c.relname
LIMIT %d`, maxSchemaExploreObjects+1)

const postgresSchemaColumnsSQL = `
SELECT
	n.nspname AS schema_name,
	c.relname AS object_name,
	a.attname AS column_name,
	pg_catalog.format_type(a.atttypid, a.atttypmod) AS data_type,
	NOT a.attnotnull AS nullable,
	a.attnum::int AS ordinal_position,
	pg_catalog.pg_get_expr(ad.adbin, ad.adrelid) AS column_default
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid
LEFT JOIN pg_catalog.pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
WHERE c.relkind IN ('r', 'p', 'v', 'm', 'f')
	AND a.attnum > 0
	AND NOT a.attisdropped
	AND n.nspname NOT IN ('pg_catalog', 'information_schema')
	AND ($1 = '' OR n.nspname = $1)
	AND ($2 = '' OR c.relname = $2)
ORDER BY n.nspname, c.relname, a.attnum`
