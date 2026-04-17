package core

import (
	"context"
	"testing"

	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

func TestCreateAgentRevealRotateAndAuthenticate(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()
	ctx := context.Background()
	seedDatasource(t, service, "postgres")

	query, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{
		NaturalLanguagePrompt: "Connectivity check",
		SQLQuery:              "SELECT true AS connected",
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}

	created, err := service.CreateAgent(ctx, AgentInput{
		Name:               "Warehouse analyst",
		ApprovedQueryScope: storepkg.ApprovedQueryScopeSelected,
		ApprovedQueryIDs:   []string{query.ID},
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if created.APIKey == "" {
		t.Fatal("expected plaintext api key on create")
	}
	if created.MaskedAPIKey == "" || created.MaskedAPIKey == created.APIKey {
		t.Fatalf("expected masked key to differ from plaintext, got %#v", created.MaskedAPIKey)
	}

	revealed, err := service.RevealAgentKey(ctx, created.ID)
	if err != nil {
		t.Fatalf("RevealAgentKey: %v", err)
	}
	if revealed.APIKey != created.APIKey {
		t.Fatalf("expected reveal to return original key %q, got %q", created.APIKey, revealed.APIKey)
	}

	authenticated, err := service.AuthenticateAgent(ctx, created.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent(create): %v", err)
	}
	if authenticated == nil || authenticated.ID != created.ID {
		t.Fatalf("expected authentication to resolve created agent, got %#v", authenticated)
	}

	rotated, err := service.RotateAgentKey(ctx, created.ID)
	if err != nil {
		t.Fatalf("RotateAgentKey: %v", err)
	}
	if rotated.APIKey == created.APIKey {
		t.Fatal("expected rotated key to differ from original")
	}

	oldAgent, err := service.AuthenticateAgent(ctx, created.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent(old): %v", err)
	}
	if oldAgent != nil {
		t.Fatalf("expected old key to stop authenticating, got %#v", oldAgent)
	}
	newAgent, err := service.AuthenticateAgent(ctx, rotated.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent(rotated): %v", err)
	}
	if newAgent == nil || newAgent.ID != created.ID {
		t.Fatalf("expected rotated key to authenticate same agent, got %#v", newAgent)
	}
}

func TestUpdateAgentChangesScopeAndValidatesSelections(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()
	ctx := context.Background()
	seedDatasource(t, service, "postgres")

	query, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{
		NaturalLanguagePrompt: "List accounts",
		SQLQuery:              "SELECT * FROM accounts",
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}

	created, err := service.CreateAgent(ctx, AgentInput{Name: "Planner", CanQuery: true})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	updated, err := service.UpdateAgent(ctx, created.ID, AgentInput{
		Name:               "Planner",
		ApprovedQueryScope: storepkg.ApprovedQueryScopeSelected,
		ApprovedQueryIDs:   []string{query.ID},
	})
	if err != nil {
		t.Fatalf("UpdateAgent: %v", err)
	}
	if updated.Name != "Planner" {
		t.Fatalf("expected name to persist, got %#v", updated.Name)
	}
	if len(updated.ApprovedQueryIDs) != 1 || updated.ApprovedQueryIDs[0] != query.ID {
		t.Fatalf("unexpected approved query ids: %#v", updated.ApprovedQueryIDs)
	}

	if _, err := service.UpdateAgent(ctx, created.ID, AgentInput{Name: "Planner", ApprovedQueryScope: storepkg.ApprovedQueryScopeSelected}); err == nil {
		t.Fatal("expected selected scope without query ids to fail")
	}
}

func TestCreateAgentRejectsDuplicateNameCaseInsensitive(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()
	ctx := context.Background()
	seedDatasource(t, service, "postgres")

	if _, err := service.CreateAgent(ctx, AgentInput{Name: "Finance Bot", CanQuery: true}); err != nil {
		t.Fatalf("CreateAgent first: %v", err)
	}
	if _, err := service.CreateAgent(ctx, AgentInput{Name: "finance bot", CanQuery: true}); err == nil {
		t.Fatal("expected case-insensitive duplicate name to fail")
	}
}

func TestDeleteDatasourcePreservesAgentsAndFailsClosed(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()
	ctx := context.Background()
	seedDatasource(t, service, "postgres")

	_, err := service.CreateQuery(ctx, &storepkg.ApprovedQuery{
		NaturalLanguagePrompt: "List accounts",
		SQLQuery:              "SELECT * FROM accounts",
	})
	if err != nil {
		t.Fatalf("CreateQuery: %v", err)
	}
	created, err := service.CreateAgent(ctx, AgentInput{ApprovedQueryScope: storepkg.ApprovedQueryScopeAll, Name: "Reader"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	status := service.Status()
	if got, _ := status["agent_count"].(int); got != 1 {
		t.Fatalf("expected agent_count=1, got %#v", status["agent_count"])
	}

	internalAgent, err := service.AuthenticateAgent(ctx, created.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent: %v", err)
	}
	if internalAgent == nil {
		t.Fatal("expected authenticated agent before datasource deletion")
	}

	if err := service.DeleteDatasource(ctx); err != nil {
		t.Fatalf("DeleteDatasource: %v", err)
	}

	status = service.Status()
	if got, _ := status["agent_count"].(int); got != 1 {
		t.Fatalf("expected agent_count to remain 1 after datasource deletion, got %#v", status["agent_count"])
	}
	if configured, _ := status["datasource_configured"].(bool); configured {
		t.Fatal("expected datasource_configured=false after deletion")
	}

	listed, err := service.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 persisted agent after datasource delete, got %d", len(listed))
	}
	if _, err := service.ListQueriesForAgent(ctx, internalAgent); err == nil {
		t.Fatal("expected ListQueriesForAgent to fail closed without datasource")
	}
}
