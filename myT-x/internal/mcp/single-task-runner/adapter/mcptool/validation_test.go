package mcptool

import "testing"

func TestRequiredEnqueueTasksDefaultsTitleFromMessage(t *testing.T) {
	t.Parallel()

	tasks, err := requiredEnqueueTasks(map[string]any{
		"tasks": []any{
			map[string]any{
				"message":      "run diagnostics",
				"clear_before": true,
			},
		},
	}, "tasks", 20)
	if err != nil {
		t.Fatalf("requiredEnqueueTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("len(tasks) = %d, want 1", len(tasks))
	}
	if tasks[0].Title != "run diagnostics" {
		t.Fatalf("Title = %q, want message fallback", tasks[0].Title)
	}
	if !tasks[0].ClearBefore {
		t.Fatal("expected ClearBefore to be true")
	}
}

func TestRequiredEnqueueTasksRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	if _, err := requiredEnqueueTasks(map[string]any{
		"tasks": []any{},
	}, "tasks", 20); err == nil {
		t.Fatal("expected error for empty task list")
	}

	if _, err := requiredEnqueueTasks(map[string]any{
		"tasks": []any{
			map[string]any{
				"message": 123,
			},
		},
	}, "tasks", 20); err == nil {
		t.Fatal("expected error for non-string message")
	}

	tooMany := make([]any, 21)
	for i := range tooMany {
		tooMany[i] = map[string]any{"message": "task"}
	}
	if _, err := requiredEnqueueTasks(map[string]any{
		"tasks": tooMany,
	}, "tasks", 20); err == nil {
		t.Fatal("expected error for too many tasks")
	}
}

func TestOptionalBoolRejectsInvalidType(t *testing.T) {
	t.Parallel()

	if _, err := optionalBool(map[string]any{
		"clear_before": "yes",
	}, "clear_before", false); err == nil {
		t.Fatal("expected error for non-boolean clear_before")
	}
}
