package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ekaya-inc/dataclaw/internal/core"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/pkg/models"
)

type API struct {
	service *core.Service
}

func New(service *core.Service) *API { return &API{service: service} }

func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /ping", a.handlePing)
	mux.HandleFunc("HEAD /ping", a.handlePing)
	mux.HandleFunc("GET /bundles/", a.handleBundleBySlug)
	mux.HandleFunc("GET /api/status", a.handleStatus)
	mux.HandleFunc("GET /api/mcp-events", a.handleListMCPEvents)
	mux.HandleFunc("GET /api/mcp-events/", a.handleGetMCPEvent)
	mux.HandleFunc("GET /api/datasource", a.handleGetDatasource)
	mux.HandleFunc("GET /api/datasource/types", a.handleGetDatasourceTypes)
	mux.HandleFunc("PUT /api/datasource", a.handlePutDatasource)
	mux.HandleFunc("DELETE /api/datasource", a.handleDeleteDatasource)
	mux.HandleFunc("POST /api/datasource/test", a.handleTestDatasource)
	mux.HandleFunc("GET /api/queries", a.handleListQueries)
	mux.HandleFunc("POST /api/queries", a.handleCreateQuery)
	mux.HandleFunc("POST /api/queries/test", a.handleTestQuery)
	mux.HandleFunc("POST /api/queries/validate", a.handleValidateQuery)
	mux.HandleFunc("GET /api/queries/", a.handleQueryByID)
	mux.HandleFunc("PUT /api/queries/", a.handleQueryByID)
	mux.HandleFunc("DELETE /api/queries/", a.handleQueryByID)
	mux.HandleFunc("POST /api/queries/", a.handleQueryByID)
	mux.HandleFunc("GET /api/agents", a.handleListAgents)
	mux.HandleFunc("POST /api/agents", a.handleCreateAgent)
	mux.HandleFunc("GET /api/agents/", a.handleAgentByID)
	mux.HandleFunc("PUT /api/agents/", a.handleAgentByID)
	mux.HandleFunc("DELETE /api/agents/", a.handleAgentByID)
	mux.HandleFunc("POST /api/agents/", a.handleAgentByID)
}

type response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

type datasourceRequest struct {
	Name        string         `json:"name"`
	DisplayName string         `json:"display_name"`
	Type        string         `json:"type"`
	Provider    string         `json:"provider,omitempty"`
	Config      map[string]any `json:"config"`
	Host        string         `json:"host"`
	Port        any            `json:"port"`
	User        string         `json:"user"`
	Username    string         `json:"username"`
	Password    string         `json:"password"`
	SSLMode     string         `json:"ssl_mode"`
	Options     map[string]any `json:"options"`
}

type queryRequest struct {
	NaturalLanguagePrompt string                  `json:"natural_language_prompt"`
	AdditionalContext     string                  `json:"additional_context"`
	SQLQuery              string                  `json:"sql_query"`
	AllowsModification    bool                    `json:"allows_modification"`
	Parameters            []models.QueryParameter `json:"parameters"`
	OutputColumns         []models.OutputColumn   `json:"output_columns"`
	Constraints           string                  `json:"constraints"`
}

type executeRequest struct {
	Parameters map[string]any `json:"parameters,omitempty"`
	Limit      int            `json:"limit,omitempty"`
}

type validateRequest struct {
	SQLQuery           string                  `json:"sql_query"`
	Parameters         []models.QueryParameter `json:"parameters,omitempty"`
	AllowsModification bool                    `json:"allows_modification"`
}

type queryTestRequest struct {
	SQLQuery           string                  `json:"sql_query"`
	Parameters         []models.QueryParameter `json:"parameters,omitempty"`
	ParameterValues    map[string]any          `json:"parameter_values,omitempty"`
	AllowsModification bool                    `json:"allows_modification"`
	Limit              int                     `json:"limit,omitempty"`
}

type pingResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Service string `json:"service"`
}

func (a *API) handlePing(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	if configured, _ := a.service.HasDatasource(r.Context()); !configured {
		status = "no datasource"
	}
	body := pingResponse{Status: status, Version: a.service.Version(), Service: "dataclaw"}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func (a *API) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, response{Success: true, Data: a.service.Status()})
}

func (a *API) handleGetDatasource(w http.ResponseWriter, r *http.Request) {
	ds, err := a.service.GetDatasource(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"datasource": a.flattenDatasource(ds)}})
}

func (a *API) handleGetDatasourceTypes(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"types": a.service.DatasourceTypes()}})
}

func (a *API) handlePutDatasource(w http.ResponseWriter, r *http.Request) {
	var req datasourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Error: "invalid request body"})
		return
	}
	ds, err := a.service.UpsertDatasource(r.Context(), parseDatasourceRequest(req))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"datasource": a.flattenDatasource(ds)}})
}

func (a *API) handleDeleteDatasource(w http.ResponseWriter, r *http.Request) {
	if err := a.service.DeleteDatasource(r.Context()); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"deleted": true}})
}

func (a *API) handleTestDatasource(w http.ResponseWriter, r *http.Request) {
	var req datasourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Error: "invalid request body"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := a.service.TestDatasource(ctx, parseDatasourceRequest(req)); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"message": "connection successful"}})
}

func (a *API) handleListQueries(w http.ResponseWriter, r *http.Request) {
	queries, err := a.service.ListQueries(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"queries": queries}})
}

func (a *API) handleCreateQuery(w http.ResponseWriter, r *http.Request) {
	var req queryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Error: "invalid request body"})
		return
	}
	query, err := a.service.CreateQuery(r.Context(), approvedQueryFromRequest(req))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, response{Success: true, Data: map[string]any{"query": query}})
}

func (a *API) handleTestQuery(w http.ResponseWriter, r *http.Request) {
	var req queryTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Error: "invalid request body"})
		return
	}
	result, err := a.service.TestDraftQuery(r.Context(), req.SQLQuery, req.Parameters, req.ParameterValues, req.AllowsModification, req.Limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: result})
}

func (a *API) handleValidateQuery(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Error: "invalid request body"})
		return
	}
	normalized, err := a.service.ValidateQuerySQL(req.SQLQuery, req.Parameters, req.AllowsModification)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"normalized_sql": normalized, "valid": true}})
}

func (a *API) handleQueryByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/queries/")
	path = strings.Trim(path, "/")
	if path == "" {
		writeJSON(w, http.StatusNotFound, response{Error: "query id required"})
		return
	}
	if strings.HasSuffix(path, "/execute") {
		id := strings.TrimSuffix(path, "/execute")
		a.handleExecuteQuery(w, r, id)
		return
	}
	id := path
	switch r.Method {
	case http.MethodGet:
		q, err := a.service.GetQuery(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		if q == nil {
			writeJSON(w, http.StatusNotFound, response{Error: "query not found"})
			return
		}
		writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"query": q}})
	case http.MethodPut:
		var req queryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{Error: "invalid request body"})
			return
		}
		q, err := a.service.UpdateQuery(r.Context(), id, approvedQueryFromRequest(req))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"query": q}})
	case http.MethodDelete:
		if err := a.service.DeleteQuery(r.Context(), id); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"deleted": true}})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Error: "method not allowed"})
	}
}

func (a *API) handleExecuteQuery(w http.ResponseWriter, r *http.Request, id string) {
	var req executeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, context.Canceled) {
		writeJSON(w, http.StatusBadRequest, response{Error: "invalid request body"})
		return
	}
	result, err := a.service.ExecuteStoredQuery(r.Context(), id, req.Parameters, req.Limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: result})
}

func writeJSON(w http.ResponseWriter, status int, payload response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "unsupported") || strings.Contains(strings.ToLower(msg), "required") || strings.Contains(strings.ToLower(msg), "cannot be changed") || strings.Contains(strings.ToLower(msg), "only read-only") || strings.Contains(strings.ToLower(msg), "multiple sql statements") {
		status = http.StatusBadRequest
	} else if strings.Contains(strings.ToLower(msg), "not found") {
		status = http.StatusNotFound
	} else if strings.Contains(strings.ToLower(msg), "no datasource") {
		status = http.StatusConflict
	} else if strings.Contains(strings.ToLower(msg), "connect") || strings.Contains(strings.ToLower(msg), "ping") {
		status = http.StatusBadGateway
	}
	writeJSON(w, status, response{Error: msg})
}

func parseDatasourceRequest(req datasourceRequest) *storepkg.Datasource {
	config := req.Config
	if config == nil {
		config = map[string]any{}
		if req.Host != "" {
			config["host"] = req.Host
		}
		if req.Port != nil {
			config["port"] = req.Port
		}
		username := req.Username
		if username == "" {
			username = req.User
		}
		if username != "" {
			config["user"] = username
		}
		if req.Password != "" {
			config["password"] = req.Password
		}
		if req.Name != "" {
			config["name"] = req.Name
			config["database"] = req.Name
		}
		if req.SSLMode != "" {
			config["ssl_mode"] = req.SSLMode
		}
		for key, value := range req.Options {
			config[key] = value
		}
	}
	name := req.DisplayName
	if name == "" {
		name = req.Name
	}
	return &storepkg.Datasource{Name: name, Type: req.Type, Provider: req.Provider, Config: config}
}

func (a *API) flattenDatasource(ds *storepkg.Datasource) map[string]any {
	if ds == nil {
		return nil
	}
	typeInfo, _ := a.service.DatasourceTypeInfo(ds.Type)
	out := map[string]any{
		"id":           ds.ID,
		"type":         ds.Type,
		"provider":     ds.Provider,
		"sql_dialect":  typeInfo.SQLDialect,
		"display_name": ds.Name,
		"name":         stringFromMap(ds.Config, "database", "name"),
		"database":     stringFromMap(ds.Config, "database", "name"),
		"host":         stringFromMap(ds.Config, "host"),
		"port":         firstValue(ds.Config, "port"),
		"user":         stringFromMap(ds.Config, "user", "username"),
		"username":     stringFromMap(ds.Config, "user", "username"),
		"password":     stringFromMap(ds.Config, "password"),
		"ssl_mode":     stringFromMap(ds.Config, "ssl_mode"),
		"options": map[string]any{
			"encrypt":                  firstValue(ds.Config, "encrypt"),
			"trust_server_certificate": firstValue(ds.Config, "trust_server_certificate"),
		},
		"created_at": ds.CreatedAt,
		"updated_at": ds.UpdatedAt,
	}
	return out
}

func stringFromMap(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok && value != nil {
			if str, ok := value.(string); ok {
				return str
			}
			return fmt.Sprint(value)
		}
	}
	return ""
}

func firstValue(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

func approvedQueryFromRequest(req queryRequest) *storepkg.ApprovedQuery {
	parameters := req.Parameters
	if parameters == nil {
		parameters = []models.QueryParameter{}
	}
	outputs := req.OutputColumns
	if outputs == nil {
		outputs = []models.OutputColumn{}
	}
	return &storepkg.ApprovedQuery{
		NaturalLanguagePrompt: req.NaturalLanguagePrompt,
		AdditionalContext:     req.AdditionalContext,
		SQLQuery:              req.SQLQuery,
		AllowsModification:    req.AllowsModification,
		Parameters:            parameters,
		OutputColumns:         outputs,
		Constraints:           req.Constraints,
	}
}
