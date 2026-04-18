package mssql

import (
	"context"

	datasource "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
)

type DatasourceIntrospector struct {
	adapter *Adapter
}

func NewDatasourceIntrospector(ctx context.Context, config map[string]any) (*DatasourceIntrospector, error) {
	adapter, err := NewAdapter(ctx, config)
	if err != nil {
		return nil, err
	}
	return &DatasourceIntrospector{adapter: adapter}, nil
}

func (i *DatasourceIntrospector) GetDatasourceInfo(ctx context.Context) (*datasource.DatasourceInfo, error) {
	info := &datasource.DatasourceInfo{}
	err := i.adapter.db.QueryRowContext(ctx, `SELECT DB_NAME(), SCHEMA_NAME(), SUSER_SNAME(), @@VERSION`).Scan(
		&info.DatabaseName,
		&info.SchemaName,
		&info.CurrentUser,
		&info.Version,
	)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (i *DatasourceIntrospector) Close() error {
	if i == nil || i.adapter == nil {
		return nil
	}
	return i.adapter.Close()
}
