package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// testSplitter はテスト用の PaneSplitter。
type testSplitter struct {
	newPaneID string
	err       error
	called    bool
	lastPane  string
	lastHoriz bool
}

func (s *testSplitter) SplitPane(_ context.Context, targetPaneID string, horizontal bool) (string, error) {
	s.called = true
	s.lastPane = targetPaneID
	s.lastHoriz = horizontal
	if s.err != nil {
		return "", s.err
	}
	return s.newPaneID, nil
}

// testPasteSender はテスト用の PanePasteSender。
type testPasteSender struct {
	err    error
	called bool
	sent   []sentCall
}

func (p *testPasteSender) SendKeysPaste(_ context.Context, paneID string, text string) error {
	p.called = true
	if p.err != nil {
		return p.err
	}
	p.sent = append(p.sent, sentCall{paneID: paneID, text: text})
	return nil
}

// testTitleSetter はテスト用の PaneTitleSetter。
type testTitleSetter struct {
	err       error
	called    bool
	lastPane  string
	lastTitle string
}

func (t *testTitleSetter) SetPaneTitle(_ context.Context, paneID string, title string) error {
	t.called = true
	t.lastPane = paneID
	t.lastTitle = title
	return t.err
}

type memberBootstrapDeps struct {
	agents      *testAgentRepo
	paneOps     *testPaneOps
	splitter    *testSplitter
	titleSetter *testTitleSetter
	pasteSender *testPasteSender
}

func newMemberBootstrapDeps() memberBootstrapDeps {
	return memberBootstrapDeps{
		agents:      newTestAgentRepo(),
		paneOps:     &testPaneOps{selfPane: "%1"},
		splitter:    &testSplitter{newPaneID: "%5"},
		titleSetter: &testTitleSetter{},
		pasteSender: &testPasteSender{},
	}
}

func buildMemberBootstrapService(d memberBootstrapDeps) *MemberBootstrapService {
	svc := NewMemberBootstrapService(
		d.agents, d.paneOps, d.splitter, d.titleSetter, d.paneOps, d.pasteSender,
		"/project/root", discardLogger(),
	)
	// テスト時は sleep をスキップ
	svc.SetSleepFn(func(_ context.Context, _ time.Duration) error { return nil })
	return svc
}

func registerTestCaller(repo *testAgentRepo, name, paneID string) {
	repo.agents[name] = domain.Agent{Name: name, PaneID: paneID}
}

func TestAddMemberHappyPath(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")
	svc := buildMemberBootstrapService(d)

	result, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle: "worker",
		Role:      "コード実装",
		Command:   "claude",
		Args:      []string{"--model", "sonnet"},
		TeamName:  "テストチーム",
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if result.PaneID != "%5" {
		t.Fatalf("PaneID = %q, want %%5", result.PaneID)
	}
	if result.PaneTitle != "worker" {
		t.Fatalf("PaneTitle = %q, want worker", result.PaneTitle)
	}
	if result.AgentName != "worker" {
		t.Fatalf("AgentName = %q, want worker", result.AgentName)
	}

	// splitter が正しく呼ばれたか
	if !d.splitter.called {
		t.Fatal("splitter should be called")
	}
	if d.splitter.lastPane != "%1" {
		t.Fatalf("split from = %q, want %%1", d.splitter.lastPane)
	}
	if !d.splitter.lastHoriz {
		t.Fatal("should split horizontally by default")
	}

	// titleSetter が呼ばれたか
	if !d.titleSetter.called {
		t.Fatal("titleSetter should be called")
	}
	if d.titleSetter.lastTitle != "worker" {
		t.Fatalf("title = %q, want worker", d.titleSetter.lastTitle)
	}

	// SendKeys でcd + launchが送られたか
	if len(d.paneOps.sent) < 2 {
		t.Fatalf("expected at least 2 SendKeys calls, got %d", len(d.paneOps.sent))
	}
	cdCall := d.paneOps.sent[0]
	if !strings.Contains(cdCall.text, "/project/root") {
		t.Fatalf("cd command = %q, want to contain /project/root", cdCall.text)
	}
	launchCall := d.paneOps.sent[1]
	if !strings.Contains(launchCall.text, "claude") {
		t.Fatalf("launch command = %q, want to contain claude", launchCall.text)
	}

	// Claude コマンドなのでPasteSenderが呼ばれるべき
	if !d.pasteSender.called {
		t.Fatal("pasteSender should be called for claude command")
	}

	// warnings なし
	if len(result.Warnings) != 0 {
		t.Fatalf("Warnings = %v, want empty", result.Warnings)
	}
}

func TestAddMemberClaudeDetection(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		wantPaste bool
	}{
		{"claude", "claude", true},
		{"claude.exe", "claude.exe", true},
		{"claude.cmd", "claude.cmd", true},
		{"claude-code-agent", "claude-code-agent", true},
		{"python", "python", false},
		{"node", "node", false},
		{"path/to/claude", "/usr/local/bin/claude", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newMemberBootstrapDeps()
			registerTestCaller(d.agents, "orch", "%1")
			svc := buildMemberBootstrapService(d)

			_, err := svc.AddMember(context.Background(), AddMemberCmd{
				PaneTitle: "test",
				Role:      "test",
				Command:   tt.command,
			})
			if err != nil {
				t.Fatalf("AddMember: %v", err)
			}

			if tt.wantPaste && !d.pasteSender.called {
				t.Fatal("expected pasteSender to be called")
			}
			if !tt.wantPaste && d.pasteSender.called {
				t.Fatal("expected pasteSender NOT to be called")
			}
		})
	}
}

func TestAddMemberSplitFromDefault(t *testing.T) {
	d := newMemberBootstrapDeps()
	d.paneOps.selfPane = "%3"
	registerTestCaller(d.agents, "caller", "%3")
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle: "test",
		Role:      "test",
		Command:   "python",
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if d.splitter.lastPane != "%3" {
		t.Fatalf("split from = %q, want %%3 (caller pane)", d.splitter.lastPane)
	}
}

func TestAddMemberSplitFromExplicit(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orch", "%1")
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle: "test",
		Role:      "test",
		Command:   "python",
		SplitFrom: "%7",
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if d.splitter.lastPane != "%7" {
		t.Fatalf("split from = %q, want %%7 (explicit)", d.splitter.lastPane)
	}
}

func TestAddMemberSplitFailure(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orch", "%1")
	d.splitter.err = errors.New("no space for split")
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle: "test",
		Role:      "test",
		Command:   "python",
	})
	if err == nil {
		t.Fatal("expected error when split fails")
	}
}

func TestAddMemberVerticalSplit(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orch", "%1")
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle:      "test",
		Role:           "test",
		Command:        "python",
		SplitDirection: "vertical",
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if d.splitter.lastHoriz {
		t.Fatal("should split vertically")
	}
}

func TestAddMemberTitleFailureIsWarning(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orch", "%1")
	d.titleSetter.err = errors.New("title set failed")
	svc := buildMemberBootstrapService(d)

	result, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle: "test",
		Role:      "test",
		Command:   "python",
	})
	if err != nil {
		t.Fatalf("AddMember should not fail: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for title failure")
	}
	if !strings.Contains(result.Warnings[0], "pane title") {
		t.Fatalf("warning = %q, want to contain 'pane title'", result.Warnings[0])
	}
}

func TestAddMemberBootstrapFailureIsWarning(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orch", "%1")
	d.pasteSender.err = errors.New("paste failed")
	svc := buildMemberBootstrapService(d)

	result, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle: "test",
		Role:      "test",
		Command:   "claude", // Claude → uses pasteSender
	})
	if err != nil {
		t.Fatalf("AddMember should not fail: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Fatal("expected warning for bootstrap failure")
	}
	if !strings.Contains(result.Warnings[0], "bootstrap") {
		t.Fatalf("warning = %q, want to contain 'bootstrap'", result.Warnings[0])
	}
}

func TestAddMemberUnregisteredAgentRejected(t *testing.T) {
	d := newMemberBootstrapDeps()
	// 登録しない
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle: "test",
		Role:      "test",
		Command:   "python",
	})
	if err == nil {
		t.Fatal("expected error for unregistered agent")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Fatalf("error = %q, want to contain 'not registered'", err.Error())
	}
}

func TestAddMemberTrustedCallerWithoutSplitFrom(t *testing.T) {
	d := newMemberBootstrapDeps()
	d.paneOps.selfPane = "" // pipe bridge mode → trusted caller
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle: "test",
		Role:      "test",
		Command:   "python",
	})
	if err == nil {
		t.Fatal("expected error for trusted caller without split_from")
	}
	if !strings.Contains(err.Error(), "split_from is required") {
		t.Fatalf("error = %q, want to contain 'split_from is required'", err.Error())
	}
}

func TestAddMemberTrustedCallerWithSplitFrom(t *testing.T) {
	d := newMemberBootstrapDeps()
	d.paneOps.selfPane = "" // pipe bridge mode → trusted caller
	svc := buildMemberBootstrapService(d)

	result, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle: "test",
		Role:      "test",
		Command:   "python",
		SplitFrom: "%2",
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if d.splitter.lastPane != "%2" {
		t.Fatalf("split from = %q, want %%2", d.splitter.lastPane)
	}
	if result.PaneID != "%5" {
		t.Fatalf("PaneID = %q, want %%5", result.PaneID)
	}
}

func TestAddMemberCustomMessage(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orch", "%1")
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle:     "test",
		Role:          "test",
		Command:       "claude",
		CustomMessage: "テストのカスタムメッセージ",
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if len(d.pasteSender.sent) == 0 {
		t.Fatal("expected pasteSender to be called")
	}
	bootstrapMsg := d.pasteSender.sent[0].text
	if !strings.Contains(bootstrapMsg, "テストのカスタムメッセージ") {
		t.Fatalf("bootstrap message should contain custom message, got: %s", bootstrapMsg)
	}
}

func TestAddMemberWithSkills(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orch", "%1")
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle: "test",
		Role:      "test",
		Command:   "claude",
		Skills: []domain.Skill{
			{Name: "Go", Description: "バックエンド"},
			{Name: "React"},
		},
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if len(d.pasteSender.sent) == 0 {
		t.Fatal("expected pasteSender to be called")
	}
	msg := d.pasteSender.sent[0].text
	if !strings.Contains(msg, "Go") || !strings.Contains(msg, "バックエンド") {
		t.Fatalf("bootstrap message should contain skills, got: %s", msg)
	}
	if !strings.Contains(msg, "React") {
		t.Fatalf("bootstrap message should contain React skill, got: %s", msg)
	}
}

func TestClampBootstrapDelay(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"zero defaults", 0, BootstrapDelayDefault},
		{"negative defaults", -1, BootstrapDelayDefault},
		{"below minimum", 500, BootstrapDelayMin},
		{"at minimum", 1000, 1000},
		{"above maximum", 99999, BootstrapDelayMax},
		{"at maximum", 30000, 30000},
		{"normal", 5000, 5000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampBootstrapDelay(tt.input)
			if got != tt.want {
				t.Fatalf("clampBootstrapDelay(%d) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeMemberAgentName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercase", "Worker", "worker"},
		{"spaces", "My Worker", "my-worker"},
		{"special chars", "Worker@#1", "worker-1"},
		{"leading trailing", "  -Worker-  ", "worker"},
		{"empty", "", "member"},
		{"all special", "@#$%", "member"},
		{"japanese", "コードレビュー", "member"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeMemberAgentName(tt.input)
			if got != tt.want {
				t.Fatalf("sanitizeMemberAgentName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsClaudeCLI(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"claude", "claude", true},
		{"claude.exe", "claude.exe", true},
		{"claude.cmd", "claude.cmd", true},
		{"claude-code", "claude-code", true},
		{"claude-code-agent", "claude-code-agent", true},
		{"python", "python", false},
		{"empty", "", false},
		{"path", "/usr/bin/claude", true},
		{"windows path", `C:\Users\bin\claude.exe`, true},
		{"windows cmd path", `C:\Users\bin\claude.cmd`, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isClaudeCLI(tt.command)
			if got != tt.want {
				t.Fatalf("isClaudeCLI(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestBuildMemberLaunchCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{"no args", "claude", nil, "claude"},
		{"with args", "claude", []string{"--model", "sonnet"}, "claude --model sonnet"},
		{"args with spaces", "python", []string{"-c", "print('hello world')"}, `python -c "print('hello world')"`},
		{"arg with quote", "echo", []string{`say "hello"`}, `echo "say \"hello\""`},
		{"arg with backslash", "echo", []string{`C:\path\to`}, `echo C:\path\to`},
		{"arg with backslash and quote", "echo", []string{`path\"x`}, `echo "path\\\"x"`},
		{"empty arg", "cmd", []string{""}, `cmd ""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildMemberLaunchCommand(tt.command, tt.args)
			if got != tt.want {
				t.Fatalf("buildMemberLaunchCommand = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildMemberBootstrapMessage(t *testing.T) {
	cmd := AddMemberCmd{
		PaneTitle:     "worker",
		Role:          "コード実装",
		Command:       "claude",
		CustomMessage: "テスト指示",
		Skills: []domain.Skill{
			{Name: "Go", Description: "バックエンド開発"},
		},
	}
	msg := buildMemberBootstrapMessage("テストチーム", cmd, "%5", "worker")

	checks := []string{
		"テストチーム",
		"コード実装",
		"テスト指示",
		"Go",
		"バックエンド開発",
		"register_agent",
		"%5",
		"worker",
		"ワークフロー",
	}
	for _, check := range checks {
		if !strings.Contains(msg, check) {
			t.Fatalf("bootstrap message should contain %q, got: %s", check, msg)
		}
	}
}

func TestBuildMemberBootstrapMessageEscapesRole(t *testing.T) {
	cmd := AddMemberCmd{
		PaneTitle: "test",
		Role:      `backend "dev"` + "\ninjection",
		Command:   "claude",
	}
	msg := buildMemberBootstrapMessage("team", cmd, "%5", "test")

	// register_agent 行の role 内で " がエスケープされていること
	if strings.Contains(msg, `role="backend "dev""`) {
		t.Fatal("role double-quotes should be escaped in register_agent call")
	}
	if !strings.Contains(msg, `role="backend \"dev\" injection"`) {
		t.Fatalf("role should be escaped, got: %s", msg)
	}
	// 改行が除去され1行内に収まること
	for line := range strings.SplitSeq(msg, "\n") {
		if strings.HasPrefix(line, "register_agent(") {
			if strings.Contains(line, "\n") {
				t.Fatal("register_agent line should not contain newlines")
			}
			break
		}
	}
}

func TestAddMemberDefaultTeamName(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orch", "%1")
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle: "test",
		Role:      "test",
		Command:   "claude",
		// TeamName 未指定
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	if len(d.pasteSender.sent) == 0 {
		t.Fatal("expected pasteSender to be called")
	}
	msg := d.pasteSender.sent[0].text
	if !strings.Contains(msg, "動的チーム") {
		t.Fatalf("bootstrap message should contain default team name, got: %s", msg)
	}
}

func TestAddMemberSleepFnCalledCorrectly(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orch", "%1")
	svc := NewMemberBootstrapService(
		d.agents, d.paneOps, d.splitter, d.titleSetter, d.paneOps, d.pasteSender,
		"/project", discardLogger(),
	)

	var sleepDurations []time.Duration
	svc.SetSleepFn(func(_ context.Context, d time.Duration) error {
		sleepDurations = append(sleepDurations, d)
		return nil
	})

	_, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle:        "test",
		Role:             "test",
		Command:          "claude",
		BootstrapDelayMs: 5000,
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	// 3回のsleep: shellInit(500ms), cd(300ms), bootstrap(5000ms)
	if len(sleepDurations) != 3 {
		t.Fatalf("sleep called %d times, want 3", len(sleepDurations))
	}
	if sleepDurations[0] != 500*time.Millisecond {
		t.Fatalf("first sleep = %v, want 500ms", sleepDurations[0])
	}
	if sleepDurations[1] != 300*time.Millisecond {
		t.Fatalf("second sleep = %v, want 300ms", sleepDurations[1])
	}
	if sleepDurations[2] != 5000*time.Millisecond {
		t.Fatalf("third sleep = %v, want 5000ms", sleepDurations[2])
	}
}

func TestAddMemberSleepFnWithEmptyProjectRoot(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orch", "%1")
	svc := NewMemberBootstrapService(
		d.agents, d.paneOps, d.splitter, d.titleSetter, d.paneOps, d.pasteSender,
		"", discardLogger(), // projectRoot=""
	)

	var sleepDurations []time.Duration
	svc.SetSleepFn(func(_ context.Context, d time.Duration) error {
		sleepDurations = append(sleepDurations, d)
		return nil
	})

	_, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle:        "test",
		Role:             "test",
		Command:          "python",
		BootstrapDelayMs: 3000,
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	// 2回のsleep: shellInit(500ms), bootstrap(3000ms) — cd スキップ
	if len(sleepDurations) != 2 {
		t.Fatalf("sleep called %d times, want 2 (no cd)", len(sleepDurations))
	}
	if sleepDurations[0] != 500*time.Millisecond {
		t.Fatalf("first sleep = %v, want 500ms", sleepDurations[0])
	}
	if sleepDurations[1] != 3000*time.Millisecond {
		t.Fatalf("second sleep = %v, want 3000ms", sleepDurations[1])
	}
}
