package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/ekaya-inc/dataclaw/internal/core"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

type agentRequest struct {
	Name               string   `json:"name"`
	CanQuery           bool     `json:"can_query"`
	CanExecute         bool     `json:"can_execute"`
	ApprovedQueryScope string   `json:"approved_query_scope"`
	ApprovedQueryIDs   []string `json:"approved_query_ids"`
}

func (a *API) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := a.service.ListAgents(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	payload := make([]map[string]any, 0, len(agents))
	for _, agent := range agents {
		payload = append(payload, agentResponse(agent))
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"agents": payload}})
}

func (a *API) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	input, err := parseAgentRequest(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response{Error: "invalid request body"})
		return
	}
	agent, err := a.service.CreateAgent(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, response{Success: true, Data: map[string]any{"agent": agentCredentialResponse(agent)}})
}

func (a *API) handleAgentByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/agents/")
	if path == r.URL.Path || path == "" {
		writeJSON(w, http.StatusNotFound, response{Error: "agent not found"})
		return
	}
	id := path
	suffix := ""
	if cutID, cutSuffix, ok := strings.Cut(path, "/"); ok {
		id = cutID
		suffix = cutSuffix
	}
	if id == "" {
		writeJSON(w, http.StatusNotFound, response{Error: "agent not found"})
		return
	}

	switch {
	case suffix == "" && r.Method == http.MethodGet:
		a.handleGetAgent(w, r, id)
	case suffix == "" && r.Method == http.MethodPut:
		a.handleUpdateAgent(w, r, id)
	case suffix == "" && r.Method == http.MethodDelete:
		a.handleDeleteAgent(w, r, id)
	case suffix == "key" && r.Method == http.MethodGet:
		a.handleGetAgentKey(w, r, id)
	case suffix == "reveal-key" && r.Method == http.MethodPost:
		a.handleRevealAgentKey(w, r, id)
	case suffix == "rotate-key" && r.Method == http.MethodPost:
		a.handleRotateAgentKey(w, r, id)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Error: "method not allowed"})
	}
}

func (a *API) handleGetAgent(w http.ResponseWriter, r *http.Request, id string) {
	agent, err := a.service.GetAgent(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	if agent == nil {
		writeJSON(w, http.StatusNotFound, response{Error: "agent not found"})
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"agent": agentResponse(agent)}})
}

func (a *API) handleUpdateAgent(w http.ResponseWriter, r *http.Request, id string) {
	input, err := parseAgentRequest(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response{Error: "invalid request body"})
		return
	}
	agent, err := a.service.UpdateAgent(r.Context(), id, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"agent": agentResponse(agent)}})
}

func (a *API) handleDeleteAgent(w http.ResponseWriter, r *http.Request, id string) {
	if err := a.service.DeleteAgent(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"deleted": true}})
}

func (a *API) handleGetAgentKey(w http.ResponseWriter, r *http.Request, id string) {
	if r.URL.Query().Get("reveal") == "true" {
		agent, err := a.service.RevealAgentKey(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"key": agent.APIKey, "masked": false}})
		return
	}
	agent, err := a.service.GetAgent(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	if agent == nil {
		writeJSON(w, http.StatusNotFound, response{Error: "agent not found"})
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"key": agent.MaskedAPIKey, "masked": true}})
}

func (a *API) handleRevealAgentKey(w http.ResponseWriter, r *http.Request, id string) {
	agent, err := a.service.RevealAgentKey(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"agent": agentCredentialResponse(agent)}})
}

func (a *API) handleRotateAgentKey(w http.ResponseWriter, r *http.Request, id string) {
	agent, err := a.service.RotateAgentKey(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response{Success: true, Data: map[string]any{"agent": agentCredentialResponse(agent)}})
}

func parseAgentRequest(body io.Reader) (core.AgentInput, error) {
	var req agentRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return core.AgentInput{}, err
	}
	return core.AgentInput{
		Name:               req.Name,
		CanQuery:           req.CanQuery,
		CanExecute:         req.CanExecute,
		ApprovedQueryScope: storepkg.ApprovedQueryScope(strings.TrimSpace(req.ApprovedQueryScope)),
		ApprovedQueryIDs:   append([]string(nil), req.ApprovedQueryIDs...),
	}, nil
}

func agentResponse(agent *core.AgentView) map[string]any {
	if agent == nil {
		return map[string]any{}
	}
	return map[string]any{
		"id":                   agent.ID,
		"name":                 agent.Name,
		"masked_api_key":       agent.MaskedAPIKey,
		"can_query":            agent.CanQuery,
		"can_execute":          agent.CanExecute,
		"approved_query_scope": string(agent.ApprovedQueryScope),
		"approved_query_ids":   append([]string(nil), agent.ApprovedQueryIDs...),
		"created_at":           agent.CreatedAt,
		"updated_at":           agent.UpdatedAt,
		"last_used_at":         agent.LastUsedAt,
	}
}

func agentCredentialResponse(agent *core.AgentCredentialView) map[string]any {
	payload := agentResponse(&agent.AgentView)
	payload["api_key"] = agent.APIKey
	return payload
}
