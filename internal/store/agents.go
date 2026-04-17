package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ApprovedQueryScope string

const (
	ApprovedQueryScopeNone     ApprovedQueryScope = "none"
	ApprovedQueryScopeAll      ApprovedQueryScope = "all"
	ApprovedQueryScopeSelected ApprovedQueryScope = "selected"
)

type Agent struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	InstallAlias       string             `json:"install_alias"`
	APIKeyEncrypted    string             `json:"-"`
	CanQuery           bool               `json:"can_query"`
	CanExecute         bool               `json:"can_execute"`
	ApprovedQueryScope ApprovedQueryScope `json:"approved_query_scope"`
	ApprovedQueryIDs   []string           `json:"approved_query_ids,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
	LastUsedAt         *time.Time         `json:"last_used_at,omitempty"`
}

const agentColumns = `id, name, install_alias, api_key_encrypted, can_query, can_execute, approved_query_scope, created_at, updated_at, last_used_at`

func (s *Store) CountAgents(ctx context.Context) (int, error) {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agents`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) ListAgents(ctx context.Context) ([]*Agent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+agentColumns+` FROM agents ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}

	var agents []*Agent
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		agents = append(agents, agent)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, agent := range agents {
		agent.ApprovedQueryIDs, err = s.ListAgentApprovedQueryIDs(ctx, agent.ID)
		if err != nil {
			return nil, err
		}
	}
	return agents, nil
}

func (s *Store) GetAgent(ctx context.Context, id string) (*Agent, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+agentColumns+` FROM agents WHERE id = ?`, id)
	agent, err := scanAgent(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	agent.ApprovedQueryIDs, err = s.ListAgentApprovedQueryIDs(ctx, agent.ID)
	if err != nil {
		return nil, err
	}
	return agent, nil
}

func (s *Store) CreateAgent(ctx context.Context, agent *Agent) error {
	return s.upsertAgent(ctx, agent, true)
}

func (s *Store) UpdateAgent(ctx context.Context, agent *Agent) error {
	return s.upsertAgent(ctx, agent, false)
}

func (s *Store) DeleteAgent(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM agents WHERE id = ?`, id)
	return err
}

func (s *Store) ReplaceAgentApprovedQueries(ctx context.Context, agentID string, queryIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if err = replaceAgentApprovedQueriesTx(ctx, tx, agentID, queryIDs); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListAgentApprovedQueryIDs(ctx context.Context, agentID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT q.id
		FROM approved_queries q
		JOIN agent_approved_queries aq ON aq.query_id = q.id
		WHERE aq.agent_id = ?
		ORDER BY q.created_at ASC
	`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) SetAgentLastUsedAt(ctx context.Context, agentID string, lastUsedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE agents SET last_used_at = ? WHERE id = ?`, lastUsedAt.UTC().Format(time.RFC3339), agentID)
	return err
}

func (s *Store) ListQueriesByIDs(ctx context.Context, ids []string) ([]*ApprovedQuery, error) {
	if len(ids) == 0 {
		return []*ApprovedQuery{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+approvedQueryColumns+` FROM approved_queries WHERE id IN (`+placeholders+`) ORDER BY created_at ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	queries := make([]*ApprovedQuery, 0, len(ids))
	for rows.Next() {
		query, err := scanQuery(rows)
		if err != nil {
			return nil, err
		}
		queries = append(queries, query)
	}
	return queries, rows.Err()
}

func (s *Store) upsertAgent(ctx context.Context, agent *Agent, create bool) (err error) {
	if agent == nil {
		return fmt.Errorf("agent is required")
	}
	if agent.ID == "" {
		agent.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = now
	}
	agent.UpdatedAt = now

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if create {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO agents(
				id, name, install_alias, api_key_encrypted, can_query, can_execute, approved_query_scope, created_at, updated_at, last_used_at
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, agent.ID, agent.Name, agent.InstallAlias, agent.APIKeyEncrypted, boolToInt(agent.CanQuery), boolToInt(agent.CanExecute), string(agent.ApprovedQueryScope), agent.CreatedAt.Format(time.RFC3339), agent.UpdatedAt.Format(time.RFC3339), formatNullableTime(agent.LastUsedAt))
	} else {
		_, err = tx.ExecContext(ctx, `
			UPDATE agents
			SET name = ?, api_key_encrypted = ?, can_query = ?, can_execute = ?, approved_query_scope = ?, updated_at = ?, last_used_at = ?
			WHERE id = ?
		`, agent.Name, agent.APIKeyEncrypted, boolToInt(agent.CanQuery), boolToInt(agent.CanExecute), string(agent.ApprovedQueryScope), agent.UpdatedAt.Format(time.RFC3339), formatNullableTime(agent.LastUsedAt), agent.ID)
	}
	if err != nil {
		return err
	}
	if err = replaceAgentApprovedQueriesTx(ctx, tx, agent.ID, agent.ApprovedQueryIDs); err != nil {
		return err
	}
	return tx.Commit()
}

func replaceAgentApprovedQueriesTx(ctx context.Context, tx *sql.Tx, agentID string, queryIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM agent_approved_queries WHERE agent_id = ?`, agentID); err != nil {
		return err
	}
	for _, queryID := range queryIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO agent_approved_queries(agent_id, query_id) VALUES(?, ?)`, agentID, queryID); err != nil {
			return err
		}
	}
	return nil
}

func scanAgent(scanner interface{ Scan(dest ...any) error }) (*Agent, error) {
	var agent Agent
	var canQuery, canExecute int
	var createdAt, updatedAt string
	var lastUsedAt sql.NullString
	if err := scanner.Scan(&agent.ID, &agent.Name, &agent.InstallAlias, &agent.APIKeyEncrypted, &canQuery, &canExecute, &agent.ApprovedQueryScope, &createdAt, &updatedAt, &lastUsedAt); err != nil {
		return nil, err
	}
	agent.CanQuery = canQuery == 1
	agent.CanExecute = canExecute == 1
	agent.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	agent.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if lastUsedAt.Valid && strings.TrimSpace(lastUsedAt.String) != "" {
		parsed, err := time.Parse(time.RFC3339, lastUsedAt.String)
		if err == nil {
			agent.LastUsedAt = &parsed
		}
	}
	agent.ApprovedQueryIDs = []string{}
	return &agent, nil
}

func formatNullableTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339)
}
