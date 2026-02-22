package tmux

import (
	"runtime"
	"sync"
	"testing"

	"myT-x/internal/ipc"
)

func TestUpdatePaneEnvBasic(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{})

	// Initially nil/empty.
	if got := router.getPaneEnv(); got != nil {
		t.Fatalf("getPaneEnv() before update = %v, want nil", got)
	}

	// Update with new values.
	router.UpdatePaneEnv(map[string]string{"KEY1": "val1", "KEY2": "val2"})

	got := router.getPaneEnv()
	if len(got) != 2 {
		t.Fatalf("getPaneEnv() len = %d, want 2", len(got))
	}
	if got["KEY1"] != "val1" {
		t.Errorf("getPaneEnv()[KEY1] = %q, want %q", got["KEY1"], "val1")
	}
	if got["KEY2"] != "val2" {
		t.Errorf("getPaneEnv()[KEY2] = %q, want %q", got["KEY2"], "val2")
	}
}

func TestUpdatePaneEnvOverwrite(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{
		PaneEnv: map[string]string{"OLD": "old"},
	})

	router.UpdatePaneEnv(map[string]string{"NEW": "new"})

	got := router.getPaneEnv()
	if _, exists := got["OLD"]; exists {
		t.Error("getPaneEnv() still contains OLD key after overwrite")
	}
	if got["NEW"] != "new" {
		t.Errorf("getPaneEnv()[NEW] = %q, want %q", got["NEW"], "new")
	}
}

func TestUpdatePaneEnvDeepCopy(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{})

	input := map[string]string{"A": "1"}
	router.UpdatePaneEnv(input)

	// Mutate the input map after update.
	input["A"] = "mutated"
	input["B"] = "extra"

	got := router.getPaneEnv()
	if got["A"] != "1" {
		t.Errorf("getPaneEnv()[A] = %q, want %q (input mutation leaked)", got["A"], "1")
	}
	if _, exists := got["B"]; exists {
		t.Error("getPaneEnv() contains B key (input mutation leaked)")
	}
}

func TestGetPaneEnvReturnsIndependentCopy(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{
		PaneEnv: map[string]string{"X": "1"},
	})

	copy1 := router.getPaneEnv()
	copy1["X"] = "mutated"

	copy2 := router.getPaneEnv()
	if copy2["X"] != "1" {
		t.Errorf("getPaneEnv() returned shared reference; copy2[X] = %q, want %q", copy2["X"], "1")
	}
}

// TestPaneEnvSnapshotIndependence verifies that PaneEnvSnapshot returns an
// independent deep copy. Mutating the returned snapshot must not affect the
// router's internal state, symmetric to TestClaudeEnvSnapshot in command_router_env_test.go.
func TestPaneEnvSnapshotIndependence(t *testing.T) {
	tests := []struct {
		name         string
		initialEnv   map[string]string
		wantSnapshot map[string]string
		wantNil      bool
	}{
		{
			name:       "nil env returns nil snapshot",
			initialEnv: nil,
			wantNil:    true,
		},
		{
			name:       "empty map returns nil snapshot (PaneEnv normalizes empty to nil, unlike ClaudeEnv)",
			initialEnv: map[string]string{},
			wantNil:    true,
		},
		{
			name:         "single entry snapshot is independent",
			initialEnv:   map[string]string{"KEY": "value"},
			wantSnapshot: map[string]string{"KEY": "value"},
		},
		{
			name: "multiple entries snapshot is independent",
			initialEnv: map[string]string{
				"KEY1": "v1",
				"KEY2": "v2",
			},
			wantSnapshot: map[string]string{
				"KEY1": "v1",
				"KEY2": "v2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewCommandRouter(nil, nil, RouterOptions{PaneEnv: tt.initialEnv})

			snapshot := router.PaneEnvSnapshot()

			if tt.wantNil {
				if snapshot != nil {
					t.Fatalf("PaneEnvSnapshot() = %v, want nil", snapshot)
				}
				return
			}
			if snapshot == nil {
				t.Fatal("PaneEnvSnapshot() = nil, want non-nil")
			}
			if len(snapshot) != len(tt.wantSnapshot) {
				t.Fatalf("PaneEnvSnapshot() len = %d, want %d", len(snapshot), len(tt.wantSnapshot))
			}
			for k, want := range tt.wantSnapshot {
				if got := snapshot[k]; got != want {
					t.Errorf("PaneEnvSnapshot()[%q] = %q, want %q", k, got, want)
				}
			}

			// Mutate the snapshot and verify the router's internal state is unaffected.
			snapshot["KEY"] = "mutated"
			snapshot["INJECTED"] = "injected"

			fresh := router.PaneEnvSnapshot()
			if fresh == nil {
				t.Fatal("PaneEnvSnapshot() after mutation = nil, want non-nil")
			}
			for k, want := range tt.wantSnapshot {
				if got := fresh[k]; got != want {
					t.Errorf("after snapshot mutation, PaneEnvSnapshot()[%q] = %q, want %q (unchanged)", k, got, want)
				}
			}
			if _, injected := fresh["INJECTED"]; injected {
				t.Error("PaneEnvSnapshot() contains INJECTED key after external mutation; deep copy failed")
			}
		})
	}
}

// TestUpdatePaneEnvSnapshotReturnIndependence verifies that after calling
// UpdatePaneEnv, the PaneEnvSnapshot returned value is independent from the
// router's internal state. This is the write-then-read independence test,
// symmetric to TestUpdateClaudeEnv in command_router_env_test.go.
func TestUpdatePaneEnvSnapshotReturnIndependence(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{})

	input := map[string]string{"A": "1", "B": "2"}
	router.UpdatePaneEnv(input)

	// Verify snapshot matches the update.
	snapshot := router.PaneEnvSnapshot()
	if snapshot["A"] != "1" || snapshot["B"] != "2" {
		t.Fatalf("PaneEnvSnapshot() = %v, want {A:1 B:2}", snapshot)
	}

	// Mutate both input and snapshot after the fact.
	input["A"] = "input_mutated"
	input["C"] = "input_added"
	snapshot["B"] = "snapshot_mutated"
	snapshot["D"] = "snapshot_added"

	// Internal state must be unaffected by both mutations.
	fresh := router.PaneEnvSnapshot()
	if fresh["A"] != "1" {
		t.Errorf("PaneEnvSnapshot()[A] = %q after input mutation, want %q", fresh["A"], "1")
	}
	if fresh["B"] != "2" {
		t.Errorf("PaneEnvSnapshot()[B] = %q after snapshot mutation, want %q", fresh["B"], "2")
	}
	if _, ok := fresh["C"]; ok {
		t.Error("PaneEnvSnapshot() contains C (leaked from input mutation)")
	}
	if _, ok := fresh["D"]; ok {
		t.Error("PaneEnvSnapshot() contains D (leaked from snapshot mutation)")
	}
}

func TestUpdatePaneEnvConcurrent(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{})

	const goroutines = 20
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Writers.
	for i := range goroutines {
		go func(id int) {
			defer wg.Done()
			for range iterations {
				router.UpdatePaneEnv(map[string]string{
					"KEY": "value",
				})
			}
		}(i)
	}

	// Readers.
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				_ = router.getPaneEnv()
			}
		}()
	}

	wg.Wait()
}

func TestUpdatePaneEnvEmpty(t *testing.T) {
	router := NewCommandRouter(nil, nil, RouterOptions{
		PaneEnv: map[string]string{"A": "1"},
	})

	// Update with empty map clears PaneEnv.
	router.UpdatePaneEnv(map[string]string{})

	got := router.getPaneEnv()
	if got != nil {
		t.Errorf("getPaneEnv() after empty update = %v, want nil", got)
	}
}

func TestBuildPaneEnvSkipDefaults(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	router := NewCommandRouter(sessions, &captureEmitter{}, RouterOptions{
		PipeName:      `\\.\pipe\test-skip`,
		HostPID:       12345,
		ShimAvailable: true,
		PaneEnv: map[string]string{
			"CLAUDE_CODE_EFFORT_LEVEL": "high",
			"CUSTOM_VAR":               "custom-value",
		},
	})

	env := router.buildPaneEnvSkipDefaults(map[string]string{"REQ_KEY": "req-val"}, 0, 0)

	// Request key is preserved.
	if env["REQ_KEY"] != "req-val" {
		t.Errorf("REQ_KEY = %q, want %q", env["REQ_KEY"], "req-val")
	}
	// Tmux environment variables are still set.
	if env["TMUX"] == "" {
		t.Error("TMUX not set; addTmuxEnvironment should still run")
	}
	// PaneEnv defaults must NOT be applied.
	if _, ok := env["CLAUDE_CODE_EFFORT_LEVEL"]; ok {
		t.Error("CLAUDE_CODE_EFFORT_LEVEL should not be set when skipping defaults")
	}
	if _, ok := env["CUSTOM_VAR"]; ok {
		t.Error("CUSTOM_VAR should not be set when skipping defaults")
	}
}

func TestBuildPaneEnvWithDefaults(t *testing.T) {
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	router := NewCommandRouter(sessions, &captureEmitter{}, RouterOptions{
		PipeName:      `\\.\pipe\test-with`,
		HostPID:       12345,
		ShimAvailable: true,
		PaneEnv: map[string]string{
			"CLAUDE_CODE_EFFORT_LEVEL": "high",
			"CUSTOM_VAR":               "custom-value",
		},
	})

	env := router.buildPaneEnv(map[string]string{"REQ_KEY": "req-val"}, 0, 0)

	// Request key is preserved.
	if env["REQ_KEY"] != "req-val" {
		t.Errorf("REQ_KEY = %q, want %q", env["REQ_KEY"], "req-val")
	}
	// PaneEnv defaults ARE applied.
	if env["CLAUDE_CODE_EFFORT_LEVEL"] != "high" {
		t.Errorf("CLAUDE_CODE_EFFORT_LEVEL = %q, want %q", env["CLAUDE_CODE_EFFORT_LEVEL"], "high")
	}
	if env["CUSTOM_VAR"] != "custom-value" {
		t.Errorf("CUSTOM_VAR = %q, want %q", env["CUSTOM_VAR"], "custom-value")
	}
}

func TestNewSessionOperatorSkipsPaneEnvDefaults(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only: requires terminal")
	}
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	emitter := &captureEmitter{}
	router := NewCommandRouter(sessions, emitter, RouterOptions{
		DefaultShell:  "powershell.exe",
		PipeName:      `\\.\pipe\test-op-skip`,
		HostPID:       12345,
		ShimAvailable: true,
		PaneEnv: map[string]string{
			"CLAUDE_CODE_EFFORT_LEVEL": "high",
		},
	})

	// Operator-initiated session: no CLAUDE_CODE_AGENT_TYPE in env.
	resp := router.Execute(ipc.TmuxRequest{
		Command: "new-session",
		Flags: map[string]any{
			"-s": "operator-session",
			"-d": true,
		},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("new-session failed: %s", resp.Stderr)
	}

	// Verify pane env does NOT have CLAUDE_CODE_EFFORT_LEVEL.
	// pane ID 0 is the initial pane created by new-session.
	paneCtx, err := sessions.GetPaneContextSnapshot(0)
	if err != nil {
		t.Fatalf("GetPaneContextSnapshot() error = %v", err)
	}
	if _, ok := paneCtx.Env["CLAUDE_CODE_EFFORT_LEVEL"]; ok {
		t.Error("operator-initiated pane should NOT have CLAUDE_CODE_EFFORT_LEVEL")
	}
}

func TestNewSessionAgentInitialPaneAlsoSkipsPaneEnvDefaults(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows only: requires terminal")
	}
	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	emitter := &captureEmitter{}
	router := NewCommandRouter(sessions, emitter, RouterOptions{
		DefaultShell:  "powershell.exe",
		PipeName:      `\\.\pipe\test-agent-env`,
		HostPID:       12345,
		ShimAvailable: true,
		PaneEnv: map[string]string{
			"CLAUDE_CODE_EFFORT_LEVEL": "high",
		},
	})

	// Agent-initiated session: CLAUDE_CODE_AGENT_TYPE present in env.
	// Even for agent sessions, the initial pane should NOT get pane_env defaults.
	// pane_env is intended for additional panes only (split-window, new-window).
	resp := router.Execute(ipc.TmuxRequest{
		Command: "new-session",
		Flags: map[string]any{
			"-s": "agent-session",
			"-d": true,
		},
		Env: map[string]string{
			"CLAUDE_CODE_AGENT_TYPE": "lead",
		},
	})
	if resp.ExitCode != 0 {
		t.Fatalf("new-session failed: %s", resp.Stderr)
	}

	// Verify pane env does NOT have CLAUDE_CODE_EFFORT_LEVEL.
	// pane ID 0 is the initial pane created by new-session.
	paneCtx, err := sessions.GetPaneContextSnapshot(0)
	if err != nil {
		t.Fatalf("GetPaneContextSnapshot() error = %v", err)
	}
	if _, ok := paneCtx.Env["CLAUDE_CODE_EFFORT_LEVEL"]; ok {
		t.Error("initial pane of agent session should NOT have CLAUDE_CODE_EFFORT_LEVEL")
	}
}
