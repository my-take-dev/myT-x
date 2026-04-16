package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const codexSample = `{"timestamp":"2026-01-23T02:15:40.000Z","type":"session_meta","payload":{"id":"sess-1","timestamp":"2026-01-23T02:15:40.000Z","cwd":"D:\\myT-x\\dev-myT-x"}}
{"timestamp":"2026-01-23T02:15:42.000Z","type":"response_item","payload":{"type":"function_call","name":"spawn_agent","arguments":"{\"agent_type\":\"code-reviewer\"}"}}
{"timestamp":"2026-01-23T02:15:45.000Z","type":"response_item","payload":{"type":"function_call","name":"spawn_agent","arguments":"{\"agent_type\":\"code-reviewer\"}"}}
{"timestamp":"2026-01-23T02:15:50.000Z","type":"response_item","payload":{"type":"function_call","name":"shell_command","arguments":"{\"command\":\"cat .claude/skills/go-test-patterns/SKILL.md\",\"workdir\":\"D:\\\\foo\"}"}}
{"timestamp":"2026-01-23T02:15:55.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}
bogus-line
{"timestamp":"2026-01-23T02:16:00.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}}
`

func writeFile(t *testing.T, dir, name, contents string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestParseFileMatchesCwd(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "rollout-sess-1.jsonl", codexSample)
	matcher := func(cwd string) bool {
		return strings.EqualFold(strings.TrimSpace(cwd), `D:\myT-x\dev-myT-x`)
	}
	var partial []string
	session, matched, err := ParseFile(path, matcher, &partial)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if !matched {
		t.Fatal("expected matched=true for cwd match")
	}
	if session.SessionID != "sess-1" {
		t.Errorf("SessionID = %q", session.SessionID)
	}

	var spawns, skills, userMsgs, assistantMsgs int
	for _, rec := range session.Records {
		switch rec.Type {
		case RecordSpawnAgent:
			spawns++
		case RecordSkillRead:
			skills++
			if rec.Name != "go-test-patterns" {
				t.Errorf("skill name = %q, want go-test-patterns", rec.Name)
			}
		case RecordUserPrompt:
			userMsgs++
		case RecordAssistantMessage:
			assistantMsgs++
		}
	}
	if spawns != 2 {
		t.Errorf("spawn_agent count = %d, want 2", spawns)
	}
	if skills != 1 {
		t.Errorf("skill reads = %d, want 1", skills)
	}
	if userMsgs != 1 {
		t.Errorf("user prompts = %d, want 1", userMsgs)
	}
	if assistantMsgs != 1 {
		t.Errorf("assistant messages = %d, want 1", assistantMsgs)
	}
	if len(partial) != 1 {
		t.Errorf("partial errors = %d, want 1 (bogus-line): %+v", len(partial), partial)
	}
}

func TestParseFileRejectsNonMatchingCwd(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "rollout-sess-2.jsonl", codexSample)
	matcher := func(cwd string) bool { return false }
	var partial []string
	_, matched, err := ParseFile(path, matcher, &partial)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if matched {
		t.Error("expected matched=false when matcher returns false")
	}
}

func TestParseFileReportsMalformedSessionMetaPayload(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "rollout-bad-meta.jsonl", `{"timestamp":"2026-01-23T02:15:40.000Z","type":"session_meta","payload":"not-an-object"}
{"timestamp":"2026-01-23T02:15:42.000Z","type":"response_item","payload":{"type":"function_call","name":"spawn_agent","arguments":"{\"agent_type\":\"code-reviewer\"}"}}`)
	matcher := func(cwd string) bool { return true }

	var partial []string
	session, matched, err := ParseFile(path, matcher, &partial)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if matched {
		t.Fatal("expected matched=false when no valid session_meta payload exists")
	}
	if session.SessionID != "" {
		t.Fatalf("SessionID = %q, want empty", session.SessionID)
	}
	if len(partial) != 2 {
		t.Fatalf("partial errors = %d, want 2: %+v", len(partial), partial)
	}
	if !strings.Contains(partial[0], "session_meta payload unmarshal") {
		t.Fatalf("first partial error = %q", partial[0])
	}
	if !strings.Contains(partial[1], "no valid session_meta line found") {
		t.Fatalf("second partial error = %q", partial[1])
	}
}

func TestParseFileReportsMissingSessionMeta(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "rollout-no-meta.jsonl", `{"timestamp":"2026-01-23T02:15:42.000Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}}`)
	matcher := func(cwd string) bool { return true }

	var partial []string
	_, matched, err := ParseFile(path, matcher, &partial)
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	if matched {
		t.Fatal("expected matched=false when session_meta is missing")
	}
	if len(partial) != 1 {
		t.Fatalf("partial errors = %d, want 1: %+v", len(partial), partial)
	}
	if !strings.Contains(partial[0], "no valid session_meta line found") {
		t.Fatalf("partial error = %q", partial[0])
	}
}

func TestExtractSkillReadName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple path", `{"command":"cat .claude/skills/go-test-patterns/SKILL.md"}`, "go-test-patterns"},
		{"pwsh path", `{"command":"Get-Content .codex\\skills\\mcp-integration\\SKILL.md"}`, "mcp-integration"},
		{"quoted unicode path", "{\"command\":\"Get-Content \\\"C:\\\\\\\\Users\\\\\\\\\u0130nput\\\\\\\\.codex\\\\\\\\skills\\\\\\\\\u65e5\u672c\u8a9e Skill\\\\\\\\SKILL.md\\\"\"}", "日本語 Skill"},
		{"no SKILL.md", `{"command":"ls"}`, ""},
		{"empty", ``, ""},
		{"invalid json", `not json`, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractSkillReadName(tc.in)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestListRolloutFilesSorted(t *testing.T) {
	root := t.TempDir()
	day := filepath.Join(root, "2026", "04", "15")
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(day, "rollout-b.jsonl"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(day, "rollout-a.jsonl"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(day, "unrelated.txt"), []byte(""), 0o644)

	paths, err := ListRolloutFiles(root)
	if err != nil {
		t.Fatalf("ListRolloutFiles: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 rollout files, got %d: %v", len(paths), paths)
	}
	if !strings.HasSuffix(paths[0], "rollout-a.jsonl") {
		t.Errorf("sorted order violated: %v", paths)
	}
}
