package core

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	dsadapter "github.com/ekaya-inc/dataclaw/internal/adapters/datasource"
	"github.com/ekaya-inc/dataclaw/internal/security"
	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

type AgentInput struct {
	Name               string
	CanQuery           bool
	CanExecute         bool
	ApprovedQueryScope storepkg.ApprovedQueryScope
	ApprovedQueryIDs   []string
}

type AgentView struct {
	ID                 string                      `json:"id"`
	Name               string                      `json:"name"`
	InstallAlias       string                      `json:"install_alias"`
	MaskedAPIKey       string                      `json:"masked_api_key"`
	CanQuery           bool                        `json:"can_query"`
	CanExecute         bool                        `json:"can_execute"`
	ApprovedQueryScope storepkg.ApprovedQueryScope `json:"approved_query_scope"`
	ApprovedQueryIDs   []string                    `json:"approved_query_ids"`
	CreatedAt          time.Time                   `json:"created_at"`
	UpdatedAt          time.Time                   `json:"updated_at"`
	LastUsedAt         *time.Time                  `json:"last_used_at,omitempty"`
}

type AgentCredentialView struct {
	AgentView
	APIKey string `json:"api_key"`
}

func (s *Service) HasDatasource(ctx context.Context) (bool, error) {
	ds, err := s.store.GetDatasource(ctx)
	if err != nil {
		return false, err
	}
	return ds != nil, nil
}

func (s *Service) ListAgents(ctx context.Context) ([]*AgentView, error) {
	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]*AgentView, 0, len(agents))
	for _, agent := range agents {
		view, err := s.agentView(agent, "")
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) GetAgent(ctx context.Context, id string) (*AgentView, error) {
	agent, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, nil
	}
	return s.agentView(agent, "")
}

func (s *Service) CreateAgent(ctx context.Context, input AgentInput) (*AgentCredentialView, error) {
	normalized, err := s.normalizeAgentInput(ctx, input)
	if err != nil {
		return nil, err
	}
	plainKey, encryptedKey, err := generateAPIKey(s.secret)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		agent := &storepkg.Agent{
			Name:               normalized.Name,
			InstallAlias:       generateInstallAlias(normalized.Name),
			APIKeyEncrypted:    encryptedKey,
			CanQuery:           normalized.CanQuery,
			CanExecute:         normalized.CanExecute,
			ApprovedQueryScope: normalized.ApprovedQueryScope,
			ApprovedQueryIDs:   append([]string(nil), normalized.ApprovedQueryIDs...),
		}
		if err := s.store.CreateAgent(ctx, agent); err != nil {
			lastErr = err
			if isAgentAliasConflict(err) {
				continue
			}
			return nil, err
		}
		view, err := s.agentView(agent, plainKey)
		if err != nil {
			return nil, err
		}
		return &AgentCredentialView{AgentView: *view, APIKey: plainKey}, nil
	}
	if lastErr == nil {
		lastErr = errors.New("failed to create agent")
	}
	return nil, fmt.Errorf("could not generate a unique install alias: %w", lastErr)
}

func (s *Service) UpdateAgent(ctx context.Context, id string, input AgentInput) (*AgentView, error) {
	existing, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("agent not found")
	}
	normalized, err := s.normalizeAgentInput(ctx, input)
	if err != nil {
		return nil, err
	}
	existing.Name = normalized.Name
	existing.CanQuery = normalized.CanQuery
	existing.CanExecute = normalized.CanExecute
	existing.ApprovedQueryScope = normalized.ApprovedQueryScope
	existing.ApprovedQueryIDs = append([]string(nil), normalized.ApprovedQueryIDs...)
	if err := s.store.UpdateAgent(ctx, existing); err != nil {
		return nil, err
	}
	updated, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, errors.New("agent not found")
	}
	return s.agentView(updated, "")
}

func (s *Service) DeleteAgent(ctx context.Context, id string) error {
	existing, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return errors.New("agent not found")
	}
	return s.store.DeleteAgent(ctx, id)
}

func (s *Service) RevealAgentKey(ctx context.Context, id string) (*AgentCredentialView, error) {
	agent, plainKey, err := s.getAgentWithPlainKey(ctx, id)
	if err != nil {
		return nil, err
	}
	view, err := s.agentView(agent, plainKey)
	if err != nil {
		return nil, err
	}
	return &AgentCredentialView{AgentView: *view, APIKey: plainKey}, nil
}

func (s *Service) RotateAgentKey(ctx context.Context, id string) (*AgentCredentialView, error) {
	agent, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	if agent == nil {
		return nil, errors.New("agent not found")
	}
	plainKey, encryptedKey, err := generateAPIKey(s.secret)
	if err != nil {
		return nil, err
	}
	agent.APIKeyEncrypted = encryptedKey
	if err := s.store.UpdateAgent(ctx, agent); err != nil {
		return nil, err
	}
	updated, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, errors.New("agent not found")
	}
	view, err := s.agentView(updated, plainKey)
	if err != nil {
		return nil, err
	}
	return &AgentCredentialView{AgentView: *view, APIKey: plainKey}, nil
}

func (s *Service) AuthenticateAgent(ctx context.Context, key string) (*storepkg.Agent, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, nil
	}
	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		return nil, err
	}
	for _, agent := range agents {
		plainKey, err := security.DecryptString(s.secret, agent.APIKeyEncrypted)
		if err != nil {
			return nil, err
		}
		if subtle.ConstantTimeCompare([]byte(plainKey), []byte(key)) != 1 {
			continue
		}
		return agent, nil
	}
	return nil, nil
}

func (s *Service) RecordAgentToolUse(ctx context.Context, agentID string) error {
	if strings.TrimSpace(agentID) == "" {
		return errors.New("agent id is required")
	}
	now := time.Now().UTC()
	return s.store.SetAgentLastUsedAt(ctx, agentID, now)
}

func (s *Service) ListQueriesForAgent(ctx context.Context, agent *storepkg.Agent) ([]*storepkg.ApprovedQuery, error) {
	if agent == nil {
		return nil, errors.New("agent is required")
	}
	if agent.ApprovedQueryScope == storepkg.ApprovedQueryScopeNone {
		return nil, errors.New("agent is not allowed to list approved queries")
	}
	if _, err := s.requireDatasource(ctx); err != nil {
		return nil, err
	}
	if agent.ApprovedQueryScope == storepkg.ApprovedQueryScopeAll {
		return s.store.ListQueries(ctx)
	}
	return s.store.ListQueriesByIDs(ctx, agent.ApprovedQueryIDs)
}

func (s *Service) ExecuteStoredQueryForAgent(ctx context.Context, agent *storepkg.Agent, id string, values map[string]any, limit int) (*QueryResult, error) {
	allowed, err := s.agentHasQueryAccess(ctx, agent, id)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, errors.New("agent is not allowed to execute this approved query")
	}
	return s.ExecuteStoredQuery(ctx, id, values, limit)
}

func (s *Service) ExecuteRawMutation(ctx context.Context, sqlQuery string, limit int) (*QueryResult, error) {
	ds, err := s.requireDatasource(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := dsadapter.ValidateMutatingSQL(sqlQuery)
	if err != nil {
		return nil, err
	}
	executor, err := s.adapters.NewQueryExecutor(ctx, ds.Type, ds.Config)
	if err != nil {
		return nil, err
	}
	defer executor.Close()
	return executor.ExecuteMutatingQuery(ctx, normalized, nil, nil, limit)
}

func (s *Service) normalizeAgentInput(ctx context.Context, input AgentInput) (AgentInput, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return AgentInput{}, errors.New("name is required")
	}
	scope := input.ApprovedQueryScope
	if scope == "" {
		scope = storepkg.ApprovedQueryScopeNone
	}
	switch scope {
	case storepkg.ApprovedQueryScopeNone, storepkg.ApprovedQueryScopeAll, storepkg.ApprovedQueryScopeSelected:
	default:
		return AgentInput{}, errors.New("approved_query_scope must be one of none, all, or selected")
	}
	queryIDs := dedupeStrings(input.ApprovedQueryIDs)
	if scope != storepkg.ApprovedQueryScopeSelected && len(queryIDs) > 0 {
		return AgentInput{}, errors.New("approved_query_ids can only be set when approved_query_scope is selected")
	}
	if scope == storepkg.ApprovedQueryScopeSelected {
		if len(queryIDs) == 0 {
			return AgentInput{}, errors.New("selected approved_query_scope requires at least one approved_query_id")
		}
		for _, queryID := range queryIDs {
			query, err := s.store.GetQuery(ctx, queryID)
			if err != nil {
				return AgentInput{}, err
			}
			if query == nil {
				return AgentInput{}, fmt.Errorf("approved query %s not found", queryID)
			}
		}
	}
	if !input.CanQuery && !input.CanExecute && scope == storepkg.ApprovedQueryScopeNone {
		return AgentInput{}, errors.New("enable query, execute, or approved queries access before saving the agent")
	}
	return AgentInput{
		Name:               name,
		CanQuery:           input.CanQuery,
		CanExecute:         input.CanExecute,
		ApprovedQueryScope: scope,
		ApprovedQueryIDs:   queryIDs,
	}, nil
}

func (s *Service) agentHasQueryAccess(ctx context.Context, agent *storepkg.Agent, queryID string) (bool, error) {
	if agent == nil {
		return false, errors.New("agent is required")
	}
	switch agent.ApprovedQueryScope {
	case storepkg.ApprovedQueryScopeNone:
		return false, nil
	case storepkg.ApprovedQueryScopeAll:
		query, err := s.store.GetQuery(ctx, queryID)
		return query != nil, err
	case storepkg.ApprovedQueryScopeSelected:
		for _, allowedID := range agent.ApprovedQueryIDs {
			if allowedID == queryID {
				query, err := s.store.GetQuery(ctx, queryID)
				return query != nil, err
			}
		}
		return false, nil
	default:
		return false, errors.New("unsupported approved_query_scope")
	}
}

func (s *Service) agentView(agent *storepkg.Agent, plainKey string) (*AgentView, error) {
	if agent == nil {
		return nil, errors.New("agent is required")
	}
	if plainKey == "" {
		var err error
		plainKey, err = security.DecryptString(s.secret, agent.APIKeyEncrypted)
		if err != nil {
			return nil, err
		}
	}
	return &AgentView{
		ID:                 agent.ID,
		Name:               agent.Name,
		InstallAlias:       agent.InstallAlias,
		MaskedAPIKey:       maskAPIKey(plainKey),
		CanQuery:           agent.CanQuery,
		CanExecute:         agent.CanExecute,
		ApprovedQueryScope: agent.ApprovedQueryScope,
		ApprovedQueryIDs:   append([]string(nil), agent.ApprovedQueryIDs...),
		CreatedAt:          agent.CreatedAt,
		UpdatedAt:          agent.UpdatedAt,
		LastUsedAt:         agent.LastUsedAt,
	}, nil
}

func (s *Service) getAgentWithPlainKey(ctx context.Context, id string) (*storepkg.Agent, string, error) {
	agent, err := s.store.GetAgent(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if agent == nil {
		return nil, "", errors.New("agent not found")
	}
	plainKey, err := security.DecryptString(s.secret, agent.APIKeyEncrypted)
	if err != nil {
		return nil, "", err
	}
	return agent, plainKey, nil
}

func maskAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	keep := 8
	if len(key) < keep {
		keep = len(key)
	}
	return key[:keep] + "••••"
}

func generateInstallAlias(name string) string {
	seed := slugifyAliasSeed(name)
	suffix := strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	if len(suffix) > 6 {
		suffix = suffix[len(suffix)-6:]
	}
	return fmt.Sprintf("%s-%s", seed, suffix)
}

func slugifyAliasSeed(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	lastHyphen := false
	for _, r := range lower {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastHyphen = false
		case !lastHyphen:
			b.WriteRune('-')
			lastHyphen = true
		}
	}
	seed := strings.Trim(b.String(), "-")
	if seed == "" {
		seed = "agent"
	}
	if len(seed) > 24 {
		seed = strings.Trim(seed[:24], "-")
	}
	if seed == "" {
		seed = "agent"
	}
	return seed
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func isAgentAliasConflict(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint failed: agents.install_alias") || strings.Contains(msg, "agents.install_alias")
}
