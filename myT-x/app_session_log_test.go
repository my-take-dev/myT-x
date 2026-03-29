package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"myT-x/internal/sessionlog"
)

func TestInitSessionLog_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "test_config.yaml")),
	}

	stubRuntimeEventsEmit(t)

	a.initSessionLog(a.configState.ConfigPath())
	defer a.closeSessionLog()

	expectedDir := filepath.Join(tmpDir, "session-logs")
	info, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatalf("expected session-logs directory to exist at %s, got error: %v", expectedDir, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory, but it is not", expectedDir)
	}
}

func TestInitSessionLog_CreatesJSONLFile(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}

	stubRuntimeEventsEmit(t)

	a.initSessionLog(a.configState.ConfigPath())
	defer a.closeSessionLog()

	path := a.GetSessionLogFilePath()
	if path == "" {
		t.Fatal("expected sessionLogPath to be set, got empty string")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected session log file to exist at %s, got error: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("expected %s to be a file, but it is a directory", path)
	}

	// Verify filename matches pattern: session-YYYYMMDD-HHMMSS-PID.jsonl (A-2)
	filename := filepath.Base(path)
	if !strings.HasPrefix(filename, "session-") || !strings.HasSuffix(filename, ".jsonl") {
		t.Fatalf("expected filename matching session-*.jsonl pattern, got %s", filename)
	}
	pidPattern := regexp.MustCompile(`^session-\d{8}-\d{6}-\d+\.jsonl$`)
	if !pidPattern.MatchString(filename) {
		t.Fatalf("expected filename matching session-YYYYMMDD-HHMMSS-PID.jsonl pattern, got %s", filename)
	}
}

func TestWriteSessionLogEntry_WritesJSONL(t *testing.T) {
	tests := []struct {
		name  string
		entry SessionLogEntry
	}{
		{
			name: "writes entry as single-line JSON",
			entry: SessionLogEntry{
				Timestamp: "20060102150405",
				Level:     "error",
				Message:   "test error message",
				Source:    "test-source",
			},
		},
		{
			name: "writes entry with special characters",
			entry: SessionLogEntry{
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
			a := &App{
				configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog(a.configState.ConfigPath())
			defer a.closeSessionLog()

			a.writeSessionLogEntry(tt.entry)

			// Read the file and verify JSONL format
			content, err := os.ReadFile(a.GetSessionLogFilePath())
			if err != nil {
				t.Fatalf("failed to read log file: %v", err)
			}

			lines := strings.Split(strings.TrimSpace(string(content)), "\n")
			if len(lines) == 0 {
				t.Fatal("expected at least one line in the log file")
			}

			var parsedEntry SessionLogEntry
			if err := json.Unmarshal([]byte(lines[0]), &parsedEntry); err != nil {
				t.Fatalf("failed to unmarshal JSON from log file: %v", err)
			}

			if parsedEntry.Timestamp != tt.entry.Timestamp {
				t.Errorf("timestamp mismatch: got %s, want %s", parsedEntry.Timestamp, tt.entry.Timestamp)
			}
			if parsedEntry.Level != tt.entry.Level {
				t.Errorf("level mismatch: got %s, want %s", parsedEntry.Level, tt.entry.Level)
			}
			if parsedEntry.Message != tt.entry.Message {
				t.Errorf("message mismatch: got %s, want %s", parsedEntry.Message, tt.entry.Message)
			}
			if parsedEntry.Source != tt.entry.Source {
				t.Errorf("source mismatch: got %s, want %s", parsedEntry.Source, tt.entry.Source)
			}
			if parsedEntry.Seq == 0 {
				t.Error("expected seq > 0 in persisted JSONL entry, got 0")
			}
		})
	}
}

func TestWriteSessionLogEntry_AppendsToMemory(t *testing.T) {
	tests := []struct {
		name    string
		entries []SessionLogEntry
		count   int
	}{
		{
			name: "appends single entry to memory",
			entries: []SessionLogEntry{
				{
					Timestamp: "20060102150405",
					Level:     "error",
					Message:   "test error",
					Source:    "source1",
				},
			},
			count: 1,
		},
		{
			name: "appends multiple entries to memory",
			entries: []SessionLogEntry{
				{Timestamp: "20060102150405", Level: "error", Message: "error 1", Source: "source1"},
				{Timestamp: "20060102150406", Level: "warn", Message: "warn 1", Source: "source2"},
				{Timestamp: "20060102150407", Level: "error", Message: "error 2", Source: "source3"},
			},
			count: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog(a.configState.ConfigPath())
			defer a.closeSessionLog()

			for _, entry := range tt.entries {
				a.writeSessionLogEntry(entry)
			}

			result := a.GetSessionErrorLog()
			if len(result) != tt.count {
				t.Errorf("expected %d entries in memory, got %d", tt.count, len(result))
			}

			for i, expected := range tt.entries {
				if i >= len(result) {
					break
				}
				if result[i].Timestamp != expected.Timestamp {
					t.Errorf("entry %d timestamp mismatch: got %s, want %s", i, result[i].Timestamp, expected.Timestamp)
				}
				if result[i].Level != expected.Level {
					t.Errorf("entry %d level mismatch: got %s, want %s", i, result[i].Level, expected.Level)
				}
				if result[i].Message != expected.Message {
					t.Errorf("entry %d message mismatch: got %s, want %s", i, result[i].Message, expected.Message)
				}
				if result[i].Source != expected.Source {
					t.Errorf("entry %d source mismatch: got %s, want %s", i, result[i].Source, expected.Source)
				}
			}

			// Verify seq is monotonically increasing (A-seq).
			for i := 1; i < len(result); i++ {
				if result[i].Seq <= result[i-1].Seq {
					t.Errorf("entry %d seq (%d) is not strictly greater than entry %d seq (%d)",
						i, result[i].Seq, i-1, result[i-1].Seq)
				}
			}
		})
	}
}

func TestWriteSessionLogEntry_CapsInMemory(t *testing.T) {
	tests := []struct {
		name           string
		entriesToWrite int
		expectedCount  int
	}{
		{
			name:           "keeps only the newest entries when exceeding max",
			entriesToWrite: sessionlog.MaxEntries + 100,
			expectedCount:  sessionlog.MaxEntries,
		},
		{
			name:           "does not cap when below max",
			entriesToWrite: sessionlog.MaxEntries - 100,
			expectedCount:  sessionlog.MaxEntries - 100,
		},
		{
			name:           "handles exact max entries",
			entriesToWrite: sessionlog.MaxEntries,
			expectedCount:  sessionlog.MaxEntries,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog(a.configState.ConfigPath())
			defer a.closeSessionLog()

			for i := range tt.entriesToWrite {
				entry := SessionLogEntry{
					Timestamp: "20060102150405",
					Level:     "info",
					Message:   fmt.Sprintf("entry %d", i),
					Source:    "test",
				}
				a.writeSessionLogEntry(entry)
			}

			result := a.GetSessionErrorLog()
			if len(result) != tt.expectedCount {
				t.Errorf("expected %d entries in memory after capping, got %d", tt.expectedCount, len(result))
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

func TestGetSessionErrorLog_EmptyReturnsEmptySlice(t *testing.T) {
	tests := []struct {
		name        string
		writeEntry  bool
		expectCount int
	}{
		{
			name:        "returns empty slice when no entries written",
			writeEntry:  false,
			expectCount: 0,
		},
		{
			name:        "returns single entry after one write",
			writeEntry:  true,
			expectCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog(a.configState.ConfigPath())
			defer a.closeSessionLog()

			if tt.writeEntry {
				a.writeSessionLogEntry(SessionLogEntry{
					Timestamp: "20060102150405",
					Level:     "error",
					Message:   "test",
					Source:    "test",
				})
			}

			result := a.GetSessionErrorLog()
			if len(result) != tt.expectCount {
				t.Errorf("expected %d entries, got %d", tt.expectCount, len(result))
			}

			if result == nil {
				t.Error("expected non-nil empty slice for empty log, got nil")
			}
		})
	}
}

func TestGetSessionErrorLog_ReturnsIndependentCopy(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}

	stubRuntimeEventsEmit(t)

	a.initSessionLog(a.configState.ConfigPath())
	defer a.closeSessionLog()

	for i := range 5 {
		a.writeSessionLogEntry(SessionLogEntry{
			Timestamp: "20060102150405",
			Level:     "error",
			Message:   fmt.Sprintf("entry %d", i),
			Source:    "test",
		})
	}

	result1 := a.GetSessionErrorLog()
	result2 := a.GetSessionErrorLog()

	if len(result1) != len(result2) {
		t.Errorf("expected same length, got %d and %d", len(result1), len(result2))
	}

	if len(result1) > 0 {
		originalMessage := result1[0].Message
		result1[0].Message = "modified"
		if result2[0].Message == "modified" {
			t.Error("modifying one copy affected the other - copies are not independent")
		}
		result1[0].Message = originalMessage
	}
}

func TestCloseSessionLog_ClosesFile(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}

	stubRuntimeEventsEmit(t)

	a.initSessionLog(a.configState.ConfigPath())

	path := a.GetSessionLogFilePath()
	if path == "" {
		t.Fatal("expected path to be set after initSessionLog()")
	}

	a.closeSessionLog()

	// File should be closed (removable on Windows).
	if err := os.Remove(path); err != nil {
		t.Fatalf("expected file to be removable after close: %v", err)
	}
}

func TestGetSessionLogFilePath_ReturnsCorrectPath(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}

	stubRuntimeEventsEmit(t)

	a.initSessionLog(a.configState.ConfigPath())
	defer a.closeSessionLog()

	result := a.GetSessionLogFilePath()

	if result == "" {
		t.Error("expected non-empty path, got empty string")
	}

	if _, err := os.Stat(result); err != nil {
		t.Errorf("expected file to exist at %s, got error: %v", result, err)
	}
}

func TestWriteSessionLogEntry_WithoutInitializedFile(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}

	stubRuntimeEventsEmit(t)

	entry := SessionLogEntry{
		Timestamp: "20060102150405",
		Level:     "error",
		Message:   "test",
		Source:    "test",
	}

	// Should not panic even though file is nil.
	a.writeSessionLogEntry(entry)

	result := a.GetSessionErrorLog()
	if len(result) != 1 {
		t.Errorf("expected 1 entry in memory, got %d", len(result))
	}
	if result[0].Message != entry.Message {
		t.Errorf("entry message mismatch: got %s, want %s", result[0].Message, entry.Message)
	}
}

func TestWriteSessionLogEntry_SeqMonotonicallyIncreasing(t *testing.T) {
	tests := []struct {
		name       string
		entryCount int
	}{
		{
			name:       "seq starts at 1 and increments by 1",
			entryCount: 10,
		},
		{
			name:       "seq remains monotonic across many entries",
			entryCount: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &App{
				configState: newConfigStateForTest("config.yaml"),
			}

			stubRuntimeEventsEmit(t)

			for i := range tt.entryCount {
				a.writeSessionLogEntry(SessionLogEntry{
					Timestamp: "20060102150405",
					Level:     "warn",
					Message:   fmt.Sprintf("msg %d", i),
					Source:    "test",
				})
			}

			result := a.GetSessionErrorLog()
			if len(result) != tt.entryCount {
				t.Fatalf("expected %d entries, got %d", tt.entryCount, len(result))
			}

			if result[0].Seq != 1 {
				t.Errorf("expected first seq to be 1, got %d", result[0].Seq)
			}

			for i := 1; i < len(result); i++ {
				if result[i].Seq != result[i-1].Seq+1 {
					t.Errorf("entry %d: expected seq %d, got %d", i, result[i-1].Seq+1, result[i].Seq)
				}
			}
		})
	}
}

func TestWriteSessionLogEntry_EmitsCorrectEventName(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	// Set runtime context so emitRuntimeEvent does not drop the event.
	a.setRuntimeContext(context.Background())

	var capturedEventName string
	var capturedPayload any
	origEmitFn := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = origEmitFn })
	runtimeEventsEmitFn = func(_ context.Context, eventName string, optionalData ...any) {
		capturedEventName = eventName
		if len(optionalData) > 0 {
			capturedPayload = optionalData[0]
		}
	}

	a.writeSessionLogEntry(SessionLogEntry{
		Timestamp: "20060102150405",
		Level:     "error",
		Message:   "test error",
		Source:    "test",
	})

	if capturedEventName != "app:session-log-updated" {
		t.Errorf("expected event name %q, got %q", "app:session-log-updated", capturedEventName)
	}
	if capturedPayload != nil {
		t.Errorf("expected nil payload for ping event, got %v", capturedPayload)
	}
}

func TestInitSessionLog_FilenameContainsPID(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}

	stubRuntimeEventsEmit(t)

	a.initSessionLog(a.configState.ConfigPath())
	defer a.closeSessionLog()

	filename := filepath.Base(a.GetSessionLogFilePath())
	expectedPIDSuffix := fmt.Sprintf("-%d.jsonl", os.Getpid())
	if !strings.HasSuffix(filename, expectedPIDSuffix) {
		t.Errorf("expected filename to end with %q, got %q", expectedPIDSuffix, filename)
	}
}

func TestLogFrontendEvent_WritesToSessionLog(t *testing.T) {
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
			name:          "debug level normalized to debug",
			level:         "debug",
			msg:           "some debug info",
			source:        "frontend/ui",
			expectedLevel: "debug",
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
			tmpDir := t.TempDir()
			a := &App{
				configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
			}
			stubRuntimeEventsEmit(t)
			a.initSessionLog(a.configState.ConfigPath())
			defer a.closeSessionLog()

			a.LogFrontendEvent(tt.level, tt.msg, tt.source)

			result := a.GetSessionErrorLog()
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
			name:         "msg at exact limit preserved",
			msgRunes:     sessionlog.FrontendLogMaxMsgLen,
			sourceRunes:  10,
			expectMsgLen: sessionlog.FrontendLogMaxMsgLen,
			expectSrcLen: 10,
		},
		{
			name:         "msg over limit truncated",
			msgRunes:     sessionlog.FrontendLogMaxMsgLen + 100,
			sourceRunes:  10,
			expectMsgLen: sessionlog.FrontendLogMaxMsgLen,
			expectSrcLen: 10,
		},
		{
			name:         "source over limit truncated",
			msgRunes:     10,
			sourceRunes:  sessionlog.FrontendLogMaxSourceLen + 50,
			expectMsgLen: 10,
			expectSrcLen: sessionlog.FrontendLogMaxSourceLen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
			}
			stubRuntimeEventsEmit(t)
			a.initSessionLog(a.configState.ConfigPath())
			defer a.closeSessionLog()

			msg := strings.Repeat("日", tt.msgRunes) // multibyte rune to verify rune-safe truncation
			source := strings.Repeat("x", tt.sourceRunes)

			a.LogFrontendEvent("error", msg, source)

			result := a.GetSessionErrorLog()
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
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)
	a.initSessionLog(a.configState.ConfigPath())
	defer a.closeSessionLog()

	msg := strings.Repeat("日", sessionlog.FrontendLogMaxMsgLen+1)
	a.LogFrontendEvent("error", msg, "src")

	result := a.GetSessionErrorLog()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	runeCount := len([]rune(result[0].Message))
	if runeCount != sessionlog.FrontendLogMaxMsgLen {
		t.Errorf("expected %d runes after truncation, got %d", sessionlog.FrontendLogMaxMsgLen, runeCount)
	}
	if !utf8.ValidString(result[0].Message) {
		t.Error("truncated message is not valid UTF-8")
	}
}
