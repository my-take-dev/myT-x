package usecase

import (
	"testing"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// TestAuthorizeAssigneeCaller_TrustedCallerBypassesAssigneeCheck_ByDesign
// locks in the design decision that trusted callers (pipe bridge) may act as
// ANY assignee, regardless of task.AssigneePaneID / task.AgentName.
//
// This is required for pipe bridge environments where TMUX_PANE is
// unresolvable — see agent-orchestrator/CLAUDE.md §認可モデル and the
// project-root ACCEPTED_DESIGN_DECISIONS.md entry AD-001.
//
// Reviewers: if this test fails after a change intended to tighten
// trusted-caller authorization, that change MUST update AD-001 and the
// §認可モデル section of agent-orchestrator/CLAUDE.md in the same PR.
func TestAuthorizeAssigneeCaller_TrustedCallerBypassesAssigneeCheck_ByDesign(t *testing.T) {
	trusted := domain.Agent{Name: trustedCallerName}

	cases := []struct {
		name string
		task domain.Task
	}{
		{
			name: "assignee pane and agent name both mismatch",
			task: domain.Task{
				AssigneePaneID: "%999",
				AgentName:      "some_other_agent",
			},
		},
		{
			name: "empty assignee pane",
			task: domain.Task{
				AssigneePaneID: "",
				AgentName:      "any_agent",
			},
		},
		{
			name: "empty agent name",
			task: domain.Task{
				AssigneePaneID: "%1",
				AgentName:      "",
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			mode, allowed := authorizeAssigneeCaller(tt.task, trusted)
			if !allowed {
				t.Fatalf("trusted caller MUST be allowed (AD-001); got mode=%q allowed=%v", mode, allowed)
			}
			if mode != "trusted" {
				t.Fatalf("expected mode=%q, got %q — the trusted early return may have moved or been removed", "trusted", mode)
			}
		})
	}
}

// TestAuthorizeAssigneeCaller_NonTrustedCallerRequiresMatch validates the
// non-trusted authorization path as the counterpart to the trusted bypass
// test above. If this test fails while the _ByDesign test passes, the
// trusted short-circuit may have been inverted or made unconditional.
func TestAuthorizeAssigneeCaller_NonTrustedCallerRequiresMatch(t *testing.T) {
	task := domain.Task{
		AssigneePaneID: "%1",
		AgentName:      "alice",
	}

	cases := []struct {
		name      string
		caller    domain.Agent
		wantMode  string
		wantAllow bool
	}{
		{
			name:      "pane match",
			caller:    domain.Agent{Name: "bob", PaneID: "%1"},
			wantMode:  "pane",
			wantAllow: true,
		},
		{
			name:      "agent name match",
			caller:    domain.Agent{Name: "alice", PaneID: "%2"},
			wantMode:  "agent_name",
			wantAllow: true,
		},
		{
			name:      "no match denied",
			caller:    domain.Agent{Name: "bob", PaneID: "%2"},
			wantMode:  "denied",
			wantAllow: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			mode, allowed := authorizeAssigneeCaller(task, tt.caller)
			if allowed != tt.wantAllow {
				t.Fatalf("allowed=%v want=%v (mode=%q)", allowed, tt.wantAllow, mode)
			}
			if mode != tt.wantMode {
				t.Fatalf("mode=%q want=%q", mode, tt.wantMode)
			}
		})
	}
}
