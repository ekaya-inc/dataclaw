package mssql

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

	objects, truncated, err := e.schemaObjects(ctx, request)
	if err != nil {
		return nil, err
	}

	result := &datasource.SchemaExploreResult{
		DetailMode: request.DetailMode,
		Objects:    objects,
		Summary:    summarizeObjects(objects),
	}
	if truncated {
		result.Truncated = true
		result.TruncatedReason = fmt.Sprintf("schema exploration returns at most %d objects; filter by schema_name or object_name for more detail", maxSchemaExploreObjects)
	}

	if request.DetailMode == datasource.SchemaDetailModeFull {
		if truncated {
			result.DetailMode = datasource.SchemaDetailModeCompact
			result.Limitations = append(result.Limitations, datasource.SchemaExploreLimitation{
				Feature:           "full_detail",
				UnavailableReason: "full column detail is omitted when object results are truncated",
			})
			return result, nil
		}
		if err := e.attachColumns(ctx, request, result.Objects); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (e *SchemaExplorer) Close() error {
	if e == nil || e.adapter == nil {
		return nil
	}
	return e.adapter.Close()
}

func (e *SchemaExplorer) schemaObjects(ctx context.Context, request datasource.SchemaExploreRequest) ([]datasource.SchemaObject, bool, error) {
	rows, err := e.adapter.db.QueryContext(ctx, schemaObjectsSQL,
		sql.Named("schema_name", request.SchemaName),
		sql.Named("object_name", request.ObjectName),
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	objects := make([]datasource.SchemaObject, 0)
	truncated := false
	for rows.Next() {
		var object datasource.SchemaObject
		var kind string
		var columnCount int64
		if err := rows.Scan(&object.SchemaName, &object.Name, &kind, &columnCount); err != nil {
			return nil, false, err
		}
		object.Kind = schemaObjectKind(kind)
		object.ColumnCount = int(columnCount)
		objects = append(objects, object)
		if len(objects) > maxSchemaExploreObjects {
			truncated = true
			objects = objects[:maxSchemaExploreObjects]
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	return objects, truncated, nil
}

func (e *SchemaExplorer) attachColumns(ctx context.Context, request datasource.SchemaExploreRequest, objects []datasource.SchemaObject) error {
	if len(objects) == 0 {
		return nil
	}

	objectIndex := make(map[string]int, len(objects))
	for index, object := range objects {
		objectIndex[schemaObjectKey(object.SchemaName, object.Name)] = index
	}

	rows, err := e.adapter.db.QueryContext(ctx, schemaColumnsSQL,
		sql.Named("schema_name", request.SchemaName),
		sql.Named("object_name", request.ObjectName),
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName string
		var objectName string
		var column datasource.SchemaColumn
		var nullable bool
		var defaultValue sql.NullString
		if err := rows.Scan(&schemaName, &objectName, &column.Name, &column.Type, &nullable, &column.OrdinalPosition, &defaultValue); err != nil {
			return err
		}
		column.Nullable = &nullable
		if defaultValue.Valid {
			column.Default = defaultValue.String
		}
		if index, ok := objectIndex[schemaObjectKey(schemaName, objectName)]; ok {
			objects[index].Columns = append(objects[index].Columns, column)
		}
	}
	return rows.Err()
}

func summarizeObjects(objects []datasource.SchemaObject) datasource.SchemaExploreSummary {
	schemas := make(map[string]struct{})
	summary := datasource.SchemaExploreSummary{ObjectCount: len(objects)}
	for _, object := range objects {
		if object.SchemaName != "" {
			schemas[object.SchemaName] = struct{}{}
		}
		summary.ColumnCount += object.ColumnCount
	}
	summary.SchemaCount = len(schemas)
	return summary
}

func schemaObjectKind(kind string) datasource.SchemaObjectKind {
	switch kind {
	case "table":
		return datasource.SchemaObjectKindTable
	case "view":
		return datasource.SchemaObjectKindView
	default:
		return datasource.SchemaObjectKindOther
	}
}

func schemaObjectKey(schemaName, objectName string) string {
	return schemaName + "\x00" + objectName
}

const schemaObjectsSQL = `
SELECT TOP (201)
	s.name AS schema_name,
	o.name AS object_name,
	CASE
		WHEN o.type = 'U' THEN 'table'
		WHEN o.type = 'V' THEN 'view'
		ELSE 'other'
	END AS object_kind,
	COUNT(c.column_id) AS column_count
FROM sys.objects AS o
JOIN sys.schemas AS s ON s.schema_id = o.schema_id
LEFT JOIN sys.columns AS c ON c.object_id = o.object_id
WHERE o.type IN ('U', 'V')
	AND o.is_ms_shipped = 0
	AND (@schema_name = N'' OR s.name = @schema_name)
	AND (@object_name = N'' OR o.name = @object_name)
GROUP BY s.name, o.name, o.type
ORDER BY s.name, o.name`

const schemaColumnsSQL = `
SELECT
	s.name AS schema_name,
	o.name AS object_name,
	c.name AS column_name,
	CASE
		WHEN t.name IN ('varchar', 'char', 'varbinary', 'binary') AND c.max_length = -1 THEN CONCAT(t.name, '(max)')
		WHEN t.name IN ('nvarchar', 'nchar') AND c.max_length = -1 THEN CONCAT(t.name, '(max)')
		WHEN t.name IN ('varchar', 'char', 'varbinary', 'binary') THEN CONCAT(t.name, '(', c.max_length, ')')
		WHEN t.name IN ('nvarchar', 'nchar') THEN CONCAT(t.name, '(', c.max_length / 2, ')')
		WHEN t.name IN ('decimal', 'numeric') THEN CONCAT(t.name, '(', c.precision, ',', c.scale, ')')
		WHEN t.name IN ('datetime2', 'datetimeoffset', 'time') THEN CONCAT(t.name, '(', c.scale, ')')
		ELSE t.name
	END AS data_type,
	CAST(c.is_nullable AS bit) AS is_nullable,
	c.column_id AS ordinal_position,
	dc.definition AS column_default
FROM sys.objects AS o
JOIN sys.schemas AS s ON s.schema_id = o.schema_id
JOIN sys.columns AS c ON c.object_id = o.object_id
JOIN sys.types AS t ON t.user_type_id = c.user_type_id
LEFT JOIN sys.default_constraints AS dc ON dc.object_id = c.default_object_id
WHERE o.type IN ('U', 'V')
	AND o.is_ms_shipped = 0
	AND (@schema_name = N'' OR s.name = @schema_name)
	AND (@object_name = N'' OR o.name = @object_name)
ORDER BY s.name, o.name, c.column_id`
