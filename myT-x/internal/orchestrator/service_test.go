package orchestrator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"myT-x/internal/tmux"
)

// testDeps returns a minimal Deps suitable for CRUD tests.
// Start-related deps use stubs that panic if called unexpectedly.
func testDeps(t *testing.T, configPath string) Deps {
	t.Helper()
	return Deps{
		ConfigPath:           func() string { return configPath },
		FindSessionSnapshot:  func(string) (tmux.SessionSnapshot, error) { return tmux.SessionSnapshot{}, nil },
		GetActiveSessionName: func() string { return "" },
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
		RenamePane: func(string, string) error {
			t.Fatal("unexpected RenamePane call")
			return nil
		},
		ApplyLayoutPreset: func(string, string) error {
			t.Fatal("unexpected ApplyLayoutPreset call")
			return nil
		},
		SendKeys: func(string, string) error {
			t.Fatal("unexpected SendKeys call")
			return nil
		},
		SendKeysPaste: func(string, string) error {
			t.Fatal("unexpected SendKeysPaste call")
			return nil
		},
		SleepFn:    func(time.Duration) {},
		CheckReady: func() error { return nil },
	}
}

func TestNewServicePanicsOnMissingDeps(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil deps")
		}
	}()
	NewService(Deps{})
}

func TestNewServiceDefaultsSleepFn(t *testing.T) {
	deps := testDeps(t, filepath.Join(t.TempDir(), "config.yaml"))
	deps.SleepFn = nil
	s := NewService(deps)
	if s.deps.SleepFn == nil {
		t.Fatal("expected SleepFn to default to non-nil")
	}
}

func TestSaveAndLoadTeams(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	err := s.SaveTeam(TeamDefinition{
		Name: " Test Team ",
		Members: []TeamMember{
			{PaneTitle: "Lead", Role: "Lead", Command: "codex"},
			{PaneTitle: "Builder", Role: "Builder", Command: "claude"},
		},
	}, "")
	if err != nil {
		t.Fatalf("SaveTeam() error = %v", err)
	}

	teams, err := s.LoadTeams("")
	if err != nil {
		t.Fatalf("LoadTeams() error = %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("len(teams) = %d, want 1", len(teams))
	}
	if teams[0].Name != "Test Team" {
		t.Fatalf("team.Name = %q, want 'Test Team'", teams[0].Name)
	}
	if len(teams[0].Members) != 2 {
		t.Fatalf("len(members) = %d, want 2", len(teams[0].Members))
	}
}

func TestDeleteTeamRemovesMembersAndOrphans(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	if err := s.SaveTeam(TeamDefinition{
		ID: "team-a", Name: "Alpha",
		Members: []TeamMember{{ID: "m-a", PaneTitle: "A", Role: "A", Command: "codex"}},
	}, ""); err != nil {
		t.Fatalf("SaveTeam(Alpha) error = %v", err)
	}
	if err := s.SaveTeam(TeamDefinition{
		ID: "team-b", Name: "Beta",
		Members: []TeamMember{{ID: "m-b", PaneTitle: "B", Role: "B", Command: "codex"}},
	}, ""); err != nil {
		t.Fatalf("SaveTeam(Beta) error = %v", err)
	}

	if err := s.DeleteTeam("team-a", "", ""); err != nil {
		t.Fatalf("DeleteTeam() error = %v", err)
	}

	teams, err := s.LoadTeams("")
	if err != nil {
		t.Fatalf("LoadTeams() error = %v", err)
	}
	if len(teams) != 1 || teams[0].ID != "team-b" {
		t.Fatalf("expected only team-b, got %+v", teams)
	}
}

func TestReorderTeams(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	s := NewService(testDeps(t, configPath))

	for _, team := range []TeamDefinition{
		{ID: "a", Name: "Alpha", Members: []TeamMember{{PaneTitle: "A", Role: "A", Command: "x"}}},
		{ID: "b", Name: "Beta", Members: []TeamMember{{PaneTitle: "B", Role: "B", Command: "x"}}},
		{ID: "c", Name: "Charlie", Members: []TeamMember{{PaneTitle: "C", Role: "C", Command: "x"}}},
	} {
		if err := s.SaveTeam(team, ""); err != nil {
			t.Fatalf("SaveTeam(%s) error = %v", team.ID, err)
		}
	}

	if err := s.ReorderTeams([]string{"c", "a", "b"}, "", ""); err != nil {
		t.Fatalf("ReorderTeams() error = %v", err)
	}

	teams, err := s.LoadTeams("")
	if err != nil {
		t.Fatalf("LoadTeams() error = %v", err)
	}
	if teams[0].Name != "Charlie" || teams[1].Name != "Alpha" || teams[2].Name != "Beta" {
		t.Fatalf("reordered = %q,%q,%q, want Charlie,Alpha,Beta", teams[0].Name, teams[1].Name, teams[2].Name)
	}
}

func TestSeedSamplesInternal(t *testing.T) {
	dir := t.TempDir()
	defsPath := filepath.Join(dir, definitionsFileName)
	memsPath := filepath.Join(dir, membersFileName)

	if seeded := seedSamplesInternal(defsPath, memsPath); !seeded {
		t.Fatal("expected seedSamplesInternal to return true for first-time seeding")
	}

	// Verify files exist and are valid JSON.
	data, err := os.ReadFile(defsPath)
	if err != nil {
		t.Fatalf("read definitions: %v", err)
	}
	var defs []teamFileRecord
	if err := json.Unmarshal(data, &defs); err != nil {
		t.Fatalf("definitions JSON invalid: %v", err)
	}
	if len(defs) == 0 {
		t.Fatal("seeded definitions should not be empty")
	}

	// Second call should return false (file exists).
	if seeded := seedSamplesInternal(defsPath, memsPath); seeded {
		t.Fatal("expected seedSamplesInternal to return false when file exists")
	}
}

func TestWriteTeamJSONCreatesFileAndCleansTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "teams.json")

	payload := []teamFileRecord{{ID: "t1", Name: "Team1", Order: 0}}
	if err := writeTeamJSON(path, payload, "test"); err != nil {
		t.Fatalf("writeTeamJSON() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	var read []teamFileRecord
	if err := json.Unmarshal(data, &read); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(read) != 1 || read[0].ID != "t1" {
		t.Fatalf("got %+v, want [{ID:t1 ...}]", read)
	}

	// Verify no temp files left behind.
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temp file not cleaned up: %s", entry.Name())
		}
	}
}

func TestRenameWithRetrySucceeds(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(src, []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := renameWithRetry(src, dst); err != nil {
		t.Fatalf("renameWithRetry() error = %v", err)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("source file should not exist after rename")
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "data" {
		t.Fatalf("dst content = %q, want %q", string(data), "data")
	}
}

func TestReadDefinitionsWithModeMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "definitions.json")

	if err := os.WriteFile(path, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	// allowMalformed=true → empty slice, no error.
	defs, err := readDefinitionsWithMode(path, true)
	if err != nil {
		t.Fatalf("readDefinitionsWithMode(allowMalformed=true) error = %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("expected empty definitions, got %d", len(defs))
	}

	// allowMalformed=false → error.
	_, err = readDefinitionsWithMode(path, false)
	if err == nil {
		t.Fatal("readDefinitionsWithMode(allowMalformed=false) expected error")
	}
	if !strings.Contains(err.Error(), "parse team definitions") {
		t.Fatalf("error = %v, want 'parse team definitions'", err)
	}
}

func TestReadDefinitionsFileNotExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")

	defs, err := readDefinitionsWithMode(path, true)
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(defs) != 0 {
		t.Fatalf("expected empty definitions, got %d", len(defs))
	}
}

func TestResolveSourceRootPath(t *testing.T) {
	tests := []struct {
		name    string
		session tmux.SessionSnapshot
		want    string
		wantErr bool
	}{
		{
			name:    "empty session name",
			session: tmux.SessionSnapshot{},
			wantErr: true,
		},
		{
			name:    "root path only",
			session: tmux.SessionSnapshot{Name: "s1", RootPath: "/repo"},
			want:    "/repo",
		},
		{
			name: "worktree path preferred",
			session: tmux.SessionSnapshot{
				Name:     "s1",
				RootPath: "/repo",
				Worktree: &tmux.SessionWorktreeInfo{Path: "/wt"},
			},
			want: "/wt",
		},
		{
			name:    "no paths",
			session: tmux.SessionSnapshot{Name: "s1"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveSourceRootPath(tt.session)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsClaudeCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"claude", "claude", true},
		{"claude.exe", "claude.exe", true},
		{"absolute path unix", "/usr/bin/claude", true},
		{"absolute path windows", `C:\Tools\claude.exe`, true},
		{"claude-code", "claude-code", true},
		{"claude-code-v2", "claude-code-v2", true},
		{"claude with args", "claude --model opus", true},
		{"CLAUDE uppercase", "CLAUDE", true},
		{"Claude mixed case", "Claude", true},
		{"codex is not claude", "codex", false},
		{"empty command", "", false},
		{"whitespace only", "   ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsClaudeCommand(tt.command); got != tt.want {
				t.Errorf("IsClaudeCommand(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestQuoteCommandArg(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{"empty string", "", `""`},
		{"no special chars", "--flag", "--flag"},
		{"contains space", "hello world", `"hello world"`},
		{"contains tab", "hello\tworld", "\"hello\tworld\""},
		{"contains double quote", `say "hi"`, `"say \"hi\""`},
		{"backslash before quote", `path\"end`, `"path\\\"end"`},
		{"trailing backslash no special", `path\`, `path\`},
		{"trailing backslash with space", `path to\`, `"path to\\"`},
		{"simple flag no quoting", "--sandbox", "--sandbox"},
		{"space at start", " leading", `" leading"`},
		{"multiple spaces", "a b c", `"a b c"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := QuoteCommandArg(tt.arg); got != tt.want {
				t.Errorf("QuoteCommandArg(%q) = %q, want %q", tt.arg, got, tt.want)
			}
		})
	}
}

func TestBuildSkillCompletionHintsEmpty(t *testing.T) {
	hints := BuildSkillCompletionHints("Dev", nil)
	if !strings.Contains(hints, "得意分野（skills）が未設定です") {
		t.Fatalf("expected empty skills hint, got: %s", hints)
	}
}

func TestBuildSkillCompletionHintsSufficient(t *testing.T) {
	skills := []TeamMemberSkill{
		{Name: "Go", Description: "backend"},
		{Name: "React", Description: "frontend"},
		{Name: "SQL", Description: "database"},
	}
	hints := BuildSkillCompletionHints("Full stack", skills)
	if hints != "" {
		t.Fatalf("expected empty hints for sufficient skills, got: %s", hints)
	}
}

func TestDeriveAgentNamesDeduplicate(t *testing.T) {
	names := DeriveAgentNames([]TeamMember{
		{ID: "a", PaneTitle: "Lead"},
		{ID: "b", PaneTitle: "Lead"},
		{ID: "c", PaneTitle: "  "},
	})
	if names["a"] != "lead" {
		t.Fatalf("names[a] = %q, want lead", names["a"])
	}
	if names["b"] != "lead-2" {
		t.Fatalf("names[b] = %q, want lead-2", names["b"])
	}
	if names["c"] != "member" {
		t.Fatalf("names[c] = %q, want member", names["c"])
	}
}

// startTestDeps returns Deps wired for StartTeam tests with controllable mocks.
func startTestDeps(t *testing.T, configPath string) Deps {
	t.Helper()
	deps := testDeps(t, configPath)
	deps.FindSessionSnapshot = func(name string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{
			Name:           name,
			RootPath:       "/repo",
			ActiveWindowID: 1,
			Windows: []tmux.WindowSnapshot{{
				ID:    1,
				Name:  "main",
				Panes: []tmux.PaneSnapshot{{ID: "%1", Index: 0}},
			}},
		}, nil
	}
	deps.CreateSession = func(rootPath, sessionName string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{
			Name:           sessionName,
			RootPath:       rootPath,
			ActiveWindowID: 1,
			Windows: []tmux.WindowSnapshot{{
				ID:    1,
				Name:  "main",
				Panes: []tmux.PaneSnapshot{{ID: "%10", Index: 0}},
			}},
		}, nil
	}
	deps.CreatePaneInSession = func(sessionName string) (string, error) { return "%10", nil }
	deps.KillSession = func(string) error { return nil }
	deps.SplitPane = func(paneID string, horizontal bool) (string, error) { return "%11", nil }
	deps.RenamePane = func(string, string) error { return nil }
	deps.ApplyLayoutPreset = func(string, string) error { return nil }
	deps.SendKeys = func(string, string) error { return nil }
	deps.SendKeysPaste = func(string, string) error { return nil }
	return deps
}

func TestStartTeamActiveSessionBasic(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	deps := startTestDeps(t, configPath)
	s := NewService(deps)

	if err := s.SaveTeam(TeamDefinition{
		ID:   "team-1",
		Name: "Test",
		Members: []TeamMember{
			{ID: "m1", PaneTitle: "Lead", Role: "Lead", Command: "codex"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveTeam() error = %v", err)
	}

	result, err := s.StartTeam(StartTeamRequest{
		TeamID:            "team-1",
		LaunchMode:        LaunchModeActiveSession,
		SourceSessionName: "src",
	})
	if err != nil {
		t.Fatalf("StartTeam() error = %v", err)
	}
	if result.SessionName != "src" {
		t.Fatalf("SessionName = %q, want src", result.SessionName)
	}
	if len(result.MemberPaneIDs) != 1 {
		t.Fatalf("len(MemberPaneIDs) = %d, want 1", len(result.MemberPaneIDs))
	}
}

func TestStartTeamNewSessionWithRollback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	deps := startTestDeps(t, configPath)
	var rolledBack []string
	deps.KillSession = func(name string) error {
		rolledBack = append(rolledBack, name)
		return nil
	}
	// SplitPane fails → new session mode should rollback.
	deps.SplitPane = func(string, bool) (string, error) {
		return "", errors.New("split failed")
	}
	s := NewService(deps)

	if err := s.SaveTeam(TeamDefinition{
		ID:   "team-2",
		Name: "Fail",
		Members: []TeamMember{
			{ID: "m1", PaneTitle: "A", Role: "A", Command: "codex"},
			{ID: "m2", PaneTitle: "B", Role: "B", Command: "codex"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveTeam() error = %v", err)
	}

	_, err := s.StartTeam(StartTeamRequest{
		TeamID:            "team-2",
		LaunchMode:        LaunchModeNewSession,
		SourceSessionName: "src",
	})
	if err == nil {
		t.Fatal("expected error from failed SplitPane")
	}
	if len(rolledBack) == 0 {
		t.Fatal("expected KillSession rollback call")
	}
}

func TestStartTeamCheckReadyFailsFast(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	deps := startTestDeps(t, configPath)
	deps.CheckReady = func() error { return errors.New("not ready") }
	s := NewService(deps)

	if err := s.SaveTeam(TeamDefinition{
		ID:   "team-3",
		Name: "Ready",
		Members: []TeamMember{
			{ID: "m1", PaneTitle: "A", Role: "A", Command: "codex"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveTeam() error = %v", err)
	}

	_, err := s.StartTeam(StartTeamRequest{
		TeamID:            "team-3",
		LaunchMode:        LaunchModeActiveSession,
		SourceSessionName: "src",
	})
	if err == nil {
		t.Fatal("expected CheckReady error")
	}
	if !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("error = %v, want 'not ready'", err)
	}
}

func TestStartTeamEnsurePaneCapacityPartial(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	deps := startTestDeps(t, configPath)
	splitCount := 0
	deps.SplitPane = func(string, bool) (string, error) {
		splitCount++
		if splitCount > 1 {
			return "", errors.New("no more splits")
		}
		return "%2", nil
	}
	s := NewService(deps)

	if err := s.SaveTeam(TeamDefinition{
		ID:   "team-4",
		Name: "Partial",
		Members: []TeamMember{
			{ID: "m1", PaneTitle: "A", Role: "A", Command: "codex"},
			{ID: "m2", PaneTitle: "B", Role: "B", Command: "codex"},
			{ID: "m3", PaneTitle: "C", Role: "C", Command: "codex"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveTeam() error = %v", err)
	}

	// ActiveSession mode allows partial pane allocation.
	result, err := s.StartTeam(StartTeamRequest{
		TeamID:            "team-4",
		LaunchMode:        LaunchModeActiveSession,
		SourceSessionName: "src",
	})
	if err != nil {
		t.Fatalf("StartTeam() error = %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings for partial pane allocation")
	}
}

func TestStartTeamActiveSessionEmptySessionCreatesInitialPane(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	deps := startTestDeps(t, configPath)
	createPaneCalls := 0
	deps.FindSessionSnapshot = func(name string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{
			Name:           name,
			RootPath:       "/repo",
			ActiveWindowID: -1,
			Windows:        []tmux.WindowSnapshot{},
		}, nil
	}
	deps.CreatePaneInSession = func(sessionName string) (string, error) {
		createPaneCalls++
		if sessionName != "src" {
			t.Fatalf("CreatePaneInSession(%q), want src", sessionName)
		}
		return "%42", nil
	}
	deps.SplitPane = func(paneID string, horizontal bool) (string, error) {
		if paneID != "%42" {
			t.Fatalf("SplitPane(%q, %v), want %q", paneID, horizontal, "%42")
		}
		return "%43", nil
	}
	s := NewService(deps)

	if err := s.SaveTeam(TeamDefinition{
		ID:   "team-empty",
		Name: "Empty",
		Members: []TeamMember{
			{ID: "m1", PaneTitle: "Lead", Role: "Lead", Command: "codex"},
			{ID: "m2", PaneTitle: "Builder", Role: "Builder", Command: "codex"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveTeam() error = %v", err)
	}

	result, err := s.StartTeam(StartTeamRequest{
		TeamID:            "team-empty",
		LaunchMode:        LaunchModeActiveSession,
		SourceSessionName: "src",
	})
	if err != nil {
		t.Fatalf("StartTeam() error = %v", err)
	}
	if createPaneCalls != 1 {
		t.Fatalf("CreatePaneInSession call count = %d, want 1", createPaneCalls)
	}
	if len(result.MemberPaneIDs) != 2 {
		t.Fatalf("len(MemberPaneIDs) = %d, want 2", len(result.MemberPaneIDs))
	}
	if result.MemberPaneIDs["m1"] != "%42" || result.MemberPaneIDs["m2"] != "%43" {
		t.Fatalf("MemberPaneIDs = %v, want map[%q:%q %q:%q]", result.MemberPaneIDs, "m1", "%42", "m2", "%43")
	}
}

// TestStartTeamTwoPhaseOrdering verifies that all launch commands (Phase 1)
// complete before any bootstrap message (Phase 2), with exactly one
// BootstrapDelayMs sleep separating the two phases and cdDelay sleeps
// between bootstrap messages.
func TestStartTeamTwoPhaseOrdering(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	deps := startTestDeps(t, configPath)

	// Assign distinct pane IDs per member.
	splitCount := 0
	deps.SplitPane = func(string, bool) (string, error) {
		splitCount++
		return fmt.Sprintf("%%1%d", splitCount), nil
	}

	// Record all events in order.
	type event struct {
		kind string // "sendkeys", "sendkeys-paste", "sleep"
		arg  string // paneID:text or duration
	}
	var events []event

	deps.SendKeys = func(paneID, text string) error {
		events = append(events, event{kind: "sendkeys", arg: paneID + ":" + text})
		return nil
	}
	deps.SendKeysPaste = func(paneID, text string) error {
		events = append(events, event{kind: "sendkeys-paste", arg: paneID + ":" + text})
		return nil
	}
	deps.SleepFn = func(d time.Duration) {
		events = append(events, event{kind: "sleep", arg: d.String()})
	}

	s := NewService(deps)

	if err := s.SaveTeam(TeamDefinition{
		ID:               "team-phase",
		Name:             "Phase",
		BootstrapDelayMs: 5000,
		Members: []TeamMember{
			{ID: "m1", PaneTitle: "Lead", Role: "Lead", Command: "claude"},
			{ID: "m2", PaneTitle: "Builder", Role: "Builder", Command: "claude"},
			{ID: "m3", PaneTitle: "Tester", Role: "Tester", Command: "codex"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveTeam() error = %v", err)
	}

	result, err := s.StartTeam(StartTeamRequest{
		TeamID:            "team-phase",
		LaunchMode:        LaunchModeNewSession,
		SourceSessionName: "src",
	})
	if err != nil {
		t.Fatalf("StartTeam() error = %v", err)
	}
	if len(result.MemberPaneIDs) != 3 {
		t.Fatalf("len(MemberPaneIDs) = %d, want 3", len(result.MemberPaneIDs))
	}

	// Find indices of key events.
	var lastLaunchIdx, firstBootstrapIdx int
	var bootstrapDelaySeen bool
	lastLaunchIdx = -1
	firstBootstrapIdx = -1

	for i, ev := range events {
		// Launch commands are sendkeys containing "launch command" text patterns
		// but more reliably: they are sendkeys that are NOT cd commands and NOT bootstrap messages.
		isLaunch := ev.kind == "sendkeys" &&
			!strings.Contains(ev.arg, "cd \"") &&
			!strings.Contains(ev.arg, "チーム")
		if isLaunch {
			lastLaunchIdx = i
		}
		isBootstrap := (ev.kind == "sendkeys-paste" || ev.kind == "sendkeys") &&
			strings.Contains(ev.arg, "チーム")
		if isBootstrap && firstBootstrapIdx == -1 {
			firstBootstrapIdx = i
		}
		if ev.kind == "sleep" && ev.arg == "5s" {
			bootstrapDelaySeen = true
		}
	}

	if lastLaunchIdx == -1 {
		t.Fatal("no launch command events found")
	}
	if firstBootstrapIdx == -1 {
		t.Fatal("no bootstrap message events found")
	}
	if lastLaunchIdx >= firstBootstrapIdx {
		t.Fatalf("last launch event (index %d) must precede first bootstrap event (index %d)", lastLaunchIdx, firstBootstrapIdx)
	}
	if !bootstrapDelaySeen {
		t.Fatal("expected exactly one BootstrapDelayMs (5s) sleep between phases")
	}

	// Verify bootstrap messages: claude commands use sendkeys-paste, non-claude uses sendkeys.
	var pasteCount, plainBootstrapCount int
	for _, ev := range events {
		if strings.Contains(ev.arg, "チーム") {
			if ev.kind == "sendkeys-paste" {
				pasteCount++
			} else if ev.kind == "sendkeys" {
				plainBootstrapCount++
			}
		}
	}
	// m1(claude) and m2(claude) → paste, m3(codex) → plain sendkeys
	if pasteCount != 2 {
		t.Fatalf("sendkeys-paste bootstrap count = %d, want 2", pasteCount)
	}
	if plainBootstrapCount != 1 {
		t.Fatalf("plain sendkeys bootstrap count = %d, want 1", plainBootstrapCount)
	}

	// Count cdDelay (300ms) sleeps between bootstrap messages.
	var interBootstrapDelays int
	inBootstrapPhase := false
	for _, ev := range events {
		if (ev.kind == "sendkeys-paste" || ev.kind == "sendkeys") && strings.Contains(ev.arg, "チーム") {
			inBootstrapPhase = true
			continue
		}
		if inBootstrapPhase && ev.kind == "sleep" && ev.arg == "300ms" {
			interBootstrapDelays++
		}
	}
	// 3 members → 2 inter-bootstrap delays
	if interBootstrapDelays != 2 {
		t.Fatalf("inter-bootstrap cdDelay count = %d, want 2", interBootstrapDelays)
	}
}

// TestStartTeamPhase1FailureSkipsBootstrap verifies that members whose
// launch command fails in Phase 1 do not receive bootstrap messages in Phase 2.
func TestStartTeamPhase1FailureSkipsBootstrap(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	deps := startTestDeps(t, configPath)

	// Provide 3 panes.
	deps.CreateSession = func(rootPath, sessionName string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{
			Name:           sessionName,
			RootPath:       rootPath,
			ActiveWindowID: 1,
			Windows: []tmux.WindowSnapshot{{
				ID:   1,
				Name: "main",
				Panes: []tmux.PaneSnapshot{
					{ID: "%10", Index: 0},
					{ID: "%11", Index: 1},
					{ID: "%12", Index: 2},
				},
			}},
		}, nil
	}
	deps.SplitPane = func(string, bool) (string, error) { return "%99", nil }

	var bootstrapTargets []string
	sendKeysCallCount := 0
	deps.SendKeys = func(paneID, text string) error {
		sendKeysCallCount++
		// Fail the launch command for m2 (pane %11).
		// The pattern: cd is called first, then launch command.
		// For %11: cd succeeds (call 3), launch fails (call 4).
		if paneID == "%11" && !strings.Contains(text, "cd ") {
			return errors.New("launch failed for m2")
		}
		if strings.Contains(text, "チーム") {
			bootstrapTargets = append(bootstrapTargets, paneID)
		}
		return nil
	}
	deps.SendKeysPaste = func(paneID, text string) error {
		if strings.Contains(text, "チーム") {
			bootstrapTargets = append(bootstrapTargets, paneID)
		}
		return nil
	}

	s := NewService(deps)

	if err := s.SaveTeam(TeamDefinition{
		ID:               "team-skip",
		Name:             "Skip",
		BootstrapDelayMs: 1000,
		Members: []TeamMember{
			{ID: "m1", PaneTitle: "A", Role: "A", Command: "claude"},
			{ID: "m2", PaneTitle: "B", Role: "B", Command: "claude"},
			{ID: "m3", PaneTitle: "C", Role: "C", Command: "claude"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveTeam() error = %v", err)
	}

	result, err := s.StartTeam(StartTeamRequest{
		TeamID:            "team-skip",
		LaunchMode:        LaunchModeNewSession,
		SourceSessionName: "src",
	})
	if err != nil {
		t.Fatalf("StartTeam() error = %v", err)
	}

	// All 3 members get pane IDs assigned before the launch attempt.
	// m2's launch fails but its pane ID is still in the map.
	if len(result.MemberPaneIDs) != 3 {
		t.Fatalf("len(MemberPaneIDs) = %d, want 3 (assigned before launch attempt)", len(result.MemberPaneIDs))
	}

	// Bootstrap messages should only go to %10 and %12, not %11.
	for _, target := range bootstrapTargets {
		if target == "%11" {
			t.Fatal("bootstrap message was sent to failed member's pane %%11")
		}
	}
	if len(bootstrapTargets) != 2 {
		t.Fatalf("bootstrap target count = %d, want 2 (m1 and m3 only)", len(bootstrapTargets))
	}
}

// TestStartTeamAllLaunchFailsRollback verifies that when all members fail
// in Phase 1, the BootstrapDelayMs sleep is skipped and the new session
// is rolled back via KillSession.
func TestStartTeamAllLaunchFailsRollback(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	deps := startTestDeps(t, configPath)

	var rolledBack []string
	deps.KillSession = func(name string) error {
		rolledBack = append(rolledBack, name)
		return nil
	}

	var sleepDurations []time.Duration
	deps.SleepFn = func(d time.Duration) {
		sleepDurations = append(sleepDurations, d)
	}

	// All launch commands fail (cd commands succeed).
	deps.SendKeys = func(paneID, text string) error {
		if !strings.Contains(text, "cd ") {
			return errors.New("launch failed")
		}
		return nil
	}

	s := NewService(deps)

	if err := s.SaveTeam(TeamDefinition{
		ID:               "team-allfail",
		Name:             "AllFail",
		BootstrapDelayMs: 5000,
		Members: []TeamMember{
			{ID: "m1", PaneTitle: "A", Role: "A", Command: "codex"},
			{ID: "m2", PaneTitle: "B", Role: "B", Command: "codex"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveTeam() error = %v", err)
	}

	// MemberPaneIDs are assigned before the launch attempt, so the function
	// succeeds with warnings. However, injectedAnyCommand stays false so the
	// deferred rollback fires and kills the new session.
	result, err := s.StartTeam(StartTeamRequest{
		TeamID:            "team-allfail",
		LaunchMode:        LaunchModeNewSession,
		SourceSessionName: "src",
	})
	if err != nil {
		t.Fatalf("StartTeam() unexpected error = %v", err)
	}
	if len(result.Warnings) < 2 {
		t.Fatalf("expected at least 2 warnings for failed launches, got %d", len(result.Warnings))
	}

	// Session should be rolled back because no launch command succeeded.
	if len(rolledBack) == 0 {
		t.Fatal("expected KillSession rollback call")
	}

	// BootstrapDelayMs (5s) sleep should NOT appear — only shellInitDelay and cdDelay.
	for _, d := range sleepDurations {
		if d == 5*time.Second {
			t.Fatal("BootstrapDelayMs sleep should be skipped when all launches fail")
		}
	}
}

func TestBuildBootstrapMessageContainsTeamAndRole(t *testing.T) {
	member := TeamMember{
		PaneTitle: "Arch",
		Role:      "Architect",
		Command:   "codex",
	}
	msg := BuildBootstrapMessage("MyTeam", member, "%1", "arch")
	if !strings.Contains(msg, "MyTeam") {
		t.Fatal("missing team name")
	}
	if !strings.Contains(msg, "Architect") {
		t.Fatal("missing role")
	}
	if !strings.Contains(msg, `register_agent(name="arch"`) {
		t.Fatal("missing register_agent call")
	}
}
