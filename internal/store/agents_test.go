package store

import (
	"context"
	"testing"

	"github.com/ekaya-inc/dataclaw/pkg/models"
)

func TestFreshSchemaUsesAgentTables(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defer store.Close()

	var agentTableCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'agents'`).Scan(&agentTableCount); err != nil {
		t.Fatalf("query agents table presence: %v", err)
	}
	if agentTableCount != 1 {
		t.Fatalf("expected agents table to exist once, got %d", agentTableCount)
	}

	var membershipTableCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'agent_approved_queries'`).Scan(&membershipTableCount); err != nil {
		t.Fatalf("query agent_approved_queries table presence: %v", err)
	}
	if membershipTableCount != 1 {
		t.Fatalf("expected agent_approved_queries table to exist once, got %d", membershipTableCount)
	}

}

func TestStorePersistsAgentsAndSelectedQueryMemberships(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defer store.Close()

	datasource := &Datasource{Name: "Primary", Type: "postgres", Provider: "postgres", Config: map[string]any{"host": "db.example.com"}}
	if err := store.SaveDatasource(ctx, datasource); err != nil {
		t.Fatalf("SaveDatasource: %v", err)
	}
	queryA := &ApprovedQuery{DatasourceID: datasource.ID, NaturalLanguagePrompt: "List accounts", SQLQuery: "SELECT * FROM accounts", Parameters: []models.QueryParameter{}, OutputColumns: []models.OutputColumn{}}
	if err := store.CreateQuery(ctx, queryA); err != nil {
		t.Fatalf("CreateQuery(queryA): %v", err)
	}
	queryB := &ApprovedQuery{DatasourceID: datasource.ID, NaturalLanguagePrompt: "List contacts", SQLQuery: "SELECT * FROM contacts", Parameters: []models.QueryParameter{}, OutputColumns: []models.OutputColumn{}}
	if err := store.CreateQuery(ctx, queryB); err != nil {
		t.Fatalf("CreateQuery(queryB): %v", err)
	}

	agent := &Agent{
		Name:               "Sales Assistant",
		APIKeyEncrypted:    "encrypted-key",
		CanQuery:           true,
		CanExecute:         false,
		ApprovedQueryScope: ApprovedQueryScopeSelected,
		ApprovedQueryIDs:   []string{queryA.ID, queryB.ID},
	}
	if err := store.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	count, err := store.CountAgents(ctx)
	if err != nil {
		t.Fatalf("CountAgents: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 agent, got %d", count)
	}

	loaded, err := store.GetAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected persisted agent")
	}
	if loaded.Name != agent.Name {
		t.Fatalf("expected name %q, got %q", agent.Name, loaded.Name)
	}
	if len(loaded.ApprovedQueryIDs) != 2 || !containsAll(loaded.ApprovedQueryIDs, []string{queryA.ID, queryB.ID}) {
		t.Fatalf("unexpected approved query ids: %#v", loaded.ApprovedQueryIDs)
	}
}

func TestDeleteDatasourcePreservesAgentsAndClearsSelectedMemberships(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	defer store.Close()

	datasource := &Datasource{Name: "Primary", Type: "postgres", Provider: "postgres", Config: map[string]any{"host": "db.example.com"}}
	if err := store.SaveDatasource(ctx, datasource); err != nil {
		t.Fatalf("SaveDatasource: %v", err)
	}
	query := &ApprovedQuery{DatasourceID: datasource.ID, NaturalLanguagePrompt: "List accounts", SQLQuery: "SELECT * FROM accounts"}
	if err := store.CreateQuery(ctx, query); err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}
	agent := &Agent{
		Name:               "Planner",
		APIKeyEncrypted:    "encrypted-key",
		ApprovedQueryScope: ApprovedQueryScopeSelected,
		ApprovedQueryIDs:   []string{query.ID},
	}
	if err := store.CreateAgent(ctx, agent); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	if err := store.DeleteDatasource(ctx); err != nil {
		t.Fatalf("DeleteDatasource: %v", err)
	}

	count, err := store.CountAgents(ctx)
	if err != nil {
		t.Fatalf("CountAgents: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected agent to survive datasource deletion, got %d agents", count)
	}

	loaded, err := store.GetAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected agent after datasource deletion")
	}
	if len(loaded.ApprovedQueryIDs) != 0 {
		t.Fatalf("expected selected memberships to cascade away, got %#v", loaded.ApprovedQueryIDs)
	}
}

func containsAll(have, want []string) bool {
	seen := make(map[string]struct{}, len(have))
	for _, item := range have {
		seen[item] = struct{}{}
	}
	for _, item := range want {
		if _, ok := seen[item]; !ok {
			return false
		}
	}
	return true
}
