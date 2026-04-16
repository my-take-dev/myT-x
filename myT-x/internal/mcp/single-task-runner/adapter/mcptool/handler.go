package mcptool

import (
	"context"
	"errors"
	"fmt"

	internalmcp "myT-x/internal/mcp/single-task-runner/internal/mcp"
	"myT-x/internal/singletaskrunner"
)

const maxEnqueueTasks = 20

// Handler exposes the single-task-runner service as MCP tools.
type Handler struct {
	service *singletaskrunner.Service
}

// NewHandler creates a Handler.
func NewHandler(service *singletaskrunner.Service) *Handler {
	return &Handler{service: service}
}

// BuildRegistry returns the tool registry for the runtime server.
func (h *Handler) BuildRegistry() (*internalmcp.Registry, error) {
	if h.service == nil {
		return nil, errors.New("service is required")
	}

	return internalmcp.NewRegistry([]internalmcp.Tool{
		h.enqueueTaskTool(),
		h.completeTaskTool(),
		h.failTaskTool(),
		h.listQueueTool(),
		h.cancelTaskTool(),
		helpTool(),
	})
}

func (h *Handler) enqueueTaskTool() internalmcp.Tool {
	return internalmcp.Tool{
		Name:        "enqueue_task",
		Description: "Queue 1-20 tasks for a target pane. Tasks remain pending until the queue is started from the UI or Wails API.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"target_pane": map[string]any{"type": "string", "description": "Target pane ID"},
				"tasks": map[string]any{
					"type":        "array",
					"description": "Tasks to enqueue (1-20 items)",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"title":         map[string]any{"type": "string", "description": "Task title (optional)"},
							"message":       map[string]any{"type": "string", "description": "Task instructions"},
							"clear_before":  map[string]any{"type": "boolean", "description": "Send clear_command before the task"},
							"clear_command": map[string]any{"type": "string", "description": "Clear command to send before the task"},
						},
						"required": []string{"message"},
					},
				},
			},
			"required": []string{"target_pane", "tasks"},
		},
		Handler: h.handleEnqueueTask,
	}
}

func (h *Handler) handleEnqueueTask(_ context.Context, args map[string]any) (any, error) {
	targetPane, err := requiredBoundedString(args, "target_pane", 200)
	if err != nil {
		return nil, err
	}
	tasks, err := requiredEnqueueTasks(args, "tasks", maxEnqueueTasks)
	if err != nil {
		return nil, err
	}

	queued, err := h.service.EnqueueTasks(targetPane, tasks)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(queued))
	for _, entry := range queued {
		items = append(items, map[string]any{
			"task_id":     entry.TaskID,
			"queue_index": entry.OrderIndex,
		})
	}

	status := h.service.GetStatus()
	return map[string]any{
		"queued":       items,
		"queue_length": len(status.Items),
	}, nil
}

func (h *Handler) completeTaskTool() internalmcp.Tool {
	return internalmcp.Tool{
		Name:        "complete_task",
		Description: "Mark the current active task as completed and allow the queue to proceed to the next pending task.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{"type": "string", "description": "Task ID to complete"},
				"result":  map[string]any{"type": "string", "description": "Completion result (max 4000 chars)"},
			},
			"required": []string{"task_id"},
		},
		Handler: h.handleCompleteTask,
	}
}

func (h *Handler) handleCompleteTask(_ context.Context, args map[string]any) (any, error) {
	taskID, err := requiredTaskID(args, "task_id")
	if err != nil {
		return nil, err
	}
	result, err := optionalBoundedString(args, "result", 4000)
	if err != nil {
		return nil, err
	}

	if err := h.service.CompleteTask(taskID, result); err != nil {
		return nil, err
	}
	nextTaskID := firstPendingTaskID(h.service.GetStatus(), taskID)

	return map[string]any{
		"task_id":      taskID,
		"status":       "done",
		"next_task_id": nextTaskID,
	}, nil
}

func (h *Handler) failTaskTool() internalmcp.Tool {
	return internalmcp.Tool{
		Name:        "fail_task",
		Description: "Mark the current active task as failed. The queue stops and remaining pending tasks stay queued.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{"type": "string", "description": "Task ID to fail"},
				"reason":  map[string]any{"type": "string", "description": "Failure reason (max 2000 chars)"},
			},
			"required": []string{"task_id"},
		},
		Handler: h.handleFailTask,
	}
}

func (h *Handler) handleFailTask(_ context.Context, args map[string]any) (any, error) {
	taskID, err := requiredTaskID(args, "task_id")
	if err != nil {
		return nil, err
	}
	reason, err := optionalBoundedString(args, "reason", 2000)
	if err != nil {
		return nil, err
	}

	if err := h.service.FailTask(taskID, reason); err != nil {
		return nil, err
	}
	remaining := countPendingTasks(h.service.GetStatus(), taskID)

	return map[string]any{
		"task_id":         taskID,
		"status":          "failed",
		"remaining_tasks": remaining,
	}, nil
}

func (h *Handler) listQueueTool() internalmcp.Tool {
	return internalmcp.Tool{
		Name:        "list_queue",
		Description: "Return the current queue snapshot, including item statuses and queue state.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Handler: h.handleListQueue,
	}
}

func (h *Handler) handleListQueue(_ context.Context, _ map[string]any) (any, error) {
	status := h.service.GetStatus()
	items := make([]map[string]any, 0, len(status.Items))
	for _, item := range status.Items {
		entry := map[string]any{
			"task_id":        item.ID,
			"title":          item.Title,
			"message":        item.Message,
			"target_pane_id": item.TargetPaneID,
			"queue_index":    item.OrderIndex,
			"status":         item.Status,
			"created_at":     item.CreatedAt,
			"clear_before":   item.ClearBefore,
			"clear_command":  item.ClearCommand,
		}
		if item.StartedAt != "" {
			entry["started_at"] = item.StartedAt
		}
		if item.CompletedAt != "" {
			entry["completed_at"] = item.CompletedAt
		}
		if item.ErrorMessage != "" {
			entry["error_message"] = item.ErrorMessage
		}
		if item.ResultMessage != "" {
			entry["result_message"] = item.ResultMessage
		}
		items = append(items, entry)
	}

	return map[string]any{
		"run_status":      status.RunStatus,
		"current_index":   status.CurrentIndex,
		"session_name":    status.SessionName,
		"clear_delay_sec": status.ClearDelaySec,
		"items":           items,
	}, nil
}

func (h *Handler) cancelTaskTool() internalmcp.Tool {
	return internalmcp.Tool{
		Name:        "cancel_task",
		Description: "Cancel a pending or active task. Pending tasks are marked cancelled and remain visible in queue history; active tasks are marked cancelled and the queue continues to the next pending task.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task_id": map[string]any{"type": "string", "description": "Task ID to cancel"},
				"reason":  map[string]any{"type": "string", "description": "Cancellation reason (max 2000 chars)"},
			},
			"required": []string{"task_id"},
		},
		Handler: h.handleCancelTask,
	}
}

func (h *Handler) handleCancelTask(_ context.Context, args map[string]any) (any, error) {
	taskID, err := requiredTaskID(args, "task_id")
	if err != nil {
		return nil, err
	}
	reason, err := optionalBoundedString(args, "reason", 2000)
	if err != nil {
		return nil, err
	}
	if err := h.service.CancelTask(taskID, reason); err != nil {
		return nil, err
	}

	return map[string]any{
		"task_id": taskID,
		"status":  "cancelled",
	}, nil
}

func firstPendingTaskID(status singletaskrunner.QueueStatus, completedTaskID string) any {
	for _, item := range status.Items {
		if item.ID == completedTaskID {
			continue
		}
		if item.Status == singletaskrunner.ItemStatusPending {
			return item.ID
		}
	}
	return nil
}

func countPendingTasks(status singletaskrunner.QueueStatus, failedTaskID string) int {
	count := 0
	for _, item := range status.Items {
		if item.ID == failedTaskID {
			continue
		}
		if item.Status == singletaskrunner.ItemStatusPending {
			count++
		}
	}
	return count
}

func helpTool() internalmcp.Tool {
	return internalmcp.Tool{
		Name:        "help",
		Description: "Return usage help for the single-task-runner MCP tools.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"topic": map[string]any{"type": "string", "description": "Tool name to explain. Omit for overview."},
			},
		},
		Handler: handleHelp,
	}
}

func handleHelp(_ context.Context, args map[string]any) (any, error) {
	topic, err := optionalBoundedString(args, "topic", 100)
	if err != nil {
		return nil, err
	}

	if topic == "" {
		return map[string]any{
			"overview": "single-task-runner executes queued tasks in sequence, sending each item to its configured target pane. The active task is resolved by " + singletaskrunner.ResolutionToolNames + ", and the queue updates from that result.",
			"tools": []string{
				"enqueue_task",
				"complete_task",
				"fail_task",
				"list_queue",
				"cancel_task",
				"help",
			},
			"workflow": []string{
				"1. enqueue_task to add tasks for one pane.",
				"2. Start the queue from the UI or Wails API.",
				"3. When a task is sent to the pane, call " + singletaskrunner.ResolutionToolNames + " with the provided task_id as needed.",
			},
		}, nil
	}

	switch topic {
	case "enqueue_task":
		return map[string]any{
			"topic": "enqueue_task",
			"help":  "Adds 1-20 tasks for a target pane. Each task accepts message, optional title, clear_before, and clear_command.",
		}, nil
	case "complete_task":
		return map[string]any{
			"topic": "complete_task",
			"help":  "Marks the active task as done and allows the next pending task to be sent.",
		}, nil
	case "fail_task":
		return map[string]any{
			"topic": "fail_task",
			"help":  "Marks the active task as failed and stops the queue. Remaining pending tasks stay queued.",
		}, nil
	case "list_queue":
		return map[string]any{
			"topic": "list_queue",
			"help":  "Returns the full queue snapshot, including current queue state and all items.",
		}, nil
	case "cancel_task":
		return map[string]any{
			"topic": "cancel_task",
			"help":  "Cancels a pending or active task. Pending tasks stay visible as cancelled history entries, and cancelling an active task moves the queue to the next pending task.",
		}, nil
	case "help":
		return map[string]any{
			"topic": "help",
			"help":  "Returns an overview or a tool-specific help entry.",
		}, nil
	default:
		return nil, fmt.Errorf("unknown help topic %q", topic)
	}
}
