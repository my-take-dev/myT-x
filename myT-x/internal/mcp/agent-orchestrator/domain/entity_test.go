package domain

import (
	"reflect"
	"testing"
)

func TestIsVirtualPaneID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		paneID string
		want   bool
	}{
		{"virtual task-master", "%virtual-task-master", true},
		{"virtual other", "%virtual-scheduler", true},
		{"prefix only", "%virtual-", true},
		{"real pane", "%0", false},
		{"real pane multi-digit", "%123", false},
		{"empty string", "", false},
		{"similar but no prefix", "virtual-task-master", false},
		{"percent but not virtual", "%not-virtual", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsVirtualPaneID(tt.paneID)
			if got != tt.want {
				t.Errorf("IsVirtualPaneID(%q) = %v, want %v", tt.paneID, got, tt.want)
			}
		})
	}
}

func TestVirtualPaneIDPrefix(t *testing.T) {
	t.Parallel()
	if VirtualPaneIDPrefix != "%virtual-" {
		t.Errorf("VirtualPaneIDPrefix = %q, want %q", VirtualPaneIDPrefix, "%virtual-")
	}
}

func TestValidatePaneID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		paneID  string
		wantErr bool
	}{
		{"valid real pane", "%0", false},
		{"valid multi-digit", "%123", false},
		{"virtual task-master", "%virtual-task-master", false},
		{"virtual scheduler", "%virtual-scheduler", false},
		{"empty", "", true},
		{"no percent prefix", "0", true},
		{"invalid format", "%abc", true},
		{"similar but not virtual", "%not-virtual", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidatePaneID(tt.paneID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePaneID(%q) error = %v, wantErr %v", tt.paneID, err, tt.wantErr)
			}
		})
	}
}

func TestEntityFieldCounts(t *testing.T) {
	if got := reflect.TypeFor[AgentStatus]().NumField(); got != 5 {
		t.Fatalf("AgentStatus field count = %d, want 5; update this test when AgentStatus changes", got)
	}
	if got := reflect.TypeFor[TaskGroup]().NumField(); got != 3 {
		t.Fatalf("TaskGroup field count = %d, want 3; update this test when TaskGroup changes", got)
	}
	if got := reflect.TypeFor[Task]().NumField(); got != 20 {
		t.Fatalf("Task field count = %d, want 20; update this test when Task changes", got)
	}
}

func TestTaskStatusIsTerminal(t *testing.T) {
	tests := []struct {
		name   string
		status TaskStatus
		want   bool
	}{
		{name: "pending", status: TaskStatusPending, want: false},
		{name: "blocked", status: TaskStatusBlocked, want: false},
		{name: "completed", status: TaskStatusCompleted, want: true},
		{name: "failed", status: TaskStatusFailed, want: true},
		{name: "abandoned", status: TaskStatusAbandoned, want: true},
		{name: "cancelled", status: TaskStatusCancelled, want: true},
		{name: "expired", status: TaskStatusExpired, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.want {
				t.Fatalf("IsTerminal(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestIsValidTaskStatusFilter(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "all", value: "all", want: true},
		{name: "pending", value: "pending", want: true},
		{name: "blocked", value: "blocked", want: true},
		{name: "completed", value: "completed", want: true},
		{name: "failed", value: "failed", want: true},
		{name: "abandoned", value: "abandoned", want: true},
		{name: "cancelled", value: "cancelled", want: true},
		{name: "expired", value: "expired", want: true},
		{name: "invalid", value: "offline", want: false},
		{name: "empty", value: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidTaskStatusFilter(tt.value); got != tt.want {
				t.Fatalf("IsValidTaskStatusFilter(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestTaskStatusFilterMatchesTaskStatus(t *testing.T) {
	tests := []struct {
		name   string
		filter TaskStatusFilter
		status TaskStatus
		want   bool
	}{
		{name: "empty matches all", filter: "", status: TaskStatusPending, want: true},
		{name: "all matches all", filter: TaskStatusFilterAll, status: TaskStatusFailed, want: true},
		{name: "pending matches pending", filter: TaskStatusFilterPending, status: TaskStatusPending, want: true},
		{name: "pending rejects completed", filter: TaskStatusFilterPending, status: TaskStatusCompleted, want: false},
		{name: "blocked matches blocked", filter: TaskStatusFilterBlocked, status: TaskStatusBlocked, want: true},
		{name: "completed matches completed", filter: TaskStatusFilterCompleted, status: TaskStatusCompleted, want: true},
		{name: "failed matches failed", filter: TaskStatusFilterFailed, status: TaskStatusFailed, want: true},
		{name: "abandoned matches abandoned", filter: TaskStatusFilterAbandoned, status: TaskStatusAbandoned, want: true},
		{name: "cancelled matches cancelled", filter: TaskStatusFilterCancelled, status: TaskStatusCancelled, want: true},
		{name: "expired matches expired", filter: TaskStatusFilterExpired, status: TaskStatusExpired, want: true},
		{name: "unknown filter fails closed", filter: TaskStatusFilter("unknown"), status: TaskStatusPending, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.MatchesTaskStatus(tt.status); got != tt.want {
				t.Fatalf("MatchesTaskStatus(%q, %q) = %v, want %v", tt.filter, tt.status, got, tt.want)
			}
		})
	}
}
