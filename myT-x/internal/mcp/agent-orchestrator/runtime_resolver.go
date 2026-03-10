package orchestrator

import (
	"context"
	"log"
	"sync"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// stickySelfResolver caches the first successfully resolved pane id for the life
// of the MCP server process so later tool calls do not depend on repeated tmux
// display-message lookups.
type stickySelfResolver struct {
	base   domain.SelfPaneResolver
	logger *log.Logger

	mu     sync.RWMutex
	paneID string
}

func newStickySelfResolver(base domain.SelfPaneResolver, logger *log.Logger) *stickySelfResolver {
	return &stickySelfResolver{
		base:   base,
		logger: logger,
	}
}

func (r *stickySelfResolver) GetPaneID(ctx context.Context) (string, error) {
	r.mu.RLock()
	if r.paneID != "" {
		paneID := r.paneID
		r.mu.RUnlock()
		return paneID, nil
	}
	r.mu.RUnlock()

	paneID, err := r.base.GetPaneID(ctx)
	if err != nil {
		return "", err
	}

	r.mu.Lock()
	if r.paneID == "" {
		r.paneID = paneID
	}
	cached := r.paneID
	r.mu.Unlock()
	return cached, nil
}

func (r *stickySelfResolver) SetPaneID(paneID string) {
	if paneID == "" {
		return
	}
	r.mu.Lock()
	r.paneID = paneID
	r.mu.Unlock()
}
