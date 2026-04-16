// Package codex parses Codex JSONL session rollouts under
// ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl.
//
// Relevant record shapes:
//
//	{"type":"session_meta","payload":{"id":"...","timestamp":"...","cwd":"..."}}
//	{"type":"response_item","payload":{"type":"function_call","name":"spawn_agent","arguments":"{\"agent_type\":\"...\"}"}}
//	{"type":"response_item","payload":{"type":"function_call","name":"shell_command","arguments":"{\"command\":\"...\",\"workdir\":\"...\"}"}}
//	{"type":"response_item","payload":{"type":"message","role":"user","content":[...]}}
//
// Filtering scope: only sessions whose session_meta.cwd matches the active
// session's work directory (exact case-insensitive path match, Windows
// filesystem semantics).
//
// Parse failures are classified per defensive-coding-checklist #170:
//   - Expected (missing field, wrong payload type) → silently skipped
//   - Unexpected (malformed JSON)                  → appended to PartialErrors
package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// RecordType classifies aggregated observations.
type RecordType uint8

const (
	RecordSpawnAgent RecordType = iota + 1
	RecordSkillRead
	RecordUserPrompt
	RecordAssistantMessage
)

// Record is one normalized observation.
type Record struct {
	Type      RecordType
	Name      string
	Timestamp time.Time
	SessionID string
}

// Session is one rollout JSONL file's aggregated state.
type Session struct {
	SessionID string
	Cwd       string
	FirstSeen time.Time
	Records   []Record
}

type lineEnvelope struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type sessionMetaPayload struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Cwd       string `json:"cwd"`
}

type responseItemPayload struct {
	Type      string          `json:"type"`
	Name      string          `json:"name"`
	Arguments string          `json:"arguments"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
}

type spawnAgentArgs struct {
	AgentType string `json:"agent_type"`
	Type      string `json:"type"`
}

type shellCommandArgs struct {
	Command string `json:"command"`
	Workdir string `json:"workdir"`
}

var skillReadPathRE = regexp.MustCompile(`(?i)(?:"([^"\r\n]*SKILL\.md)"|'([^'\r\n]*SKILL\.md)'|` + "`" + `([^` + "`" + `\r\n]*SKILL\.md)` + "`" + `|([^\s"'` + "`" + `]*SKILL\.md))`)

// ParseFile reads one rollout JSONL and returns a Session only when the embedded
// session_meta.cwd matches cwdMatcher. Returns (Session{}, nil) for unmatched
// files so callers can cheaply filter out-of-scope sessions.
//
// partialErrors is appended with "path:line: reason" for malformed JSON lines.
func ParseFile(path string, cwdMatcher func(cwd string) bool, partialErrors *[]string) (Session, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return Session{}, false, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			slog.Warn("[usage-dashboard] close codex rollout file", "path", path, "error", closeErr)
		}
	}()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	var (
		session Session
		matched bool
		checked bool
		lineNo  int
	)

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
		var env lineEnvelope
		if err := json.Unmarshal(line, &env); err != nil {
			if partialErrors != nil {
				*partialErrors = append(*partialErrors, fmt.Sprintf("%s:%d: unmarshal: %s", filepath.Base(path), lineNo, err.Error()))
			}
			continue
		}
		ts := parseTime(env.Timestamp)
		if env.Type == "session_meta" {
			var meta sessionMetaPayload
			if err := json.Unmarshal(env.Payload, &meta); err != nil {
				if partialErrors != nil {
					*partialErrors = append(*partialErrors, fmt.Sprintf("%s:%d: session_meta payload unmarshal: %s", filepath.Base(path), lineNo, err.Error()))
				}
				continue
			}
			session.SessionID = meta.ID
			session.Cwd = meta.Cwd
			if metaTs := parseTime(meta.Timestamp); !metaTs.IsZero() {
				session.FirstSeen = metaTs
			} else if !ts.IsZero() {
				session.FirstSeen = ts
			}
			matched = cwdMatcher == nil || cwdMatcher(meta.Cwd)
			checked = true
			if !matched {
				return Session{Cwd: meta.Cwd}, false, nil
			}
			continue
		}
		if !checked {
			// Defensive: files without session_meta cannot be scoped.
			continue
		}
		if !matched {
			continue
		}
		if env.Type == "response_item" {
			addResponseItem(&session, env.Payload, ts)
		}
	}
	if err := scanner.Err(); err != nil {
		if partialErrors != nil {
			*partialErrors = append(*partialErrors, fmt.Sprintf("%s: scanner: %s", filepath.Base(path), err.Error()))
		}
	}
	if !checked && partialErrors != nil {
		*partialErrors = append(*partialErrors, fmt.Sprintf("%s: no valid session_meta line found", filepath.Base(path)))
	}
	return session, matched, nil
}

func addResponseItem(session *Session, payload json.RawMessage, ts time.Time) {
	if len(payload) == 0 {
		return
	}
	var body responseItemPayload
	if err := json.Unmarshal(payload, &body); err != nil {
		return
	}
	switch strings.ToLower(body.Type) {
	case "function_call":
		addFunctionCall(session, body, ts)
	case "message":
		if strings.EqualFold(body.Role, "user") {
			session.Records = append(session.Records, Record{Type: RecordUserPrompt, Timestamp: ts, SessionID: session.SessionID})
		} else if strings.EqualFold(body.Role, "assistant") {
			session.Records = append(session.Records, Record{Type: RecordAssistantMessage, Timestamp: ts, SessionID: session.SessionID})
		}
	}
}

func addFunctionCall(session *Session, body responseItemPayload, ts time.Time) {
	name := strings.TrimSpace(body.Name)
	switch name {
	case "spawn_agent":
		var args spawnAgentArgs
		if args.AgentType = extractAgentType(body.Arguments); args.AgentType == "" {
			return
		}
		session.Records = append(session.Records, Record{
			Type:      RecordSpawnAgent,
			Name:      args.AgentType,
			Timestamp: ts,
			SessionID: session.SessionID,
		})
	case "shell_command", "run":
		if skill := extractSkillReadName(body.Arguments); skill != "" {
			session.Records = append(session.Records, Record{
				Type:      RecordSkillRead,
				Name:      skill,
				Timestamp: ts,
				SessionID: session.SessionID,
			})
		}
	}
}

func extractAgentType(arguments string) string {
	if arguments == "" {
		return ""
	}
	var args spawnAgentArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	if s := strings.TrimSpace(args.AgentType); s != "" {
		return s
	}
	return strings.TrimSpace(args.Type)
}

// extractSkillReadName infers that a shell_command read a SKILL.md and returns
// the owning directory's basename. Returns "" when the command does not appear
// to read a SKILL.md.
func extractSkillReadName(arguments string) string {
	if arguments == "" {
		return ""
	}
	var args shellCommandArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	cmd := args.Command
	if cmd == "" {
		return ""
	}
	match := skillReadPathRE.FindStringSubmatch(cmd)
	if match == nil {
		return ""
	}
	pathLike := ""
	for _, candidate := range match[1:] {
		if candidate != "" {
			pathLike = candidate
			break
		}
	}
	if !strings.EqualFold(filepath.Base(pathLike), "SKILL.md") {
		return ""
	}
	parent := filepath.Dir(pathLike)
	name := filepath.Base(parent)
	if name == "" || name == "." || name == "/" || name == `\` {
		return ""
	}
	return name
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

// ListRolloutFiles walks sessionsRoot recursively and returns all
// rollout-*.jsonl paths sorted by name.
func ListRolloutFiles(sessionsRoot string) ([]string, error) {
	if _, err := os.Stat(sessionsRoot); err != nil {
		return nil, err
	}
	paths := make([]string, 0, 128)
	err := filepath.WalkDir(sessionsRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".jsonl") && strings.HasPrefix(lower, "rollout-") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}
