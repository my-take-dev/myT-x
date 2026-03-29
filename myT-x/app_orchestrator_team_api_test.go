package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"myT-x/internal/config"
	"myT-x/internal/orchestrator"
	"myT-x/internal/tmux"
)

func newOrchestratorTeamTestApp(t *testing.T) *App {
	t.Helper()

	app := NewApp()
	app.configState.Initialize(filepath.Join(t.TempDir(), "config.yaml"), config.DefaultConfig())
	app.sessions = tmux.NewSessionManager()
	return app
}

func createOrchestratorSourceSession(t *testing.T, app *App, sessionName, rootPath string, paneCount int) tmux.SessionSnapshot {
	t.Helper()

	session, pane, err := app.sessions.CreateSession(sessionName, "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := app.sessions.SetRootPath(sessionName, rootPath); err != nil {
		t.Fatalf("SetRootPath() error = %v", err)
	}
	for created := 1; created < paneCount; created++ {
		if _, err := app.sessions.SplitPane(pane.ID, tmux.SplitVertical); err != nil {
			t.Fatalf("SplitPane() error = %v", err)
		}
	}
	snapshot, ok := app.sessions.GetSession(session.Name)
	if !ok {
		t.Fatalf("GetSession(%q) returned not found", sessionName)
	}
	return tmux.SessionSnapshot{
		ID:             snapshot.ID,
		Name:           snapshot.Name,
		ActiveWindowID: snapshot.ActiveWindowID,
		Windows:        app.sessions.Snapshot()[0].Windows,
		RootPath:       rootPath,
	}
}

// orchestratorStartMocks provides replaceable function hooks for start tests.
// Deps closures point through these fields so tests can swap individual operations.
//
// Design note: default mocks delegate to real App methods intentionally —
// these are integration tests that exercise the full Wails-bound path.
// For isolated unit tests, see internal/orchestrator/service_test.go's testDeps
// which uses t.Fatal stubs to detect unexpected calls.
type orchestratorStartMocks struct {
	sleepFn             func(time.Duration)
	createSession       func(rootPath, sessionName string) (tmux.SessionSnapshot, error)
	createPaneInSession func(sessionName string) (string, error)
	killSession         func(sessionName string) error
	splitPane           func(paneID string, horizontal bool) (string, error)
	renamePane          func(paneID, title string) error
	sendKeys            func(paneID, text string) error
	sendKeysPaste       func(paneID, text string) error
	applyLayoutPreset   func(sessionName, preset string) error
}

// withOrchestratorStartMocks installs a new orchestrator service with replaceable
// mock deps on the given App, restoring the original service on cleanup.
// The returned mocks struct allows tests to swap individual function hooks.
func withOrchestratorStartMocks(t *testing.T, app *App) *orchestratorStartMocks {
	t.Helper()

	m := &orchestratorStartMocks{
		sleepFn: func(time.Duration) {},
		createSession: func(rootPath, sessionName string) (tmux.SessionSnapshot, error) {
			return app.CreateSession(rootPath, sessionName, CreateSessionOptions{})
		},
		createPaneInSession: func(sessionName string) (string, error) {
			return app.CreatePaneInSession(sessionName)
		},
		killSession: func(sessionName string) error {
			return app.KillSession(sessionName, false)
		},
		splitPane: func(paneID string, horizontal bool) (string, error) {
			return app.SplitPane(paneID, horizontal)
		},
		renamePane: func(paneID, title string) error {
			return app.RenamePane(paneID, title)
		},
		sendKeys: func(paneID, text string) error {
			if app.router == nil {
				return errors.New("router not initialized")
			}
			return app.sendKeys.sendKeysLiteralWithEnter(app.router, paneID, text)
		},
		sendKeysPaste: func(paneID, text string) error {
			if app.router == nil {
				return errors.New("router not initialized")
			}
			return app.sendKeys.sendKeysLiteralPasteWithEnter(app.router, paneID, text)
		},
		applyLayoutPreset: func(sessionName, preset string) error {
			return app.ApplyLayoutPreset(sessionName, preset)
		},
	}

	origService := app.orchestratorService
	deps := orchestrator.Deps{
		ConfigPath: func() string { return app.configState.ConfigPath() },
		FindSessionSnapshot: func(sessionName string) (tmux.SessionSnapshot, error) {
			return app.sessionService.FindSessionSnapshotByName(sessionName)
		},
		GetActiveSessionName: app.sessionService.GetActiveSessionName,
		CreateSession: func(rootPath, sessionName string) (tmux.SessionSnapshot, error) {
			return m.createSession(rootPath, sessionName)
		},
		CreatePaneInSession: func(sessionName string) (string, error) {
			return m.createPaneInSession(sessionName)
		},
		KillSession:       func(sessionName string) error { return m.killSession(sessionName) },
		SplitPane:         func(paneID string, horizontal bool) (string, error) { return m.splitPane(paneID, horizontal) },
		RenamePane:        func(paneID, title string) error { return m.renamePane(paneID, title) },
		ApplyLayoutPreset: func(sessionName, preset string) error { return m.applyLayoutPreset(sessionName, preset) },
		SendKeys:          func(paneID, text string) error { return m.sendKeys(paneID, text) },
		SendKeysPaste:     func(paneID, text string) error { return m.sendKeysPaste(paneID, text) },
		SleepFn:           func(d time.Duration) { m.sleepFn(d) },
		CheckReady: func() error {
			if app.router == nil {
				return errors.New("router not initialized")
			}
			return nil
		},
	}
	app.orchestratorService = orchestrator.NewService(deps)
	t.Cleanup(func() { app.orchestratorService = origService })

	return m
}

func TestSaveAndLoadOrchestratorTeams(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)

	err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		Name: " Delivery ",
		Members: []OrchestratorTeamMember{
			{
				PaneTitle:     " Lead ",
				Role:          " Lead engineer ",
				Command:       "codex",
				Args:          []string{" --dangerously-skip-permissions ", "", " --sandbox workspace-write "},
				CustomMessage: " Coordinate the team ",
			},
			{
				PaneTitle: "Reviewer",
				Role:      "Code reviewer",
				Command:   "claude",
			},
		},
	}, "")
	if err != nil {
		t.Fatalf("SaveOrchestratorTeam() error = %v", err)
	}

	teams, err := app.LoadOrchestratorTeams("")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() error = %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("len(teams) = %d, want 1", len(teams))
	}

	team := teams[0]
	if team.Name != "Delivery" {
		t.Fatalf("team.Name = %q, want Delivery", team.Name)
	}
	if team.ID == "" {
		t.Fatal("team.ID should be populated")
	}
	if len(team.Members) != 2 {
		t.Fatalf("len(team.Members) = %d, want 2", len(team.Members))
	}
	if team.Members[0].Order != 0 || team.Members[1].Order != 1 {
		t.Fatalf("member orders = %d,%d, want 0,1", team.Members[0].Order, team.Members[1].Order)
	}
	if team.Members[0].TeamID != team.ID || team.Members[1].TeamID != team.ID {
		t.Fatal("member TeamID should match team ID")
	}
	if !slices.Equal(team.Members[0].Args, []string{"--dangerously-skip-permissions", "--sandbox workspace-write"}) {
		t.Fatalf("member args = %#v, want trimmed non-empty args", team.Members[0].Args)
	}
	if team.BootstrapDelayMs != orchestrator.BootstrapDelayMsDefault {
		t.Fatalf("team.BootstrapDelayMs = %d, want %d", team.BootstrapDelayMs, orchestrator.BootstrapDelayMsDefault)
	}
}

func TestLoadOrchestratorTeamsReturnsEmptyForMalformedJSON(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)
	definitionsPath, membersPath := app.resolveOrchestratorTeamStoragePaths()

	dir := filepath.Dir(definitionsPath)
	if err := mkdirAll(dir); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := writeFile(definitionsPath, []byte("{")); err != nil {
		t.Fatalf("WriteFile(definitions) error = %v", err)
	}
	if err := writeFile(membersPath, []byte("{")); err != nil {
		t.Fatalf("WriteFile(members) error = %v", err)
	}

	teams, err := app.LoadOrchestratorTeams("")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() error = %v", err)
	}
	if len(teams) != 0 {
		t.Fatalf("len(teams) = %d, want 0", len(teams))
	}
}

func TestSaveOrchestratorTeamRefusesMalformedOverwrite(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)
	definitionsPath, _ := app.resolveOrchestratorTeamStoragePaths()

	dir := filepath.Dir(definitionsPath)
	if err := mkdirAll(dir); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := writeFile(definitionsPath, []byte("{")); err != nil {
		t.Fatalf("WriteFile(definitions) error = %v", err)
	}

	err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		Name: "Ops",
		Members: []OrchestratorTeamMember{{
			PaneTitle: "Ops",
			Role:      "Operations",
			Command:   "codex",
		}},
	}, "")
	if err == nil {
		t.Fatal("SaveOrchestratorTeam() expected malformed JSON error")
	}
	if !strings.Contains(err.Error(), "parse team definitions") {
		t.Fatalf("SaveOrchestratorTeam() error = %v, want parse team definitions", err)
	}
}

func TestDeleteOrchestratorTeamRemovesMembersAndOrphans(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)

	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:   "team-a",
		Name: "Alpha",
		Members: []OrchestratorTeamMember{{
			ID:        "member-a",
			PaneTitle: "Alpha",
			Role:      "Lead",
			Command:   "codex",
		}},
	}, ""); err != nil {
		t.Fatalf("SaveOrchestratorTeam(team-a) error = %v", err)
	}
	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:   "team-b",
		Name: "Beta",
		Members: []OrchestratorTeamMember{{
			ID:        "member-b",
			PaneTitle: "Beta",
			Role:      "Reviewer",
			Command:   "claude",
		}},
	}, ""); err != nil {
		t.Fatalf("SaveOrchestratorTeam(team-b) error = %v", err)
	}

	_, membersPath := app.resolveOrchestratorTeamStoragePaths()
	if err := writeFile(membersPath, []byte(`[
  {"id":"member-a","team_id":"team-a","order":0,"pane_title":"Alpha","role":"Lead","command":"codex","args":[],"custom_message":""},
  {"id":"member-b","team_id":"team-b","order":0,"pane_title":"Beta","role":"Reviewer","command":"claude","args":[],"custom_message":""},
  {"id":"orphan","team_id":"missing","order":0,"pane_title":"Ghost","role":"Ghost","command":"codex","args":[],"custom_message":""}
]`)); err != nil {
		t.Fatalf("WriteFile(members) error = %v", err)
	}

	if err := app.DeleteOrchestratorTeam("team-a", "", ""); err != nil {
		t.Fatalf("DeleteOrchestratorTeam() error = %v", err)
	}

	teams, err := app.LoadOrchestratorTeams("")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() error = %v", err)
	}
	if len(teams) != 1 {
		t.Fatalf("len(teams) = %d, want 1", len(teams))
	}
	if teams[0].ID != "team-b" {
		t.Fatalf("remaining team = %q, want team-b", teams[0].ID)
	}
	if len(teams[0].Members) != 1 || teams[0].Members[0].ID != "member-b" {
		t.Fatalf("remaining members = %#v, want member-b only", teams[0].Members)
	}
}

func TestBuildOrchestratorBootstrapMessageIncludesRequiredContext(t *testing.T) {
	member := OrchestratorTeamMember{
		PaneTitle:     "Architect",
		Role:          "System architect",
		Command:       "codex",
		Args:          []string{"--profile", "team"},
		CustomMessage: "Check the release branch first.",
	}

	message := orchestrator.BuildBootstrapMessage("Launch Team", member, "%42", "architect")

	for _, fragment := range []string{
		"Launch Team",
		"役割名: System architect",
		"Check the release branch first.",
		"ペインID: %42",
		`register_agent(name="architect", pane_id="%42", role="System architect")`,
		"$TMUX_PANE",
	} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("bootstrap message missing %q:\n%s", fragment, message)
		}
	}
}

func TestBuildOrchestratorBootstrapMessageWithoutCustomMessage(t *testing.T) {
	member := OrchestratorTeamMember{
		PaneTitle: "Builder",
		Role:      "Implementation engineer",
		Command:   "claude",
	}

	message := orchestrator.BuildBootstrapMessage("Dev Team", member, "%10", "builder")

	if !strings.Contains(message, "役割名: Implementation engineer") {
		t.Fatalf("bootstrap message missing role: %q", message)
	}
	if strings.Contains(message, "Additional") {
		t.Fatalf("bootstrap message should not contain additional section when no custom message: %q", message)
	}
}

func TestBuildOrchestratorBootstrapMessageWithSkills(t *testing.T) {
	member := OrchestratorTeamMember{
		PaneTitle: "Developer",
		Role:      "Backend engineer",
		Command:   "claude",
		Skills: []OrchestratorTeamMemberSkill{
			{Name: "Go", Description: "Goコードのセキュリティ・パフォーマンスレビュー"},
			{Name: "API設計"},
		},
	}

	message := orchestrator.BuildBootstrapMessage("Dev Team", member, "%5", "developer")

	for _, fragment := range []string{
		"得意分野:",
		"- Go: Goコードのセキュリティ・パフォーマンスレビュー",
		"- API設計",
		`role="Backend engineer"`,
		`skills=[{"name":"Go","description":"Goコードのセキュリティ・パフォーマンスレビュー"},{"name":"API設計"}]`,
	} {
		if !strings.Contains(message, fragment) {
			t.Fatalf("bootstrap message missing %q:\n%s", fragment, message)
		}
	}
}

func TestBuildOrchestratorBootstrapMessageWithoutSkills(t *testing.T) {
	member := OrchestratorTeamMember{
		PaneTitle: "Worker",
		Role:      "General worker",
		Command:   "claude",
	}

	message := orchestrator.BuildBootstrapMessage("Team", member, "%3", "worker")

	if strings.Contains(message, "skills=") {
		t.Fatalf("register_agent should not contain skills param when no skills:\n%s", message)
	}
	if !strings.Contains(message, "得意分野の補完") {
		t.Fatalf("bootstrap should contain skill completion hints when no skills:\n%s", message)
	}
	if !strings.Contains(message, "得意分野（skills）が未設定です") {
		t.Fatalf("bootstrap should contain pattern A hint:\n%s", message)
	}
}

func TestBuildSkillCompletionHints(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		skills         []OrchestratorTeamMemberSkill
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:         "empty skills → pattern A",
			role:         "Backend engineer",
			skills:       nil,
			wantContains: []string{"得意分野（skills）が未設定です", "Backend engineer"},
		},
		{
			name:         "skills with empty description → pattern B",
			role:         "Frontend dev",
			skills:       []OrchestratorTeamMemberSkill{{Name: "React"}, {Name: "CSS"}},
			wantContains: []string{"説明（description）がありません", "Frontend dev", "得意分野が少ない可能性があります"},
		},
		{
			name: "few skills with all descriptions → pattern C only",
			role: "Tester",
			skills: []OrchestratorTeamMemberSkill{
				{Name: "Unit test", Description: "Go unit testing"},
			},
			wantContains:   []string{"得意分野が少ない可能性があります", "Tester"},
			wantNotContain: []string{"説明（description）がありません"},
		},
		{
			name: "3+ skills with all descriptions → no hints",
			role: "Full stack",
			skills: []OrchestratorTeamMemberSkill{
				{Name: "Go", Description: "backend"},
				{Name: "React", Description: "frontend"},
				{Name: "SQL", Description: "database"},
			},
			wantContains:   nil,
			wantNotContain: []string{"得意分野"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints := orchestrator.BuildSkillCompletionHints(tt.role, tt.skills)
			for _, want := range tt.wantContains {
				if !strings.Contains(hints, want) {
					t.Errorf("hints missing %q:\n%s", want, hints)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(hints, notWant) {
					t.Errorf("hints should not contain %q:\n%s", notWant, hints)
				}
			}
		})
	}
}

func TestMemberValidateRoleLength(t *testing.T) {
	m := &OrchestratorTeamMember{
		ID: "m1", TeamID: "t1", PaneTitle: "Dev", Command: "claude",
	}

	// 50 chars: OK
	m.Role = strings.Repeat("あ", 50)
	if err := m.Validate(); err != nil {
		t.Fatalf("expected valid with 50 char role: %v", err)
	}

	// 51 chars: NG
	m.Role = strings.Repeat("あ", 51)
	if err := m.Validate(); err == nil {
		t.Fatal("expected error with 51 char role")
	}
}

func TestMemberValidateSkillsLimits(t *testing.T) {
	m := &OrchestratorTeamMember{
		ID: "m1", TeamID: "t1", PaneTitle: "Dev", Role: "Dev", Command: "claude",
	}
	for i := range 20 {
		m.Skills = append(m.Skills, OrchestratorTeamMemberSkill{
			Name: fmt.Sprintf("skill-%d", i),
		})
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("expected valid with 20 skills: %v", err)
	}

	m.Skills = append(m.Skills, OrchestratorTeamMemberSkill{Name: "overflow"})
	if err := m.Validate(); err == nil {
		t.Fatal("expected error with 21 skills")
	}
}

func TestDeriveOrchestratorAgentNamesSanitizesAndDeduplicates(t *testing.T) {
	names := orchestrator.DeriveAgentNames([]OrchestratorTeamMember{
		{ID: "a", PaneTitle: "Lead Engineer"},
		{ID: "b", PaneTitle: "Lead Engineer"},
		{ID: "c", PaneTitle: "  "},
	})

	if names["a"] != "lead-engineer" {
		t.Fatalf("names[a] = %q, want lead-engineer", names["a"])
	}
	if names["b"] != "lead-engineer-2" {
		t.Fatalf("names[b] = %q, want lead-engineer-2", names["b"])
	}
	if names["c"] != "member" {
		t.Fatalf("names[c] = %q, want member", names["c"])
	}
}

func TestStartOrchestratorTeamActiveSessionUsesExistingPanesAndReturnsWarnings(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)
	source := createOrchestratorSourceSession(t, app, "src", `C:\repo`, 2)

	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:   "team-1",
		Name: "Delivery",
		Members: []OrchestratorTeamMember{
			{ID: "m1", PaneTitle: "Lead", Role: "Lead", Command: "codex"},
			{ID: "m2", PaneTitle: "Builder", Role: "Builder", Command: "codex"},
			{ID: "m3", PaneTitle: "Reviewer", Role: "Reviewer", Command: "codex"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveOrchestratorTeam() error = %v", err)
	}

	m := withOrchestratorStartMocks(t, app)
	m.splitPane = func(paneID string, horizontal bool) (string, error) {
		return "", errors.New("split failed")
	}
	renamed := make([]string, 0)
	sent := make([]string, 0)
	m.renamePane = func(paneID, title string) error {
		renamed = append(renamed, paneID+":"+title)
		return nil
	}
	m.sendKeys = func(paneID, text string) error {
		sent = append(sent, paneID+":"+strings.TrimSpace(text))
		return nil
	}

	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	result, err := app.StartOrchestratorTeam(StartOrchestratorTeamRequest{
		TeamID:            "team-1",
		LaunchMode:        orchestrator.LaunchModeActiveSession,
		SourceSessionName: source.Name,
	})
	if err != nil {
		t.Fatalf("StartOrchestratorTeam() error = %v", err)
	}

	if result.SessionName != "src" {
		t.Fatalf("result.SessionName = %q, want src", result.SessionName)
	}
	if len(result.MemberPaneIDs) != 2 {
		t.Fatalf("len(result.MemberPaneIDs) = %d, want 2", len(result.MemberPaneIDs))
	}
	if len(result.Warnings) < 2 {
		t.Fatalf("len(result.Warnings) = %d, want at least 2 warnings", len(result.Warnings))
	}
	if len(renamed) != 2 {
		t.Fatalf("len(renamed) = %d, want 2", len(renamed))
	}
	if len(sent) != 6 {
		t.Fatalf("len(sent) = %d, want 6 (cd + launch + bootstrap for 2 members)", len(sent))
	}
}

func TestStartOrchestratorTeamNewSessionCreatesPanesAndAppliesLayout(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)
	source := createOrchestratorSourceSession(t, app, "src", `C:\repo`, 1)

	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:   "team-2",
		Name: "Design Review",
		Members: []OrchestratorTeamMember{
			{ID: "m1", PaneTitle: "Lead", Role: "Lead", Command: "codex", Args: []string{"--sandbox", "workspace-write"}},
			{ID: "m2", PaneTitle: "Reviewer", Role: "Reviewer", Command: "claude"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveOrchestratorTeam() error = %v", err)
	}

	m := withOrchestratorStartMocks(t, app)
	m.createSession = func(rootPath, sessionName string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{
			Name:           "design-review-2",
			RootPath:       rootPath,
			ActiveWindowID: 1,
			Windows: []tmux.WindowSnapshot{{
				ID:    1,
				Name:  "main",
				Panes: []tmux.PaneSnapshot{{ID: "%11", Index: 0}},
			}},
		}, nil
	}
	m.splitPane = func(paneID string, horizontal bool) (string, error) {
		return "%12", nil
	}
	layouts := make([]string, 0)
	m.applyLayoutPreset = func(sessionName, preset string) error {
		layouts = append(layouts, sessionName+":"+preset)
		return nil
	}
	sent := make([]string, 0)
	m.sendKeys = func(paneID, text string) error {
		sent = append(sent, paneID+":"+strings.TrimSpace(text))
		return nil
	}
	pastedSent := make([]string, 0)
	m.sendKeysPaste = func(paneID, text string) error {
		pastedSent = append(pastedSent, paneID+":"+strings.TrimSpace(text))
		return nil
	}

	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	result, err := app.StartOrchestratorTeam(StartOrchestratorTeamRequest{
		TeamID:            "team-2",
		LaunchMode:        orchestrator.LaunchModeNewSession,
		SourceSessionName: source.Name,
	})
	if err != nil {
		t.Fatalf("StartOrchestratorTeam() error = %v", err)
	}

	if result.SessionName != "design-review-2" {
		t.Fatalf("result.SessionName = %q, want design-review-2", result.SessionName)
	}
	if len(layouts) != 1 || layouts[0] != "design-review-2:tiled" {
		t.Fatalf("layouts = %#v, want [design-review-2:tiled]", layouts)
	}
	if len(result.MemberPaneIDs) != 2 {
		t.Fatalf("len(result.MemberPaneIDs) = %d, want 2", len(result.MemberPaneIDs))
	}
	totalSent := len(sent) + len(pastedSent)
	if totalSent != 6 {
		t.Fatalf("total sent = %d (regular=%d, paste=%d), want 6 (cd + launch + bootstrap for 2 members)", totalSent, len(sent), len(pastedSent))
	}
	if len(pastedSent) != 1 {
		t.Fatalf("len(pastedSent) = %d, want 1 (bootstrap for claude member)", len(pastedSent))
	}
	if !strings.Contains(sent[0], `cd "`) {
		t.Fatalf("first send-keys should be cd command: %q", sent[0])
	}
	if !strings.Contains(sent[1], `--sandbox`) {
		t.Fatalf("second send-keys should be launch command with args: %q", sent[1])
	}
}

func TestStartOrchestratorTeamNewSessionRollsBackBeforeFirstCommand(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)
	source := createOrchestratorSourceSession(t, app, "src", `C:\repo`, 1)

	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:   "team-3",
		Name: "Broken Launch",
		Members: []OrchestratorTeamMember{
			{ID: "m1", PaneTitle: "Lead", Role: "Lead", Command: "codex"},
			{ID: "m2", PaneTitle: "Reviewer", Role: "Reviewer", Command: "claude"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveOrchestratorTeam() error = %v", err)
	}

	m := withOrchestratorStartMocks(t, app)
	m.createSession = func(rootPath, sessionName string) (tmux.SessionSnapshot, error) {
		return tmux.SessionSnapshot{
			Name:           "broken-launch",
			RootPath:       rootPath,
			ActiveWindowID: 1,
			Windows: []tmux.WindowSnapshot{{
				ID:    1,
				Name:  "main",
				Panes: []tmux.PaneSnapshot{{ID: "%31", Index: 0}},
			}},
		}, nil
	}
	m.splitPane = func(paneID string, horizontal bool) (string, error) {
		return "", errors.New("split failed")
	}
	rolledBack := make([]string, 0)
	m.killSession = func(sessionName string) error {
		rolledBack = append(rolledBack, sessionName)
		return nil
	}

	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	_, err := app.StartOrchestratorTeam(StartOrchestratorTeamRequest{
		TeamID:            "team-3",
		LaunchMode:        orchestrator.LaunchModeNewSession,
		SourceSessionName: source.Name,
	})
	if err == nil {
		t.Fatal("StartOrchestratorTeam() expected error")
	}
	if len(rolledBack) != 1 || rolledBack[0] != "broken-launch" {
		t.Fatalf("rolledBack = %#v, want [broken-launch]", rolledBack)
	}
}

func TestReorderOrchestratorTeams(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)

	for _, team := range []OrchestratorTeamDefinition{
		{ID: "team-a", Name: "Alpha", Members: []OrchestratorTeamMember{{PaneTitle: "A", Role: "A", Command: "codex"}}},
		{ID: "team-b", Name: "Beta", Members: []OrchestratorTeamMember{{PaneTitle: "B", Role: "B", Command: "codex"}}},
		{ID: "team-c", Name: "Charlie", Members: []OrchestratorTeamMember{{PaneTitle: "C", Role: "C", Command: "codex"}}},
	} {
		if err := app.SaveOrchestratorTeam(team, ""); err != nil {
			t.Fatalf("SaveOrchestratorTeam(%s) error = %v", team.ID, err)
		}
	}

	teams, err := app.LoadOrchestratorTeams("")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() error = %v", err)
	}
	if len(teams) != 3 {
		t.Fatalf("len(teams) = %d, want 3", len(teams))
	}
	if teams[0].Name != "Alpha" || teams[1].Name != "Beta" || teams[2].Name != "Charlie" {
		t.Fatalf("initial order = %q,%q,%q, want Alpha,Beta,Charlie", teams[0].Name, teams[1].Name, teams[2].Name)
	}

	if err := app.ReorderOrchestratorTeams([]string{"team-c", "team-a", "team-b"}, "", ""); err != nil {
		t.Fatalf("ReorderOrchestratorTeams() error = %v", err)
	}

	teams, err = app.LoadOrchestratorTeams("")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() after reorder error = %v", err)
	}
	if teams[0].Name != "Charlie" || teams[1].Name != "Alpha" || teams[2].Name != "Beta" {
		t.Fatalf("reordered = %q,%q,%q, want Charlie,Alpha,Beta", teams[0].Name, teams[1].Name, teams[2].Name)
	}
	if teams[0].Order != 0 || teams[1].Order != 1 || teams[2].Order != 2 {
		t.Fatalf("orders = %d,%d,%d, want 0,1,2", teams[0].Order, teams[1].Order, teams[2].Order)
	}
}

func TestOrchestratorTeamBootstrapDelayMsNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"zero defaults to BootstrapDelayMsDefault", 0, orchestrator.BootstrapDelayMsDefault},
		{"negative defaults to BootstrapDelayMsDefault", -1, orchestrator.BootstrapDelayMsDefault},
		{"custom value preserved", 7000, 7000},
		{"minimum preserved", 1000, 1000},
		{"maximum preserved", 30000, 30000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			team := &OrchestratorTeamDefinition{
				Name:             "Test",
				BootstrapDelayMs: tt.input,
				Members: []OrchestratorTeamMember{
					{PaneTitle: "A", Role: "A", Command: "codex"},
				},
			}
			team.Normalize()
			if team.BootstrapDelayMs != tt.expected {
				t.Fatalf("BootstrapDelayMs = %d, want %d", team.BootstrapDelayMs, tt.expected)
			}
		})
	}
}

func TestOrchestratorTeamBootstrapDelayMsValidate(t *testing.T) {
	base := func(delayMs int) *OrchestratorTeamDefinition {
		return &OrchestratorTeamDefinition{
			ID:               "t1",
			Name:             "Test",
			BootstrapDelayMs: delayMs,
			Members: []OrchestratorTeamMember{
				{ID: "m1", TeamID: "t1", PaneTitle: "A", Role: "A", Command: "codex"},
			},
		}
	}

	for _, ms := range []int{1000, 3000, 15000, 30000} {
		if err := base(ms).Validate(); err != nil {
			t.Fatalf("expected valid with %dms: %v", ms, err)
		}
	}

	for _, ms := range []int{0, 999, 30001, -1} {
		if err := base(ms).Validate(); err == nil {
			t.Fatalf("expected error with %dms", ms)
		}
	}
}

func TestStartOrchestratorTeamUsesCustomBootstrapDelay(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)
	source := createOrchestratorSourceSession(t, app, "src", `C:\repo`, 1)

	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:               "team-delay",
		Name:             "DelayTest",
		BootstrapDelayMs: 5000,
		Members: []OrchestratorTeamMember{
			{ID: "m1", PaneTitle: "Lead", Role: "Lead", Command: "codex"},
		},
	}, ""); err != nil {
		t.Fatalf("SaveOrchestratorTeam() error = %v", err)
	}

	m := withOrchestratorStartMocks(t, app)
	var sleepDurations []time.Duration
	m.sleepFn = func(d time.Duration) {
		sleepDurations = append(sleepDurations, d)
	}
	m.sendKeys = func(paneID, text string) error { return nil }

	app.router = tmux.NewCommandRouter(app.sessions, nil, tmux.RouterOptions{})
	_, err := app.StartOrchestratorTeam(StartOrchestratorTeamRequest{
		TeamID:            "team-delay",
		LaunchMode:        orchestrator.LaunchModeActiveSession,
		SourceSessionName: source.Name,
	})
	if err != nil {
		t.Fatalf("StartOrchestratorTeam() error = %v", err)
	}

	if !slices.Contains(sleepDurations, 5*time.Second) {
		t.Fatalf("expected 5s bootstrap delay in sleep durations: %v", sleepDurations)
	}
}

func TestQuoteOrchestratorCommandArg(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{name: "empty string", arg: "", want: `""`},
		{name: "no special chars", arg: "--flag", want: "--flag"},
		{name: "contains space", arg: "hello world", want: `"hello world"`},
		{name: "contains tab", arg: "hello\tworld", want: "\"hello\tworld\""},
		{name: "contains double quote", arg: `say "hi"`, want: `"say \"hi\""`},
		{name: "backslash before quote", arg: `path\"end`, want: `"path\\\"end"`},
		{name: "trailing backslash no special chars", arg: `path\`, want: `path\`},
		{name: "trailing backslash with space", arg: `path to\`, want: `"path to\\"`},
		{name: "no quoting needed for simple flag", arg: "--sandbox", want: "--sandbox"},
		{name: "space at start", arg: " leading", want: `" leading"`},
		{name: "multiple spaces", arg: "a b c", want: `"a b c"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := orchestrator.QuoteCommandArg(tt.arg)
			if got != tt.want {
				t.Fatalf("QuoteCommandArg(%q) = %q, want %q", tt.arg, got, tt.want)
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
		{name: "claude", command: "claude", want: true},
		{name: "claude.exe", command: "claude.exe", want: true},
		{name: "absolute path claude", command: "/usr/bin/claude", want: true},
		{name: "windows path no space claude.exe", command: `C:\Tools\claude.exe`, want: true},
		{name: "claude-code", command: "claude-code", want: true},
		{name: "claude-code-v2", command: "claude-code-v2", want: true},
		{name: "codex is not claude", command: "codex", want: false},
		{name: "empty command", command: "", want: false},
		{name: "whitespace only", command: "   ", want: false},
		{name: "claude with args ignored", command: "claude --model opus", want: true},
		{name: "CLAUDE uppercase", command: "CLAUDE", want: true},
		{name: "Claude mixed case", command: "Claude", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := orchestrator.IsClaudeCommand(tt.command)
			if got != tt.want {
				t.Fatalf("IsClaudeCommand(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestReorderOrchestratorTeamsDuplicateID(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)

	for _, team := range []OrchestratorTeamDefinition{
		{ID: "team-a", Name: "Alpha", Members: []OrchestratorTeamMember{{PaneTitle: "A", Role: "A", Command: "codex"}}},
		{ID: "team-b", Name: "Beta", Members: []OrchestratorTeamMember{{PaneTitle: "B", Role: "B", Command: "codex"}}},
	} {
		if err := app.SaveOrchestratorTeam(team, ""); err != nil {
			t.Fatalf("SaveOrchestratorTeam(%s) error = %v", team.ID, err)
		}
	}

	err := app.ReorderOrchestratorTeams([]string{"team-a", "team-b", "team-a"}, "", "")
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			t.Logf("ReorderOrchestratorTeams with duplicate ID returned: %v", err)
		}
	}
}

func TestReorderOrchestratorTeamsSubsetIDs(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)

	for _, team := range []OrchestratorTeamDefinition{
		{ID: "team-a", Name: "Alpha", Members: []OrchestratorTeamMember{{PaneTitle: "A", Role: "A", Command: "codex"}}},
		{ID: "team-b", Name: "Beta", Members: []OrchestratorTeamMember{{PaneTitle: "B", Role: "B", Command: "codex"}}},
		{ID: "team-c", Name: "Charlie", Members: []OrchestratorTeamMember{{PaneTitle: "C", Role: "C", Command: "codex"}}},
	} {
		if err := app.SaveOrchestratorTeam(team, ""); err != nil {
			t.Fatalf("SaveOrchestratorTeam(%s) error = %v", team.ID, err)
		}
	}

	if err := app.ReorderOrchestratorTeams([]string{"team-b", "team-a"}, "", ""); err != nil {
		t.Fatalf("ReorderOrchestratorTeams() error = %v", err)
	}

	teams, err := app.LoadOrchestratorTeams("")
	if err != nil {
		t.Fatalf("LoadOrchestratorTeams() error = %v", err)
	}
	if len(teams) != 3 {
		t.Fatalf("len(teams) = %d, want 3", len(teams))
	}

	if teams[0].Name != "Beta" {
		t.Fatalf("teams[0].Name = %q, want Beta", teams[0].Name)
	}
	if teams[1].Name != "Alpha" {
		t.Fatalf("teams[1].Name = %q, want Alpha", teams[1].Name)
	}
	if teams[2].Name != "Charlie" {
		t.Fatalf("teams[2].Name = %q, want Charlie", teams[2].Name)
	}
}

func TestReorderOrchestratorTeamsRejectsUnknownID(t *testing.T) {
	app := newOrchestratorTeamTestApp(t)

	if err := app.SaveOrchestratorTeam(OrchestratorTeamDefinition{
		ID:      "team-a",
		Name:    "Alpha",
		Members: []OrchestratorTeamMember{{PaneTitle: "A", Role: "A", Command: "codex"}},
	}, ""); err != nil {
		t.Fatalf("SaveOrchestratorTeam() error = %v", err)
	}

	err := app.ReorderOrchestratorTeams([]string{"team-a", "unknown"}, "", "")
	if err == nil {
		t.Fatal("ReorderOrchestratorTeams() expected error for unknown ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v, want 'not found'", err)
	}
}

// resolveOrchestratorTeamStoragePaths returns the global storage paths
// for team definitions and members. Test-only helper.
func (a *App) resolveOrchestratorTeamStoragePaths() (string, string) {
	return a.orchestratorService.ResolveGlobalStoragePaths()
}

// Test helpers for file operations with unified permissions.
func mkdirAll(path string) error {
	return os.MkdirAll(path, 0o755)
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}
