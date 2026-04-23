package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/core"
	"github.com/ekaya-inc/dataclaw/internal/security"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/migrations"
	"github.com/ekaya-inc/dataclaw/pkg/models"
)

type fakeHTTPAdapterFactory struct {
	newConnectionTester       func(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error)
	newDatasourceIntrospector func(context.Context, string, map[string]any) (dsadapter.DatasourceIntrospector, error)
	newQueryExecutor          func(context.Context, string, map[string]any) (dsadapter.QueryExecutor, error)
}

type fakeHTTPConnectionTester struct{}
type fakeHTTPQueryExecutor struct {
	query               func(context.Context, string, int) (*core.QueryResult, error)
	queryWithParameters func(context.Context, string, []models.QueryParameter, map[string]any, int) (*core.QueryResult, error)
	executeDMLQuery     func(context.Context, string, []models.QueryParameter, map[string]any, int) (*core.QueryResult, error)
	execute             func(context.Context, string, int) (*core.ExecuteResult, error)
}

func (fakeHTTPConnectionTester) TestConnection(context.Context) error { return nil }
func (fakeHTTPConnectionTester) Close() error                         { return nil }

func (f fakeHTTPAdapterFactory) NewConnectionTester(ctx context.Context, dsType string, config map[string]any) (dsadapter.ConnectionTester, error) {
	if f.newConnectionTester != nil {
		return f.newConnectionTester(ctx, dsType, config)
	}
	return fakeHTTPConnectionTester{}, nil
}

func (f fakeHTTPAdapterFactory) NewDatasourceIntrospector(ctx context.Context, dsType string, config map[string]any) (dsadapter.DatasourceIntrospector, error) {
	if f.newDatasourceIntrospector != nil {
		return f.newDatasourceIntrospector(ctx, dsType, config)
	}
	return nil, errors.New("unexpected datasource introspection in httpapi tests")
}

func (f fakeHTTPAdapterFactory) NewQueryExecutor(ctx context.Context, dsType string, config map[string]any) (dsadapter.QueryExecutor, error) {
	if f.newQueryExecutor != nil {
		return f.newQueryExecutor(ctx, dsType, config)
	}
	return nil, errors.New("unexpected query execution in httpapi tests")
}

func (fakeHTTPAdapterFactory) ConfigFingerprint(_ string, config map[string]any) (string, error) {
	return dsadapter.CanonicalFingerprint(config)
}

func (fakeHTTPAdapterFactory) ListTypes() []dsadapter.AdapterInfo {
	return []dsadapter.AdapterInfo{
		{Type: "postgres", DisplayName: "PostgreSQL", SQLDialect: "PostgreSQL"},
		{Type: "mssql", DisplayName: "Microsoft SQL Server", SQLDialect: "MSSQL"},
	}
}

func (fakeHTTPAdapterFactory) TypeInfo(dsType string) (dsadapter.AdapterInfo, bool) {
	for _, info := range (fakeHTTPAdapterFactory{}).ListTypes() {
		if info.Type == dsType {
			return info, true
		}
	}
	return dsadapter.AdapterInfo{}, false
}

func (fakeHTTPAdapterFactory) SupportsType(dsType string) bool {
	_, ok := (fakeHTTPAdapterFactory{}).TypeInfo(dsType)
	return ok
}

func (f fakeHTTPQueryExecutor) Query(ctx context.Context, sqlQuery string, limit int) (*core.QueryResult, error) {
	if f.query != nil {
		return f.query(ctx, sqlQuery, limit)
	}
	return nil, errors.New("unexpected Query call")
}

func (f fakeHTTPQueryExecutor) QueryWithParameters(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*core.QueryResult, error) {
	if f.queryWithParameters != nil {
		return f.queryWithParameters(ctx, sqlQuery, paramDefs, values, limit)
	}
	return nil, errors.New("unexpected QueryWithParameters call")
}

func (f fakeHTTPQueryExecutor) ExecuteDMLQuery(ctx context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*core.QueryResult, error) {
	if f.executeDMLQuery != nil {
		return f.executeDMLQuery(ctx, sqlQuery, paramDefs, values, limit)
	}
	return nil, errors.New("unexpected ExecuteDMLQuery call")
}

func (f fakeHTTPQueryExecutor) Execute(ctx context.Context, sqlQuery string, limit int) (*core.ExecuteResult, error) {
	if f.execute != nil {
		return f.execute(ctx, sqlQuery, limit)
	}
	return nil, errors.New("unexpected Execute call")
}

func (fakeHTTPQueryExecutor) Close() error { return nil }

func newTestAPI(t *testing.T) *API {
	return newTestAPIWithFactory(t, fakeHTTPAdapterFactory{})
}

func newTestAPIWithFactory(t *testing.T, factory dsadapter.Factory) *API {
	t.Helper()
	ctx := context.Background()
	store, err := storepkg.Open(ctx, filepath.Join(t.TempDir(), "dataclaw.sqlite"), migrations.FS)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	secret, err := security.LoadOrCreateSecret(filepath.Join(t.TempDir(), "secret.key"))
	if err != nil {
		t.Fatalf("load secret: %v", err)
	}
	service := core.New(store, secret, "test", func() string { return "http://127.0.0.1:18790" }, factory)
	t.Cleanup(func() {
		_ = store.Close()
	})
	return New(service)
}

func performJSONRequest(t *testing.T, api *API, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux := http.NewServeMux()
	api.Register(mux)
	mux.ServeHTTP(rec, req)
	return rec
}

func performRawRequest(t *testing.T, api *API, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux := http.NewServeMux()
	api.Register(mux)
	mux.ServeHTTP(rec, req)
	return rec
}

func decodeData(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if data, ok := payload["data"].(map[string]any); ok {
		return data
	}
	return payload
}

func TestPingReportsStatusAndVersion(t *testing.T) {
	api := newTestAPI(t)

	rec := performJSONRequest(t, api, http.MethodGet, "/ping", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := payload["status"]; got != "no datasource" {
		t.Fatalf("expected status=no datasource before configuring, got %#v", got)
	}
	if got := payload["version"]; got != "test" {
		t.Fatalf("expected version=test, got %#v", got)
	}
	if got := payload["service"]; got != "dataclaw" {
		t.Fatalf("expected service=dataclaw, got %#v", got)
	}

	if _, err := api.service.UpsertDatasource(context.Background(), &storepkg.Datasource{
		Name:     "Primary",
		Type:     "postgres",
		Provider: "postgres",
		Config: map[string]any{
			"host":     "db.example.com",
			"database": "warehouse",
			"user":     "analyst",
			"password": "secret",
		},
	}); err != nil {
		t.Fatalf("UpsertDatasource: %v", err)
	}

	rec = performJSONRequest(t, api, http.MethodGet, "/ping", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 after configure, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := payload["status"]; got != "ok" {
		t.Fatalf("expected status=ok once datasource configured, got %#v", got)
	}

	head := performJSONRequest(t, api, http.MethodHead, "/ping", nil)
	if head.Code != http.StatusOK {
		t.Fatalf("expected HEAD 200, got %d", head.Code)
	}
	if head.Body.Len() != 0 {
		t.Fatalf("expected empty HEAD body, got %q", head.Body.String())
	}
}

func TestStatusReturnsAgentCount(t *testing.T) {
	api := newTestAPI(t)
	created := performJSONRequest(t, api, http.MethodPost, "/api/agents", map[string]any{"name": "Status agent", "can_query": true})
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create to succeed, got %d: %s", created.Code, created.Body.String())
	}

	rec := performJSONRequest(t, api, http.MethodGet, "/api/status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	data := decodeData(t, rec)
	if got := data["agent_count"]; got != float64(1) {
		t.Fatalf("expected agent_count=1, got %#v", got)
	}
}

func TestAgentCRUDAndSecretHandling(t *testing.T) {
	api := newTestAPI(t)
	service := api.service

	createRec := performJSONRequest(t, api, http.MethodPost, "/api/agents", map[string]any{
		"name":      "Ops agent",
		"can_query": true,
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create status 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	createData := decodeData(t, createRec)
	createdAgent := createData["agent"].(map[string]any)
	agentID := createdAgent["id"].(string)
	createdKey := createdAgent["api_key"].(string)
	if createdKey == "" {
		t.Fatal("expected create response to include plaintext api_key")
	}

	listRec := performJSONRequest(t, api, http.MethodGet, "/api/agents", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d", listRec.Code)
	}
	listData := decodeData(t, listRec)
	agents := listData["agents"].([]any)
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	listed := agents[0].(map[string]any)
	if _, ok := listed["api_key"]; ok {
		t.Fatalf("did not expect plaintext api_key on list response: %#v", listed)
	}
	if listed["masked_api_key"] == createdKey {
		t.Fatalf("expected list response to keep key masked, got %#v", listed["masked_api_key"])
	}

	getRec := performJSONRequest(t, api, http.MethodGet, "/api/agents/"+agentID, nil)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected get status 200, got %d", getRec.Code)
	}
	getData := decodeData(t, getRec)
	got := getData["agent"].(map[string]any)
	if _, ok := got["api_key"]; ok {
		t.Fatalf("did not expect plaintext api_key on get response: %#v", got)
	}

	keyRec := performJSONRequest(t, api, http.MethodGet, "/api/agents/"+agentID+"/key", nil)
	if keyRec.Code != http.StatusOK {
		t.Fatalf("expected key status 200, got %d: %s", keyRec.Code, keyRec.Body.String())
	}
	keyData := decodeData(t, keyRec)
	if keyData["masked"] != true {
		t.Fatalf("expected masked key response, got %#v", keyData)
	}
	if keyData["key"] == createdKey {
		t.Fatalf("expected masked key payload, got %#v", keyData["key"])
	}

	if _, err := service.UpsertDatasource(context.Background(), &storepkg.Datasource{
		Name:     "Primary",
		Type:     "postgres",
		Provider: "postgres",
		Config: map[string]any{
			"host":     "db.example.com",
			"database": "warehouse",
			"user":     "analyst",
			"password": "secret",
		},
	}); err != nil {
		t.Fatalf("UpsertDatasource: %v", err)
	}
	query, err := service.CreateQuery(context.Background(), &storepkg.ApprovedQuery{
		NaturalLanguagePrompt: "List accounts",
		SQLQuery:              "SELECT * FROM accounts",
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}
	updateRec := performJSONRequest(t, api, http.MethodPut, "/api/agents/"+agentID, map[string]any{
		"name":                 "Ops agent v2",
		"can_execute":          true,
		"approved_query_scope": "selected",
		"approved_query_ids":   []string{query.ID},
	})
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update status 200, got %d: %s", updateRec.Code, updateRec.Body.String())
	}
	updateData := decodeData(t, updateRec)
	updated := updateData["agent"].(map[string]any)
	if updated["name"] != "Ops agent v2" {
		t.Fatalf("expected updated name, got %#v", updated["name"])
	}

	revealRec := performJSONRequest(t, api, http.MethodPost, "/api/agents/"+agentID+"/reveal-key", map[string]any{})
	if revealRec.Code != http.StatusOK {
		t.Fatalf("expected reveal status 200, got %d: %s", revealRec.Code, revealRec.Body.String())
	}
	revealData := decodeData(t, revealRec)
	revealed := revealData["agent"].(map[string]any)
	if revealed["api_key"] != createdKey {
		t.Fatalf("expected revealed key %q, got %#v", createdKey, revealed["api_key"])
	}

	revealKeyRec := performJSONRequest(t, api, http.MethodGet, "/api/agents/"+agentID+"/key?reveal=true", nil)
	if revealKeyRec.Code != http.StatusOK {
		t.Fatalf("expected reveal key status 200, got %d: %s", revealKeyRec.Code, revealKeyRec.Body.String())
	}
	revealKeyData := decodeData(t, revealKeyRec)
	if revealKeyData["masked"] != false || revealKeyData["key"] != createdKey {
		t.Fatalf("expected plaintext key response, got %#v", revealKeyData)
	}

	bundleCodeRec := performJSONRequest(t, api, http.MethodPost, "/api/agents/"+agentID+"/bundle-code", map[string]any{})
	if bundleCodeRec.Code != http.StatusOK {
		t.Fatalf("expected bundle-code status 200, got %d: %s", bundleCodeRec.Code, bundleCodeRec.Body.String())
	}
	bundleCodeData := decodeData(t, bundleCodeRec)
	bundleInstall := bundleCodeData["bundle_install"].(map[string]any)
	if !strings.Contains(bundleInstall["bundle_url"].(string), "/bundles/ops_agent_v2?code=") {
		t.Fatalf("expected one-time bundle url, got %#v", bundleInstall["bundle_url"])
	}

	rotateRec := performJSONRequest(t, api, http.MethodPost, "/api/agents/"+agentID+"/rotate-key", map[string]any{})
	if rotateRec.Code != http.StatusOK {
		t.Fatalf("expected rotate status 200, got %d: %s", rotateRec.Code, rotateRec.Body.String())
	}
	rotateData := decodeData(t, rotateRec)
	rotated := rotateData["agent"].(map[string]any)
	if rotated["api_key"] == createdKey {
		t.Fatalf("expected rotated key to change, got %#v", rotated["api_key"])
	}

	deleteRec := performJSONRequest(t, api, http.MethodDelete, "/api/agents/"+agentID, nil)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("expected delete status 200, got %d: %s", deleteRec.Code, deleteRec.Body.String())
	}
}

func TestAgentManagerCapabilityNormalizesCanQueryInAPI(t *testing.T) {
	api := newTestAPI(t)

	createRec := performJSONRequest(t, api, http.MethodPost, "/api/agents", map[string]any{
		"name":                        "Manager agent",
		"can_query":                   false,
		"can_manage_approved_queries": true,
		"approved_query_scope":        "none",
		"approved_query_ids":          []string{},
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected create status 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	created := decodeData(t, createRec)["agent"].(map[string]any)
	if created["can_query"] != true {
		t.Fatalf("expected manager create response to normalize can_query=true, got %#v", created["can_query"])
	}
	if created["can_manage_approved_queries"] != true {
		t.Fatalf("expected manager flag in create response, got %#v", created["can_manage_approved_queries"])
	}
	if created["approved_query_scope"] != "all" {
		t.Fatalf("expected manager create response to normalize approved_query_scope=all, got %#v", created["approved_query_scope"])
	}
	if ids, _ := created["approved_query_ids"].([]any); len(ids) != 0 {
		t.Fatalf("expected manager create response to clear approved_query_ids, got %#v", created["approved_query_ids"])
	}

	agentID := created["id"].(string)
	updateRec := performJSONRequest(t, api, http.MethodPut, "/api/agents/"+agentID, map[string]any{
		"name":                        "Manager agent",
		"can_query":                   false,
		"can_manage_approved_queries": true,
		"approved_query_scope":        "none",
		"approved_query_ids":          []string{},
	})
	if updateRec.Code != http.StatusOK {
		t.Fatalf("expected update status 200, got %d: %s", updateRec.Code, updateRec.Body.String())
	}
	updated := decodeData(t, updateRec)["agent"].(map[string]any)
	if updated["can_query"] != true {
		t.Fatalf("expected manager update response to normalize can_query=true, got %#v", updated["can_query"])
	}
	if updated["can_manage_approved_queries"] != true {
		t.Fatalf("expected manager flag in update response, got %#v", updated["can_manage_approved_queries"])
	}
	if updated["approved_query_scope"] != "all" {
		t.Fatalf("expected manager update response to normalize approved_query_scope=all, got %#v", updated["approved_query_scope"])
	}
	if ids, _ := updated["approved_query_ids"].([]any); len(ids) != 0 {
		t.Fatalf("expected manager update response to clear approved_query_ids, got %#v", updated["approved_query_ids"])
	}
}

func TestWriteErrorMapsStatuses(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "bad request keyword", err: errors.New("unsupported datasource type: mysql"), wantStatus: http.StatusBadRequest},
		{name: "not found keyword", err: errors.New("query not found"), wantStatus: http.StatusNotFound},
		{name: "no datasource keyword", err: errors.New("no datasource configured"), wantStatus: http.StatusConflict},
		{name: "connection keyword", err: errors.New("ping postgres: timeout"), wantStatus: http.StatusBadGateway},
		{name: "fallback", err: errors.New("unexpected failure"), wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			writeError(rec, tt.err)
			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}

			var payload map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if payload["error"] != tt.err.Error() {
				t.Fatalf("expected error %q, got %#v", tt.err.Error(), payload["error"])
			}
		})
	}
}

func TestParseDatasourceRequest(t *testing.T) {
	t.Run("builds config from legacy fields", func(t *testing.T) {
		ds := parseDatasourceRequest(datasourceRequest{
			Name:        "warehouse",
			DisplayName: "Primary Warehouse",
			Type:        "postgres",
			Provider:    "postgres",
			Host:        "db.example.com",
			Port:        5432,
			User:        "legacy-user",
			Password:    "secret",
			SSLMode:     "require",
			Options: map[string]any{
				"search_path": "analytics",
			},
		})

		if ds.Name != "Primary Warehouse" {
			t.Fatalf("expected display name to win, got %q", ds.Name)
		}
		if ds.Type != "postgres" || ds.Provider != "postgres" {
			t.Fatalf("expected type/provider to be preserved, got %#v", ds)
		}
		if got := ds.Config["host"]; got != "db.example.com" {
			t.Fatalf("expected host, got %#v", got)
		}
		if got := ds.Config["port"]; got != 5432 {
			t.Fatalf("expected port 5432, got %#v", got)
		}
		if got := ds.Config["user"]; got != "legacy-user" {
			t.Fatalf("expected user fallback, got %#v", got)
		}
		if got := ds.Config["password"]; got != "secret" {
			t.Fatalf("expected password, got %#v", got)
		}
		if got := ds.Config["database"]; got != "warehouse" {
			t.Fatalf("expected database derived from name, got %#v", got)
		}
		if got := ds.Config["name"]; got != "warehouse" {
			t.Fatalf("expected config name derived from name, got %#v", got)
		}
		if got := ds.Config["ssl_mode"]; got != "require" {
			t.Fatalf("expected ssl_mode, got %#v", got)
		}
		if got := ds.Config["search_path"]; got != "analytics" {
			t.Fatalf("expected options to merge into config, got %#v", got)
		}
	})

	t.Run("preserves explicit config", func(t *testing.T) {
		ds := parseDatasourceRequest(datasourceRequest{
			Name:        "warehouse",
			DisplayName: "Primary Warehouse",
			Type:        "postgres",
			Provider:    "postgres",
			Config: map[string]any{
				"host": "db.internal",
			},
			Host:     "ignored.example.com",
			Port:     5432,
			Username: "ignored-user",
			Password: "ignored-password",
			Options: map[string]any{
				"search_path": "ignored",
			},
		})

		if ds.Name != "Primary Warehouse" {
			t.Fatalf("expected display name to win, got %q", ds.Name)
		}
		if got := ds.Config["host"]; got != "db.internal" {
			t.Fatalf("expected explicit config host to survive, got %#v", got)
		}
		if _, ok := ds.Config["port"]; ok {
			t.Fatalf("did not expect legacy port to be merged into explicit config: %#v", ds.Config)
		}
		if _, ok := ds.Config["user"]; ok {
			t.Fatalf("did not expect legacy user to be merged into explicit config: %#v", ds.Config)
		}
		if _, ok := ds.Config["database"]; ok {
			t.Fatalf("did not expect name-derived database when config is provided: %#v", ds.Config)
		}
		if _, ok := ds.Config["search_path"]; ok {
			t.Fatalf("did not expect options to be merged when config is provided: %#v", ds.Config)
		}
	})
}

func TestHandleQueryByIDBranches(t *testing.T) {
	t.Run("missing id returns not found", func(t *testing.T) {
		api := newTestAPI(t)
		rec := performJSONRequest(t, api, http.MethodGet, "/api/queries/", nil)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if payload["error"] != "query id required" {
			t.Fatalf("expected query id required error, got %#v", payload["error"])
		}
	})

	t.Run("get missing query returns not found", func(t *testing.T) {
		api := newTestAPI(t)
		rec := performJSONRequest(t, api, http.MethodGet, "/api/queries/missing-query", nil)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if payload["error"] != "query not found" {
			t.Fatalf("expected query not found error, got %#v", payload["error"])
		}
	})

	t.Run("put invalid body returns bad request", func(t *testing.T) {
		api := newTestAPI(t)
		rec := performRawRequest(t, api, http.MethodPut, "/api/queries/query-123", `{"natural_language_prompt":`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if payload["error"] != "invalid request body" {
			t.Fatalf("expected invalid request body error, got %#v", payload["error"])
		}
	})

	t.Run("delete existing query succeeds", func(t *testing.T) {
		api := newTestAPI(t)
		if _, err := api.service.UpsertDatasource(context.Background(), &storepkg.Datasource{
			Name:     "Primary",
			Type:     "postgres",
			Provider: "postgres",
			Config: map[string]any{
				"host":     "db.example.com",
				"database": "warehouse",
				"user":     "analyst",
				"password": "secret",
			},
		}); err != nil {
			t.Fatalf("UpsertDatasource: %v", err)
		}
		query, err := api.service.CreateQuery(context.Background(), &storepkg.ApprovedQuery{
			NaturalLanguagePrompt: "List accounts",
			SQLQuery:              "SELECT * FROM accounts",
		})
		if err != nil {
			t.Fatalf("CreateQuery: %v", err)
		}

		rec := performJSONRequest(t, api, http.MethodDelete, "/api/queries/"+query.ID, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		data := decodeData(t, rec)
		if data["deleted"] != true {
			t.Fatalf("expected deleted=true, got %#v", data["deleted"])
		}

		deleted, err := api.service.GetQuery(context.Background(), query.ID)
		if err != nil {
			t.Fatalf("GetQuery after delete: %v", err)
		}
		if deleted != nil {
			t.Fatalf("expected query to be deleted, got %#v", deleted)
		}
	})
}

func TestHandleExecuteQueryBranches(t *testing.T) {
	t.Run("bad body returns bad request", func(t *testing.T) {
		api := newTestAPI(t)
		rec := performRawRequest(t, api, http.MethodPost, "/api/queries/query-123/execute", `{"parameters":`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if payload["error"] != "invalid request body" {
			t.Fatalf("expected invalid request body error, got %#v", payload["error"])
		}
	})

	t.Run("success returns execution result", func(t *testing.T) {
		var gotQuery string
		var gotParams []models.QueryParameter
		var gotValues map[string]any
		var gotLimit int
		api := newTestAPIWithFactory(t, fakeHTTPAdapterFactory{
			newQueryExecutor: func(_ context.Context, dsType string, config map[string]any) (dsadapter.QueryExecutor, error) {
				if dsType != "postgres" {
					t.Fatalf("expected postgres datasource type, got %q", dsType)
				}
				return fakeHTTPQueryExecutor{
					queryWithParameters: func(_ context.Context, sqlQuery string, paramDefs []models.QueryParameter, values map[string]any, limit int) (*core.QueryResult, error) {
						gotQuery = sqlQuery
						gotParams = paramDefs
						gotValues = values
						gotLimit = limit
						return &core.QueryResult{
							Columns:  []dsadapter.QueryColumn{{Name: "id", Type: "uuid"}},
							Rows:     []map[string]any{{"id": "550e8400-e29b-41d4-a716-446655440000"}},
							RowCount: 1,
						}, nil
					},
				}, nil
			},
		})

		if _, err := api.service.UpsertDatasource(context.Background(), &storepkg.Datasource{
			Name:     "Primary",
			Type:     "postgres",
			Provider: "postgres",
			Config: map[string]any{
				"host":     "db.example.com",
				"database": "warehouse",
				"user":     "analyst",
				"password": "secret",
			},
		}); err != nil {
			t.Fatalf("UpsertDatasource: %v", err)
		}
		query, err := api.service.CreateQuery(context.Background(), &storepkg.ApprovedQuery{
			NaturalLanguagePrompt: "Find account by id",
			SQLQuery:              "SELECT * FROM accounts WHERE id = {{account_id}}",
			Parameters: []models.QueryParameter{
				{Name: "account_id", Type: "uuid", Required: true},
			},
		})
		if err != nil {
			t.Fatalf("CreateQuery: %v", err)
		}

		rec := performJSONRequest(t, api, http.MethodPost, "/api/queries/"+query.ID+"/execute", map[string]any{
			"parameters": map[string]any{
				"account_id": "550e8400-e29b-41d4-a716-446655440000",
			},
			"limit": 25,
		})
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotQuery != "SELECT * FROM accounts WHERE id = {{account_id}}" {
			t.Fatalf("expected stored SQL to execute, got %q", gotQuery)
		}
		if len(gotParams) != 1 || gotParams[0].Name != "account_id" {
			t.Fatalf("expected query parameters to flow through, got %#v", gotParams)
		}
		if gotValues["account_id"] != "550e8400-e29b-41d4-a716-446655440000" {
			t.Fatalf("expected execution values to flow through, got %#v", gotValues)
		}
		if gotLimit != 25 {
			t.Fatalf("expected limit 25, got %d", gotLimit)
		}

		data := decodeData(t, rec)
		if data["row_count"] != float64(1) {
			t.Fatalf("expected row_count 1, got %#v", data["row_count"])
		}
		rows, ok := data["rows"].([]any)
		if !ok || len(rows) != 1 {
			t.Fatalf("expected one result row, got %#v", data["rows"])
		}
	})

	t.Run("service error returns mapped status", func(t *testing.T) {
		api := newTestAPIWithFactory(t, fakeHTTPAdapterFactory{
			newQueryExecutor: func(_ context.Context, _ string, _ map[string]any) (dsadapter.QueryExecutor, error) {
				t.Fatal("did not expect executor creation for missing query")
				return nil, nil
			},
		})
		if _, err := api.service.UpsertDatasource(context.Background(), &storepkg.Datasource{
			Name:     "Primary",
			Type:     "postgres",
			Provider: "postgres",
			Config: map[string]any{
				"host":     "db.example.com",
				"database": "warehouse",
				"user":     "analyst",
				"password": "secret",
			},
		}); err != nil {
			t.Fatalf("UpsertDatasource: %v", err)
		}

		rec := performJSONRequest(t, api, http.MethodPost, "/api/queries/missing-query/execute", map[string]any{})
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
		}
		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if payload["error"] != "query not found" {
			t.Fatalf("expected query not found error, got %#v", payload["error"])
		}
	})
}
