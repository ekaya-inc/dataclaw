package core

import (
	"context"
	"strings"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
)

type SchemaDetailMode = dsadapter.SchemaDetailMode
type SchemaExploreLimitation = dsadapter.SchemaExploreLimitation
type SchemaExploreRequest = dsadapter.SchemaExploreRequest
type SchemaExploreResult = dsadapter.SchemaExploreResult
type SchemaExploreSummary = dsadapter.SchemaExploreSummary
type SchemaObject = dsadapter.SchemaObject
type SchemaObjectKind = dsadapter.SchemaObjectKind
type SchemaColumn = dsadapter.SchemaColumn

const (
	SchemaDetailModeCompact = dsadapter.SchemaDetailModeCompact
	SchemaDetailModeFull    = dsadapter.SchemaDetailModeFull
)

func (s *Service) ExploreDatasourceSchema(ctx context.Context, request SchemaExploreRequest) (*SchemaExploreResult, error) {
	ds, err := s.requireDatasource(ctx)
	if err != nil {
		return nil, err
	}
	request = request.Normalized()

	factory, ok := s.adapters.(dsadapter.SchemaExplorerFactory)
	if s.adapters == nil || !ok {
		return unavailableSchemaExploreResult(request, "schema exploration is not available for the configured datasource"), nil
	}

	explorer, err := factory.NewSchemaExplorer(ctx, ds.Type, ds.Config)
	if err != nil {
		return unavailableSchemaExploreResult(request, err.Error()), nil
	}
	if explorer == nil {
		return unavailableSchemaExploreResult(request, "schema exploration returned no adapter explorer"), nil
	}
	defer explorer.Close()

	result, err := explorer.ExploreSchema(ctx, request)
	if err != nil {
		return unavailableSchemaExploreResult(request, err.Error()), nil
	}
	if result == nil {
		return unavailableSchemaExploreResult(request, "schema exploration returned no result"), nil
	}
	if result.DetailMode == "" {
		result.DetailMode = request.DetailMode
	}
	return result, nil
}

func unavailableSchemaExploreResult(request SchemaExploreRequest, reason string) *SchemaExploreResult {
	request = request.Normalized()
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "schema exploration is unavailable"
	}
	return &SchemaExploreResult{
		DetailMode:        request.DetailMode,
		Summary:           SchemaExploreSummary{},
		UnavailableReason: reason,
	}
}
