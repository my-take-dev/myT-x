package tmux

import (
	"reflect"
	"testing"
)

// TestUpdateClaudeEnv verifies UpdateClaudeEnv atomically replaces claudeEnv
// with a deep copy, preventing caller mutations from affecting router state.
func TestUpdateClaudeEnv(t *testing.T) {
	tests := []struct {
		name            string
		input           map[string]string
		wantSnapshot    map[string]string
		wantCallerMutex bool // whether caller can mutate without affecting router
	}{
		{
			name:            "nil input stores nil",
			input:           nil,
			wantSnapshot:    nil,
			wantCallerMutex: false,
		},
		{
			name:            "empty map stores empty",
			input:           map[string]string{},
			wantSnapshot:    map[string]string{},
			wantCallerMutex: true,
		},
		{
			name:            "single entry copied",
			input:           map[string]string{"KEY": "value"},
			wantSnapshot:    map[string]string{"KEY": "value"},
			wantCallerMutex: true,
		},
		{
			name: "multiple entries copied",
			input: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
			},
			wantSnapshot: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
			},
			wantCallerMutex: true,
		},
		{
			name:            "empty string value copied",
			input:           map[string]string{"KEY": ""},
			wantSnapshot:    map[string]string{"KEY": ""},
			wantCallerMutex: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewCommandRouter(nil, nil, RouterOptions{})

			// Update with initial input
			router.UpdateClaudeEnv(tt.input)

			// Verify snapshot matches expected
			snapshot := router.ClaudeEnvSnapshot()
			if !reflect.DeepEqual(snapshot, tt.wantSnapshot) {
				t.Errorf("ClaudeEnvSnapshot() = %v, want %v", snapshot, tt.wantSnapshot)
			}

			// Verify deep copy: caller cannot mutate router state
			if tt.wantCallerMutex && tt.input != nil {
				tt.input["KEY"] = "mutated"
				tt.input["NEWKEY"] = "added"
				newSnapshot := router.ClaudeEnvSnapshot()
				if !reflect.DeepEqual(newSnapshot, tt.wantSnapshot) {
					t.Errorf("after caller mutation, ClaudeEnvSnapshot() = %v, want %v (unchanged)", newSnapshot, tt.wantSnapshot)
				}
			}
		})
	}
}

// TestClaudeEnvSnapshot verifies ClaudeEnvSnapshot returns deep copy
// and handles nil vs empty map correctly.
func TestClaudeEnvSnapshot(t *testing.T) {
	tests := []struct {
		name            string
		initialEnv      map[string]string
		wantSnapshot    map[string]string
		wantSnapshotNil bool
	}{
		{
			name:            "nil env returns nil snapshot",
			initialEnv:      nil,
			wantSnapshot:    nil,
			wantSnapshotNil: true,
		},
		{
			name:            "empty env returns empty snapshot",
			initialEnv:      map[string]string{},
			wantSnapshot:    map[string]string{},
			wantSnapshotNil: false,
		},
		{
			name:         "single entry snapshot",
			initialEnv:   map[string]string{"KEY": "value"},
			wantSnapshot: map[string]string{"KEY": "value"},
		},
		{
			name: "multiple entries snapshot",
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
			router := NewCommandRouter(nil, nil, RouterOptions{ClaudeEnv: tt.initialEnv})

			snapshot := router.ClaudeEnvSnapshot()

			if tt.wantSnapshotNil && snapshot != nil {
				t.Errorf("ClaudeEnvSnapshot() = %v, want nil", snapshot)
				return
			}
			if !tt.wantSnapshotNil && snapshot == nil {
				t.Errorf("ClaudeEnvSnapshot() = nil, want non-nil")
				return
			}

			if !reflect.DeepEqual(snapshot, tt.wantSnapshot) {
				t.Errorf("ClaudeEnvSnapshot() = %v, want %v", snapshot, tt.wantSnapshot)
			}

			// Verify deep copy: mutating snapshot doesn't affect router
			if snapshot != nil {
				snapshot["KEY"] = "mutated"
				snapshot["NEWKEY"] = "added"
				newSnapshot := router.ClaudeEnvSnapshot()
				if !reflect.DeepEqual(newSnapshot, tt.wantSnapshot) {
					t.Errorf("after snapshot mutation, ClaudeEnvSnapshot() = %v, want %v (unchanged)", newSnapshot, tt.wantSnapshot)
				}
			}
		})
	}
}

// TestSetUseClaudeEnv verifies SetUseClaudeEnv stores boolean flag correctly
// and marks state mutations.
func TestSetUseClaudeEnv(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		enabled     bool
		wantErr     bool
	}{
		{
			name:        "enable on new session",
			sessionName: "test-session",
			enabled:     true,
			wantErr:     true, // session doesn't exist yet
		},
		{
			name:        "disable on new session",
			sessionName: "test-session",
			enabled:     false,
			wantErr:     true,
		},
		{
			name:        "empty session name",
			sessionName: "",
			enabled:     true,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()

			// Pre-create session if test expects success
			if !tt.wantErr {
				sessions.CreateSession("test-session", "", 0, 0)
			}

			err := sessions.SetUseClaudeEnv(tt.sessionName, tt.enabled)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetUseClaudeEnv() error = %v, wantErr = %v", err, tt.wantErr)
			} else if tt.wantErr && err == nil {
				t.Error("SetUseClaudeEnv() expected error for non-existent session, got nil")
			}

			if !tt.wantErr {
				// Verify flag was set
				snap, _ := sessions.GetSession("test-session")
				if snap.UseClaudeEnv == nil || *snap.UseClaudeEnv != tt.enabled {
					t.Errorf("UseClaudeEnv = %v, want %v", snap.UseClaudeEnv, tt.enabled)
				}
			}
		})
	}

	t.Run("transition: nil to true", func(t *testing.T) {
		sessions := NewSessionManager()
		sessions.CreateSession("sess", "", 0, 0)

		snap, _ := sessions.GetSession("sess")
		if snap.UseClaudeEnv != nil {
			t.Errorf("initial UseClaudeEnv = %v, want nil", snap.UseClaudeEnv)
		}

		sessions.SetUseClaudeEnv("sess", true)
		snap, _ = sessions.GetSession("sess")
		if snap.UseClaudeEnv == nil || !*snap.UseClaudeEnv {
			t.Errorf("after set true: UseClaudeEnv = %v, want true", snap.UseClaudeEnv)
		}
	})

	t.Run("transition: nil to false", func(t *testing.T) {
		sessions := NewSessionManager()
		sessions.CreateSession("sess", "", 0, 0)

		snap, _ := sessions.GetSession("sess")
		if snap.UseClaudeEnv != nil {
			t.Errorf("initial UseClaudeEnv = %v, want nil", snap.UseClaudeEnv)
		}

		sessions.SetUseClaudeEnv("sess", false)
		snap, _ = sessions.GetSession("sess")
		if snap.UseClaudeEnv == nil {
			t.Fatal("after set false: UseClaudeEnv = nil, want *false")
		}
		if *snap.UseClaudeEnv {
			t.Errorf("after set false: UseClaudeEnv = %v, want false", *snap.UseClaudeEnv)
		}
	})

	t.Run("transition: true to false", func(t *testing.T) {
		sessions := NewSessionManager()
		sessions.CreateSession("sess", "", 0, 0)
		sessions.SetUseClaudeEnv("sess", true)

		sessions.SetUseClaudeEnv("sess", false)
		snap, _ := sessions.GetSession("sess")
		if snap.UseClaudeEnv == nil || *snap.UseClaudeEnv {
			t.Errorf("after set false: UseClaudeEnv = %v, want false", snap.UseClaudeEnv)
		}
	})

	t.Run("idempotent: set same value twice", func(t *testing.T) {
		sessions := NewSessionManager()
		sessions.CreateSession("sess", "", 0, 0)

		err1 := sessions.SetUseClaudeEnv("sess", true)
		err2 := sessions.SetUseClaudeEnv("sess", true)

		if err1 != nil || err2 != nil {
			t.Errorf("SetUseClaudeEnv() errors: %v, %v", err1, err2)
		}

		snap, _ := sessions.GetSession("sess")
		if snap.UseClaudeEnv == nil || !*snap.UseClaudeEnv {
			t.Errorf("UseClaudeEnv = %v, want true", snap.UseClaudeEnv)
		}
	})
}

// TestSetUsePaneEnv verifies SetUsePaneEnv stores boolean flag correctly.
func TestSetUsePaneEnv(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		enabled     bool
		wantErr     bool
	}{
		{
			name:        "enable on new session",
			sessionName: "test-session",
			enabled:     true,
			wantErr:     true,
		},
		{
			name:        "disable on new session",
			sessionName: "test-session",
			enabled:     false,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()

			err := sessions.SetUsePaneEnv(tt.sessionName, tt.enabled)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetUsePaneEnv() error = %v, wantErr = %v", err, tt.wantErr)
			} else if tt.wantErr && err == nil {
				t.Error("SetUsePaneEnv() expected error for non-existent session, got nil")
			}
		})
	}

	t.Run("transition: nil to true", func(t *testing.T) {
		sessions := NewSessionManager()
		sessions.CreateSession("sess", "", 0, 0)

		snap, _ := sessions.GetSession("sess")
		if snap.UsePaneEnv != nil {
			t.Errorf("initial UsePaneEnv = %v, want nil", snap.UsePaneEnv)
		}

		sessions.SetUsePaneEnv("sess", true)
		snap, _ = sessions.GetSession("sess")
		if snap.UsePaneEnv == nil || !*snap.UsePaneEnv {
			t.Errorf("after set true: UsePaneEnv = %v, want true", snap.UsePaneEnv)
		}
	})

	t.Run("transition: nil to false", func(t *testing.T) {
		sessions := NewSessionManager()
		sessions.CreateSession("sess", "", 0, 0)

		snap, _ := sessions.GetSession("sess")
		if snap.UsePaneEnv != nil {
			t.Errorf("initial UsePaneEnv = %v, want nil", snap.UsePaneEnv)
		}

		sessions.SetUsePaneEnv("sess", false)
		snap, _ = sessions.GetSession("sess")
		if snap.UsePaneEnv == nil {
			t.Fatal("after set false: UsePaneEnv = nil, want *false")
		}
		if *snap.UsePaneEnv {
			t.Errorf("after set false: UsePaneEnv = %v, want false", *snap.UsePaneEnv)
		}
	})

	t.Run("transition: false to true", func(t *testing.T) {
		sessions := NewSessionManager()
		sessions.CreateSession("sess", "", 0, 0)
		sessions.SetUsePaneEnv("sess", false)

		sessions.SetUsePaneEnv("sess", true)
		snap, _ := sessions.GetSession("sess")
		if snap.UsePaneEnv == nil || !*snap.UsePaneEnv {
			t.Errorf("after transition: UsePaneEnv = %v, want true", snap.UsePaneEnv)
		}
	})
}

// TestCopyBoolPtr verifies copyBoolPtr handles nil, true, false correctly
// and ensures pointer independence.
func TestCopyBoolPtr(t *testing.T) {
	tests := []struct {
		name      string
		input     *bool
		wantNil   bool
		wantValue bool
	}{
		{
			name:    "nil pointer returns nil",
			input:   nil,
			wantNil: true,
		},
		{
			name:      "true pointer copied",
			input:     new(true),
			wantNil:   false,
			wantValue: true,
		},
		{
			name:      "false pointer copied",
			input:     new(false),
			wantNil:   false,
			wantValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := copyBoolPtr(tt.input)

			if tt.wantNil && got != nil {
				t.Errorf("copyBoolPtr() = %v, want nil", got)
				return
			}
			if !tt.wantNil && got == nil {
				t.Errorf("copyBoolPtr() = nil, want non-nil")
				return
			}

			if !tt.wantNil && *got != tt.wantValue {
				t.Errorf("*copyBoolPtr() = %v, want %v", *got, tt.wantValue)
			}

			// Verify pointer independence: mutating source doesn't affect copy
			if tt.input != nil && got != nil {
				*tt.input = !*tt.input
				if *got == *tt.input {
					t.Errorf("copy was mutated when source changed: copy = %v, src = %v", *got, *tt.input)
				}
			}
		})
	}
}

// TestBuildPaneEnvForSession tests the 5-layer environment merge priority.
// Layer 1: claude_env (when useClaudeEnv=true)
// Layer 2: inheritedEnv (source pane env)
// Layer 3: pane_env (when usePaneEnv=true)
// Layer 4: shimEnv (shim -e flag, highest custom priority)
// Layer 5: tmux internal vars (always final)
func TestBuildPaneEnvForSession(t *testing.T) {
	tests := []struct {
		name         string
		claudeEnv    map[string]string
		paneEnv      map[string]string
		inheritedEnv map[string]string
		shimEnv      map[string]string
		useClaudeEnv bool
		usePaneEnv   bool
		sessionID    int
		paneID       int
		verify       func(t *testing.T, env map[string]string)
	}{
		{
			name:         "layer 1 only: claude_env fills base",
			claudeEnv:    map[string]string{"CLAUDE_KEY": "claude_value"},
			paneEnv:      map[string]string{},
			inheritedEnv: map[string]string{},
			shimEnv:      map[string]string{},
			useClaudeEnv: true,
			usePaneEnv:   false,
			sessionID:    1,
			paneID:       1,
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				if env["CLAUDE_KEY"] != "claude_value" {
					t.Errorf("env[\"CLAUDE_KEY\"] = %q, want claude_value", env["CLAUDE_KEY"])
				}
			},
		},
		{
			name:         "layer 1 disabled: no claude_env",
			claudeEnv:    map[string]string{"CLAUDE_KEY": "claude_value"},
			paneEnv:      map[string]string{},
			inheritedEnv: map[string]string{"INHERITED_KEY": "inherited_value"},
			shimEnv:      map[string]string{},
			useClaudeEnv: false,
			usePaneEnv:   false,
			sessionID:    1,
			paneID:       1,
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				if _, ok := env["CLAUDE_KEY"]; ok {
					t.Errorf("env[\"CLAUDE_KEY\"] should not be present when useClaudeEnv=false")
				}
				if env["INHERITED_KEY"] != "inherited_value" {
					t.Errorf("env[\"INHERITED_KEY\"] = %q, want inherited_value", env["INHERITED_KEY"])
				}
			},
		},
		{
			name:         "layer 2 overwrites layer 1",
			claudeEnv:    map[string]string{"KEY": "claude_value"},
			paneEnv:      map[string]string{},
			inheritedEnv: map[string]string{"KEY": "inherited_value"},
			shimEnv:      map[string]string{},
			useClaudeEnv: true,
			usePaneEnv:   false,
			sessionID:    1,
			paneID:       1,
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				if env["KEY"] != "inherited_value" {
					t.Errorf("env[\"KEY\"] = %q, want inherited_value (layer 2 overwrite)", env["KEY"])
				}
			},
		},
		{
			name:         "layer 3 overwrite when both useClaudeEnv and usePaneEnv",
			claudeEnv:    map[string]string{"KEY": "claude"},
			paneEnv:      map[string]string{"KEY": "pane", "PANE_ONLY": "pane_only_value"},
			inheritedEnv: map[string]string{},
			shimEnv:      map[string]string{},
			useClaudeEnv: true,
			usePaneEnv:   true,
			sessionID:    1,
			paneID:       1,
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				if env["KEY"] != "pane" {
					t.Errorf("env[\"KEY\"] = %q, want pane (layer 3 overwrite)", env["KEY"])
				}
				if env["PANE_ONLY"] != "pane_only_value" {
					t.Errorf("env[\"PANE_ONLY\"] = %q, want pane_only_value", env["PANE_ONLY"])
				}
			},
		},
		{
			name:         "layer 3 fill-only when usePaneEnv but not useClaudeEnv",
			claudeEnv:    map[string]string{"CLAUDE_KEY": "claude"},
			paneEnv:      map[string]string{"PANE_KEY": "pane", "KEY": "pane_value"},
			inheritedEnv: map[string]string{"KEY": "inherited"},
			shimEnv:      map[string]string{},
			useClaudeEnv: false,
			usePaneEnv:   true,
			sessionID:    1,
			paneID:       1,
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				// pane_env fill-only: doesn't overwrite inherited KEY
				if env["KEY"] != "inherited" {
					t.Errorf("env[\"KEY\"] = %q, want inherited (fill-only preserves)", env["KEY"])
				}
				// pane_env adds new PANE_KEY
				if env["PANE_KEY"] != "pane" {
					t.Errorf("env[\"PANE_KEY\"] = %q, want pane", env["PANE_KEY"])
				}
				// claude_env not applied when useClaudeEnv=false
				if _, ok := env["CLAUDE_KEY"]; ok {
					t.Errorf("env[\"CLAUDE_KEY\"] should not be present when useClaudeEnv=false")
				}
			},
		},
		{
			name:         "layer 4 shim overwrites all custom layers",
			claudeEnv:    map[string]string{"KEY": "claude"},
			paneEnv:      map[string]string{"KEY": "pane"},
			inheritedEnv: map[string]string{"KEY": "inherited"},
			shimEnv:      map[string]string{"KEY": "shim"},
			useClaudeEnv: true,
			usePaneEnv:   true,
			sessionID:    1,
			paneID:       1,
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				if env["KEY"] != "shim" {
					t.Errorf("env[\"KEY\"] = %q, want shim (layer 4 overwrite)", env["KEY"])
				}
			},
		},
		{
			name:         "layer 5 tmux internal vars final",
			claudeEnv:    map[string]string{"TMUX_PANE": "claude_pane"},
			paneEnv:      map[string]string{},
			inheritedEnv: map[string]string{},
			shimEnv:      map[string]string{},
			useClaudeEnv: true,
			usePaneEnv:   false,
			sessionID:    5,
			paneID:       10,
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				// tmux internal should override claude_env
				if env["TMUX_PANE"] != "%10" {
					t.Errorf("env[\"TMUX_PANE\"] = %q, want %%10 (layer 5 final)", env["TMUX_PANE"])
				}
			},
		},
		{
			name:         "empty maps: no env vars except tmux internal",
			claudeEnv:    map[string]string{},
			paneEnv:      map[string]string{},
			inheritedEnv: map[string]string{},
			shimEnv:      map[string]string{},
			useClaudeEnv: false,
			usePaneEnv:   false,
			sessionID:    1,
			paneID:       1,
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				// Only tmux internal vars should be present
				if _, ok := env["TMUX_PANE"]; !ok {
					t.Error("env[\"TMUX_PANE\"] should be present (tmux internal)")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewCommandRouter(nil, nil, RouterOptions{
				ClaudeEnv: tt.claudeEnv,
				PaneEnv:   tt.paneEnv,
			})

			env := router.buildPaneEnvForSession(
				tt.inheritedEnv,
				tt.shimEnv,
				tt.sessionID,
				tt.paneID,
				tt.useClaudeEnv,
				tt.usePaneEnv,
			)

			tt.verify(t, env)
		})
	}
}

// TestBuildPaneEnvForSessionNilClaudeEnv tests buildPaneEnvForSession when
// the router's claudeEnv is nil (config has no claude_env section).
// Layer 1 (claude_env) must be skipped, and only Layer 3 (pane_env) should be effective.
func TestBuildPaneEnvForSessionNilClaudeEnv(t *testing.T) {
	tests := []struct {
		name         string
		paneEnv      map[string]string
		inheritedEnv map[string]string
		shimEnv      map[string]string
		useClaudeEnv bool
		usePaneEnv   bool
		verify       func(t *testing.T, env map[string]string)
	}{
		{
			name:         "claudeEnv nil + useClaudeEnv=true: Layer 1 skipped, no panic",
			paneEnv:      map[string]string{"PANE_KEY": "pane_val"},
			inheritedEnv: map[string]string{},
			shimEnv:      map[string]string{},
			useClaudeEnv: true,
			usePaneEnv:   true,
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				// claudeEnvView returns nil; Layer 1 loop iterates 0 times (no panic).
				// Layer 3 pane_env should still be applied (overwrite mode since both true).
				if got := env["PANE_KEY"]; got != "pane_val" {
					t.Errorf("env[PANE_KEY] = %q, want %q", got, "pane_val")
				}
				// tmux internal vars should still be present.
				if _, ok := env["TMUX_PANE"]; !ok {
					t.Error("TMUX_PANE should be present (Layer 5)")
				}
			},
		},
		{
			name:         "claudeEnv nil + useClaudeEnv=false: Layer 1 skipped, pane_env fill-only",
			paneEnv:      map[string]string{"PANE_KEY": "pane_val"},
			inheritedEnv: map[string]string{"PANE_KEY": "inherited_val"},
			shimEnv:      map[string]string{},
			useClaudeEnv: false,
			usePaneEnv:   true,
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				// useClaudeEnv=false -> Layer 1 skipped.
				// usePaneEnv=true + useClaudeEnv=false -> fill-only mode.
				// inherited PANE_KEY must NOT be overwritten by pane_env.
				if got := env["PANE_KEY"]; got != "inherited_val" {
					t.Errorf("env[PANE_KEY] = %q, want %q (fill-only preserves inherited)", got, "inherited_val")
				}
			},
		},
		{
			name:         "claudeEnv nil + pane_env only: Layer 3 applied as sole data source",
			paneEnv:      map[string]string{"EFFORT": "high", "CUSTOM": "val"},
			inheritedEnv: map[string]string{},
			shimEnv:      map[string]string{},
			useClaudeEnv: false,
			usePaneEnv:   true,
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				if got := env["EFFORT"]; got != "high" {
					t.Errorf("env[EFFORT] = %q, want %q", got, "high")
				}
				if got := env["CUSTOM"]; got != "val" {
					t.Errorf("env[CUSTOM] = %q, want %q", got, "val")
				}
			},
		},
		{
			name:         "claudeEnv nil + both flags false: only tmux internal vars",
			paneEnv:      map[string]string{"PANE_KEY": "should_not_appear"},
			inheritedEnv: map[string]string{},
			shimEnv:      map[string]string{},
			useClaudeEnv: false,
			usePaneEnv:   false,
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				if _, ok := env["PANE_KEY"]; ok {
					t.Error("env[PANE_KEY] should not be present when usePaneEnv=false")
				}
				if _, ok := env["TMUX_PANE"]; !ok {
					t.Error("TMUX_PANE should be present (Layer 5 always applied)")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ClaudeEnv intentionally nil in RouterOptions.
			router := NewCommandRouter(nil, nil, RouterOptions{
				ClaudeEnv: nil,
				PaneEnv:   tt.paneEnv,
			})

			env := router.buildPaneEnvForSession(
				tt.inheritedEnv,
				tt.shimEnv,
				1, // sessionID
				1, // paneID
				tt.useClaudeEnv,
				tt.usePaneEnv,
			)

			tt.verify(t, env)
		})
	}
}

// TestResolveEnvForPaneCreation tests the branching between new path
// (session-level flags) and legacy path (buildPaneEnv).
func TestResolveEnvForPaneCreation(t *testing.T) {
	tests := []struct {
		name          string
		sessionSnap   *TmuxSession
		sessionName   string
		inheritedEnv  map[string]string
		shimEnv       map[string]string
		sessionID     int
		paneID        int
		createSession bool
		useClaudeEnv  *bool
		usePaneEnv    *bool
		claudeEnv     map[string]string
		paneEnv       map[string]string
		verify        func(t *testing.T, env map[string]string)
	}{
		{
			name:          "new path: both flags set",
			sessionSnap:   nil, // nil triggers internal GetSession lookup
			sessionName:   "test-sess",
			inheritedEnv:  map[string]string{},
			shimEnv:       map[string]string{},
			sessionID:     1,
			paneID:        1,
			createSession: true,
			useClaudeEnv:  new(true),
			usePaneEnv:    new(true),
			claudeEnv:     map[string]string{"CLAUDE_KEY": "value"},
			paneEnv:       map[string]string{"PANE_KEY": "value"},
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				if env["CLAUDE_KEY"] != "value" {
					t.Errorf("CLAUDE_KEY should be present in new path")
				}
				if env["PANE_KEY"] != "value" {
					t.Errorf("PANE_KEY should be present in new path")
				}
			},
		},
		{
			name:          "legacy path: no flags set",
			sessionSnap:   nil, // nil triggers internal GetSession lookup
			sessionName:   "test-sess",
			inheritedEnv:  map[string]string{},
			shimEnv:       map[string]string{},
			sessionID:     1,
			paneID:        1,
			createSession: true,
			useClaudeEnv:  nil,
			usePaneEnv:    nil,
			claudeEnv:     map[string]string{"CLAUDE_KEY": "value"},
			paneEnv:       map[string]string{"PANE_KEY": "value"},
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				// Legacy path uses buildPaneEnv which doesn't apply claude_env
				if _, ok := env["CLAUDE_KEY"]; ok {
					t.Errorf("CLAUDE_KEY should not be present in legacy path")
				}
				// pane_env fill-only behavior
				if env["PANE_KEY"] != "value" {
					t.Errorf("PANE_KEY should be present in legacy path (fill-only)")
				}
			},
		},
		{
			name:          "nil defaults: UseClaudeEnv nil -> false, UsePaneEnv nil -> true",
			sessionSnap:   nil, // nil triggers internal GetSession lookup
			sessionName:   "test-sess",
			inheritedEnv:  map[string]string{},
			shimEnv:       map[string]string{},
			sessionID:     1,
			paneID:        1,
			createSession: true,
			useClaudeEnv:  nil,
			usePaneEnv:    nil,
			claudeEnv:     map[string]string{},
			paneEnv:       map[string]string{},
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				// When both are nil, takes legacy path
				if _, ok := env["TMUX_PANE"]; !ok {
					t.Error("tmux internal should be present")
				}
			},
		},
		{
			name:          "session not found: falls back to legacy",
			sessionSnap:   nil,
			sessionName:   "nonexistent",
			inheritedEnv:  map[string]string{},
			shimEnv:       map[string]string{},
			sessionID:     1,
			paneID:        1,
			createSession: false,
			claudeEnv:     map[string]string{"CLAUDE_KEY": "value"},
			paneEnv:       map[string]string{"PANE_KEY": "value"},
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				// Session not found: uses legacy path
				if _, ok := env["CLAUDE_KEY"]; ok {
					t.Errorf("CLAUDE_KEY should not be present when session not found")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			router := NewCommandRouter(sessions, nil, RouterOptions{
				ClaudeEnv: tt.claudeEnv,
				PaneEnv:   tt.paneEnv,
			})

			if tt.createSession {
				sessions.CreateSession(tt.sessionName, "", 0, 0)
				if tt.useClaudeEnv != nil || tt.usePaneEnv != nil {
					if tt.useClaudeEnv != nil {
						sessions.SetUseClaudeEnv(tt.sessionName, *tt.useClaudeEnv)
					}
					if tt.usePaneEnv != nil {
						sessions.SetUsePaneEnv(tt.sessionName, *tt.usePaneEnv)
					}
				}
			}

			env := router.resolveEnvForPaneCreation(
				tt.sessionSnap,
				tt.sessionName,
				tt.inheritedEnv,
				tt.shimEnv,
				tt.sessionID,
				tt.paneID,
			)

			tt.verify(t, env)
		})
	}
}

// TestResolveEnvForPaneCreationSnapshotPath tests the pre-fetched sessionSnap path
// of resolveEnvForPaneCreation. When a non-nil sessionSnap is provided, the internal
// GetSession lookup must be skipped and the snapshot's flags used directly.
// This is the symmetric counterpart to the nil-sessionSnap tests above.
func TestResolveEnvForPaneCreationSnapshotPath(t *testing.T) {
	tests := []struct {
		name         string
		sessionSnap  *TmuxSession
		claudeEnv    map[string]string
		paneEnv      map[string]string
		inheritedEnv map[string]string
		shimEnv      map[string]string
		verify       func(t *testing.T, env map[string]string)
	}{
		{
			name: "non-nil snapshot with both flags true: new path used (claude_env + pane_env overwrite)",
			sessionSnap: &TmuxSession{
				UseClaudeEnv: new(true),
				UsePaneEnv:   new(true),
			},
			claudeEnv:    map[string]string{"CLAUDE_KEY": "claude_val"},
			paneEnv:      map[string]string{"PANE_KEY": "pane_val", "CLAUDE_KEY": "pane_overwrite"},
			inheritedEnv: map[string]string{},
			shimEnv:      map[string]string{},
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				// pane_env overwrites claude_env when both flags are true
				if got := env["CLAUDE_KEY"]; got != "pane_overwrite" {
					t.Errorf("env[CLAUDE_KEY] = %q, want %q (pane_env overwrite)", got, "pane_overwrite")
				}
				if got := env["PANE_KEY"]; got != "pane_val" {
					t.Errorf("env[PANE_KEY] = %q, want %q", got, "pane_val")
				}
			},
		},
		{
			name: "non-nil snapshot with UseClaudeEnv=true, UsePaneEnv=nil: new path with overwrite (nil defaults to true)",
			sessionSnap: &TmuxSession{
				UseClaudeEnv: new(true),
				UsePaneEnv:   nil, // nil defaults to true -> overwrite mode (both flags true)
			},
			claudeEnv:    map[string]string{"CLAUDE_KEY": "claude_val"},
			paneEnv:      map[string]string{"CLAUDE_KEY": "pane_overwrite", "PANE_ONLY": "pane_val"},
			inheritedEnv: map[string]string{},
			shimEnv:      map[string]string{},
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				// UseClaudeEnv=true triggers new path (at least one flag non-nil).
				// UsePaneEnv nil defaults to true. Both true -> overwrite mode.
				if got := env["CLAUDE_KEY"]; got != "pane_overwrite" {
					t.Errorf("env[CLAUDE_KEY] = %q, want %q (pane_env overwrite when both flags true)", got, "pane_overwrite")
				}
				if got := env["PANE_ONLY"]; got != "pane_val" {
					t.Errorf("env[PANE_ONLY] = %q, want %q", got, "pane_val")
				}
			},
		},
		{
			name: "non-nil snapshot with UseClaudeEnv=false, UsePaneEnv=true: claude_env not applied",
			sessionSnap: &TmuxSession{
				UseClaudeEnv: new(false),
				UsePaneEnv:   new(true),
			},
			claudeEnv:    map[string]string{"CLAUDE_KEY": "should_not_appear"},
			paneEnv:      map[string]string{"PANE_KEY": "pane_val"},
			inheritedEnv: map[string]string{"INHERITED": "inherited_val"},
			shimEnv:      map[string]string{},
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				if _, ok := env["CLAUDE_KEY"]; ok {
					t.Error("env[CLAUDE_KEY] should not be present when UseClaudeEnv=false")
				}
				if got := env["PANE_KEY"]; got != "pane_val" {
					t.Errorf("env[PANE_KEY] = %q, want %q", got, "pane_val")
				}
				if got := env["INHERITED"]; got != "inherited_val" {
					t.Errorf("env[INHERITED] = %q, want %q", got, "inherited_val")
				}
			},
		},
		{
			name: "non-nil snapshot with both flags nil: takes legacy path (no flag set trigger)",
			sessionSnap: &TmuxSession{
				UseClaudeEnv: nil,
				UsePaneEnv:   nil,
			},
			claudeEnv:    map[string]string{"CLAUDE_KEY": "should_not_appear"},
			paneEnv:      map[string]string{"PANE_KEY": "pane_val"},
			inheritedEnv: map[string]string{},
			shimEnv:      map[string]string{},
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				// Both nil -> legacy path (no explicit flag set)
				if _, ok := env["CLAUDE_KEY"]; ok {
					t.Error("env[CLAUDE_KEY] should not be present in legacy path")
				}
				// Legacy path applies pane_env fill-only
				if got := env["PANE_KEY"]; got != "pane_val" {
					t.Errorf("env[PANE_KEY] = %q, want %q (fill-only in legacy path)", got, "pane_val")
				}
			},
		},
		{
			name: "non-nil snapshot skips internal GetSession lookup (session not in manager)",
			sessionSnap: &TmuxSession{
				Name:         "external-session",
				UseClaudeEnv: new(true),
				UsePaneEnv:   new(false),
			},
			claudeEnv:    map[string]string{"CLAUDE_KEY": "from_snapshot"},
			paneEnv:      map[string]string{},
			inheritedEnv: map[string]string{},
			shimEnv:      map[string]string{},
			verify: func(t *testing.T, env map[string]string) {
				t.Helper()
				// Even though the session doesn't exist in the SessionManager,
				// the pre-fetched snapshot should be used directly.
				if got := env["CLAUDE_KEY"]; got != "from_snapshot" {
					t.Errorf("env[CLAUDE_KEY] = %q, want %q (from pre-fetched snapshot)", got, "from_snapshot")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create router with empty session manager — snapshot path should
			// not attempt any internal lookup when sessionSnap is non-nil.
			sessions := NewSessionManager()
			router := NewCommandRouter(sessions, nil, RouterOptions{
				ClaudeEnv: tt.claudeEnv,
				PaneEnv:   tt.paneEnv,
			})

			env := router.resolveEnvForPaneCreation(
				tt.sessionSnap,
				"unused-session-name",
				tt.inheritedEnv,
				tt.shimEnv,
				1, // sessionID
				1, // paneID
			)

			tt.verify(t, env)
		})
	}
}

// TestApplySessionEnvFlags tests the SetUseClaudeEnv/SetUsePaneEnv behavior
// that the production applySessionEnvFlags (in app_session_api.go) relies on.
//
// The production function is void: it calls SetUseClaudeEnv and SetUsePaneEnv,
// logging warnings on failure via slog.Warn. This test exercises the underlying
// SessionManager methods directly, which is what the production function delegates to.
func TestApplySessionEnvFlags(t *testing.T) {
	tests := []struct {
		name          string
		sessionName   string
		useClaudeEnv  bool
		usePaneEnv    bool
		createSession bool
		wantClaudeEnv *bool
		wantPaneEnv   *bool
	}{
		{
			name:          "both flags on",
			sessionName:   "sess",
			useClaudeEnv:  true,
			usePaneEnv:    true,
			createSession: true,
			wantClaudeEnv: new(true),
			wantPaneEnv:   new(true),
		},
		{
			name:          "both flags off",
			sessionName:   "sess",
			useClaudeEnv:  false,
			usePaneEnv:    false,
			createSession: true,
			wantClaudeEnv: new(false),
			wantPaneEnv:   new(false),
		},
		{
			name:          "session not found: errors returned but production logs and continues",
			sessionName:   "nonexistent",
			useClaudeEnv:  true,
			usePaneEnv:    true,
			createSession: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			if tt.createSession {
				sessions.CreateSession(tt.sessionName, "", 0, 0)
			}

			// Mirror production applySessionEnvFlags behavior:
			// call both setters; on error, production logs slog.Warn and continues.
			if setErr := sessions.SetUseClaudeEnv(tt.sessionName, tt.useClaudeEnv); setErr != nil {
				if tt.createSession {
					t.Errorf("SetUseClaudeEnv() unexpected error = %v", setErr)
				}
				// Non-existent session: error is expected; production logs and continues.
			}
			if setErr := sessions.SetUsePaneEnv(tt.sessionName, tt.usePaneEnv); setErr != nil {
				if tt.createSession {
					t.Errorf("SetUsePaneEnv() unexpected error = %v", setErr)
				}
			}

			if tt.createSession {
				snap, _ := sessions.GetSession(tt.sessionName)
				if !reflect.DeepEqual(snap.UseClaudeEnv, tt.wantClaudeEnv) {
					t.Errorf("UseClaudeEnv = %v, want %v", snap.UseClaudeEnv, tt.wantClaudeEnv)
				}
				if !reflect.DeepEqual(snap.UsePaneEnv, tt.wantPaneEnv) {
					t.Errorf("UsePaneEnv = %v, want %v", snap.UsePaneEnv, tt.wantPaneEnv)
				}
			}
		})
	}
}
