package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/core"
	"github.com/ekaya-inc/dataclaw/internal/security"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/migrations"
)

type fakeHTTPAdapterFactory struct{}

type fakeHTTPConnectionTester struct{}

func (fakeHTTPConnectionTester) TestConnection(context.Context) error { return nil }
func (fakeHTTPConnectionTester) Close() error                         { return nil }

func (fakeHTTPAdapterFactory) NewConnectionTester(context.Context, string, map[string]any) (dsadapter.ConnectionTester, error) {
	return fakeHTTPConnectionTester{}, nil
}

func (fakeHTTPAdapterFactory) NewQueryExecutor(context.Context, string, map[string]any) (dsadapter.QueryExecutor, error) {
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

func newTestAPI(t *testing.T) *API {
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
	service := core.New(store, secret, "test", func() string { return "http://127.0.0.1:18790" }, fakeHTTPAdapterFactory{})
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
