package scheduler

import (
	"context"
	"errors"
	"strings"
)

const (
	// InfiniteCount means the scheduler runs until manually stopped or the target pane disappears.
	InfiniteCount = 0

	// templateFileName is the JSON file name for scheduler templates within the session directory.
	templateFileName = "scheduler-templates.json"

	// templateDir is the subdirectory under a session's root path that holds myT-x metadata.
	templateDir = ".myT-x"
)

// runGeneration is a monotonically increasing token that identifies
// a particular run of a scheduler entry. Used to detect stale goroutines
// after Resume() re-launches an entry.
type runGeneration uint64

// entry is the internal state of a single scheduler instance.
// Protected by Service.mu.
type entry struct {
	ID              string
	Title           string
	PaneID          string
	Message         string
	IntervalSeconds int
	MaxCount        int // 0 = infinite (until manual stop or pane gone)
	CurrentCount    int
	Running         bool
	RunToken        runGeneration
	StopReason      string
	cancel          context.CancelFunc
}

// EntryStatus is the frontend-safe representation of a scheduler.
type EntryStatus struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	PaneID          string `json:"pane_id"`
	Message         string `json:"message"`
	IntervalSeconds int    `json:"interval_seconds"`
	MaxCount        int    `json:"max_count"`
	CurrentCount    int    `json:"current_count"`
	Running         bool   `json:"running"`
	StopReason      string `json:"stop_reason,omitempty"`
}

// Template is a reusable scheduler preset.
// PaneID is not included (specified at start time).
// Title is the unique key (same-name save overwrites).
type Template struct {
	Title           string `json:"title"`
	Message         string `json:"message"`
	IntervalSeconds int    `json:"interval_seconds"`
	MaxCount        int    `json:"max_count"`
}

// Normalize trims whitespace from mutable template fields.
// Message is NOT trimmed because leading/trailing whitespace may be
// intentional in terminal commands (e.g. indented heredoc input).
func (t *Template) Normalize() {
	if t == nil {
		return
	}
	t.Title = strings.TrimSpace(t.Title)
}

// Validate checks that required template fields satisfy business rules.
// Message is intentionally not validated (empty message is allowed for auto-enter).
func (t *Template) Validate() error {
	if t == nil {
		return errors.New("template is required")
	}
	if strings.TrimSpace(t.Title) == "" {
		return errors.New("title is required")
	}
	if t.IntervalSeconds < 10 {
		return errors.New("interval must be at least 10 seconds")
	}
	if t.MaxCount < InfiniteCount {
		return errors.New("send count must be 0 for infinite or at least 1")
	}
	return nil
}
