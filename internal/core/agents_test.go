package core

import (
	"context"
	"strings"
	"testing"

	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
	"github.com/ekaya-inc/dataclaw/pkg/models"
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

func TestManagerCapabilityForcesRawQueryAndAllowsCatalogCRUD(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()
	ctx := context.Background()
	seedDatasource(t, service, "postgres")

	manager, err := service.CreateAgent(ctx, AgentInput{
		Name:                     "Catalog manager",
		CanManageApprovedQueries: true,
		ApprovedQueryScope:       storepkg.ApprovedQueryScopeNone,
	})
	if err != nil {
		t.Fatalf("CreateAgent(manager): %v", err)
	}
	if !manager.CanQuery {
		t.Fatal("expected manager capability to force can_query=true")
	}
	if !manager.CanManageApprovedQueries {
		t.Fatal("expected manager capability to round-trip on create")
	}
	if manager.ApprovedQueryScope != storepkg.ApprovedQueryScopeAll {
		t.Fatalf("expected manager capability to force approved_query_scope=all, got %q", manager.ApprovedQueryScope)
	}
	if len(manager.ApprovedQueryIDs) != 0 {
		t.Fatalf("expected manager capability to clear approved_query_ids, got %#v", manager.ApprovedQueryIDs)
	}

	internalManager, err := service.AuthenticateAgent(ctx, manager.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent(manager): %v", err)
	}
	if internalManager == nil {
		t.Fatal("expected manager agent to authenticate")
	}

	queries, err := service.ListQueriesForAgent(ctx, internalManager)
	if err != nil {
		t.Fatalf("ListQueriesForAgent(manager): %v", err)
	}
	if len(queries) != 0 {
		t.Fatalf("expected empty catalog for new manager, got %#v", queries)
	}

	createdQuery, err := service.CreateQueryForAgent(ctx, internalManager, &storepkg.ApprovedQuery{
		NaturalLanguagePrompt: "List accounts",
		SQLQuery:              "SELECT account_id FROM accounts",
	})
	if err != nil {
		t.Fatalf("CreateQueryForAgent(manager): %v", err)
	}
	if _, err := service.ExecuteStoredQueryForAgent(ctx, internalManager, createdQuery.ID, nil, 10); err == nil || strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected manager capability to pass authorization for execute_query, got %v", err)
	}

	updatedQuery, err := service.UpdateQueryForAgent(ctx, internalManager, createdQuery.ID, &storepkg.ApprovedQuery{
		NaturalLanguagePrompt: "Rename account",
		SQLQuery:              "UPDATE accounts SET account_name = {{account_name}} WHERE account_id = {{account_id}}",
		AllowsModification:    true,
		Parameters: []models.QueryParameter{
			{Name: "account_id", Type: "uuid", Description: "Account identifier", Required: true},
			{Name: "account_name", Type: "string", Description: "New account name", Required: true},
		},
	})
	if err != nil {
		t.Fatalf("UpdateQueryForAgent(manager): %v", err)
	}
	if !updatedQuery.AllowsModification {
		t.Fatalf("expected updated query to persist allows_modification, got %#v", updatedQuery)
	}

	if err := service.DeleteQueryForAgent(ctx, internalManager, createdQuery.ID); err != nil {
		t.Fatalf("DeleteQueryForAgent(manager): %v", err)
	}
	deletedQuery, err := service.GetQuery(ctx, createdQuery.ID)
	if err != nil {
		t.Fatalf("GetQuery(after delete): %v", err)
	}
	if deletedQuery != nil {
		t.Fatalf("expected query to be deleted, got %#v", deletedQuery)
	}
}

func TestApprovedQueryCRUDRequiresManagerCapability(t *testing.T) {
	service := newTestService(t)
	defer service.store.Close()
	ctx := context.Background()
	seedDatasource(t, service, "postgres")

	manager, err := service.CreateAgent(ctx, AgentInput{
		Name:                     "Catalog manager",
		CanManageApprovedQueries: true,
	})
	if err != nil {
		t.Fatalf("CreateAgent(manager): %v", err)
	}
	reader, err := service.CreateAgent(ctx, AgentInput{Name: "Reader"})
	if err != nil {
		t.Fatalf("CreateAgent(reader): %v", err)
	}

	internalManager, err := service.AuthenticateAgent(ctx, manager.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent(manager): %v", err)
	}
	internalReader, err := service.AuthenticateAgent(ctx, reader.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAgent(reader): %v", err)
	}

	createdQuery, err := service.CreateQueryForAgent(ctx, internalManager, &storepkg.ApprovedQuery{
		NaturalLanguagePrompt: "List accounts",
		SQLQuery:              "SELECT account_id FROM accounts",
	})
	if err != nil {
		t.Fatalf("CreateQueryForAgent(manager): %v", err)
	}

	if _, err := service.CreateQueryForAgent(ctx, internalReader, &storepkg.ApprovedQuery{
		NaturalLanguagePrompt: "Should fail",
		SQLQuery:              "SELECT 1",
	}); err == nil {
		t.Fatal("expected non-manager create to fail")
	}
	if _, err := service.UpdateQueryForAgent(ctx, internalReader, createdQuery.ID, &storepkg.ApprovedQuery{
		NaturalLanguagePrompt: "Should fail",
		SQLQuery:              "SELECT 1",
	}); err == nil {
		t.Fatal("expected non-manager update to fail")
	}
	if err := service.DeleteQueryForAgent(ctx, internalReader, createdQuery.ID); err == nil {
		t.Fatal("expected non-manager delete to fail")
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
