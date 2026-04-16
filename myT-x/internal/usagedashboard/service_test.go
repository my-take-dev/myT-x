package usagedashboard

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"myT-x/internal/usagedashboard/claude"
)

func setupFakeHome(t *testing.T) (homeDir, workDir string) {
	t.Helper()
	home := t.TempDir()
	// Claude side: projects/<slug>/session.jsonl
	workDir = filepath.Join(home, "myT-x", "dev-myT-x")
	slug := AbsPathToClaudeSlug(workDir)
	claudeProjects := filepath.Join(home, ".claude", "projects", slug)
	if err := os.MkdirAll(claudeProjects, 0o755); err != nil {
		t.Fatalf("mkdir claude: %v", err)
	}
	sampleClaude := `{"type":"assistant","timestamp":"2026-04-15T10:00:00Z","sessionId":"claude-s1","message":{"role":"assistant","content":[{"type":"tool_use","name":"Skill","input":{"skill":"foo"}}]}}
{"type":"user","timestamp":"2026-04-15T10:00:05Z","sessionId":"claude-s1","message":{"role":"user","content":"<command-name>/review</command-name>"}}
`
	if err := os.WriteFile(filepath.Join(claudeProjects, "session.jsonl"), []byte(sampleClaude), 0o644); err != nil {
		t.Fatalf("write claude sample: %v", err)
	}

	// Codex side: sessions/2026/04/15/rollout-x.jsonl
	codexDay := filepath.Join(home, ".codex", "sessions", "2026", "04", "15")
	if err := os.MkdirAll(codexDay, 0o755); err != nil {
		t.Fatalf("mkdir codex: %v", err)
	}
	sampleCodex := `{"timestamp":"2026-04-15T10:00:00Z","type":"session_meta","payload":{"id":"codex-s1","timestamp":"2026-04-15T10:00:00Z","cwd":"` + escapePath(workDir) + `"}}
{"timestamp":"2026-04-15T10:00:02Z","type":"response_item","payload":{"type":"function_call","name":"spawn_agent","arguments":"{\"agent_type\":\"test-agent\"}"}}
{"timestamp":"2026-04-15T10:00:04Z","type":"response_item","payload":{"type":"message","role":"user","content":[]}}
`
	if err := os.WriteFile(filepath.Join(codexDay, "rollout-codex-s1.jsonl"), []byte(sampleCodex), 0o644); err != nil {
		t.Fatalf("write codex sample: %v", err)
	}
	return home, workDir
}

func escapePath(p string) string {
	out := make([]byte, 0, len(p)*2)
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '\\' {
			out = append(out, '\\', '\\')
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

func setupSlowAggregationHome(t *testing.T, repeats int) (homeDir, workDir string) {
	t.Helper()

	home, workDir := setupFakeHome(t)
	slug := AbsPathToClaudeSlug(workDir)
	claudeProjects := filepath.Join(home, ".claude", "projects", slug)
	codexDay := filepath.Join(home, ".codex", "sessions", "2026", "04", "15")

	heavyClaude := strings.Repeat(
		"{\"type\":\"assistant\",\"timestamp\":\"2026-04-15T10:00:00Z\",\"sessionId\":\"claude-heavy\",\"message\":{\"role\":\"assistant\",\"content\":[{\"type\":\"tool_use\",\"name\":\"Skill\",\"input\":{\"skill\":\"foo\"}}]}}\n"+
			"{\"type\":\"user\",\"timestamp\":\"2026-04-15T10:00:05Z\",\"sessionId\":\"claude-heavy\",\"message\":{\"role\":\"user\",\"content\":\"<command-name>/review</command-name>\"}}\n",
		repeats,
	)
	if err := os.WriteFile(filepath.Join(claudeProjects, "session-heavy.jsonl"), []byte(heavyClaude), 0o644); err != nil {
		t.Fatalf("write heavy claude sample: %v", err)
	}

	heavyCodex := strings.Repeat(
		"{\"timestamp\":\"2026-04-15T10:00:00Z\",\"type\":\"session_meta\",\"payload\":{\"id\":\"codex-heavy\",\"timestamp\":\"2026-04-15T10:00:00Z\",\"cwd\":\""+escapePath(workDir)+"\"}}\n"+
			"{\"timestamp\":\"2026-04-15T10:00:02Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"function_call\",\"name\":\"spawn_agent\",\"arguments\":\"{\\\"agent_type\\\":\\\"test-agent\\\"}\"}}\n"+
			"{\"timestamp\":\"2026-04-15T10:00:04Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"message\",\"role\":\"user\",\"content\":[]}}\n",
		repeats,
	)
	if err := os.WriteFile(filepath.Join(codexDay, "rollout-heavy.jsonl"), []byte(heavyCodex), 0o644); err != nil {
		t.Fatalf("write heavy codex sample: %v", err)
	}

	return home, workDir
}

func newTestService(t *testing.T, home, workDir string) *Service {
	t.Helper()
	return NewService(Deps{
		ResolveSessionWorkDir: func(sessionName string) (string, error) {
			if sessionName == "" {
				return "", errors.New("empty session")
			}
			return workDir, nil
		},
		HomeDir: func() (string, error) { return home, nil },
		NowFunc: func() time.Time {
			return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		},
	})
}

func TestGetUsageDashboardModeClaude(t *testing.T) {
	home, workDir := setupFakeHome(t)
	svc := newTestService(t, home, workDir)

	snapshot, err := svc.GetUsageDashboard("session-1", "claude", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.Claude == nil {
		t.Fatal("Claude stats nil in mode=claude")
	}
	if snapshot.Codex != nil {
		t.Error("Codex stats must be nil in mode=claude")
	}
	if snapshot.LastUpdatedAt.IsZero() {
		t.Error("LastUpdatedAt must be set")
	}
	if snapshot.Claude.Skills == nil {
		t.Error("Skills slice must be non-nil (#157 contract)")
	}
	if snapshot.Claude.Health.PartialErrors == nil {
		t.Error("PartialErrors must be non-nil slice (#157 contract)")
	}
	if len(snapshot.Claude.Skills) == 0 || snapshot.Claude.Skills[0].Name != "foo" {
		t.Errorf("expected skill 'foo' top-ranked: %+v", snapshot.Claude.Skills)
	}
}

func TestGetUsageDashboardModeCodex(t *testing.T) {
	home, workDir := setupFakeHome(t)
	svc := newTestService(t, home, workDir)

	snapshot, err := svc.GetUsageDashboard("session-1", "codex", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.Codex == nil {
		t.Fatal("Codex stats nil in mode=codex")
	}
	if snapshot.Claude != nil {
		t.Error("Claude stats must be nil in mode=codex")
	}
	if len(snapshot.Codex.Agents) == 0 || snapshot.Codex.Agents[0].Name != "test-agent" {
		t.Errorf("expected agent 'test-agent' ranked: %+v", snapshot.Codex.Agents)
	}
}

func TestGetUsageDashboardModeBoth(t *testing.T) {
	home, workDir := setupFakeHome(t)
	svc := newTestService(t, home, workDir)

	snapshot, err := svc.GetUsageDashboard("session-1", "both", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.Claude == nil || snapshot.Codex == nil {
		t.Fatalf("mode=both must return both stats: claude=%v codex=%v", snapshot.Claude, snapshot.Codex)
	}
	if snapshot.Claude.DailyActivity == nil || snapshot.Codex.DailyActivity == nil {
		t.Error("DailyActivity must be non-nil for both stats")
	}
}

func TestGetUsageDashboardDefaultsToBoth(t *testing.T) {
	home, workDir := setupFakeHome(t)
	svc := newTestService(t, home, workDir)

	snapshot, err := svc.GetUsageDashboard("session-1", "invalid-mode", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.Claude == nil || snapshot.Codex == nil {
		t.Error("unknown mode must default to 'both'")
	}
}

func TestGetUsageDashboardRejectsEmptySession(t *testing.T) {
	home, workDir := setupFakeHome(t)
	svc := newTestService(t, home, workDir)

	if _, err := svc.GetUsageDashboard("   ", "claude", false); err == nil {
		t.Error("expected error for empty session name")
	}
}

func TestGetUsageDashboardRejectsEmptyWorkDir(t *testing.T) {
	svc := NewService(Deps{
		ResolveSessionWorkDir: func(string) (string, error) { return "   ", nil },
		HomeDir:               func() (string, error) { return t.TempDir(), nil },
		NowFunc:               func() time.Time { return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC) },
	})

	if _, err := svc.GetUsageDashboard("session-1", "both", false); err == nil {
		t.Fatal("expected error when ResolveSessionWorkDir returns an empty path")
	}
}

func TestGetUsageDashboardPropagatesHomeDirFailure(t *testing.T) {
	svc := NewService(Deps{
		ResolveSessionWorkDir: func(string) (string, error) { return `D:\myT-x\dev-myT-x`, nil },
		HomeDir: func() (string, error) {
			return "", errors.New("synthetic home failure")
		},
		NowFunc: func() time.Time { return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC) },
	})

	if _, err := svc.GetUsageDashboard("session-1", "both", false); err == nil {
		t.Fatal("expected HomeDir failure to propagate")
	}
}

func TestGetUsageDashboardCodexMissingSessionsRootDoesNotWarn(t *testing.T) {
	home := t.TempDir()
	workDir := filepath.Join(home, "myT-x", "dev-myT-x")
	svc := NewService(Deps{
		ResolveSessionWorkDir: func(string) (string, error) { return workDir, nil },
		HomeDir:               func() (string, error) { return home, nil },
		NowFunc:               func() time.Time { return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC) },
	})

	snapshot, err := svc.GetUsageDashboard("session-1", "codex", false)
	if err != nil {
		t.Fatalf("GetUsageDashboard: %v", err)
	}
	if snapshot.Codex == nil {
		t.Fatal("Codex stats nil")
	}
	if len(snapshot.Codex.Health.PartialErrors) != 0 {
		t.Fatalf("missing sessions root should be treated as normal empty state: %+v", snapshot.Codex.Health.PartialErrors)
	}
}

func TestGetUsageDashboardPartialClaudeMiss(t *testing.T) {
	home := t.TempDir()
	workDir := filepath.Join(home, "empty-project")
	// Codex only: no claude slug directory.
	codexDay := filepath.Join(home, ".codex", "sessions", "2026", "04", "15")
	_ = os.MkdirAll(codexDay, 0o755)
	sample := `{"timestamp":"2026-04-15T10:00:00Z","type":"session_meta","payload":{"id":"x","cwd":"` + escapePath(workDir) + `"}}`
	_ = os.WriteFile(filepath.Join(codexDay, "rollout-x.jsonl"), []byte(sample), 0o644)

	svc := NewService(Deps{
		ResolveSessionWorkDir: func(string) (string, error) { return workDir, nil },
		HomeDir:               func() (string, error) { return home, nil },
		NowFunc:               func() time.Time { return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC) },
	})
	snapshot, err := svc.GetUsageDashboard("s", "both", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.Claude == nil || snapshot.Codex == nil {
		t.Fatal("mode=both must still return both stats even when one source is empty")
	}
	if snapshot.Claude.Health.JsonlAvailable {
		t.Error("Claude.Health.JsonlAvailable must be false when project dir is missing")
	}
	if len(snapshot.Claude.Health.PartialErrors) == 0 {
		t.Error("Claude.Health.PartialErrors should report missing project dir")
	}
}

func TestNewServicePanicsOnMissingDeps(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing ResolveSessionWorkDir")
		}
	}()
	_ = NewService(Deps{})
}

// writeClaudeFixture writes a Claude-only fake home with a custom main
// session jsonl body. Returns (homeDir, workDir, claudeProjectDir).
func writeClaudeFixture(t *testing.T, mainJSONL string) (string, string, string) {
	t.Helper()
	home := t.TempDir()
	workDir := filepath.Join(home, "myT-x", "dev-myT-x")
	slug := AbsPathToClaudeSlug(workDir)
	projectDir := filepath.Join(home, ".claude", "projects", slug)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir project: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "main.jsonl"), []byte(mainJSONL), 0o644); err != nil {
		t.Fatalf("write main: %v", err)
	}
	return home, workDir, projectDir
}

// TestGetUsageDashboardClaudeAgentsDetected verifies that "Agent" tool_use
// blocks are correctly surfaced in snapshot.Claude.Agents (regression test for
// the historical "case \"Task\"" bug that caused zero detection).
func TestGetUsageDashboardClaudeAgentsDetected(t *testing.T) {
	const fixture = `{"type":"assistant","timestamp":"2026-04-15T10:00:00Z","sessionId":"s1","message":{"role":"assistant","content":[{"type":"tool_use","name":"Agent","input":{"subagent_type":"Explore"}}]}}
{"type":"assistant","timestamp":"2026-04-15T10:00:01Z","sessionId":"s1","message":{"role":"assistant","content":[{"type":"tool_use","name":"Agent","input":{"subagent_type":"Explore"}}]}}
{"type":"assistant","timestamp":"2026-04-15T10:00:02Z","sessionId":"s1","message":{"role":"assistant","content":[{"type":"tool_use","name":"Agent","input":{"subagent_type":"Plan"}}]}}
`
	home, workDir, _ := writeClaudeFixture(t, fixture)
	svc := newTestService(t, home, workDir)
	snapshot, err := svc.GetUsageDashboard("sess", "claude", false)
	if err != nil {
		t.Fatalf("GetUsageDashboard: %v", err)
	}
	if snapshot.Claude == nil {
		t.Fatal("Claude stats nil")
	}
	if len(snapshot.Claude.Agents) == 0 {
		t.Fatalf("Agents empty — regression: Agent tool_use not detected. agents=%+v", snapshot.Claude.Agents)
	}
	if snapshot.Claude.Agents[0].Name != "Explore" || snapshot.Claude.Agents[0].Count != 2 {
		t.Errorf("top agent = %+v, want Explore count=2", snapshot.Claude.Agents[0])
	}
	if snapshot.Claude.TotalToolUses != 3 {
		t.Errorf("TotalToolUses = %d, want 3", snapshot.Claude.TotalToolUses)
	}
}

// TestGetUsageDashboardClaudeQualifiedSlashToSkills verifies that qualified
// slash commands (feature-dev:feature-dev) end up in Skills, while built-in
// commands (/clear, /model) remain in SlashCommands.
func TestGetUsageDashboardClaudeQualifiedSlashToSkills(t *testing.T) {
	const fixture = `{"type":"user","timestamp":"2026-04-15T10:00:00Z","sessionId":"s1","message":{"role":"user","content":"<command-name>/feature-dev:feature-dev</command-name>"}}
{"type":"user","timestamp":"2026-04-15T10:00:01Z","sessionId":"s1","message":{"role":"user","content":"<command-name>/feature-dev:feature-dev</command-name>"}}
{"type":"user","timestamp":"2026-04-15T10:00:02Z","sessionId":"s1","message":{"role":"user","content":"<command-name>/pr-review-toolkit:review-pr</command-name>"}}
{"type":"user","timestamp":"2026-04-15T10:00:03Z","sessionId":"s1","message":{"role":"user","content":"<command-name>/clear</command-name>"}}
{"type":"user","timestamp":"2026-04-15T10:00:04Z","sessionId":"s1","message":{"role":"user","content":"<command-name>/model</command-name>"}}
`
	home, workDir, _ := writeClaudeFixture(t, fixture)
	svc := newTestService(t, home, workDir)
	snapshot, err := svc.GetUsageDashboard("sess", "claude", false)
	if err != nil {
		t.Fatalf("GetUsageDashboard: %v", err)
	}
	c := snapshot.Claude
	if c == nil {
		t.Fatal("Claude stats nil")
	}
	// Skills should contain the 2 qualified command names.
	skillNames := make(map[string]int)
	for _, e := range c.Skills {
		skillNames[e.Name] = e.Count
	}
	if skillNames["feature-dev:feature-dev"] != 2 {
		t.Errorf("feature-dev:feature-dev count = %d, want 2: skills=%+v", skillNames["feature-dev:feature-dev"], c.Skills)
	}
	if skillNames["pr-review-toolkit:review-pr"] != 1 {
		t.Errorf("pr-review-toolkit:review-pr count = %d, want 1: skills=%+v", skillNames["pr-review-toolkit:review-pr"], c.Skills)
	}
	// SlashCommands must NOT contain qualified names.
	slashNames := make(map[string]int)
	for _, e := range c.SlashCommands {
		slashNames[e.Name] = e.Count
	}
	if _, leaked := slashNames["feature-dev:feature-dev"]; leaked {
		t.Errorf("qualified name leaked into SlashCommands: %+v", c.SlashCommands)
	}
	if slashNames["clear"] != 1 || slashNames["model"] != 1 {
		t.Errorf("built-in slashes missing: slash=%+v", c.SlashCommands)
	}
}

// TestGetUsageDashboardClaudeSubagentAggregation verifies that subagent
// subdirectory jsonl files contribute to Skills/Agents/ToolCalls but do NOT
// inflate TotalSessions or TotalMessages.
func TestGetUsageDashboardClaudeSubagentAggregation(t *testing.T) {
	const mainFixture = `{"type":"assistant","timestamp":"2026-04-15T10:00:00Z","sessionId":"main-s1","message":{"role":"assistant","content":[{"type":"tool_use","name":"Agent","input":{"subagent_type":"Explore"}}]}}
{"type":"user","timestamp":"2026-04-15T10:00:01Z","sessionId":"main-s1","message":{"role":"user","content":"hello"}}
`
	// Subagent file: has isSidechain=true and its own Agent/Skill tool_use.
	const subFixture = `{"type":"assistant","timestamp":"2026-04-15T10:05:00Z","sessionId":"sub-a","isSidechain":true,"message":{"role":"assistant","content":[{"type":"tool_use","name":"Agent","input":{"subagent_type":"golang-expert"}}]}}
{"type":"assistant","timestamp":"2026-04-15T10:05:01Z","sessionId":"sub-a","isSidechain":true,"message":{"role":"assistant","content":[{"type":"tool_use","name":"Skill","input":{"skill":"go-test-patterns"}}]}}
{"type":"user","timestamp":"2026-04-15T10:05:02Z","sessionId":"sub-a","isSidechain":true,"message":{"role":"user","content":"inner"}}
`
	home, workDir, projectDir := writeClaudeFixture(t, mainFixture)
	subDir := filepath.Join(projectDir, "main-s1", "subagents")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "agent-abc.jsonl"), []byte(subFixture), 0o644); err != nil {
		t.Fatalf("write sub: %v", err)
	}

	svc := newTestService(t, home, workDir)
	snapshot, err := svc.GetUsageDashboard("sess", "claude", false)
	if err != nil {
		t.Fatalf("GetUsageDashboard: %v", err)
	}
	c := snapshot.Claude
	if c == nil {
		t.Fatal("Claude stats nil")
	}
	// Agents: 1 from main (Explore) + 1 from sub (golang-expert) = 2 entries.
	agentNames := make(map[string]int)
	for _, e := range c.Agents {
		agentNames[e.Name] = e.Count
	}
	if agentNames["Explore"] != 1 {
		t.Errorf("main Explore agent missing: agents=%+v", c.Agents)
	}
	if agentNames["golang-expert"] != 1 {
		t.Errorf("subagent golang-expert not aggregated: agents=%+v", c.Agents)
	}
	// Skills: subagent's direct Skill tool_use must be counted.
	skillNames := make(map[string]int)
	for _, e := range c.Skills {
		skillNames[e.Name] = e.Count
	}
	if skillNames["go-test-patterns"] != 1 {
		t.Errorf("subagent skill not aggregated: skills=%+v", c.Skills)
	}
	// TotalSessions: only main counts (subagent must not inflate).
	if c.TotalSessions != 1 {
		t.Errorf("TotalSessions = %d, want 1 (subagent must not be counted as new session)", c.TotalSessions)
	}
	// TotalMessages: main has 1 user + 1 assistant = 2; subagent messages must NOT count.
	if c.TotalMessages != 2 {
		t.Errorf("TotalMessages = %d, want 2 (subagent messages must be excluded)", c.TotalMessages)
	}
	// TotalToolUses: main Agent(1) + sub Agent(1) + sub Skill(1) = 3.
	if c.TotalToolUses != 3 {
		t.Errorf("TotalToolUses = %d, want 3 (main 1 + sub 2)", c.TotalToolUses)
	}
}

// fakeSnapshotRepository is a programmable in-memory SnapshotRepository
// used to verify cache hit/miss/force behavior in Service tests.
type fakeSnapshotRepository struct {
	mu        sync.Mutex
	loadFn    func(workDir string) (PersistedSnapshot, bool, error)
	saveFn    func(snap PersistedSnapshot) error
	stored    PersistedSnapshot
	hasStored bool
	loadCalls int
	saveCalls int
}

func (r *fakeSnapshotRepository) Load(workDir string) (PersistedSnapshot, bool, error) {
	r.mu.Lock()
	r.loadCalls++
	loadFn := r.loadFn
	hasStored := r.hasStored
	stored := r.stored
	r.mu.Unlock()
	if loadFn != nil {
		return loadFn(workDir)
	}
	if !hasStored {
		return PersistedSnapshot{}, false, nil
	}
	return stored, true, nil
}

func (r *fakeSnapshotRepository) Save(snap PersistedSnapshot) error {
	r.mu.Lock()
	r.saveCalls++
	saveFn := r.saveFn
	r.mu.Unlock()
	if saveFn != nil {
		return saveFn(snap)
	}
	r.mu.Lock()
	r.stored = snap
	r.hasStored = true
	r.mu.Unlock()
	return nil
}

func newTestServiceWithRepo(t *testing.T, home, workDir string, repo SnapshotRepository, ttl time.Duration) *Service {
	t.Helper()
	return NewService(Deps{
		ResolveSessionWorkDir: func(sessionName string) (string, error) {
			if sessionName == "" {
				return "", errors.New("empty session")
			}
			return workDir, nil
		},
		HomeDir: func() (string, error) { return home, nil },
		NowFunc: func() time.Time {
			return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
		},
		SnapshotRepo: repo,
		SnapshotTTL:  ttl,
	})
}

// TestGetUsageDashboard_PersistsAfterFirstAggregation verifies that the
// first call writes a snapshot to the repository.
func TestGetUsageDashboard_PersistsAfterFirstAggregation(t *testing.T) {
	home, workDir := setupFakeHome(t)
	repo := &fakeSnapshotRepository{}
	svc := newTestServiceWithRepo(t, home, workDir, repo, 24*time.Hour)

	if _, err := svc.GetUsageDashboard("session-1", "both", false); err != nil {
		t.Fatalf("GetUsageDashboard: %v", err)
	}
	if repo.saveCalls != 1 {
		t.Errorf("Save called %d times, want 1", repo.saveCalls)
	}
	if !repo.hasStored {
		t.Fatal("repo did not store snapshot")
	}
	if repo.stored.SchemaVersion != snapshotSchemaVersion {
		t.Errorf("stored SchemaVersion = %d, want %d", repo.stored.SchemaVersion, snapshotSchemaVersion)
	}
	if repo.stored.Claude == nil || repo.stored.Codex == nil {
		t.Errorf("stored snapshot must contain BOTH claude and codex regardless of mode (%v / %v)",
			repo.stored.Claude, repo.stored.Codex)
	}
}

// TestGetUsageDashboard_UsesCachedSnapshotWithinTTL verifies that the
// second call within the TTL window returns the cached snapshot without
// triggering a new Save (and therefore without re-aggregating).
func TestGetUsageDashboard_UsesCachedSnapshotWithinTTL(t *testing.T) {
	home, workDir := setupFakeHome(t)
	repo := &fakeSnapshotRepository{}
	svc := newTestServiceWithRepo(t, home, workDir, repo, 24*time.Hour)

	// First call: cache miss → aggregate + save.
	if _, err := svc.GetUsageDashboard("session-1", "both", false); err != nil {
		t.Fatalf("first call: %v", err)
	}
	saveCallsAfterFirst := repo.saveCalls

	// Second call: cache hit → must NOT save again.
	snap2, err := svc.GetUsageDashboard("session-1", "both", false)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if repo.saveCalls != saveCallsAfterFirst {
		t.Errorf("Save called again on cache hit: saves=%d (want still %d)", repo.saveCalls, saveCallsAfterFirst)
	}
	if snap2.LastUpdatedAt.IsZero() {
		t.Error("cached snapshot LastUpdatedAt must be set")
	}
}

// TestGetUsageDashboard_ForceBypassesCache verifies force=true triggers
// re-aggregation even when a fresh snapshot exists.
func TestGetUsageDashboard_ForceBypassesCache(t *testing.T) {
	home, workDir := setupFakeHome(t)
	repo := &fakeSnapshotRepository{}
	svc := newTestServiceWithRepo(t, home, workDir, repo, 24*time.Hour)

	if _, err := svc.GetUsageDashboard("session-1", "both", false); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if repo.saveCalls != 1 {
		t.Fatalf("expected 1 save after first call, got %d", repo.saveCalls)
	}

	if _, err := svc.GetUsageDashboard("session-1", "both", true); err != nil {
		t.Fatalf("forced call: %v", err)
	}
	if repo.saveCalls != 2 {
		t.Errorf("Save called %d times after force=true, want 2", repo.saveCalls)
	}
}

// TestGetUsageDashboard_TTLExpiredReAggregates verifies that a snapshot
// older than the configured TTL triggers re-aggregation.
func TestGetUsageDashboard_TTLExpiredReAggregates(t *testing.T) {
	home, workDir := setupFakeHome(t)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	staleSnap := PersistedSnapshot{
		SchemaVersion: snapshotSchemaVersion,
		WorkDir:       workDir,
		SavedAt:       now.Add(-25 * time.Hour),
		Claude:        &ClaudeUsageStats{Health: SourceHealth{PartialErrors: []string{}}},
		Codex:         &CodexUsageStats{Health: SourceHealth{PartialErrors: []string{}}},
	}
	repo := &fakeSnapshotRepository{stored: staleSnap, hasStored: true}
	svc := newTestServiceWithRepo(t, home, workDir, repo, 24*time.Hour)

	if _, err := svc.GetUsageDashboard("session-1", "both", false); err != nil {
		t.Fatalf("GetUsageDashboard: %v", err)
	}
	if repo.saveCalls != 1 {
		t.Errorf("expected re-aggregate + save when TTL expired, saves=%d", repo.saveCalls)
	}
}

// TestGetUsageDashboard_CorruptCacheRecovers verifies that a Load error
// is logged and treated as a miss, triggering re-aggregation.
func TestGetUsageDashboard_CorruptCacheRecovers(t *testing.T) {
	home, workDir := setupFakeHome(t)
	repo := &fakeSnapshotRepository{
		loadFn: func(string) (PersistedSnapshot, bool, error) {
			return PersistedSnapshot{}, false, errors.New("synthetic parse failure")
		},
	}
	svc := newTestServiceWithRepo(t, home, workDir, repo, 24*time.Hour)

	snap, err := svc.GetUsageDashboard("session-1", "both", false)
	if err != nil {
		t.Fatalf("GetUsageDashboard: %v", err)
	}
	if repo.saveCalls != 1 {
		t.Errorf("expected re-aggregate + save on corrupt cache, saves=%d", repo.saveCalls)
	}
	if snap.LastUpdatedAt.IsZero() {
		t.Error("LastUpdatedAt must be set after recovery")
	}
}

// TestGetUsageDashboard_PartialCacheTreatedAsMiss verifies the defensive
// guard: if a cached snapshot somehow lacks Claude or Codex (e.g. legacy
// writer), the service re-aggregates instead of serving partial data.
func TestGetUsageDashboard_PartialCacheTreatedAsMiss(t *testing.T) {
	home, workDir := setupFakeHome(t)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	partial := PersistedSnapshot{
		SchemaVersion: snapshotSchemaVersion,
		WorkDir:       workDir,
		SavedAt:       now.Add(-1 * time.Hour),
		Claude:        &ClaudeUsageStats{Health: SourceHealth{PartialErrors: []string{}}},
		// Codex deliberately nil
	}
	repo := &fakeSnapshotRepository{stored: partial, hasStored: true}
	svc := newTestServiceWithRepo(t, home, workDir, repo, 24*time.Hour)

	if _, err := svc.GetUsageDashboard("session-1", "both", false); err != nil {
		t.Fatalf("GetUsageDashboard: %v", err)
	}
	if repo.saveCalls != 1 {
		t.Errorf("partial cache must trigger re-aggregate, saves=%d", repo.saveCalls)
	}
	if repo.stored.Codex == nil {
		t.Error("after re-aggregation Codex must be populated")
	}
}

func TestGetUsageDashboard_PartialCacheWithMissingClaudeTreatedAsMiss(t *testing.T) {
	home, workDir := setupFakeHome(t)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	partial := PersistedSnapshot{
		SchemaVersion: snapshotSchemaVersion,
		WorkDir:       workDir,
		SavedAt:       now.Add(-1 * time.Hour),
		Codex:         &CodexUsageStats{Health: SourceHealth{PartialErrors: []string{}}},
	}
	repo := &fakeSnapshotRepository{stored: partial, hasStored: true}
	svc := newTestServiceWithRepo(t, home, workDir, repo, 24*time.Hour)

	if _, err := svc.GetUsageDashboard("session-1", "both", false); err != nil {
		t.Fatalf("GetUsageDashboard: %v", err)
	}
	if repo.saveCalls != 1 {
		t.Errorf("partial cache must trigger re-aggregate, saves=%d", repo.saveCalls)
	}
	if repo.stored.Claude == nil {
		t.Error("after re-aggregation Claude must be populated")
	}
}

func TestGetUsageDashboard_NormalizesCachedSlices(t *testing.T) {
	home, workDir := setupFakeHome(t)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	cached := PersistedSnapshot{
		SchemaVersion: snapshotSchemaVersion,
		WorkDir:       workDir,
		SavedAt:       now.Add(-1 * time.Hour),
		Claude:        &ClaudeUsageStats{},
		Codex:         &CodexUsageStats{},
	}
	repo := &fakeSnapshotRepository{stored: cached, hasStored: true}
	svc := newTestServiceWithRepo(t, home, workDir, repo, 24*time.Hour)

	snap, err := svc.GetUsageDashboard("session-1", "both", false)
	if err != nil {
		t.Fatalf("GetUsageDashboard: %v", err)
	}
	if repo.saveCalls != 0 {
		t.Errorf("normalized cache should remain a cache hit, saves=%d", repo.saveCalls)
	}
	if snap.Claude == nil || snap.Codex == nil {
		t.Fatalf("cache hit must return both stats: %+v", snap)
	}
	if snap.Claude.Skills == nil || snap.Claude.Agents == nil || snap.Claude.SlashCommands == nil || snap.Claude.DailyActivity == nil {
		t.Fatalf("Claude cached slices must be non-nil: %+v", snap.Claude)
	}
	if snap.Codex.Skills == nil || snap.Codex.Agents == nil || snap.Codex.DailyActivity == nil {
		t.Fatalf("Codex cached slices must be non-nil: %+v", snap.Codex)
	}
	if snap.Claude.Health.PartialErrors == nil || snap.Codex.Health.PartialErrors == nil {
		t.Fatalf("cached PartialErrors must be normalized: claude=%v codex=%v", snap.Claude.Health.PartialErrors, snap.Codex.Health.PartialErrors)
	}
}

// TestGetUsageDashboard_SaveFailureStillReturnsFreshData verifies that a
// snapshot persistence failure is logged but does NOT propagate as an
// IPC error: dashboards must remain usable on read-only project dirs.
func TestGetUsageDashboard_SaveFailureStillReturnsFreshData(t *testing.T) {
	home, workDir := setupFakeHome(t)
	repo := &fakeSnapshotRepository{
		saveFn: func(PersistedSnapshot) error {
			return errors.New("synthetic disk full")
		},
	}
	svc := newTestServiceWithRepo(t, home, workDir, repo, 24*time.Hour)

	snap, err := svc.GetUsageDashboard("session-1", "both", false)
	if err != nil {
		t.Fatalf("Save failure must not surface as IPC error: %v", err)
	}
	if snap.Claude == nil || snap.Codex == nil {
		t.Errorf("fresh snapshot must be returned despite save failure: %+v", snap)
	}
	if snap.LastUpdatedAt.IsZero() {
		t.Error("LastUpdatedAt must be set even when persistence failed")
	}
	if repo.saveCalls != 1 {
		t.Errorf("Save called %d times, want 1", repo.saveCalls)
	}
}

func TestGetUsageDashboard_ReleasesMutexBeforeAggregation(t *testing.T) {
	home, workDir := setupSlowAggregationHome(t, 25000)
	firstLoadReturned := make(chan struct{})
	secondLoadObserved := make(chan struct{})
	var loadMu sync.Mutex
	loadCalls := 0

	repo := &fakeSnapshotRepository{
		loadFn: func(string) (PersistedSnapshot, bool, error) {
			loadMu.Lock()
			loadCalls++
			call := loadCalls
			loadMu.Unlock()
			switch call {
			case 1:
				close(firstLoadReturned)
			case 2:
				close(secondLoadObserved)
			}
			return PersistedSnapshot{}, false, nil
		},
	}
	svc := newTestServiceWithRepo(t, home, workDir, repo, 24*time.Hour)

	firstDone := make(chan error, 1)
	go func() {
		_, err := svc.GetUsageDashboard("session-1", "both", false)
		firstDone <- err
	}()

	select {
	case <-firstLoadReturned:
	case err := <-firstDone:
		t.Fatalf("first call finished before cache miss observation: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first cache miss")
	}

	secondDone := make(chan error, 1)
	go func() {
		_, err := svc.GetUsageDashboard("session-1", "both", false)
		secondDone <- err
	}()

	select {
	case <-secondLoadObserved:
	case err := <-firstDone:
		t.Fatalf("second call stayed blocked until first call completed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for second call to reach cache load during aggregation")
	}

	if err := <-firstDone; err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second call: %v", err)
	}
}

// TestGetUsageDashboard_ModeFilterServesCachedCodexOnly verifies the codex
// counterpart of mode-filter cache hit (sibling parity with the claude case).
func TestGetUsageDashboard_ModeFilterServesCachedCodexOnly(t *testing.T) {
	home, workDir := setupFakeHome(t)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	full := PersistedSnapshot{
		SchemaVersion: snapshotSchemaVersion,
		WorkDir:       workDir,
		SavedAt:       now.Add(-1 * time.Hour),
		Claude:        &ClaudeUsageStats{TotalSessions: 7, Health: SourceHealth{PartialErrors: []string{}}},
		Codex:         &CodexUsageStats{TotalPrompts: 11, Health: SourceHealth{PartialErrors: []string{}}},
	}
	repo := &fakeSnapshotRepository{stored: full, hasStored: true}
	svc := newTestServiceWithRepo(t, home, workDir, repo, 24*time.Hour)

	snap, err := svc.GetUsageDashboard("session-1", "codex", false)
	if err != nil {
		t.Fatalf("GetUsageDashboard: %v", err)
	}
	if repo.saveCalls != 0 {
		t.Errorf("cache hit must not Save, saves=%d", repo.saveCalls)
	}
	if snap.Codex == nil || snap.Claude != nil {
		t.Errorf("mode=codex must drop Claude: claude=%v codex=%v", snap.Claude, snap.Codex)
	}
	if snap.Codex.TotalPrompts != 11 {
		t.Errorf("cached Codex.TotalPrompts = %d, want 11", snap.Codex.TotalPrompts)
	}
}

// TestGetUsageDashboard_ModeFilterServesCachedBoth verifies that cache hit
// for mode="claude" returns Claude only even though the file contains both,
// preserving the #157 mode contract.
func TestGetUsageDashboard_ModeFilterServesCachedBoth(t *testing.T) {
	home, workDir := setupFakeHome(t)
	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	full := PersistedSnapshot{
		SchemaVersion: snapshotSchemaVersion,
		WorkDir:       workDir,
		SavedAt:       now.Add(-1 * time.Hour),
		Claude:        &ClaudeUsageStats{TotalSessions: 7, Health: SourceHealth{PartialErrors: []string{}}},
		Codex:         &CodexUsageStats{TotalPrompts: 3, Health: SourceHealth{PartialErrors: []string{}}},
	}
	repo := &fakeSnapshotRepository{stored: full, hasStored: true}
	svc := newTestServiceWithRepo(t, home, workDir, repo, 24*time.Hour)

	snap, err := svc.GetUsageDashboard("session-1", "claude", false)
	if err != nil {
		t.Fatalf("GetUsageDashboard: %v", err)
	}
	if repo.saveCalls != 0 {
		t.Errorf("cache hit must not Save, saves=%d", repo.saveCalls)
	}
	if snap.Claude == nil || snap.Codex != nil {
		t.Errorf("mode=claude must drop Codex: claude=%v codex=%v", snap.Claude, snap.Codex)
	}
	if snap.Claude.TotalSessions != 7 {
		t.Errorf("cached Claude.TotalSessions = %d, want 7", snap.Claude.TotalSessions)
	}
}

func TestApplyClaudeRecordCountsToolCall(t *testing.T) {
	stats := &ClaudeUsageStats{}
	skills := NewUsageCounter()
	agents := NewUsageCounter()
	slash := NewUsageCounter()
	daily := NewDailyAggregator(time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC), 3)
	ts := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)

	applyClaudeRecord(claude.Record{Type: claude.RecordToolCall, Timestamp: ts}, stats, skills, agents, slash, daily, true)

	if stats.TotalToolUses != 1 {
		t.Fatalf("TotalToolUses = %d, want 1", stats.TotalToolUses)
	}
	buckets := daily.Buckets()
	if buckets[len(buckets)-1].ToolCalls != 1 {
		t.Fatalf("ToolCalls = %d, want 1", buckets[len(buckets)-1].ToolCalls)
	}
}
