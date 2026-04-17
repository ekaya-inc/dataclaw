package core

import (
	"context"
	"errors"
	"strings"
	"time"

	storepkg "github.com/ekaya-inc/dataclaw/internal/store"
)

func (s *Service) RecordAgentToolEvent(ctx context.Context, event *storepkg.MCPToolEvent, updateLastUsedAt bool) error {
	if event == nil {
		return errors.New("mcp tool event is required")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	} else {
		event.CreatedAt = event.CreatedAt.UTC()
	}
	var lastUsedAt *time.Time
	if updateLastUsedAt {
		at := event.CreatedAt
		lastUsedAt = &at
	}
	if event.AgentID != nil {
		trimmed := strings.TrimSpace(*event.AgentID)
		if trimmed == "" {
			event.AgentID = nil
		} else if trimmed != *event.AgentID {
			event.AgentID = &trimmed
		}
	}
	return s.store.RecordMCPToolEvent(ctx, event, lastUsedAt)
}

func (s *Service) ListMCPToolEvents(ctx context.Context, options storepkg.ListMCPToolEventOptions) (*storepkg.MCPToolEventPage, error) {
	return s.store.ListMCPToolEvents(ctx, options)
}
