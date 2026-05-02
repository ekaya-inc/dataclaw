package core

import (
	"context"
	"errors"
	"testing"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

func TestExploreDatasourceSchemaUsesAdapterContractAndNormalizesRequest(t *testing.T) {
	ctx := context.Background()
	factory := newFakeAdapterFactory()
	var gotType string
	var gotConfig map[string]any
	var gotRequest dsadapter.SchemaExploreRequest
	closed := false
	factory.newSchema = func(_ context.Context, dsType string, config map[string]any) (dsadapter.SchemaExplorer, error) {
		gotType = dsType
		gotConfig = config
		return fakeSchemaExplorer{
			explore: func(_ context.Context, request dsadapter.SchemaExploreRequest) (*dsadapter.SchemaExploreResult, error) {
				gotRequest = request
				return &dsadapter.SchemaExploreResult{
					Summary: dsadapter.SchemaExploreSummary{SchemaCount: 1, ObjectCount: 1, ColumnCount: 2},
					Objects: []dsadapter.SchemaObject{{
						SchemaName:  "public",
						Name:        "accounts",
						Kind:        dsadapter.SchemaObjectKindTable,
						ColumnCount: 2,
					}},
				}, nil
			},
			close: func() error {
				closed = true
				return nil
			},
		}, nil
	}
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedSchemaDatasource(t, service)

	result, err := service.ExploreDatasourceSchema(ctx, SchemaExploreRequest{
		SchemaName: " public ",
		ObjectName: " accounts ",
		DetailMode: SchemaDetailMode("verbose"),
	})
	if err != nil {
		t.Fatalf("ExploreDatasourceSchema: %v", err)
	}
	if gotType != "postgres" {
		t.Fatalf("expected postgres adapter, got %q", gotType)
	}
	if gotConfig["database"] != "warehouse" || gotConfig["password"] != "secret" {
		t.Fatalf("expected decrypted datasource config to reach adapter, got %#v", gotConfig)
	}
	if gotRequest.SchemaName != "public" || gotRequest.ObjectName != "accounts" || gotRequest.DetailMode != SchemaDetailModeCompact {
		t.Fatalf("expected normalized request, got %#v", gotRequest)
	}
	if result.DetailMode != SchemaDetailModeCompact {
		t.Fatalf("expected defaulted detail_mode compact, got %q", result.DetailMode)
	}
	if result.Summary.ObjectCount != 1 || len(result.Objects) != 1 {
		t.Fatalf("unexpected schema result: %#v", result)
	}
	if !closed {
		t.Fatal("expected schema explorer to be closed")
	}
}

func TestExploreDatasourceSchemaReturnsUnavailableReasonForAdapterFailure(t *testing.T) {
	ctx := context.Background()
	factory := newFakeAdapterFactory()
	factory.newSchema = func(context.Context, string, map[string]any) (dsadapter.SchemaExplorer, error) {
		return nil, errors.New("metadata unavailable")
	}
	service := newTestServiceWithFactory(t, factory)
	defer service.store.Close()
	seedSchemaDatasource(t, service)

	result, err := service.ExploreDatasourceSchema(ctx, SchemaExploreRequest{DetailMode: SchemaDetailModeFull})
	if err != nil {
		t.Fatalf("ExploreDatasourceSchema: %v", err)
	}
	if result.UnavailableReason != "metadata unavailable" {
		t.Fatalf("expected unavailable reason, got %#v", result)
	}
	if result.DetailMode != SchemaDetailModeFull {
		t.Fatalf("expected requested detail mode in unavailable result, got %q", result.DetailMode)
	}
}

func TestExploreDatasourceSchemaRequiresConfiguredDatasource(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()

	if _, err := service.ExploreDatasourceSchema(context.Background(), SchemaExploreRequest{}); !errors.Is(err, ErrNoDatasourceConfigured) {
		t.Fatalf("expected ErrNoDatasourceConfigured, got %v", err)
	}
}

func seedSchemaDatasource(t *testing.T, service *Service) {
	t.Helper()
	_, err := service.UpsertDatasource(context.Background(), &storepkg.Datasource{
		Name:     "Primary",
		Type:     "postgres",
		Provider: "postgres",
		Config: map[string]any{
			"host":     "db.example.com",
			"database": "warehouse",
			"user":     "analyst",
			"password": "secret",
		},
	})
	if err != nil {
		t.Fatalf("UpsertDatasource: %v", err)
	}
}
