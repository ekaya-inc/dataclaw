package core

import (
	"context"
	"errors"
	"maps"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
)

var ErrNoDatasourceConfigured = errors.New("no datasource configured")

type DatasourceInformation struct {
	Name         string         `json:"name,omitempty"`
	Type         string         `json:"type,omitempty"`
	SQLDialect   string         `json:"sql_dialect,omitempty"`
	DatabaseName string         `json:"database_name,omitempty"`
	SchemaName   string         `json:"schema_name,omitempty"`
	CurrentUser  string         `json:"current_user,omitempty"`
	Version      string         `json:"version,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
}

func (s *Service) GetDatasourceInformation(ctx context.Context) (*DatasourceInformation, error) {
	ds, err := s.requireDatasource(ctx)
	if err != nil {
		return nil, err
	}

	info := &DatasourceInformation{
		Name: ds.Name,
		Type: ds.Type,
	}
	if adapterInfo, ok := s.DatasourceTypeInfo(ds.Type); ok {
		info.SQLDialect = adapterInfo.SQLDialect
	}

	if s.adapters == nil {
		return info, errors.New("datasource adapter factory is not configured")
	}

	introspector, err := s.adapters.NewDatasourceIntrospector(ctx, ds.Type, ds.Config)
	if err != nil {
		return info, err
	}
	defer introspector.Close()

	runtimeInfo, err := introspector.GetDatasourceInfo(ctx)
	if err != nil {
		return info, err
	}
	mergeDatasourceInformation(info, runtimeInfo)
	return info, nil
}

func mergeDatasourceInformation(dst *DatasourceInformation, src *dsadapter.DatasourceInfo) {
	if dst == nil || src == nil {
		return
	}
	dst.DatabaseName = src.DatabaseName
	dst.SchemaName = src.SchemaName
	dst.CurrentUser = src.CurrentUser
	dst.Version = src.Version
	if extra := src.Extra; len(extra) > 0 {
		dst.Extra = maps.Clone(extra)
	}
}
