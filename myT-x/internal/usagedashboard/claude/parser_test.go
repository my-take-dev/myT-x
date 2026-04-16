package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleJSONL keeps "Task" intentionally to verify backward compatibility with
// the legacy tool name. "/review" and "/commit" are built-in slash commands
// (no namespace) so they must be classified as RecordSlash.
const sampleJSONL = `{"type":"user","timestamp":"2026-04-15T10:00:00Z","sessionId":"s1","message":{"role":"user","content":"<command-name>/review</command-name> please"}}
{"type":"assistant","timestamp":"2026-04-15T10:00:05Z","sessionId":"s1","message":{"role":"assistant","content":[{"type":"tool_use","name":"Skill","input":{"skill":"go-test-patterns"}}]}}
{"type":"assistant","timestamp":"2026-04-15T10:00:10Z","sessionId":"s1","message":{"role":"assistant","content":[{"type":"tool_use","name":"Task","input":{"subagent_type":"golang-expert"}}]}}
{"type":"assistant","timestamp":"2026-04-15T10:00:15Z","sessionId":"s1","message":{"role":"assistant","content":[{"type":"tool_use","name":"Read","input":{"file_path":"x"}}]}}
{"type":"user","timestamp":"2026-04-15T10:00:20Z","sessionId":"s1","message":{"role":"user","content":[{"type":"text","text":"<command-name>/commit</command-name>"}]}}
this-line-is-not-json
{"type":"user","timestamp":"2026-04-15T10:00:25Z","sessionId":"s1","message":{"role":"user","content":"plain text with no command"}}
`

func writeSample(t *testing.T, dir, contents string) string {
	t.Helper()
	p := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestParseFileHappyPath(t *testing.T) {
	dir := t.TempDir()
	path := writeSample(t, dir, sampleJSONL)
	var partial []string
	session, err := ParseFile(path, &partial)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if session.SessionID != "s1" {
		t.Errorf("SessionID = %q, want s1", session.SessionID)
	}

	var skills, agents, slash, toolCalls, userMsgs int
	for _, rec := range session.Records {
		switch rec.Type {
		case RecordSkill:
			skills++
			if rec.Name != "go-test-patterns" {
				t.Errorf("skill name = %q", rec.Name)
			}
		case RecordAgent:
			agents++
			if rec.Name != "golang-expert" {
				t.Errorf("agent name = %q", rec.Name)
			}
		case RecordSlash:
			slash++
		case RecordToolCall:
			toolCalls++
		case RecordUserMessage:
			userMsgs++
		}
	}
	if skills != 1 {
		t.Errorf("skills = %d, want 1", skills)
	}
	if agents != 1 {
		t.Errorf("agents = %d, want 1", agents)
	}
	if slash != 2 {
		t.Errorf("slash = %d, want 2 (both /review and /commit)", slash)
	}
	if toolCalls != 1 {
		t.Errorf("other tool calls = %d, want 1 (Read)", toolCalls)
	}
	if userMsgs != 3 {
		t.Errorf("user messages = %d, want 3", userMsgs)
	}

	// Malformed line should produce exactly one partial error.
	if len(partial) != 1 {
		t.Errorf("partial errors = %d, want 1: %+v", len(partial), partial)
	}
}

func TestParseFileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := writeSample(t, dir, "")
	var partial []string
	session, err := ParseFile(path, &partial)
	if err != nil {
		t.Fatalf("empty file: %v", err)
	}
	if len(session.Records) != 0 {
		t.Errorf("expected no records, got %d", len(session.Records))
	}
	if len(partial) != 0 {
		t.Errorf("expected no partial errors, got %+v", partial)
	}
}

func TestParseFileBOM(t *testing.T) {
	dir := t.TempDir()
	content := "\uFEFF" + `{"type":"assistant","timestamp":"2026-04-15T10:00:00Z","sessionId":"sx","message":{"role":"assistant","content":[{"type":"tool_use","name":"Skill","input":{"skill":"boo"}}]}}` + "\n"
	path := writeSample(t, dir, content)
	var partial []string
	session, err := ParseFile(path, &partial)
	if err != nil {
		t.Fatalf("BOM file: %v", err)
	}
	if len(session.Records) == 0 || session.Records[len(session.Records)-1].Name != "boo" {
		t.Errorf("expected skill 'boo', got records %+v", session.Records)
	}
	if len(partial) != 0 {
		t.Errorf("expected no partial errors for BOM, got %+v", partial)
	}
}

func TestListProjectFilesSorted(t *testing.T) {
	dir := t.TempDir()
	writeSample(t, dir, "")
	_ = os.WriteFile(filepath.Join(dir, "z.jsonl"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte(""), 0o644)

	paths, err := ListProjectFiles(dir)
	if err != nil {
		t.Fatalf("ListProjectFiles: %v", err)
	}
	if len(paths) != 3 {
		t.Fatalf("expected 3 jsonl paths, got %d", len(paths))
	}
	// Sorted ascending.
	for i := 1; i < len(paths); i++ {
		if paths[i] < paths[i-1] {
			t.Errorf("paths not sorted: %v", paths)
		}
	}
}

// TestExtractToolUsesAgentAndTask verifies that both the current "Agent" tool
// name and the legacy "Task" tool name are detected as subagent launches, and
// that the subagent_type/agent_type fallback order is honored.
func TestExtractToolUsesAgentAndTask(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantType  RecordType
		wantName  string
		wantCount int
	}{
		{
			name:      "agent_current_subagent_type",
			line:      `{"type":"assistant","timestamp":"2026-04-15T10:00:00Z","sessionId":"s","message":{"role":"assistant","content":[{"type":"tool_use","name":"Agent","input":{"subagent_type":"Explore"}}]}}`,
			wantType:  RecordAgent,
			wantName:  "Explore",
			wantCount: 1,
		},
		{
			name:      "agent_legacy_task_name",
			line:      `{"type":"assistant","timestamp":"2026-04-15T10:00:00Z","sessionId":"s","message":{"role":"assistant","content":[{"type":"tool_use","name":"Task","input":{"subagent_type":"golang-expert"}}]}}`,
			wantType:  RecordAgent,
			wantName:  "golang-expert",
			wantCount: 1,
		},
		{
			name:      "agent_type_fallback_when_subagent_type_missing",
			line:      `{"type":"assistant","timestamp":"2026-04-15T10:00:00Z","sessionId":"s","message":{"role":"assistant","content":[{"type":"tool_use","name":"Agent","input":{"agent_type":"Plan"}}]}}`,
			wantType:  RecordAgent,
			wantName:  "Plan",
			wantCount: 1,
		},
		{
			name:      "agent_qualified_subagent_type",
			line:      `{"type":"assistant","timestamp":"2026-04-15T10:00:00Z","sessionId":"s","message":{"role":"assistant","content":[{"type":"tool_use","name":"Agent","input":{"subagent_type":"pr-review-toolkit:code-reviewer"}}]}}`,
			wantType:  RecordAgent,
			wantName:  "pr-review-toolkit:code-reviewer",
			wantCount: 1,
		},
		{
			name:      "agent_no_type_is_skipped",
			line:      `{"type":"assistant","timestamp":"2026-04-15T10:00:00Z","sessionId":"s","message":{"role":"assistant","content":[{"type":"tool_use","name":"Agent","input":{}}]}}`,
			wantCount: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeSample(t, dir, tt.line+"\n")
			var partial []string
			session, err := ParseFile(path, &partial)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			var found int
			for _, rec := range session.Records {
				if rec.Type == RecordAgent {
					found++
					if tt.wantCount > 0 && rec.Name != tt.wantName {
						t.Errorf("agent name = %q, want %q", rec.Name, tt.wantName)
					}
				}
			}
			if found != tt.wantCount {
				t.Errorf("agent record count = %d, want %d", found, tt.wantCount)
			}
		})
	}
}

// TestExtractSlashCommandsClassification verifies that qualified slash commands
// ("namespace:name") are routed to RecordSkill while built-in and non-qualified
// commands stay in RecordSlash.
func TestExtractSlashCommandsClassification(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		wantType RecordType
	}{
		// Qualified (namespace:name) → Skill
		{"qualified_feature_dev", "feature-dev:feature-dev", RecordSkill},
		{"qualified_pr_review_toolkit", "pr-review-toolkit:review-pr", RecordSkill},
		{"qualified_codex_setup", "codex:setup", RecordSkill},
		{"qualified_coderabbit", "coderabbit:code-review", RecordSkill},
		{"qualified_claude_mem", "claude-mem:mem-search", RecordSkill},
		// Built-in / non-qualified → Slash
		{"builtin_clear", "clear", RecordSlash},
		{"builtin_model", "model", RecordSlash},
		{"builtin_help", "help", RecordSlash},
		{"builtin_review", "review", RecordSlash},
		{"plugin_simplify", "simplify", RecordSlash},
		{"plugin_loop", "loop", RecordSlash},
		{"plugin_reload_plugins", "reload-plugins", RecordSlash},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := `{"type":"user","timestamp":"2026-04-15T10:00:00Z","sessionId":"s","message":{"role":"user","content":"<command-name>/` + tt.command + `</command-name>"}}`
			dir := t.TempDir()
			path := writeSample(t, dir, line+"\n")
			var partial []string
			session, err := ParseFile(path, &partial)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			var matched bool
			for _, rec := range session.Records {
				if rec.Type == RecordSkill || rec.Type == RecordSlash {
					if rec.Name != tt.command {
						continue
					}
					matched = true
					if rec.Type != tt.wantType {
						t.Errorf("command %q: type = %d, want %d", tt.command, rec.Type, tt.wantType)
					}
				}
			}
			if !matched {
				t.Errorf("command %q not recorded at all; records=%+v", tt.command, session.Records)
			}
		})
	}
}

// TestParseFileWithRealPatterns exercises a JSONL fixture that mirrors real
// production logs: Agent tool_use, qualified-slash skills, built-in slash
// commands, and direct Skill tool_use. Verifies the final record counts match
// the classification rules (qualified-slash → Skill, built-in → Slash).
func TestParseFileWithRealPatterns(t *testing.T) {
	const fixture = `{"type":"user","timestamp":"2026-04-15T10:00:00Z","sessionId":"rp","message":{"role":"user","content":"<command-name>/feature-dev:feature-dev</command-name> do"}}
{"type":"assistant","timestamp":"2026-04-15T10:00:05Z","sessionId":"rp","message":{"role":"assistant","content":[{"type":"tool_use","name":"Agent","input":{"subagent_type":"Explore"}}]}}
{"type":"assistant","timestamp":"2026-04-15T10:00:06Z","sessionId":"rp","message":{"role":"assistant","content":[{"type":"tool_use","name":"Agent","input":{"subagent_type":"Plan"}}]}}
{"type":"assistant","timestamp":"2026-04-15T10:00:07Z","sessionId":"rp","message":{"role":"assistant","content":[{"type":"tool_use","name":"Skill","input":{"skill":"go-test-patterns"}}]}}
{"type":"user","timestamp":"2026-04-15T10:00:10Z","sessionId":"rp","message":{"role":"user","content":"<command-name>/pr-review-toolkit:review-pr</command-name>"}}
{"type":"user","timestamp":"2026-04-15T10:00:12Z","sessionId":"rp","message":{"role":"user","content":"<command-name>/clear</command-name>"}}
{"type":"user","timestamp":"2026-04-15T10:00:13Z","sessionId":"rp","message":{"role":"user","content":"<command-name>/model</command-name>"}}
`
	dir := t.TempDir()
	path := writeSample(t, dir, fixture)
	var partial []string
	session, err := ParseFile(path, &partial)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	var skills, agents, slash int
	for _, rec := range session.Records {
		switch rec.Type {
		case RecordSkill:
			skills++
		case RecordAgent:
			agents++
		case RecordSlash:
			slash++
		}
	}
	// 3 skills: direct Skill + 2 qualified slashes (feature-dev, pr-review-toolkit)
	if skills != 3 {
		t.Errorf("skills = %d, want 3 (1 direct Skill + 2 qualified slashes)", skills)
	}
	// 2 agents: Explore + Plan
	if agents != 2 {
		t.Errorf("agents = %d, want 2 (Explore + Plan)", agents)
	}
	// 2 slash: /clear + /model (built-ins only)
	if slash != 2 {
		t.Errorf("slash = %d, want 2 (/clear + /model)", slash)
	}
	if len(partial) != 0 {
		t.Errorf("partial errors = %+v, want none", partial)
	}
}

func TestListSubagentFiles(t *testing.T) {
	t.Run("empty_project_dir_no_error", func(t *testing.T) {
		dir := t.TempDir()
		paths, err := ListSubagentFiles(dir)
		if err != nil {
			t.Fatalf("ListSubagentFiles empty: %v", err)
		}
		if len(paths) != 0 {
			t.Errorf("expected empty slice, got %v", paths)
		}
	})

	t.Run("detects_nested_subagents", func(t *testing.T) {
		dir := t.TempDir()
		// Sessions without subagents subfolder are skipped silently.
		if err := os.MkdirAll(filepath.Join(dir, "sess-without-subagents"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		// Session A: 2 subagent files (unsorted input to verify sort).
		a1 := filepath.Join(dir, "sess-a", "subagents")
		if err := os.MkdirAll(a1, 0o755); err != nil {
			t.Fatalf("mkdir a: %v", err)
		}
		_ = os.WriteFile(filepath.Join(a1, "agent-zzz.jsonl"), []byte(""), 0o644)
		_ = os.WriteFile(filepath.Join(a1, "agent-aaa.jsonl"), []byte(""), 0o644)
		_ = os.WriteFile(filepath.Join(a1, "agent-aaa.meta.json"), []byte("{}"), 0o644) // not jsonl
		// Session B: 1 subagent file.
		b1 := filepath.Join(dir, "sess-b", "subagents")
		if err := os.MkdirAll(b1, 0o755); err != nil {
			t.Fatalf("mkdir b: %v", err)
		}
		_ = os.WriteFile(filepath.Join(b1, "agent-bbb.jsonl"), []byte(""), 0o644)
		// Regular main-session jsonl must not be picked up by this function.
		_ = os.WriteFile(filepath.Join(dir, "main.jsonl"), []byte(""), 0o644)

		paths, err := ListSubagentFiles(dir)
		if err != nil {
			t.Fatalf("ListSubagentFiles: %v", err)
		}
		if len(paths) != 3 {
			t.Fatalf("expected 3 subagent jsonl, got %d: %v", len(paths), paths)
		}
		// Sorted ascending across directories.
		for i := 1; i < len(paths); i++ {
			if paths[i] < paths[i-1] {
				t.Errorf("not sorted: %v", paths)
			}
		}
		// meta.json must not be included.
		for _, p := range paths {
			if strings.HasSuffix(p, ".meta.json") {
				t.Errorf("meta.json included: %s", p)
			}
		}
	})

	t.Run("nonexistent_dir_returns_error", func(t *testing.T) {
		_, err := ListSubagentFiles(filepath.Join(t.TempDir(), "does-not-exist"))
		if err == nil {
			t.Error("expected error for nonexistent dir, got nil")
		}
	})
}
