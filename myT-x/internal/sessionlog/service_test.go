package sessionlog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// --------------------------------------------------------------------
// Mock emitter for tests
// --------------------------------------------------------------------

type mockEmitter struct {
	mu       sync.Mutex
	events   []mockEvent
	disabled bool
}

type mockEvent struct {
	name    string
	payload any
}

func (m *mockEmitter) Emit(name string, payload any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.disabled {
		m.events = append(m.events, mockEvent{name: name, payload: payload})
	}
}

func (m *mockEmitter) EmitWithContext(_ context.Context, name string, payload any) {
	m.Emit(name, payload)
}

func (m *mockEmitter) getEvents() []mockEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]mockEvent, len(m.events))
	copy(copied, m.events)
	return copied
}

// formatIndexAsTimestamp formats an index as a timestamp for testing purposes.
// Uses time.Date for correct month/year boundary handling.
func formatIndexAsTimestamp(index int) string {
	base := time.Date(2006, 1, 1, 0, 0, 0, 0, time.UTC)
	ts := base.Add(time.Duration(index) * time.Second)
	return ts.Format("20060102-150405")
}

// --------------------------------------------------------------------
// Ring buffer tests
// --------------------------------------------------------------------

func TestRingBuffer_ClampsNonPositiveCapacity(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
	}{
		{name: "zero capacity clamped to 1", capacity: 0},
		{name: "negative capacity clamped to 1", capacity: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := newRingBuffer(tt.capacity)
			rb.push(Entry{Seq: 1, Message: "first"})

			entries := rb.snapshot()
			if len(entries) != 1 {
				t.Fatalf("snapshot length = %d, want 1", len(entries))
			}
			if entries[0].Seq != 1 {
				t.Fatalf("snapshot[0].Seq = %d, want 1", entries[0].Seq)
			}
		})
	}
}

func TestRingBuffer_PushAndSnapshot(t *testing.T) {
	tests := []struct {
		name      string
		capacity  int
		pushCount int
		wantCount int
		wantFirst string
		wantLast  string
	}{
		{
			name:      "below capacity",
			capacity:  5,
			pushCount: 3,
			wantCount: 3,
			wantFirst: "msg-0",
			wantLast:  "msg-2",
		},
		{
			name:      "at capacity",
			capacity:  5,
			pushCount: 5,
			wantCount: 5,
			wantFirst: "msg-0",
			wantLast:  "msg-4",
		},
		{
			name:      "overflow wraps around",
			capacity:  5,
			pushCount: 8,
			wantCount: 5,
			wantFirst: "msg-3",
			wantLast:  "msg-7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := newRingBuffer(tt.capacity)
			for i := range tt.pushCount {
				rb.push(Entry{Message: fmt.Sprintf("msg-%d", i)})
			}

			entries := rb.snapshot()
			if len(entries) != tt.wantCount {
				t.Fatalf("snapshot length = %d, want %d", len(entries), tt.wantCount)
			}
			if entries[0].Message != tt.wantFirst {
				t.Errorf("first entry = %q, want %q", entries[0].Message, tt.wantFirst)
			}
			if entries[len(entries)-1].Message != tt.wantLast {
				t.Errorf("last entry = %q, want %q", entries[len(entries)-1].Message, tt.wantLast)
			}
		})
	}
}

func TestRingBuffer_EmptySnapshot(t *testing.T) {
	rb := newRingBuffer(10)
	entries := rb.snapshot()
	if entries == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(entries))
	}
}

// --------------------------------------------------------------------
// Init tests
// --------------------------------------------------------------------

func TestInit_CreatesDirectoryAndFile(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(nil, nil)

	svc.Init(filepath.Join(tmpDir, "config.yaml"))
	defer svc.Close()

	expectedDir := filepath.Join(tmpDir, Dir)
	info, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatalf("expected directory at %s: %v", expectedDir, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", expectedDir)
	}

	path := svc.FilePath()
	if path == "" {
		t.Fatal("expected file path to be set")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected log file at %s: %v", path, err)
	}

	filename := filepath.Base(path)
	if !strings.HasPrefix(filename, "session-") || !strings.HasSuffix(filename, ".jsonl") {
		t.Fatalf("expected session-*.jsonl filename, got %s", filename)
	}

	expectedPIDSuffix := fmt.Sprintf("-%d.jsonl", os.Getpid())
	if !strings.HasSuffix(filename, expectedPIDSuffix) {
		t.Errorf("expected filename ending with %q, got %q", expectedPIDSuffix, filename)
	}
}

// --------------------------------------------------------------------
// WriteEntry tests
// --------------------------------------------------------------------

func TestWriteEntry_WritesJSONL(t *testing.T) {
	tests := []struct {
		name  string
		entry Entry
	}{
		{
			name: "basic entry",
			entry: Entry{
				Timestamp: "20060102150405",
				Level:     "error",
				Message:   "test error message",
				Source:    "test-source",
			},
		},
		{
			name: "entry with special characters",
			entry: Entry{
				Timestamp: "20060102150405",
				Level:     "warn",
				Message:   `message with "quotes" and \backslash`,
				Source:    "special-source",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			svc := NewService(nil, nil)
			svc.Init(filepath.Join(tmpDir, "config.yaml"))
			defer svc.Close()

			svc.WriteEntry(tt.entry)

			content, err := os.ReadFile(svc.FilePath())
			if err != nil {
				t.Fatalf("failed to read log file: %v", err)
			}

			lines := strings.Split(strings.TrimSpace(string(content)), "\n")
			if len(lines) == 0 {
				t.Fatal("expected at least one line in the log file")
			}

			var parsed Entry
			if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
				t.Fatalf("failed to unmarshal JSONL: %v", err)
			}

			if parsed.Timestamp != tt.entry.Timestamp {
				t.Errorf("timestamp = %q, want %q", parsed.Timestamp, tt.entry.Timestamp)
			}
			if parsed.Level != tt.entry.Level {
				t.Errorf("level = %q, want %q", parsed.Level, tt.entry.Level)
			}
			if parsed.Message != tt.entry.Message {
				t.Errorf("message = %q, want %q", parsed.Message, tt.entry.Message)
			}
			if parsed.Source != tt.entry.Source {
				t.Errorf("source = %q, want %q", parsed.Source, tt.entry.Source)
			}
			if parsed.Seq == 0 {
				t.Error("expected seq > 0")
			}
		})
	}
}

func TestWriteEntry_AppendsToMemory(t *testing.T) {
	svc := NewService(nil, nil)

	entries := []Entry{
		{Timestamp: "20060102150405", Level: "error", Message: "error 1", Source: "source1"},
		{Timestamp: "20060102150406", Level: "warn", Message: "warn 1", Source: "source2"},
		{Timestamp: "20060102150407", Level: "error", Message: "error 2", Source: "source3"},
	}
	for _, e := range entries {
		svc.WriteEntry(e)
	}

	result := svc.Snapshot()
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}

	for i, expected := range entries {
		if result[i].Message != expected.Message {
			t.Errorf("entry %d message = %q, want %q", i, result[i].Message, expected.Message)
		}
	}

	// Verify seq is monotonically increasing.
	for i := 1; i < len(result); i++ {
		if result[i].Seq <= result[i-1].Seq {
			t.Errorf("entry %d seq (%d) not > entry %d seq (%d)", i, result[i].Seq, i-1, result[i-1].Seq)
		}
	}
}

func TestWriteEntry_SeqMonotonicallyIncreasing(t *testing.T) {
	svc := NewService(nil, nil)

	for i := range 100 {
		svc.WriteEntry(Entry{
			Timestamp: "20060102150405",
			Level:     "warn",
			Message:   fmt.Sprintf("msg %d", i),
			Source:    "test",
		})
	}

	result := svc.Snapshot()
	if len(result) != 100 {
		t.Fatalf("expected 100 entries, got %d", len(result))
	}
	if result[0].Seq != 1 {
		t.Errorf("first seq = %d, want 1", result[0].Seq)
	}
	for i := 1; i < len(result); i++ {
		if result[i].Seq != result[i-1].Seq+1 {
			t.Errorf("entry %d: seq = %d, want %d", i, result[i].Seq, result[i-1].Seq+1)
		}
	}
}

func TestWriteEntry_CapsInMemory(t *testing.T) {
	tests := []struct {
		name           string
		entriesToWrite int
		expectedCount  int
	}{
		{
			name:           "keeps only newest when exceeding max",
			entriesToWrite: MaxEntries + 100,
			expectedCount:  MaxEntries,
		},
		{
			name:           "does not cap when below max",
			entriesToWrite: MaxEntries - 100,
			expectedCount:  MaxEntries - 100,
		},
		{
			name:           "handles exact max entries",
			entriesToWrite: MaxEntries,
			expectedCount:  MaxEntries,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(nil, nil)

			for i := range tt.entriesToWrite {
				svc.WriteEntry(Entry{
					Timestamp: "20060102150405",
					Level:     "info",
					Message:   fmt.Sprintf("entry %d", i),
					Source:    "test",
				})
			}

			result := svc.Snapshot()
			if len(result) != tt.expectedCount {
				t.Errorf("expected %d entries, got %d", tt.expectedCount, len(result))
			}

			if len(result) > 0 {
				lastMessage := result[len(result)-1].Message
				if !strings.HasPrefix(lastMessage, "entry") {
					t.Errorf("expected last message to start with 'entry', got %s", lastMessage)
				}
			}
		})
	}
}

func TestWriteEntry_WithoutInitializedFile(t *testing.T) {
	svc := NewService(nil, nil)

	entry := Entry{
		Timestamp: "20060102150405",
		Level:     "error",
		Message:   "test",
		Source:    "test",
	}

	// Should not panic even though file is nil.
	svc.WriteEntry(entry)

	result := svc.Snapshot()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Message != entry.Message {
		t.Errorf("message = %q, want %q", result[0].Message, entry.Message)
	}
}

func TestWriteEntry_EmitsCorrectEvent(t *testing.T) {
	emitter := &mockEmitter{}
	svc := NewService(emitter, nil)

	svc.WriteEntry(Entry{
		Timestamp: "20060102150405",
		Level:     "error",
		Message:   "test error",
		Source:    "test",
	})

	events := emitter.getEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].name != "app:session-log-updated" {
		t.Errorf("event name = %q, want %q", events[0].name, "app:session-log-updated")
	}
	if events[0].payload != nil {
		t.Errorf("expected nil payload, got %v", events[0].payload)
	}
}

// --------------------------------------------------------------------
// Close tests
// --------------------------------------------------------------------

func TestClose_ClosesFile(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(nil, nil)
	svc.Init(filepath.Join(tmpDir, "config.yaml"))

	path := svc.FilePath()
	if path == "" {
		t.Fatal("expected path to be set after Init")
	}

	svc.Close()

	// File should be closed (removable on Windows).
	if err := os.Remove(path); err != nil {
		t.Fatalf("expected file to be removable after close: %v", err)
	}
}

// --------------------------------------------------------------------
// Snapshot tests
// --------------------------------------------------------------------

func TestSnapshot_ReturnsIndependentCopy(t *testing.T) {
	svc := NewService(nil, nil)

	for i := range 5 {
		svc.WriteEntry(Entry{Message: fmt.Sprintf("entry %d", i)})
	}

	r1 := svc.Snapshot()
	r2 := svc.Snapshot()

	if len(r1) != len(r2) {
		t.Fatalf("expected same length, got %d and %d", len(r1), len(r2))
	}

	if len(r1) > 0 {
		r1[0].Message = "modified"
		if r2[0].Message == "modified" {
			t.Error("copies are not independent")
		}
	}
}

func TestSnapshot_EmptyReturnsEmptySlice(t *testing.T) {
	svc := NewService(nil, nil)
	result := svc.Snapshot()
	if result == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(result))
	}
}

// --------------------------------------------------------------------
// Cleanup tests
// --------------------------------------------------------------------

func TestCleanupOldFiles_KeepsMaxFiles(t *testing.T) {
	tests := []struct {
		name               string
		preExistingFiles   int
		expectedFilesAfter int
	}{
		{
			name:               "deletes oldest files when total exceeds max",
			preExistingFiles:   110,
			expectedFilesAfter: MaxFiles,
		},
		{
			name:               "keeps all files when total is below max",
			preExistingFiles:   50,
			expectedFilesAfter: 51, // 50 pre-existing + 1 from Init
		},
		{
			name:               "keeps max when pre-existing equals max minus one",
			preExistingFiles:   MaxFiles - 1,
			expectedFilesAfter: MaxFiles, // 99 + 1 from Init = 100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logDir := filepath.Join(tmpDir, Dir)
			if err := os.MkdirAll(logDir, 0o700); err != nil {
				t.Fatalf("failed to create log directory: %v", err)
			}

			for i := range tt.preExistingFiles {
				name := fmt.Sprintf("session-%s-%04d.jsonl", formatIndexAsTimestamp(i), i)
				if err := os.WriteFile(filepath.Join(logDir, name), []byte("x\n"), 0o600); err != nil {
					t.Fatalf("failed to create dummy log file: %v", err)
				}
			}

			svc := NewService(nil, nil)
			svc.Init(filepath.Join(tmpDir, "config.yaml"))
			defer svc.Close()

			entries, err := os.ReadDir(logDir)
			if err != nil {
				t.Fatalf("failed to read log directory: %v", err)
			}

			var count int
			for _, e := range entries {
				if !e.IsDir() && strings.HasPrefix(e.Name(), "session-") && strings.HasSuffix(e.Name(), ".jsonl") {
					count++
				}
			}
			if count != tt.expectedFilesAfter {
				t.Errorf("expected %d files after cleanup, got %d", tt.expectedFilesAfter, count)
			}
		})
	}
}

func TestCleanupOldFiles_PreservesCurrentFile(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, Dir)
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatalf("failed to create log directory: %v", err)
	}

	for i := range MaxFiles + 20 {
		name := fmt.Sprintf("session-%s-%04d.jsonl", formatIndexAsTimestamp(i), i)
		if err := os.WriteFile(filepath.Join(logDir, name), []byte("old\n"), 0o600); err != nil {
			t.Fatalf("failed to create old log file: %v", err)
		}
	}

	svc := NewService(nil, nil)
	svc.Init(filepath.Join(tmpDir, "config.yaml"))
	defer svc.Close()

	currentPath := svc.FilePath()
	if _, err := os.Stat(currentPath); err != nil {
		t.Fatalf("current log file should not be deleted: %v", err)
	}
}

func TestCleanupOldFiles_SameTimestampOrdersByNumericPID(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, Dir)
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatalf("failed to create log directory: %v", err)
	}

	pid9Name := "session-20260101-000000-9.jsonl"
	pid10Name := "session-20260101-000000-10.jsonl"
	for _, name := range []string{pid9Name, pid10Name} {
		if err := os.WriteFile(filepath.Join(logDir, name), []byte("x\n"), 0o600); err != nil {
			t.Fatalf("failed to create seed file %q: %v", name, err)
		}
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 97; i++ {
		ts := base.Add(time.Duration(i) * time.Second).Format("20060102-150405")
		name := fmt.Sprintf("session-%s-%d.jsonl", ts, 3000+i)
		if err := os.WriteFile(filepath.Join(logDir, name), []byte("x\n"), 0o600); err != nil {
			t.Fatalf("failed to create log file %q: %v", name, err)
		}
	}
	// Total: 2 + 97 = 99 files. Init adds 1 → 100 = MaxFiles. No cleanup.
	// Add 1 more to force cleanup of exactly 1 file.
	extra := fmt.Sprintf("session-%s-%d.jsonl", base.Add(98*time.Second).Format("20060102-150405"), 3098)
	if err := os.WriteFile(filepath.Join(logDir, extra), []byte("x\n"), 0o600); err != nil {
		t.Fatalf("failed to create extra log file: %v", err)
	}
	// Total: 100 files. Init adds 1 → 101. Excess = 1. Should delete pid9.

	svc := NewService(nil, nil)
	svc.Init(filepath.Join(tmpDir, "config.yaml"))
	defer svc.Close()

	if _, err := os.Stat(filepath.Join(logDir, pid9Name)); err == nil {
		t.Fatalf("expected %s to be deleted as oldest numeric PID", pid9Name)
	}
	if _, err := os.Stat(filepath.Join(logDir, pid10Name)); err != nil {
		t.Fatalf("expected %s to remain, got err: %v", pid10Name, err)
	}
}

func TestCleanupOldFiles_DeletesMalformedBeforeCanonical(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, Dir)
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatalf("failed to create log directory: %v", err)
	}

	malformedName := "session-malformed.jsonl"
	if err := os.WriteFile(filepath.Join(logDir, malformedName), []byte("bad\n"), 0o600); err != nil {
		t.Fatalf("failed to create malformed log file: %v", err)
	}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var oldestCanonical string
	for i := range MaxFiles - 1 {
		ts := base.Add(time.Duration(i) * time.Second).Format("20060102-150405")
		name := fmt.Sprintf("session-%s-%d.jsonl", ts, 4000+i)
		fullPath := filepath.Join(logDir, name)
		if err := os.WriteFile(fullPath, []byte("ok\n"), 0o600); err != nil {
			t.Fatalf("failed to create canonical log file %q: %v", name, err)
		}
		if i == 0 {
			oldestCanonical = fullPath
		}
	}
	// Total: 1 malformed + 99 canonical = 100. Init adds 1 → 101. Excess = 1.
	// Malformed sorts before canonical → malformed is deleted first.

	svc := NewService(nil, nil)
	svc.Init(filepath.Join(tmpDir, "config.yaml"))
	defer svc.Close()

	if _, err := os.Stat(filepath.Join(logDir, malformedName)); err == nil {
		t.Fatalf("expected malformed file %q to be deleted first", malformedName)
	}
	if _, err := os.Stat(oldestCanonical); err != nil {
		t.Fatalf("expected oldest canonical file to remain, got err: %v", err)
	}
	if _, err := os.Stat(svc.FilePath()); err != nil {
		t.Fatalf("expected current log file to remain, got err: %v", err)
	}
}

// --------------------------------------------------------------------
// NormalizeLogLevel tests
// --------------------------------------------------------------------

func TestNormalizeLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "error stays error", input: "error", expected: "error"},
		{name: "ERROR uppercase normalized", input: "ERROR", expected: "error"},
		{name: "warn normalized", input: "warn", expected: "warn"},
		{name: "warning alias normalized", input: "warning", expected: "warn"},
		{name: "WARN uppercase normalized", input: "WARN", expected: "warn"},
		{name: "WARNING uppercase alias normalized", input: "WARNING", expected: "warn"},
		{name: "info stays info", input: "info", expected: "info"},
		{name: "debug normalized to debug", input: "debug", expected: "debug"},
		{name: "DEBUG uppercase normalized", input: "DEBUG", expected: "debug"},
		{name: "unknown maps to info", input: "trace", expected: "info"},
		{name: "empty maps to info", input: "", expected: "info"},
		{name: "whitespace-only maps to info", input: "   ", expected: "info"},
		{name: "level with leading/trailing spaces", input: "  error  ", expected: "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeLogLevel(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeLogLevel(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --------------------------------------------------------------------
// TruncateRunes tests
// --------------------------------------------------------------------

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxRunes int
		expected string
	}{
		{name: "within limit", input: "hello", maxRunes: 10, expected: "hello"},
		{name: "at limit", input: "hello", maxRunes: 5, expected: "hello"},
		{name: "over limit", input: "hello world", maxRunes: 5, expected: "hello"},
		{name: "empty string", input: "", maxRunes: 10, expected: ""},
		{name: "zero max", input: "hello", maxRunes: 0, expected: ""},
		{name: "negative max", input: "hello", maxRunes: -1, expected: ""},
		{name: "multibyte within limit", input: "日本語", maxRunes: 5, expected: "日本語"},
		{name: "multibyte at limit", input: "日本語", maxRunes: 3, expected: "日本語"},
		{name: "multibyte over limit", input: "日本語テスト", maxRunes: 3, expected: "日本語"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateRunes(tt.input, tt.maxRunes)
			if got != tt.expected {
				t.Errorf("TruncateRunes(%q, %d) = %q, want %q", tt.input, tt.maxRunes, got, tt.expected)
			}
		})
	}
}

// --------------------------------------------------------------------
// LogFrontendEvent tests
// --------------------------------------------------------------------

func TestLogFrontendEvent_WritesToLog(t *testing.T) {
	tests := []struct {
		name          string
		level         string
		msg           string
		source        string
		expectedLevel string
		expectEntry   bool
	}{
		{
			name:          "error event written",
			level:         "error",
			msg:           "uncaught exception",
			source:        "frontend/unhandled",
			expectedLevel: "error",
			expectEntry:   true,
		},
		{
			name:          "warn event written",
			level:         "warn",
			msg:           "API call failed",
			source:        "frontend/api",
			expectedLevel: "warn",
			expectEntry:   true,
		},
		{
			name:          "unknown level normalized to info",
			level:         "trace",
			msg:           "some trace info",
			source:        "frontend/ui",
			expectedLevel: "info",
			expectEntry:   true,
		},
		{
			name:        "empty msg silently discarded",
			level:       "error",
			msg:         "",
			source:      "frontend/ui",
			expectEntry: false,
		},
		{
			name:        "whitespace-only msg silently discarded",
			level:       "error",
			msg:         "   ",
			source:      "frontend/ui",
			expectEntry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(nil, nil)
			svc.LogFrontendEvent(tt.level, tt.msg, tt.source)

			result := svc.Snapshot()
			if !tt.expectEntry {
				if len(result) != 0 {
					t.Errorf("expected no entries, got %d", len(result))
				}
				return
			}
			if len(result) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(result))
			}
			if result[0].Level != tt.expectedLevel {
				t.Errorf("level = %q, want %q", result[0].Level, tt.expectedLevel)
			}
			if result[0].Source != strings.TrimSpace(tt.source) {
				t.Errorf("source = %q, want %q", result[0].Source, strings.TrimSpace(tt.source))
			}
		})
	}
}

func TestLogFrontendEvent_TruncatesLongInputs(t *testing.T) {
	tests := []struct {
		name         string
		msgRunes     int
		sourceRunes  int
		expectMsgLen int
		expectSrcLen int
	}{
		{
			name:         "msg within limit preserved",
			msgRunes:     100,
			sourceRunes:  50,
			expectMsgLen: 100,
			expectSrcLen: 50,
		},
		{
			name:         "msg over limit truncated",
			msgRunes:     FrontendLogMaxMsgLen + 100,
			sourceRunes:  10,
			expectMsgLen: FrontendLogMaxMsgLen,
			expectSrcLen: 10,
		},
		{
			name:         "source over limit truncated",
			msgRunes:     10,
			sourceRunes:  FrontendLogMaxSourceLen + 50,
			expectMsgLen: 10,
			expectSrcLen: FrontendLogMaxSourceLen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(nil, nil)

			msg := strings.Repeat("日", tt.msgRunes)
			source := strings.Repeat("x", tt.sourceRunes)
			svc.LogFrontendEvent("error", msg, source)

			result := svc.Snapshot()
			if len(result) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(result))
			}
			gotMsgRunes := len([]rune(result[0].Message))
			gotSrcRunes := len([]rune(result[0].Source))
			if gotMsgRunes != tt.expectMsgLen {
				t.Errorf("msg rune len = %d, want %d", gotMsgRunes, tt.expectMsgLen)
			}
			if gotSrcRunes != tt.expectSrcLen {
				t.Errorf("source rune len = %d, want %d", gotSrcRunes, tt.expectSrcLen)
			}
		})
	}
}

func TestLogFrontendEvent_MultibyteSafeRune(t *testing.T) {
	svc := NewService(nil, nil)

	msg := strings.Repeat("日", FrontendLogMaxMsgLen+1)
	svc.LogFrontendEvent("error", msg, "src")

	result := svc.Snapshot()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	runeCount := len([]rune(result[0].Message))
	if runeCount != FrontendLogMaxMsgLen {
		t.Errorf("expected %d runes after truncation, got %d", FrontendLogMaxMsgLen, runeCount)
	}
	if !utf8.ValidString(result[0].Message) {
		t.Error("truncated message is not valid UTF-8")
	}
}

// --------------------------------------------------------------------
// parseFileSortKey tests
// --------------------------------------------------------------------

func TestParseFileSortKey(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantTS   string
		wantPID  int
		wantOK   bool
	}{
		{name: "valid canonical", filename: "session-20260101-000000-1234.jsonl", wantTS: "20260101-000000", wantPID: 1234, wantOK: true},
		{name: "non-session prefix", filename: "input-20260101-000000-1234.jsonl", wantOK: false},
		{name: "non-jsonl suffix", filename: "session-20260101-000000-1234.txt", wantOK: false},
		{name: "malformed", filename: "session-malformed.jsonl", wantOK: false},
		{name: "missing PID", filename: "session-20260101-000000.jsonl", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, pid, ok := parseFileSortKey(tt.filename)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok {
				if ts != tt.wantTS {
					t.Errorf("timestamp = %q, want %q", ts, tt.wantTS)
				}
				if pid != tt.wantPID {
					t.Errorf("pid = %d, want %d", pid, tt.wantPID)
				}
			}
		})
	}
}

// --------------------------------------------------------------------
// Concurrent safety tests
// --------------------------------------------------------------------

func TestWriteEntry_ConcurrentSafety(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(nil, nil)
	svc.Init(filepath.Join(tmpDir, "config.yaml"))
	defer svc.Close()

	var wg sync.WaitGroup
	for i := range 10 {
		idx := i
		wg.Go(func() {
			for j := range 50 {
				svc.WriteEntry(Entry{
					Timestamp: "20060102150405",
					Level:     "info",
					Message:   fmt.Sprintf("goroutine-%d-entry-%d", idx, j),
					Source:    "test",
				})
			}
		})
	}
	wg.Wait()

	result := svc.Snapshot()
	if len(result) != 500 {
		t.Errorf("expected 500 entries, got %d", len(result))
	}

	// Verify seq is strictly monotonically increasing.
	for i := 1; i < len(result); i++ {
		if result[i].Seq <= result[i-1].Seq {
			t.Errorf("entry %d seq (%d) not > entry %d seq (%d)", i, result[i].Seq, i-1, result[i-1].Seq)
			break
		}
	}
}
