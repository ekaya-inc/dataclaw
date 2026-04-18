package datasource

import (
	"context"
	"fmt"
)

type registryFactory struct {
	registry *Registry
}

func NewFactory(registry *Registry) Factory {
	if registry == nil {
		registry = DefaultRegistry()
	}
	return &registryFactory{registry: registry}
}

func (f *registryFactory) NewConnectionTester(ctx context.Context, dsType string, config map[string]any) (ConnectionTester, error) {
	reg, ok := f.registry.Get(dsType)
	if !ok || reg.ConnectionTesterFactory == nil {
		return nil, fmt.Errorf("unsupported datasource type: %s", dsType)
	}
	return reg.ConnectionTesterFactory(ctx, config)
}

func (f *registryFactory) NewDatasourceIntrospector(ctx context.Context, dsType string, config map[string]any) (DatasourceIntrospector, error) {
	reg, ok := f.registry.Get(dsType)
	if !ok || reg.DatasourceIntrospectorFactory == nil {
		return nil, fmt.Errorf("unsupported datasource type: %s", dsType)
	}
	return reg.DatasourceIntrospectorFactory(ctx, config)
}

func (f *registryFactory) NewQueryExecutor(ctx context.Context, dsType string, config map[string]any) (QueryExecutor, error) {
	reg, ok := f.registry.Get(dsType)
	if !ok || reg.QueryExecutorFactory == nil {
		return nil, fmt.Errorf("unsupported datasource type: %s", dsType)
	}
	return reg.QueryExecutorFactory(ctx, config)
}

func (f *registryFactory) ConfigFingerprint(dsType string, config map[string]any) (string, error) {
	reg, ok := f.registry.Get(dsType)
	if !ok {
		return "", fmt.Errorf("unsupported datasource type: %s", dsType)
	}
	if reg.ConfigFingerprint != nil {
		return reg.ConfigFingerprint(config)
	}
	return CanonicalFingerprint(config)
}

func (f *registryFactory) ListTypes() []AdapterInfo {
	return f.registry.ListTypes()
}

func (f *registryFactory) TypeInfo(dsType string) (AdapterInfo, bool) {
	reg, ok := f.registry.Get(dsType)
	if !ok {
		return AdapterInfo{}, false
	}
	return reg.Info, true
}

func (f *registryFactory) SupportsType(dsType string) bool {
	return f.registry.SupportsType(dsType)
}

var _ Factory = (*registryFactory)(nil)
