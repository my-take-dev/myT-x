// Package claude parses Claude Code JSONL session logs under
// ~/.claude/projects/<slug>/*.jsonl and its subagents/ subdirectories.
//
// Each JSONL line is one conversation record with fields such as:
//
//	{
//	  "type": "assistant",
//	  "timestamp": "2026-04-15T01:30:45.123Z",
//	  "sessionId": "002ad280-...",
//	  "cwd": "D:\\myT-x\\dev-myT-x",
//	  "message": {
//	    "role": "assistant",
//	    "content": [
//	      {"type":"tool_use","name":"Skill","input":{"skill":"foo"}},
//	      {"type":"tool_use","name":"Agent","input":{"subagent_type":"bar"}}
//	    ]
//	  }
//	}
//
// The subagent-launching tool is called "Agent" in current Claude Code builds;
// the legacy name "Task" is still accepted for backward compatibility.
//
// User messages may carry a `<command-name>/xxx</command-name>` marker inside
// their string content when a slash command was invoked. Qualified command
// names in "namespace:name" form (e.g. "feature-dev:feature-dev") are treated
// as Skill invocations; all other commands stay in the SlashCommand category.
//
// Parse failures are classified per defensive-coding-checklist #170:
//   - Expected (missing field, wrong type) → silently skipped
//   - Unexpected (malformed JSON)          → appended to PartialErrors
package claude

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Record is the normalized tool-use or slash-command observation emitted by the parser.
type Record struct {
	Type       RecordType
	Name       string
	Timestamp  time.Time
	SessionID  string
	IsToolCall bool // true for Skill/Agent/other tool uses
}

// RecordType distinguishes the source of a Record.
type RecordType uint8

const (
	RecordSkill RecordType = iota + 1
	RecordAgent
	RecordSlash
	RecordUserMessage
	RecordAssistantMessage
	RecordToolCall // non-Skill/Agent tool uses (for daily tool-call counter)
)

// Session represents one JSONL file's aggregated state.
type Session struct {
	SessionID string
	FirstSeen time.Time
	Records   []Record
}

// slashCommandRE extracts "/xxx" from <command-name>/xxx</command-name> tags
// found in user message content.
var slashCommandRE = regexp.MustCompile(`<command-name>\s*/([A-Za-z0-9_\-:.]+)`)

// qualifiedSlashRE matches slash command names in "namespace:name" form
// (e.g., "feature-dev:feature-dev", "pr-review-toolkit:review-pr"). These
// are user-installed skills invoked via slash-command syntax and are routed
// to the Skill ranking. Commands without ":" remain in SlashCommand so
// history.jsonl and session JSONL classify the same marker identically.
var qualifiedSlashRE = regexp.MustCompile(`^[A-Za-z0-9_\-]+:[A-Za-z0-9_\-]+$`)

// lineEnvelope captures only the fields needed for aggregation.
type lineEnvelope struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	SessionID string          `json:"sessionId"`
	Message   json.RawMessage `json:"message"`
}

type messageEnvelope struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type toolUseBlock struct {
	Type  string         `json:"type"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

type textBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type historyEnvelope struct {
	Display   string `json:"display"`
	Timestamp int64  `json:"timestamp"`
	Project   string `json:"project"`
	SessionID string `json:"sessionId"`
}

const maxHistoryPartialErrors = 10

// ParseFile reads one JSONL file and returns its Session. Malformed lines are
// appended to partialErrors (prefixed with path:line) and skipped.
//
// Empty files produce an empty Session with no records and no error.
func ParseFile(path string, partialErrors *[]string) (Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return Session{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			slog.Warn("[usage-dashboard] close claude session file", "path", path, "error", closeErr)
		}
	}()

	session := Session{}
	scanner := bufio.NewScanner(f)
	// Allow long lines (Claude JSONL can embed large tool results).
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		// Strip optional UTF-8 BOM on the first line.
		if lineNo == 1 && len(line) >= 3 && line[0] == 0xEF && line[1] == 0xBB && line[2] == 0xBF {
			line = line[3:]
			if len(line) == 0 {
				continue
			}
		}
		if err := appendLine(&session, line); err != nil {
			if partialErrors != nil {
				*partialErrors = append(*partialErrors, fmt.Sprintf("%s:%d: %s", filepath.Base(path), lineNo, err.Error()))
			}
		}
	}
	if err := scanner.Err(); err != nil {
		if partialErrors != nil {
			*partialErrors = append(*partialErrors, fmt.Sprintf("%s: scanner: %s", filepath.Base(path), err.Error()))
		}
	}
	return session, nil
}

// ParseHistoryFile reads Claude's history.jsonl and returns slash command
// records matching the active project. History and session JSONL must share
// classifySlashCommand so deduplication cannot drift when rules change.
func ParseHistoryFile(path string, projectMatcher func(string) bool, partialErrors *[]string) ([]Record, error) {
	if projectMatcher == nil {
		return nil, fmt.Errorf("project matcher is nil")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			slog.Warn("[usage-dashboard] close claude history file", "path", path, "error", closeErr)
		}
	}()

	records := make([]Record, 0)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	lineNo := 0
	historyErrors := make([]string, 0, maxHistoryPartialErrors)
	historyErrorCount := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if lineNo == 1 && len(line) >= 3 && line[0] == 0xEF && line[1] == 0xBB && line[2] == 0xBF {
			line = line[3:]
			if len(line) == 0 {
				continue
			}
		}
		rec, ok, err := parseHistoryLine(line, projectMatcher)
		if err != nil {
			historyErrorCount++
			if len(historyErrors) < maxHistoryPartialErrors {
				historyErrors = append(historyErrors, fmt.Sprintf("%s:%d: %s", filepath.Base(path), lineNo, err.Error()))
			}
			continue
		}
		if ok {
			records = append(records, rec)
		}
	}
	if err := scanner.Err(); err != nil {
		if partialErrors != nil {
			*partialErrors = append(*partialErrors, fmt.Sprintf("%s: scanner: %s", filepath.Base(path), err.Error()))
		}
	}
	appendHistoryPartialErrors(partialErrors, filepath.Base(path), historyErrors, historyErrorCount)
	return records, nil
}

func parseHistoryLine(line []byte, projectMatcher func(string) bool) (Record, bool, error) {
	var env historyEnvelope
	if err := json.Unmarshal(line, &env); err != nil {
		return Record{}, false, fmt.Errorf("unmarshal history envelope: %w", err)
	}
	if !projectMatcher(env.Project) {
		return Record{}, false, nil
	}
	if env.Timestamp <= 0 {
		return Record{}, false, nil
	}
	name := extractDisplayCommand(env.Display)
	if name == "" {
		return Record{}, false, nil
	}
	return Record{
		Type:      classifySlashCommand(name),
		Name:      name,
		Timestamp: time.UnixMilli(env.Timestamp).UTC(),
		SessionID: env.SessionID,
	}, true, nil
}

func extractDisplayCommand(display string) string {
	fields := strings.Fields(strings.TrimSpace(display))
	if len(fields) == 0 {
		return ""
	}
	first := fields[0]
	if !strings.HasPrefix(first, "/") {
		return ""
	}
	name := strings.TrimPrefix(first, "/")
	return normalizeSlashCommandName(name)
}

func normalizeSlashCommandName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimRightFunc(name, func(r rune) bool {
		return !isSlashCommandNameRune(r)
	})
	return strings.ToLower(strings.TrimSpace(name))
}

func isSlashCommandNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == ':' || r == '.'
}

func classifySlashCommand(name string) RecordType {
	name = normalizeSlashCommandName(name)
	if qualifiedSlashRE.MatchString(name) {
		return RecordSkill
	}
	return RecordSlash
}

func appendHistoryPartialErrors(partialErrors *[]string, base string, errors []string, total int) {
	if partialErrors == nil || total == 0 {
		return
	}
	*partialErrors = append(*partialErrors, errors...)
	if total > len(errors) {
		*partialErrors = append(*partialErrors, fmt.Sprintf("%s: omitted %d additional history parse errors", base, total-len(errors)))
	}
}

func appendLine(session *Session, line []byte) error {
	var env lineEnvelope
	if err := json.Unmarshal(line, &env); err != nil {
		return fmt.Errorf("unmarshal envelope: %w", err)
	}
	ts := parseTime(env.Timestamp)
	if session.SessionID == "" && env.SessionID != "" {
		session.SessionID = env.SessionID
	}
	if !ts.IsZero() && (session.FirstSeen.IsZero() || ts.Before(session.FirstSeen)) {
		session.FirstSeen = ts
	}
	if len(env.Message) == 0 {
		// Still record message count for activity tracking when possible.
		return nil
	}
	var msg messageEnvelope
	if err := json.Unmarshal(env.Message, &msg); err != nil {
		// Expected: message field is sometimes a string for other record types.
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(env.Type)) {
	case "user":
		session.Records = append(session.Records, Record{Type: RecordUserMessage, Timestamp: ts, SessionID: env.SessionID})
		extractSlashCommands(session, msg.Content, ts, env.SessionID)
	case "assistant":
		session.Records = append(session.Records, Record{Type: RecordAssistantMessage, Timestamp: ts, SessionID: env.SessionID})
		extractToolUses(session, msg.Content, ts, env.SessionID)
	}
	return nil
}

// extractSlashCommands scans string-or-array content for <command-name>/xxx</command-name>.
func extractSlashCommands(session *Session, content json.RawMessage, ts time.Time, sessionID string) {
	if len(content) == 0 {
		return
	}
	// Content may be a plain string or an array of content blocks.
	text := ""
	if content[0] == '"' {
		if err := json.Unmarshal(content, &text); err != nil {
			return
		}
	} else if content[0] == '[' {
		var blocks []textBlock
		if err := json.Unmarshal(content, &blocks); err != nil {
			return
		}
		for _, b := range blocks {
			if b.Type == "" || b.Type == "text" {
				text += b.Text + "\n"
			}
		}
	}
	if text == "" {
		return
	}
	matches := slashCommandRE.FindAllStringSubmatch(text, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		name := normalizeSlashCommandName(m[1])
		if name == "" {
			continue
		}
		session.Records = append(session.Records, Record{
			Type:      classifySlashCommand(name),
			Name:      name,
			Timestamp: ts,
			SessionID: sessionID,
		})
	}
}

// extractToolUses scans an assistant content array for tool_use blocks.
func extractToolUses(session *Session, content json.RawMessage, ts time.Time, sessionID string) {
	if len(content) == 0 || content[0] != '[' {
		return
	}
	var blocks []toolUseBlock
	if err := json.Unmarshal(content, &blocks); err != nil {
		return
	}
	for _, b := range blocks {
		if b.Type != "tool_use" {
			continue
		}
		switch b.Name {
		case "Skill":
			name := stringField(b.Input, "skill")
			if name == "" {
				name = stringField(b.Input, "name")
			}
			if name != "" {
				session.Records = append(session.Records, Record{Type: RecordSkill, Name: name, Timestamp: ts, SessionID: sessionID, IsToolCall: true})
			}
		case "Agent", "Task":
			// "Agent" is the current Claude Code tool name for subagent launches;
			// "Task" is kept for backward compatibility with older session logs.
			name := stringField(b.Input, "subagent_type")
			if name == "" {
				name = stringField(b.Input, "agent_type")
			}
			if name != "" {
				session.Records = append(session.Records, Record{Type: RecordAgent, Name: name, Timestamp: ts, SessionID: sessionID, IsToolCall: true})
			}
		default:
			// Other tool uses (Read/Write/Bash/...) contribute to tool-call activity.
			session.Records = append(session.Records, Record{Type: RecordToolCall, Name: b.Name, Timestamp: ts, SessionID: sessionID, IsToolCall: true})
		}
	}
}

func stringField(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func parseTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

// ListProjectFiles returns *.jsonl files under projectDir sorted by name.
// Sorting ensures deterministic aggregation ordering (#151).
func ListProjectFiles(projectDir string) ([]string, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".jsonl") {
			paths = append(paths, filepath.Join(projectDir, name))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// ListSubagentFiles returns all *.jsonl files under <projectDir>/<any>/subagents/
// sorted by name. These files carry Claude Code subagent execution logs and let
// the aggregator count Skill/Agent uses that happened inside spawned subagents.
//
// Each JSONL entry in these files has `"isSidechain": true`; callers should
// still count Skill/Agent/ToolCall records but must NOT count sessions or
// messages again because the parent session already accounts for them
// (the parent's "Agent" tool_use represents the same subagent invocation).
//
// Returns an empty slice (no error) when projectDir has no subagent directories
// yet. I/O errors from os.ReadDir on projectDir propagate; per-subdirectory
// errors are collected as partial errors and do not abort the walk.
func ListSubagentFiles(projectDir string) ([]string, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subagentDir := filepath.Join(projectDir, entry.Name(), "subagents")
		children, err := os.ReadDir(subagentDir)
		if err != nil {
			// Most session dirs do not contain a subagents folder; missing is
			// the common case and must not be reported as an error.
			continue
		}
		for _, child := range children {
			if child.IsDir() {
				continue
			}
			name := child.Name()
			if strings.HasSuffix(strings.ToLower(name), ".jsonl") {
				paths = append(paths, filepath.Join(subagentDir, name))
			}
		}
	}
	sort.Strings(paths)
	return paths, nil
}
