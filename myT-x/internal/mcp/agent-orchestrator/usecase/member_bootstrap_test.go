package usecase

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"
)

// testSplitter はテスト用の PaneSplitter。
// splitFn を設定すると呼び出しごとに異なる動作を返せる。
type testSplitter struct {
	newPaneID string
	err       error
	called    bool
	lastPane  string
	lastHoriz bool
	splitFn   func(ctx context.Context, targetPaneID string, horizontal bool) (string, error)
}

func (s *testSplitter) SplitPane(ctx context.Context, targetPaneID string, horizontal bool) (string, error) {
	s.called = true
	s.lastPane = targetPaneID
	s.lastHoriz = horizontal
	if s.splitFn != nil {
		return s.splitFn(ctx, targetPaneID, horizontal)
	}
	if s.err != nil {
		return "", s.err
	}
	return s.newPaneID, nil
}

// testPasteSender はテスト用の PanePasteSender。
// pasteFn を設定すると呼び出しごとに異なる動作を返せる。
type testPasteSender struct {
	err     error
	called  bool
	sent    []sentCall
	pasteFn func(ctx context.Context, paneID string, text string) error
}

func (p *testPasteSender) SendKeysPaste(ctx context.Context, paneID string, text string) error {
	p.called = true
	if p.pasteFn != nil {
		return p.pasteFn(ctx, paneID, text)
	}
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
	return buildMemberBootstrapServiceWithProjectRoot(d, "/project/root")
}

func buildMemberBootstrapServiceWithProjectRoot(d memberBootstrapDeps, projectRoot string) *MemberBootstrapService {
	svc := NewMemberBootstrapService(
		d.agents, d.paneOps, d.splitter, d.titleSetter, d.paneOps, d.pasteSender,
		projectRoot, discardLogger(),
	)
	// テスト時は sleep をスキップ
	svc.SetSleepFn(func(_ context.Context, _ time.Duration) error { return nil })
	return svc
}

func registerTestCaller(repo *testAgentRepo, name, paneID string) {
	repo.agents[name] = domain.Agent{Name: name, PaneID: paneID}
}

func setupSequentialSplitter(d *memberBootstrapDeps, paneIDs []string) {
	paneIDIndex := 0
	d.splitter.splitFn = func(ctx context.Context, targetPaneID string, horizontal bool) (string, error) {
		d.splitter.lastPane = targetPaneID
		d.splitter.lastHoriz = horizontal
		if paneIDIndex >= len(paneIDs) {
			return "", errors.New("too many splits")
		}
		paneID := paneIDs[paneIDIndex]
		paneIDIndex++
		return paneID, nil
	}
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

func TestAddMemberTrustedCallerUsesResolvedCallerPane(t *testing.T) {
	d := newMemberBootstrapDeps()
	d.paneOps.selfPane = "%3"
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
		t.Fatalf("split from = %q, want %%3 (resolved caller pane)", d.splitter.lastPane)
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
		"30-60秒ごと",
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

// TestAddMembersHappyPath tests successful batch addition of 3 members with cascading splits.
func TestAddMembersHappyPath(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")

	setupSequentialSplitter(&d, []string{"%5", "%6", "%7"})

	svc := buildMemberBootstrapService(d)

	result, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "worker",
				Role:      "実装",
				Command:   "claude",
				Args:      []string{"--model", "sonnet"},
			},
			{
				PaneTitle: "reviewer",
				Role:      "レビュー",
				Command:   "claude",
				Args:      []string{"--model", "sonnet"},
			},
			{
				PaneTitle: "tester",
				Role:      "テスト",
				Command:   "claude",
				Args:      []string{"--model", "sonnet"},
			},
		},
		TeamName:         "テストチーム",
		SplitFrom:        "%1",
		SplitDirection:   "horizontal",
		BootstrapDelayMs: 3000,
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	// Verify results
	if result.Summary.Created != 3 {
		t.Fatalf("Summary.Created = %d, want 3", result.Summary.Created)
	}
	if result.Summary.Failed != 0 {
		t.Fatalf("Summary.Failed = %d, want 0", result.Summary.Failed)
	}
	if len(result.Results) != 3 {
		t.Fatalf("len(Results) = %d, want 3", len(result.Results))
	}

	// Check each result
	expectedPaneIDs := []string{"%5", "%6", "%7"}
	expectedNames := []string{"worker", "reviewer", "tester"}
	for i, expectedPaneID := range expectedPaneIDs {
		item := result.Results[i]
		if item.PaneID != expectedPaneID {
			t.Fatalf("Results[%d].PaneID = %q, want %q", i, item.PaneID, expectedPaneID)
		}
		if item.PaneTitle != expectedNames[i] {
			t.Fatalf("Results[%d].PaneTitle = %q, want %q", i, item.PaneTitle, expectedNames[i])
		}
		if item.AgentName != expectedNames[i] {
			t.Fatalf("Results[%d].AgentName = %q, want %q", i, item.AgentName, expectedNames[i])
		}
		if item.Error != "" {
			t.Fatalf("Results[%d].Error = %q, want empty", i, item.Error)
		}
		if len(item.Warnings) != 0 {
			t.Fatalf("Results[%d].Warnings = %v, want empty", i, item.Warnings)
		}
	}

	// Verify bootstrap messages were sent
	if len(d.pasteSender.sent) != 3 {
		t.Fatalf("expected 3 bootstrap messages, got %d", len(d.pasteSender.sent))
	}
}

// TestAddMembersPartialFailure tests that when one member's split fails,
// remaining members split from the last successful member.
func TestAddMembersPartialFailure(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")

	// Splitter succeeds once, fails once, then succeeds again
	paneIDs := []string{"%5", "", "%7"}
	paneErrors := []error{nil, errors.New("no space"), nil}
	splitIndex := 0
	d.splitter.splitFn = func(ctx context.Context, targetPaneID string, horizontal bool) (string, error) {
		d.splitter.lastPane = targetPaneID
		d.splitter.lastHoriz = horizontal
		if splitIndex >= len(paneIDs) {
			return "", errors.New("too many splits")
		}
		paneID := paneIDs[splitIndex]
		err := paneErrors[splitIndex]
		splitIndex++
		return paneID, err
	}

	svc := buildMemberBootstrapService(d)

	result, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "worker",
				Role:      "実装",
				Command:   "claude",
			},
			{
				PaneTitle: "reviewer",
				Role:      "レビュー",
				Command:   "claude",
			},
			{
				PaneTitle: "tester",
				Role:      "テスト",
				Command:   "claude",
			},
		},
		TeamName:  "テストチーム",
		SplitFrom: "%1",
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	// Verify summary
	if result.Summary.Created != 2 {
		t.Fatalf("Summary.Created = %d, want 2", result.Summary.Created)
	}
	if result.Summary.Failed != 1 {
		t.Fatalf("Summary.Failed = %d, want 1", result.Summary.Failed)
	}

	// Verify individual results
	// First should succeed
	if result.Results[0].Error != "" {
		t.Fatalf("Results[0].Error = %q, want empty", result.Results[0].Error)
	}
	if result.Results[0].PaneID != "%5" {
		t.Fatalf("Results[0].PaneID = %q, want %%5", result.Results[0].PaneID)
	}

	// Second should fail
	if result.Results[1].Error == "" {
		t.Fatal("Results[1].Error should not be empty")
	}
	if result.Results[1].PaneID != "" {
		t.Fatalf("Results[1].PaneID = %q, want empty", result.Results[1].PaneID)
	}

	// Third should succeed and split from first (last successful: %5)
	if result.Results[2].Error != "" {
		t.Fatalf("Results[2].Error = %q, want empty", result.Results[2].Error)
	}
	if result.Results[2].PaneID != "%7" {
		t.Fatalf("Results[2].PaneID = %q, want %%7", result.Results[2].PaneID)
	}

	// Only 2 bootstrap messages should be sent (for successful members)
	if len(d.pasteSender.sent) != 2 {
		t.Fatalf("expected 2 bootstrap messages, got %d", len(d.pasteSender.sent))
	}
}

// TestAddMembersNameDeduplication tests that duplicate pane titles get suffixed with -2, -3, etc.
func TestAddMembersNameDeduplication(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")

	setupSequentialSplitter(&d, []string{"%5", "%6"})

	svc := buildMemberBootstrapService(d)

	result, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "worker",
				Role:      "実装",
				Command:   "claude",
			},
			{
				PaneTitle: "worker", // Duplicate name
				Role:      "検証",
				Command:   "claude",
			},
		},
		TeamName:  "テストチーム",
		SplitFrom: "%1",
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	// First should have no suffix
	if result.Results[0].AgentName != "worker" {
		t.Fatalf("Results[0].AgentName = %q, want worker", result.Results[0].AgentName)
	}

	// Second should have -2 suffix
	if result.Results[1].AgentName != "worker-2" {
		t.Fatalf("Results[1].AgentName = %q, want worker-2", result.Results[1].AgentName)
	}

	// Bootstrap messages should use correct names
	msg1 := d.pasteSender.sent[0].text
	msg2 := d.pasteSender.sent[1].text
	if !strings.Contains(msg1, `name="worker"`) {
		t.Fatalf("first bootstrap should use 'worker', got: %s", msg1)
	}
	if !strings.Contains(msg2, `name="worker-2"`) {
		t.Fatalf("second bootstrap should use 'worker-2', got: %s", msg2)
	}
}

// TestAddMembersTrustedCallerWithoutSplitFrom tests error when trusted caller doesn't provide split_from.
func TestAddMembersTrustedCallerWithoutSplitFrom(t *testing.T) {
	d := newMemberBootstrapDeps()
	d.paneOps.selfPane = "" // pipe bridge mode → trusted caller
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "test",
				Role:      "test",
				Command:   "python",
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for trusted caller without split_from")
	}
	if !strings.Contains(err.Error(), "split_from is required") {
		t.Fatalf("error = %q, want to contain 'split_from is required'", err.Error())
	}
}

// TestAddMembersEmptyArray tests error when members array is empty.
func TestAddMembersEmptyArray(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members:  []AddMemberBatchItemCmd{},
		TeamName: "テストチーム",
	})
	if err == nil {
		t.Fatal("expected error for empty members array")
	}
	if !strings.Contains(err.Error(), "members") {
		t.Fatalf("error = %q, want to contain 'members'", err.Error())
	}
}

// TestAddMembersAllFailed tests when every split fails.
func TestAddMembersAllFailed(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")
	d.splitter.err = errors.New("no space available")
	svc := buildMemberBootstrapService(d)

	result, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "worker1",
				Role:      "実装",
				Command:   "claude",
			},
			{
				PaneTitle: "worker2",
				Role:      "レビュー",
				Command:   "claude",
			},
			{
				PaneTitle: "worker3",
				Role:      "テスト",
				Command:   "claude",
			},
		},
		TeamName:  "テストチーム",
		SplitFrom: "%1",
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	// Verify all failed
	if result.Summary.Created != 0 {
		t.Fatalf("Summary.Created = %d, want 0", result.Summary.Created)
	}
	if result.Summary.Failed != 3 {
		t.Fatalf("Summary.Failed = %d, want 3", result.Summary.Failed)
	}

	// All results should have errors and no PaneID
	for i, item := range result.Results {
		if item.Error == "" {
			t.Fatalf("Results[%d].Error should not be empty", i)
		}
		if item.PaneID != "" {
			t.Fatalf("Results[%d].PaneID = %q, want empty", i, item.PaneID)
		}
	}

	// No bootstrap messages should be sent
	if len(d.pasteSender.sent) != 0 {
		t.Fatalf("expected 0 bootstrap messages, got %d", len(d.pasteSender.sent))
	}
}

// TestAddMembersSleepPattern tests the two-phase timing.
func TestAddMembersSleepPattern(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")

	setupSequentialSplitter(&d, []string{"%5", "%6", "%7"})

	svc := NewMemberBootstrapService(
		d.agents, d.paneOps, d.splitter, d.titleSetter, d.paneOps, d.pasteSender,
		"/project", discardLogger(),
	)

	var sleepDurations []time.Duration
	svc.SetSleepFn(func(_ context.Context, dur time.Duration) error {
		sleepDurations = append(sleepDurations, dur)
		return nil
	})

	_, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "worker",
				Role:      "実装",
				Command:   "claude",
			},
			{
				PaneTitle: "reviewer",
				Role:      "レビュー",
				Command:   "claude",
			},
			{
				PaneTitle: "tester",
				Role:      "テスト",
				Command:   "claude",
			},
		},
		TeamName:         "テストチーム",
		SplitFrom:        "%1",
		BootstrapDelayMs: 2000,
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	// Expected sleep pattern:
	// Phase 1: for each member (shellInit + cd)
	// Phase 2: bootstrapDelay + interMessageDelay × (N-1)
	expectedSleeps := []time.Duration{
		// Member 1
		500 * time.Millisecond, // shellInit
		300 * time.Millisecond, // cd
		// Member 2
		500 * time.Millisecond, // shellInit
		300 * time.Millisecond, // cd
		// Member 3
		500 * time.Millisecond, // shellInit
		300 * time.Millisecond, // cd
		// Bootstrap phase
		2000 * time.Millisecond, // bootstrapDelay
		// Inter-message delays
		300 * time.Millisecond, // before member 2 bootstrap
		300 * time.Millisecond, // before member 3 bootstrap
	}

	if len(sleepDurations) != len(expectedSleeps) {
		t.Fatalf("sleep called %d times, want %d", len(sleepDurations), len(expectedSleeps))
	}

	for i, expected := range expectedSleeps {
		if sleepDurations[i] != expected {
			t.Fatalf("sleep[%d] = %v, want %v", i, sleepDurations[i], expected)
		}
	}
}

// TestAddMembersBootstrapSendFailureAsWarning tests that bootstrap send failures become warnings.
func TestAddMembersBootstrapSendFailureAsWarning(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")

	setupSequentialSplitter(&d, []string{"%5", "%6"})

	// Make second bootstrap send fail
	sendCount := 0
	d.pasteSender.pasteFn = func(_ context.Context, paneID string, text string) error {
		sendCount++
		if sendCount == 2 {
			return errors.New("paste buffer full")
		}
		d.pasteSender.sent = append(d.pasteSender.sent, sentCall{paneID: paneID, text: text})
		return nil
	}

	svc := buildMemberBootstrapService(d)

	result, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "worker",
				Role:      "実装",
				Command:   "claude",
			},
			{
				PaneTitle: "reviewer",
				Role:      "レビュー",
				Command:   "claude",
			},
		},
		TeamName:  "テストチーム",
		SplitFrom: "%1",
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	// Both should be marked as created (not failed)
	if result.Summary.Created != 2 {
		t.Fatalf("Summary.Created = %d, want 2", result.Summary.Created)
	}
	if result.Summary.Failed != 0 {
		t.Fatalf("Summary.Failed = %d, want 0", result.Summary.Failed)
	}

	// First should have no warnings
	if len(result.Results[0].Warnings) != 0 {
		t.Fatalf("Results[0].Warnings = %v, want empty", result.Results[0].Warnings)
	}

	// Second should have a warning
	if len(result.Results[1].Warnings) == 0 {
		t.Fatal("Results[1].Warnings should not be empty")
	}
	if !strings.Contains(result.Results[1].Warnings[0], "bootstrap") {
		t.Fatalf("warning = %q, want to contain 'bootstrap'", result.Results[1].Warnings[0])
	}
}

func TestAddMembersTrustedCallerUsesResolvedCallerPane(t *testing.T) {
	d := newMemberBootstrapDeps()
	d.paneOps.selfPane = "%4"
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "test",
				Role:      "test",
				Command:   "python",
			},
		},
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	if d.splitter.lastPane != "%4" {
		t.Fatalf("split from = %q, want %%4 (resolved caller pane)", d.splitter.lastPane)
	}
}

// TestAddMembersDefaultTeamName tests that default team name is applied.
func TestAddMembersDefaultTeamName(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")
	d.splitter.newPaneID = "%5"
	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "worker",
				Role:      "実装",
				Command:   "claude",
			},
		},
		// TeamName not specified
		SplitFrom: "%1",
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	if len(d.pasteSender.sent) == 0 {
		t.Fatal("expected bootstrap message to be sent")
	}
	msg := d.pasteSender.sent[0].text
	if !strings.Contains(msg, "動的チーム") {
		t.Fatalf("bootstrap message should contain default team name, got: %s", msg)
	}
}

// TestAddMembersCascadingSplitFrom tests that each member splits from the previous member's pane.
func TestAddMembersCascadingSplitFrom(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")

	paneIDs := []string{"%5", "%6", "%7"}
	paneIDIndex := 0
	var splitFromPanes []string
	d.splitter.splitFn = func(ctx context.Context, targetPaneID string, horizontal bool) (string, error) {
		d.splitter.lastPane = targetPaneID
		d.splitter.lastHoriz = horizontal
		splitFromPanes = append(splitFromPanes, targetPaneID)
		if paneIDIndex >= len(paneIDs) {
			return "", errors.New("too many splits")
		}
		paneID := paneIDs[paneIDIndex]
		paneIDIndex++
		return paneID, nil
	}

	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "worker",
				Role:      "実装",
				Command:   "claude",
			},
			{
				PaneTitle: "reviewer",
				Role:      "レビュー",
				Command:   "claude",
			},
			{
				PaneTitle: "tester",
				Role:      "テスト",
				Command:   "claude",
			},
		},
		TeamName:  "テストチーム",
		SplitFrom: "%1",
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	// First member should split from the original SplitFrom
	if splitFromPanes[0] != "%1" {
		t.Fatalf("member 0 split from = %q, want %%1", splitFromPanes[0])
	}

	// Second member should split from the first member's pane (%5)
	if splitFromPanes[1] != "%5" {
		t.Fatalf("member 1 split from = %q, want %%5", splitFromPanes[1])
	}

	// Third member should split from the second member's pane (%6)
	if splitFromPanes[2] != "%6" {
		t.Fatalf("member 2 split from = %q, want %%6", splitFromPanes[2])
	}
}

// TestAddMembersWithCustomMessagesAndSkills tests batch with custom messages and skills.
func TestAddMembersWithCustomMessagesAndSkills(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")

	setupSequentialSplitter(&d, []string{"%5", "%6"})

	svc := buildMemberBootstrapService(d)

	result, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle:     "worker",
				Role:          "実装",
				Command:       "claude",
				CustomMessage: "JavaScriptの最新仕様を使用してください",
				Skills: []domain.Skill{
					{Name: "JavaScript", Description: "フロントエンド開発"},
					{Name: "React"},
				},
			},
			{
				PaneTitle:     "reviewer",
				Role:          "レビュー",
				Command:       "claude",
				CustomMessage: "セキュリティとパフォーマンスに重点を置いてください",
				Skills: []domain.Skill{
					{Name: "Security", Description: "脆弱性診断"},
				},
			},
		},
		TeamName:  "テストチーム",
		SplitFrom: "%1",
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	if result.Summary.Created != 2 {
		t.Fatalf("Summary.Created = %d, want 2", result.Summary.Created)
	}

	// Verify first member's bootstrap message
	msg1 := d.pasteSender.sent[0].text
	if !strings.Contains(msg1, "JavaScriptの最新仕様を使用してください") {
		t.Fatalf("msg1 should contain custom message, got: %s", msg1)
	}
	if !strings.Contains(msg1, "JavaScript") || !strings.Contains(msg1, "フロントエンド開発") {
		t.Fatalf("msg1 should contain JavaScript skill, got: %s", msg1)
	}
	if !strings.Contains(msg1, "React") {
		t.Fatalf("msg1 should contain React skill, got: %s", msg1)
	}

	// Verify second member's bootstrap message
	msg2 := d.pasteSender.sent[1].text
	if !strings.Contains(msg2, "セキュリティとパフォーマンスに重点を置いてください") {
		t.Fatalf("msg2 should contain custom message, got: %s", msg2)
	}
	if !strings.Contains(msg2, "Security") || !strings.Contains(msg2, "脆弱性診断") {
		t.Fatalf("msg2 should contain Security skill, got: %s", msg2)
	}
}

// TestAddMembersVerticalSplitDirection tests that split direction is applied to all members.
func TestAddMembersVerticalSplitDirection(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")

	paneIDs := []string{"%5", "%6"}
	paneIDIndex := 0
	var splitDirections []bool
	d.splitter.splitFn = func(ctx context.Context, targetPaneID string, horizontal bool) (string, error) {
		d.splitter.lastPane = targetPaneID
		d.splitter.lastHoriz = horizontal
		splitDirections = append(splitDirections, horizontal)
		if paneIDIndex >= len(paneIDs) {
			return "", errors.New("too many splits")
		}
		paneID := paneIDs[paneIDIndex]
		paneIDIndex++
		return paneID, nil
	}

	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "worker",
				Role:      "実装",
				Command:   "claude",
			},
			{
				PaneTitle: "reviewer",
				Role:      "レビュー",
				Command:   "claude",
			},
		},
		TeamName:       "テストチーム",
		SplitFrom:      "%1",
		SplitDirection: "vertical",
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	// All splits should be vertical (horizontal = false)
	for i, isHoriz := range splitDirections {
		if isHoriz {
			t.Fatalf("split[%d] should be vertical, got horizontal", i)
		}
	}
}

// TestAddMembersContextCancellation tests that context cancellation during Phase 1 preserves partial results.
func TestAddMembersContextCancellation(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")
	d.splitter.newPaneID = "%5"
	svc := NewMemberBootstrapService(
		d.agents, d.paneOps, d.splitter, d.titleSetter, d.paneOps, d.pasteSender,
		"/project", discardLogger(),
	)

	svc.SetSleepFn(func(ctx context.Context, dur time.Duration) error {
		// Simulate context cancellation during Phase 1
		return context.Canceled
	})

	result, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "worker",
				Role:      "実装",
				Command:   "claude",
			},
		},
		TeamName:  "テストチーム",
		SplitFrom: "%1",
	})
	if err == nil {
		t.Fatal("expected error on context cancellation")
	}
	if !strings.Contains(err.Error(), "shell init wait") {
		t.Fatalf("error = %q, want shell init wait context", err.Error())
	}
	var partial *AddMembersPartialResultError
	if !errors.As(err, &partial) {
		t.Fatalf("expected partial result error, got %T", err)
	}
	result = partial.Result
	if result.Summary.Created != 0 || result.Summary.Failed != 1 {
		t.Fatalf("summary = %+v, want created=0 failed=1", result.Summary)
	}
	if result.Results[0].PaneID != "" {
		t.Fatalf("PaneID = %q, want empty after partial failure", result.Results[0].PaneID)
	}
	if !strings.Contains(result.Results[0].Error, "%5") {
		t.Fatalf("error = %q, want pane id", result.Results[0].Error)
	}
}

func TestAddMembersPhaseTwoContextCancellationReturnsPartialResult(t *testing.T) {
	tests := []struct {
		name                string
		cancelOnSleepCall   int
		wantWarningIndex    int
		wantBootstrapSent   int
		wantWarningContains string
		wantErrorContains   string
	}{
		{
			name:                "bootstrap wait",
			cancelOnSleepCall:   5,
			wantWarningIndex:    0,
			wantBootstrapSent:   0,
			wantWarningContains: "bootstrap wait",
			wantErrorContains:   "bootstrap wait",
		},
		{
			name:                "inter-message wait",
			cancelOnSleepCall:   6,
			wantWarningIndex:    1,
			wantBootstrapSent:   1,
			wantWarningContains: "inter-message wait",
			wantErrorContains:   "inter-message wait",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newMemberBootstrapDeps()
			registerTestCaller(d.agents, "orchestrator", "%1")
			setupSequentialSplitter(&d, []string{"%5", "%6"})
			svc := buildMemberBootstrapService(d)

			sleepCalls := 0
			svc.SetSleepFn(func(_ context.Context, _ time.Duration) error {
				sleepCalls++
				if sleepCalls == tt.cancelOnSleepCall {
					return context.Canceled
				}
				return nil
			})

			_, err := svc.AddMembers(context.Background(), AddMembersCmd{
				Members: []AddMemberBatchItemCmd{
					{PaneTitle: "worker", Role: "実装", Command: "claude"},
					{PaneTitle: "reviewer", Role: "レビュー", Command: "claude"},
				},
				TeamName:  "テストチーム",
				SplitFrom: "%1",
			})
			if err == nil {
				t.Fatal("expected error on context cancellation")
			}
			if !strings.Contains(err.Error(), tt.wantErrorContains) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.wantErrorContains)
			}
			var partial *AddMembersPartialResultError
			if !errors.As(err, &partial) {
				t.Fatalf("expected partial result error, got %T", err)
			}
			result := partial.Result
			if result.Summary.Created != 2 || result.Summary.Failed != 0 {
				t.Fatalf("summary = %+v, want created=2 failed=0", result.Summary)
			}
			if len(d.pasteSender.sent) != tt.wantBootstrapSent {
				t.Fatalf("pasteSender calls = %d, want %d", len(d.pasteSender.sent), tt.wantBootstrapSent)
			}
			if len(result.Results[tt.wantWarningIndex].Warnings) == 0 {
				t.Fatalf("Results[%d].Warnings should not be empty", tt.wantWarningIndex)
			}
			if !strings.Contains(result.Results[tt.wantWarningIndex].Warnings[0], tt.wantWarningContains) {
				t.Fatalf("warning = %q, want %q", result.Results[tt.wantWarningIndex].Warnings[0], tt.wantWarningContains)
			}
		})
	}
}

// TestAddMembersNonClaudeCommandNotPasted tests that non-Claude commands don't use pasteSender.
func TestAddMembersNonClaudeCommandNotPasted(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")
	setupSequentialSplitter(&d, []string{"%5", "%6"})

	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "worker",
				Role:      "実装",
				Command:   "python",
			},
			{
				PaneTitle: "reviewer",
				Role:      "レビュー",
				Command:   "node",
			},
		},
		TeamName:  "テストチーム",
		SplitFrom: "%1",
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	// pasteSender should not be called
	if d.pasteSender.called {
		t.Fatal("pasteSender should not be called for non-Claude commands")
	}
	if len(d.paneOps.sent) != 6 {
		t.Fatalf("SendKeys calls = %d, want 6 (cd, launch, bootstrap for each member)", len(d.paneOps.sent))
	}
	if !strings.Contains(d.paneOps.sent[4].text, "register_agent") {
		t.Fatalf("fifth SendKeys call should be bootstrap text, got %q", d.paneOps.sent[4].text)
	}
	if !strings.Contains(d.paneOps.sent[5].text, "register_agent") {
		t.Fatalf("sixth SendKeys call should be bootstrap text, got %q", d.paneOps.sent[5].text)
	}
}

// TestAddMembersAllMembersCliDetection tests mixed Claude and non-Claude commands.
func TestAddMembersAllMembersCliDetection(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")
	setupSequentialSplitter(&d, []string{"%5", "%6", "%7"})

	svc := buildMemberBootstrapService(d)

	_, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{
				PaneTitle: "worker",
				Role:      "実装",
				Command:   "claude",
			},
			{
				PaneTitle: "python-agent",
				Role:      "データ処理",
				Command:   "python",
			},
			{
				PaneTitle: "node-agent",
				Role:      "Web開発",
				Command:   "node",
			},
		},
		TeamName:  "テストチーム",
		SplitFrom: "%1",
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	// Only first member (Claude) should use pasteSender
	if len(d.pasteSender.sent) != 1 {
		t.Fatalf("expected 1 pasteSender call, got %d", len(d.pasteSender.sent))
	}
}

func TestAddMemberCommandFailuresIncludePaneID(t *testing.T) {
	tests := []struct {
		name         string
		projectRoot  string
		wantContains string
		sendFn       func(ctx context.Context, paneID string, text string) error
	}{
		{
			name:         "cd command",
			projectRoot:  "/project/root",
			wantContains: "failed to send cd command for pane %5",
			sendFn: func(_ context.Context, _ string, text string) error {
				if strings.HasPrefix(text, `cd "`) {
					return errors.New("cd failed")
				}
				return nil
			},
		},
		{
			name:         "launch command",
			projectRoot:  "",
			wantContains: "failed to send launch command for pane %5",
			sendFn: func(_ context.Context, _ string, _ string) error {
				return errors.New("launch failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newMemberBootstrapDeps()
			registerTestCaller(d.agents, "orchestrator", "%1")
			d.paneOps.sendFn = tt.sendFn
			svc := buildMemberBootstrapServiceWithProjectRoot(d, tt.projectRoot)

			_, err := svc.AddMember(context.Background(), AddMemberCmd{
				PaneTitle: "worker",
				Role:      "実装",
				Command:   "claude",
			})
			if err == nil {
				t.Fatal("expected command failure")
			}
			if !strings.Contains(err.Error(), tt.wantContains) {
				t.Fatalf("error = %q, want to contain %q", err.Error(), tt.wantContains)
			}
		})
	}
}

func TestAddMemberAvoidsRegisteredAgentNameCollisions(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")
	registerTestCaller(d.agents, "worker", "%9")
	svc := buildMemberBootstrapService(d)

	result, err := svc.AddMember(context.Background(), AddMemberCmd{
		PaneTitle: "worker",
		Role:      "実装",
		Command:   "claude",
	})
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if result.AgentName != "worker-2" {
		t.Fatalf("AgentName = %q, want worker-2", result.AgentName)
	}
	if !strings.Contains(d.pasteSender.sent[0].text, `name="worker-2"`) {
		t.Fatalf("bootstrap should use worker-2, got %s", d.pasteSender.sent[0].text)
	}
}

func TestAddMembersCommandFailureMarksOrphanedPane(t *testing.T) {
	tests := []struct {
		name         string
		projectRoot  string
		wantContains string
		sendFn       func(ctx context.Context, paneID string, text string) error
	}{
		{
			name:         "cd command",
			projectRoot:  "/project/root",
			wantContains: "failed to send cd command for pane %5",
			sendFn: func(_ context.Context, _ string, text string) error {
				if strings.HasPrefix(text, `cd "`) {
					return errors.New("cd failed")
				}
				return nil
			},
		},
		{
			name:         "launch command",
			projectRoot:  "",
			wantContains: "failed to send launch command for pane %5",
			sendFn: func(_ context.Context, _ string, _ string) error {
				return errors.New("launch failed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := newMemberBootstrapDeps()
			registerTestCaller(d.agents, "orchestrator", "%1")
			d.paneOps.sendFn = tt.sendFn
			svc := buildMemberBootstrapServiceWithProjectRoot(d, tt.projectRoot)

			result, err := svc.AddMembers(context.Background(), AddMembersCmd{
				Members: []AddMemberBatchItemCmd{
					{PaneTitle: "worker", Role: "実装", Command: "claude"},
				},
				TeamName:  "テストチーム",
				SplitFrom: "%1",
			})
			if err != nil {
				t.Fatalf("AddMembers: %v", err)
			}
			if result.Summary.Created != 0 || result.Summary.Failed != 1 {
				t.Fatalf("summary = %+v, want created=0 failed=1", result.Summary)
			}
			if result.Results[0].PaneID != "" {
				t.Fatalf("PaneID = %q, want empty", result.Results[0].PaneID)
			}
			if result.Results[0].OrphanedPaneID != "%5" {
				t.Fatalf("OrphanedPaneID = %q, want %%5", result.Results[0].OrphanedPaneID)
			}
			if !strings.Contains(result.Results[0].Error, tt.wantContains) {
				t.Fatalf("error = %q, want to contain %q", result.Results[0].Error, tt.wantContains)
			}
		})
	}
}

func TestAddMembersDuplicatePaneTitleAddsWarnings(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")
	setupSequentialSplitter(&d, []string{"%5", "%6", "%7"})

	svc := buildMemberBootstrapService(d)

	result, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{PaneTitle: "worker", Role: "実装", Command: "claude"},
			{PaneTitle: "worker", Role: "レビュー", Command: "claude"},
			{PaneTitle: "tester", Role: "テスト", Command: "claude"},
		},
		SplitFrom: "%1",
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	for _, idx := range []int{0, 1} {
		if len(result.Results[idx].Warnings) == 0 {
			t.Fatalf("Results[%d].Warnings should not be empty", idx)
		}
		if !strings.Contains(result.Results[idx].Warnings[0], "duplicate pane_title") {
			t.Fatalf("Results[%d].Warnings = %v, want duplicate pane_title warning", idx, result.Results[idx].Warnings)
		}
	}
	if len(result.Results[2].Warnings) != 0 {
		t.Fatalf("Results[2].Warnings = %v, want empty", result.Results[2].Warnings)
	}
}

func TestAddMembersPhaseOneMixedFailuresKeepSplittingFromLastSuccess(t *testing.T) {
	d := newMemberBootstrapDeps()
	registerTestCaller(d.agents, "orchestrator", "%1")

	paneIDs := []string{"%5", "%6", "%7", "%8"}
	splitIndex := 0
	var splitFromPanes []string
	d.splitter.splitFn = func(_ context.Context, targetPaneID string, horizontal bool) (string, error) {
		splitFromPanes = append(splitFromPanes, targetPaneID)
		if splitIndex >= len(paneIDs) {
			return "", errors.New("too many splits")
		}
		paneID := paneIDs[splitIndex]
		splitIndex++
		return paneID, nil
	}
	d.paneOps.sendFn = func(_ context.Context, paneID string, text string) error {
		if paneID == "%6" && strings.HasPrefix(text, `cd "`) {
			return errors.New("cd failed")
		}
		if paneID == "%7" && !strings.HasPrefix(text, `cd "`) {
			return errors.New("launch failed")
		}
		d.paneOps.sent = append(d.paneOps.sent, sentCall{paneID: paneID, text: text})
		return nil
	}

	svc := buildMemberBootstrapServiceWithProjectRoot(d, "/project/root")

	result, err := svc.AddMembers(context.Background(), AddMembersCmd{
		Members: []AddMemberBatchItemCmd{
			{PaneTitle: "worker", Role: "実装", Command: "claude"},
			{PaneTitle: "reviewer", Role: "レビュー", Command: "claude"},
			{PaneTitle: "tester", Role: "テスト", Command: "claude"},
			{PaneTitle: "writer", Role: "文書化", Command: "claude"},
		},
		SplitFrom: "%1",
	})
	if err != nil {
		t.Fatalf("AddMembers: %v", err)
	}

	if !reflect.DeepEqual(splitFromPanes, []string{"%1", "%5", "%5", "%5"}) {
		t.Fatalf("splitFromPanes = %v, want [%%1 %%5 %%5 %%5]", splitFromPanes)
	}
	if result.Summary.Created != 2 || result.Summary.Failed != 2 {
		t.Fatalf("summary = %+v, want created=2 failed=2", result.Summary)
	}
	if result.Results[0].PaneID != "%5" {
		t.Fatalf("Results[0].PaneID = %q, want %%5", result.Results[0].PaneID)
	}
	if result.Results[1].OrphanedPaneID != "%6" {
		t.Fatalf("Results[1].OrphanedPaneID = %q, want %%6", result.Results[1].OrphanedPaneID)
	}
	if result.Results[2].OrphanedPaneID != "%7" {
		t.Fatalf("Results[2].OrphanedPaneID = %q, want %%7", result.Results[2].OrphanedPaneID)
	}
	if result.Results[3].PaneID != "%8" {
		t.Fatalf("Results[3].PaneID = %q, want %%8", result.Results[3].PaneID)
	}
}

func TestAddMembersPartialResultErrorRejectsNonPartialError(t *testing.T) {
	var partial *AddMembersPartialResultError
	if errors.As(errors.New("plain error"), &partial) {
		t.Fatal("errors.As should reject non-partial errors")
	}
}

func TestAddMemberBatchItemCmdToAddMemberCmdCopiesSharedFields(t *testing.T) {
	batchType := reflect.TypeFor[AddMemberBatchItemCmd]()
	memberType := reflect.TypeFor[AddMemberCmd]()
	allowedExtraFields := map[string]struct{}{
		"TeamName":         {},
		"SplitFrom":        {},
		"SplitDirection":   {},
		"BootstrapDelayMs": {},
	}

	if memberType.NumField() != batchType.NumField()+len(allowedExtraFields) {
		t.Fatalf("AddMemberCmd field count = %d, want %d shared + %d extra fields",
			memberType.NumField(), batchType.NumField(), len(allowedExtraFields))
	}

	for batchField := range batchType.Fields() {
		memberField, ok := memberType.FieldByName(batchField.Name)
		if !ok {
			t.Fatalf("AddMemberCmd is missing shared field %q", batchField.Name)
		}
		if memberField.Type != batchField.Type {
			t.Fatalf("field %q type = %v, want %v", batchField.Name, memberField.Type, batchField.Type)
		}
	}

	for memberField := range memberType.Fields() {
		if _, ok := batchType.FieldByName(memberField.Name); ok {
			continue
		}
		if _, ok := allowedExtraFields[memberField.Name]; !ok {
			t.Fatalf("unexpected AddMemberCmd-only field %q", memberField.Name)
		}
	}

	batch := AddMemberBatchItemCmd{
		PaneTitle:     "worker",
		Role:          "implement",
		Command:       "claude",
		Args:          []string{"--model", "sonnet"},
		CustomMessage: "focus on tests",
		Skills:        []domain.Skill{{Name: "Go", Description: "backend"}},
	}
	got := batch.toAddMemberCmd()
	gotValue := reflect.ValueOf(got)
	wantValue := reflect.ValueOf(batch)
	for i := range batchType.NumField() {
		fieldName := batchType.Field(i).Name
		gotField := gotValue.FieldByName(fieldName).Interface()
		wantField := wantValue.Field(i).Interface()
		if !reflect.DeepEqual(gotField, wantField) {
			t.Fatalf("field %s = %#v, want %#v", fieldName, gotField, wantField)
		}
	}
}

// TestDeduplicateMemberAgentNamesWithTaken tests the name deduplication helper function.
func TestDeduplicateMemberAgentNamesWithTaken(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		taken map[string]struct{}
		want  []string
	}{
		{
			"no duplicates",
			[]string{"worker", "reviewer", "tester"},
			nil,
			[]string{"worker", "reviewer", "tester"},
		},
		{
			"one duplicate",
			[]string{"worker", "worker", "tester"},
			nil,
			[]string{"worker", "worker-2", "tester"},
		},
		{
			"multiple duplicates same name",
			[]string{"worker", "worker", "worker"},
			nil,
			[]string{"worker", "worker-2", "worker-3"},
		},
		{
			"multiple duplicate groups",
			[]string{"worker", "worker", "reviewer", "reviewer"},
			nil,
			[]string{"worker", "worker-2", "reviewer", "reviewer-2"},
		},
		{
			"explicit suffix collides with generated name",
			[]string{"worker-2", "worker", "worker"},
			nil,
			[]string{"worker-2", "worker", "worker-3"},
		},
		{
			"empty names (fallback to 'member')",
			[]string{"", "", "tester"},
			nil,
			[]string{"member", "member-2", "tester"},
		},
		{
			"all empty",
			[]string{"", "", ""},
			nil,
			[]string{"member", "member-2", "member-3"},
		},
		{
			"existing registered name is reserved",
			[]string{"worker", "reviewer"},
			map[string]struct{}{"worker": {}},
			[]string{"worker-2", "reviewer"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			members := make([]AddMemberBatchItemCmd, len(tt.input))
			for i, title := range tt.input {
				members[i] = AddMemberBatchItemCmd{PaneTitle: title}
			}
			got := deduplicateMemberAgentNamesWithTaken(members, tt.taken)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("deduplicateMemberAgentNamesWithTaken(%v, %v) = %v, want %v", tt.input, tt.taken, got, tt.want)
			}
		})
	}
}

func TestMemberBootstrapServiceReserveMemberAgentNamesTracksInFlightNames(t *testing.T) {
	agents := newTestAgentRepo()
	svc := NewMemberBootstrapService(agents, nil, nil, nil, nil, nil, "", discardLogger())

	first, err := svc.reserveMemberAgentName(context.Background(), "Worker")
	if err != nil {
		t.Fatalf("first reserveMemberAgentName: %v", err)
	}
	second, err := svc.reserveMemberAgentName(context.Background(), "Worker")
	if err != nil {
		t.Fatalf("second reserveMemberAgentName: %v", err)
	}
	if first != "worker" {
		t.Fatalf("first reserved name = %q, want %q", first, "worker")
	}
	if second != "worker-2" {
		t.Fatalf("second reserved name = %q, want %q", second, "worker-2")
	}

	svc.releaseMemberAgentNames([]string{first, second})

	third, err := svc.reserveMemberAgentName(context.Background(), "Worker")
	if err != nil {
		t.Fatalf("third reserveMemberAgentName: %v", err)
	}
	if third != "worker" {
		t.Fatalf("third reserved name = %q, want %q", third, "worker")
	}
}
