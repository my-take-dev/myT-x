package orchestrator

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"myT-x/internal/tmux"
)

func bootstrapTestDeps(t *testing.T, configPath string) Deps {
	t.Helper()
	return Deps{
		ConfigPath: func() string { return configPath },
		FindSessionSnapshot: func(string) (tmux.SessionSnapshot, error) {
			return tmux.SessionSnapshot{Name: "test-session", RootPath: "/test/root"}, nil
		},
		GetActiveSessionName: func() string { return "test-session" },
		CreateSession: func(string, string) (tmux.SessionSnapshot, error) {
			t.Fatal("unexpected CreateSession call")
			return tmux.SessionSnapshot{}, nil
		},
		CreatePaneInSession: func(string) (string, error) {
			t.Fatal("unexpected CreatePaneInSession call")
			return "", nil
		},
		KillSession: func(string) error {
			t.Fatal("unexpected KillSession call")
			return nil
		},
		SplitPane: func(string, bool) (string, error) {
			t.Fatal("unexpected SplitPane call")
			return "", nil
		},
		RenamePane: func(string, string) error { return nil },
		ApplyLayoutPreset: func(string, string) error {
			t.Fatal("unexpected ApplyLayoutPreset call")
			return nil
		},
		SendKeys:      func(string, string) error { return nil },
		SendKeysPaste: func(string, string) error { return nil },
		SleepFn:       func(time.Duration) {},
		CheckReady:    func() error { return nil },
	}
}

func TestBootstrapMemberToPane_Validate(t *testing.T) {
	tests := []struct {
		name    string
		request BootstrapMemberToPaneRequest
		wantErr string
	}{
		{
			name:    "empty pane_id",
			request: BootstrapMemberToPaneRequest{PaneState: PaneStateCLIRunning, TeamName: "T", Member: TeamMember{TeamID: "t1", PaneTitle: "A", Role: "R", Command: "c"}},
			wantErr: "pane_id is required",
		},
		{
			name:    "invalid pane_state",
			request: BootstrapMemberToPaneRequest{PaneID: "%1", PaneState: "invalid", TeamName: "T", Member: TeamMember{TeamID: "t1", PaneTitle: "A", Role: "R", Command: "c"}},
			wantErr: "unsupported pane_state",
		},
		{
			name:    "empty team_name",
			request: BootstrapMemberToPaneRequest{PaneID: "%1", PaneState: PaneStateCLIRunning, Member: TeamMember{TeamID: "t1", PaneTitle: "A", Role: "R", Command: "c"}},
			wantErr: "team_name is required",
		},
		{
			name:    "invalid member",
			request: BootstrapMemberToPaneRequest{PaneID: "%1", PaneState: PaneStateCLIRunning, TeamName: "T", Member: TeamMember{}},
			wantErr: "member validation failed",
		},
		{
			name: "valid cli_running",
			request: BootstrapMemberToPaneRequest{
				PaneID: "%1", PaneState: PaneStateCLIRunning, TeamName: "Team",
				Member: TeamMember{TeamID: "t1", PaneTitle: "Lead", Role: "Lead", Command: "claude"},
			},
			wantErr: "",
		},
		{
			name: "valid cli_not_running",
			request: BootstrapMemberToPaneRequest{
				PaneID: "%1", PaneState: PaneStateCLINotRunning, TeamName: "Team",
				Member: TeamMember{TeamID: "t1", PaneTitle: "Lead", Role: "Lead", Command: "claude"},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.yaml")
			s := NewService(bootstrapTestDeps(t, configPath))
			_, err := s.BootstrapMemberToPane(tt.request)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestBootstrapMemberToPane_CLIRunning_SendsPasteForClaude(t *testing.T) {
	var sentPaste bool
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	deps := bootstrapTestDeps(t, configPath)
	deps.SendKeysPaste = func(paneID, text string) error {
		sentPaste = true
		return nil
	}
	s := NewService(deps)

	_, err := s.BootstrapMemberToPane(BootstrapMemberToPaneRequest{
		PaneID:    "%1",
		PaneState: PaneStateCLIRunning,
		TeamName:  "Team",
		Member:    TeamMember{TeamID: "t1", PaneTitle: "Lead", Role: "Lead", Command: "claude"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sentPaste {
		t.Fatal("expected SendKeysPaste to be called for claude command")
	}
}

func TestBootstrapMemberToPane_CLIRunning_SendsKeysForNonClaude(t *testing.T) {
	var sentKeys bool
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	deps := bootstrapTestDeps(t, configPath)
	deps.SendKeys = func(paneID, text string) error {
		sentKeys = true
		return nil
	}
	s := NewService(deps)

	_, err := s.BootstrapMemberToPane(BootstrapMemberToPaneRequest{
		PaneID:    "%1",
		PaneState: PaneStateCLIRunning,
		TeamName:  "Team",
		Member:    TeamMember{TeamID: "t1", PaneTitle: "Lead", Role: "Lead", Command: "codex"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sentKeys {
		t.Fatal("expected SendKeys to be called for non-claude command")
	}
}

func TestBootstrapMemberToPane_CLINotRunning_SendsCdAndLaunch(t *testing.T) {
	var commands []string
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	deps := bootstrapTestDeps(t, configPath)
	deps.SendKeys = func(paneID, text string) error {
		commands = append(commands, text)
		return nil
	}
	s := NewService(deps)

	_, err := s.BootstrapMemberToPane(BootstrapMemberToPaneRequest{
		PaneID:           "%1",
		PaneState:        PaneStateCLINotRunning,
		TeamName:         "Team",
		Member:           TeamMember{TeamID: "t1", PaneTitle: "Lead", Role: "Lead", Command: "codex"},
		BootstrapDelayMs: 1000,
		SessionName:      "test-session",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(commands) < 2 {
		t.Fatalf("expected at least 2 SendKeys calls (cd + launch), got %d", len(commands))
	}
	if !strings.HasPrefix(commands[0], "cd ") {
		t.Fatalf("first command should be cd, got %q", commands[0])
	}
	if commands[1] != "codex" {
		t.Fatalf("second command should be launch command, got %q", commands[1])
	}
}

func TestBootstrapMemberToPane_NormalizesDelayBounds(t *testing.T) {
	tests := []struct {
		name     string
		inputMs  int
		expected int
	}{
		{"below minimum", 100, BootstrapDelayMsMin},
		{"above maximum", 99999, BootstrapDelayMsMax},
		{"zero defaults", 0, BootstrapDelayMsDefault},
		{"within range", 5000, 5000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := BootstrapMemberToPaneRequest{
				PaneID:           "%1",
				PaneState:        PaneStateCLIRunning,
				TeamName:         "Team",
				Member:           TeamMember{TeamID: "t1", PaneTitle: "Lead", Role: "Lead", Command: "claude"},
				BootstrapDelayMs: tt.inputMs,
			}
			req.Normalize()
			if req.BootstrapDelayMs != tt.expected {
				t.Fatalf("BootstrapDelayMs = %d, want %d", req.BootstrapDelayMs, tt.expected)
			}
		})
	}
}
