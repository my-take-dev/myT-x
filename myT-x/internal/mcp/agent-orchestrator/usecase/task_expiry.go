package usecase

import (
	"context"
	"log"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

func expirePendingTasks(ctx context.Context, tasks domain.TaskRepository, logger *log.Logger) error {
	_, err := tasks.ExpirePendingTasks(ctx, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return operationError(logger, "failed to expire pending tasks", err)
	}
	return nil
}
