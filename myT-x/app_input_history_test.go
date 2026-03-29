package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// --------------------------------------------------------------------
// initInputHistory / cleanupOldInputHistory / closeInputHistory tests
// --------------------------------------------------------------------

func TestInitInputHistory_CreatesDirectoryAndFile(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)

	a.initInputHistory(a.configState.ConfigPath())
	defer a.closeInputHistory()

	// Verify directory exists.
	expectedDir := filepath.Join(tmpDir, inputHistoryDir)
	info, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatalf("expected directory at %s: %v", expectedDir, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", expectedDir)
	}

	// Verify file exists with PID in filename.
	historyPath := a.GetInputHistoryFilePath()
	if historyPath == "" {
		t.Fatal("expected inputHistoryPath to be set")
	}
	if _, err := os.Stat(historyPath); err != nil {
		t.Fatalf("expected history file at %s: %v", historyPath, err)
	}
	filename := filepath.Base(historyPath)
	pidPattern := regexp.MustCompile(`^input-\d{8}-\d{6}-\d+\.jsonl$`)
	if !pidPattern.MatchString(filename) {
		t.Fatalf("expected filename matching input-YYYYMMDD-HHMMSS-PID.jsonl, got %s", filename)
	}
	expectedPIDSuffix := fmt.Sprintf("-%d.jsonl", os.Getpid())
	if !strings.HasSuffix(filename, expectedPIDSuffix) {
		t.Errorf("expected filename to end with %q, got %q", expectedPIDSuffix, filename)
	}
}

func TestCleanupOldInputHistory_KeepsMaxFiles(t *testing.T) {
	tests := []struct {
		name          string
		filesToCreate int
	}{
		{
			name:          "deletes oldest when exceeding max",
			filesToCreate: inputHistoryMaxFiles + 20,
		},
		{
			name:          "keeps all when below max",
			filesToCreate: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			histDir := filepath.Join(tmpDir, inputHistoryDir)
			if err := os.MkdirAll(histDir, 0o700); err != nil {
				t.Fatalf("failed to create directory: %v", err)
			}

			for i := 0; i < tt.filesToCreate; i++ {
				base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
				ts := base.Add(time.Duration(i) * time.Second)
				name := fmt.Sprintf("input-%s-%04d.jsonl", ts.Format("20060102-150405"), i)
				if err := os.WriteFile(filepath.Join(histDir, name), []byte("x\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			a := &App{configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml"))}
			stubRuntimeEventsEmit(t)
			a.initInputHistory(a.configState.ConfigPath())
			defer a.closeInputHistory()

			a.cleanupOldInputHistory()

			entries, _ := os.ReadDir(histDir)
			var count int
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), "input-") && strings.HasSuffix(e.Name(), ".jsonl") {
					count++
				}
			}
			expectedAfter := min(
				// created files + current file
				tt.filesToCreate+1, inputHistoryMaxFiles)
			if count != expectedAfter {
				t.Errorf("expected %d files after cleanup, got %d", expectedAfter, count)
			}
		})
	}
}

func TestCleanupOldInputHistory_SameTimestampOrdersByNumericPID(t *testing.T) {
	tmpDir := t.TempDir()
	histDir := filepath.Join(tmpDir, inputHistoryDir)
	if err := os.MkdirAll(histDir, 0o700); err != nil {
		t.Fatalf("failed to create history directory: %v", err)
	}

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	pid9Name := "input-20260101-000000-9.jsonl"
	pid10Name := "input-20260101-000000-10.jsonl"
	for _, name := range []string{pid9Name, pid10Name} {
		if err := os.WriteFile(filepath.Join(histDir, name), []byte("x\n"), 0o600); err != nil {
			t.Fatalf("failed to create seed file %q: %v", name, err)
		}
	}
	for i := 1; i <= 48; i++ {
		ts := base.Add(time.Duration(i) * time.Second).Format("20060102-150405")
		name := fmt.Sprintf("input-%s-%d.jsonl", ts, 2000+i)
		if err := os.WriteFile(filepath.Join(histDir, name), []byte("x\n"), 0o600); err != nil {
			t.Fatalf("failed to create history file %q: %v", name, err)
		}
	}

	a := &App{configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml"))}
	stubRuntimeEventsEmit(t)
	a.initInputHistory(a.configState.ConfigPath())
	defer a.closeInputHistory()

	a.cleanupOldInputHistory()

	if _, err := os.Stat(filepath.Join(histDir, pid9Name)); !os.IsNotExist(err) {
		t.Fatalf("expected numeric-older PID file %q to be removed, err=%v", pid9Name, err)
	}
	if _, err := os.Stat(filepath.Join(histDir, pid10Name)); err != nil {
		t.Fatalf("expected newer same-second PID file %q to remain: %v", pid10Name, err)
	}
}

func TestCloseInputHistory_SetsFileToNil(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)

	a.initInputHistory(a.configState.ConfigPath())
	path := a.GetInputHistoryFilePath()
	if path == "" {
		t.Fatal("expected history path to be set after init")
	}

	a.closeInputHistory()

	if err := os.Remove(path); err != nil {
		t.Fatalf("expected file to be removable after close, got %v", err)
	}
}

// --------------------------------------------------------------------
// writeInputHistoryEntry tests
// --------------------------------------------------------------------

func TestWriteInputHistoryEntry_WritesJSONL(t *testing.T) {
	tests := []struct {
		name  string
		entry InputHistoryEntry
	}{
		{
			name:  "basic entry",
			entry: InputHistoryEntry{Timestamp: "20260223120000", PaneID: "%5", Input: "ls -la", Source: "keyboard"},
		},
		{
			name:  "entry with special characters",
			entry: InputHistoryEntry{Timestamp: "20260223120001", PaneID: "%5", Input: "echo \"hello world\"", Source: "keyboard"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
			}
			stubRuntimeEventsEmit(t)
			a.initInputHistory(a.configState.ConfigPath())
			defer a.closeInputHistory()

			a.writeInputHistoryEntry(tt.entry)

			content, err := os.ReadFile(a.GetInputHistoryFilePath())
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}

			lines := strings.Split(strings.TrimSpace(string(content)), "\n")
			if len(lines) != 1 {
				t.Fatalf("expected 1 line, got %d", len(lines))
			}

			var parsed InputHistoryEntry
			if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			if parsed.Seq == 0 {
				t.Error("expected seq > 0")
			}
			if parsed.PaneID != tt.entry.PaneID {
				t.Errorf("pane_id = %q, want %q", parsed.PaneID, tt.entry.PaneID)
			}
			if parsed.Input != tt.entry.Input {
				t.Errorf("input = %q, want %q", parsed.Input, tt.entry.Input)
			}
			if parsed.Source != tt.entry.Source {
				t.Errorf("source = %q, want %q", parsed.Source, tt.entry.Source)
			}
		})
	}
}

func TestWriteInputHistoryEntry_SeqMonotonicallyIncreasing(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	for i := range 20 {
		a.writeInputHistoryEntry(InputHistoryEntry{
			Timestamp: "20260223120000",
			PaneID:    "%1",
			Input:     fmt.Sprintf("cmd-%d", i),
			Source:    "keyboard",
		})
	}

	result := a.GetInputHistory()
	if len(result) != 20 {
		t.Fatalf("expected 20 entries, got %d", len(result))
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

func TestWriteInputHistoryEntry_WithoutInitializedFile(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	// Should not panic even though file is nil.
	a.writeInputHistoryEntry(InputHistoryEntry{
		Timestamp: "20260223120000",
		PaneID:    "%1",
		Input:     "test",
		Source:    "keyboard",
	})

	result := a.GetInputHistory()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry in memory, got %d", len(result))
	}
	if result[0].Input != "test" {
		t.Errorf("input = %q, want %q", result[0].Input, "test")
	}
}

func TestWriteInputHistoryEntry_EmitsCorrectEvent(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	a.setRuntimeContext(context.Background())

	var capturedName string
	var capturedPayload any
	orig := runtimeEventsEmitFn
	t.Cleanup(func() { runtimeEventsEmitFn = orig })
	runtimeEventsEmitFn = func(_ context.Context, name string, data ...any) {
		capturedName = name
		if len(data) > 0 {
			capturedPayload = data[0]
		}
	}

	a.writeInputHistoryEntry(InputHistoryEntry{
		Timestamp: "20260223120000",
		PaneID:    "%1",
		Input:     "ls",
		Source:    "keyboard",
	})

	if capturedName != "app:input-history-updated" {
		t.Errorf("event name = %q, want %q", capturedName, "app:input-history-updated")
	}
	if capturedPayload != nil {
		t.Errorf("expected nil payload, got %v", capturedPayload)
	}
}

func TestGetInputHistory_ReturnsIndependentCopy(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	a.writeInputHistoryEntry(InputHistoryEntry{Input: "original"})

	r1 := a.GetInputHistory()
	r2 := a.GetInputHistory()

	r1[0].Input = "mutated"
	if r2[0].Input == "mutated" {
		t.Error("copies are not independent")
	}
}

func TestGetInputHistoryFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)
	a.initInputHistory(a.configState.ConfigPath())
	defer a.closeInputHistory()

	result := a.GetInputHistoryFilePath()
	if result == "" {
		t.Error("expected non-empty path")
	}
	if _, err := os.Stat(result); err != nil {
		t.Errorf("file not found at %s: %v", result, err)
	}
}

// --------------------------------------------------------------------
// processInputString tests
// --------------------------------------------------------------------

func TestProcessInputString_CSIRemoval(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "arrow up", input: "\x1b[A", want: ""},
		{name: "arrow down", input: "\x1b[B", want: ""},
		{name: "CSI with params", input: "\x1b[0;1m", want: ""},
		{name: "CSI mid-text", input: "ls\x1b[0mfoo", want: "lsfoo"},
		{name: "cursor position", input: "\x1b[H", want: ""},
		{name: "multiple CSI", input: "\x1b[A\x1b[B", want: ""},
		{name: "CSI at end", input: "abc\x1b[A", want: "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processInputString(tt.input)
			if got != tt.want {
				t.Errorf("processInputString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProcessInputString_OSCRemoval(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "OSC with BEL terminator", input: "\x1b]0;title\x07", want: ""},
		{name: "OSC with ST terminator", input: "\x1b]0;title\x1b\\", want: ""},
		{name: "OSC mid-text", input: "abc\x1b]0;foo\x07xyz", want: "abcxyz"},
		{name: "OSC at end without terminator", input: "\x1b]0;title", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processInputString(tt.input)
			if got != tt.want {
				t.Errorf("processInputString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProcessInputString_STTerminatedSequenceRemoval(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "DCS removed up to ST", input: "\x1bP1;2|payload\x1b\\tail", want: "tail"},
		{name: "SOS removed up to ST", input: "\x1bXpayload\x1b\\tail", want: "tail"},
		{name: "PM removed up to ST", input: "\x1b^payload\x1b\\tail", want: "tail"},
		{name: "APC removed up to ST", input: "\x1b_payload\x1b\\tail", want: "tail"},
		{name: "unterminated DCS consumes remainder", input: "\x1bPpayload", want: ""},
		{name: "DCS with BEL inside", input: "\x1bPhello\x1b\\world", want: "world"},
		{name: "SOS sequence", input: "\x1bXdata\x1b\\visible", want: "visible"},
		{name: "APC sequence", input: "\x1b_command\x1b\\text", want: "text"},
		{name: "PM sequence", input: "\x1b^private\x1b\\output", want: "output"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processInputString(tt.input)
			if got != tt.want {
				t.Errorf("processInputString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProcessInputString_PreservesNormalChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain ASCII", input: "ls -la", want: "ls -la"},
		{name: "CR preserved", input: "abc\r", want: "abc\r"},
		{name: "ctrl-C preserved", input: "\x03", want: "\x03"},
		{name: "ctrl-D preserved", input: "\x04", want: "\x04"},
		{name: "backspace preserved", input: "\x08", want: "\x08"},
		{name: "DEL preserved", input: "\x7f", want: "\x7f"},
		{name: "multibyte runes", input: "日本語", want: "日本語"},
		{name: "mixed CSI and normal", input: "\x1b[Aclaude\r", want: "claude\r"},
		{name: "empty string", input: "", want: ""},
		{name: "newline stripped", input: "abc\ndef", want: "abcdef"},
		{name: "tab stripped", input: "abc\tdef", want: "abcdef"},
		{name: "lone ESC skipped", input: "\x1babc", want: "abc"},
		{name: "SS3 sequence skipped", input: "\x1bOP", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processInputString(tt.input)
			if got != tt.want {
				t.Errorf("processInputString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --------------------------------------------------------------------
// Line buffering tests
// --------------------------------------------------------------------

func TestRecordInput_EmptyInputIgnored(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	a.recordInput("%1", "", "keyboard", "")

	if got := a.GetInputHistory(); len(got) != 0 {
		t.Fatalf("expected 0 history entries for empty input, got %d", len(got))
	}
}

func TestRecordInput_IgnoredDuringShutdown(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	a.shuttingDown.Store(true)
	a.recordInput("%1", "echo test\r", "keyboard", "session-a")

	if got := a.GetInputHistory(); len(got) != 0 {
		t.Fatalf("expected no entries while shutting down, got %d", len(got))
	}
}

func TestRecordInput_CSIOnlyInputIgnored(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	// Arrow key up: CSI sequence only - should produce no history entry.
	a.recordInput("%1", "\x1b[A", "keyboard", "")

	result := a.GetInputHistory()
	if len(result) != 0 {
		t.Errorf("expected 0 entries for CSI-only input, got %d", len(result))
	}
}

func TestRecordInput_EnterFlush(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)
	a.initInputHistory(a.configState.ConfigPath())
	defer a.closeInputHistory()

	// Type "claude" keystroke by keystroke then press Enter.
	for _, ch := range "claude" {
		a.recordInput("%1", string(ch), "keyboard", "")
	}
	a.recordInput("%1", "\r", "keyboard", "")

	result := a.GetInputHistory()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry after Enter, got %d", len(result))
	}
	if result[0].Input != "claude" {
		t.Errorf("input = %q, want %q", result[0].Input, "claude")
	}
	if result[0].PaneID != "%1" {
		t.Errorf("pane_id = %q, want %%1", result[0].PaneID)
	}
	if result[0].Source != "keyboard" {
		t.Errorf("source = %q, want keyboard", result[0].Source)
	}
}

func TestRecordInput_EmptyEnter(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	// Enter on empty buffer should not create an entry.
	a.recordInput("%1", "\r", "keyboard", "")

	result := a.GetInputHistory()
	if len(result) != 0 {
		t.Errorf("expected 0 entries for empty Enter, got %d", len(result))
	}
}

func TestRecordInput_MultilineInput(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)
	a.initInputHistory(a.configState.ConfigPath())
	defer a.closeInputHistory()

	// Type two commands separated by Enter in a single call.
	a.recordInput("%1", "ls\rcd /tmp\r", "keyboard", "")

	result := a.GetInputHistory()
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0].Input != "ls" {
		t.Errorf("entry[0].Input = %q, want %q", result[0].Input, "ls")
	}
	if result[1].Input != "cd /tmp" {
		t.Errorf("entry[1].Input = %q, want %q", result[1].Input, "cd /tmp")
	}
}

func TestRecordInput_BackspaceEditing(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)
	a.initInputHistory(a.configState.ConfigPath())
	defer a.closeInputHistory()

	// Type "lss", backspace once to fix typo, then Enter.
	a.recordInput("%1", "ls", "keyboard", "")
	a.recordInput("%1", "s", "keyboard", "")    // typo
	a.recordInput("%1", "\x7f", "keyboard", "") // DEL backspace
	a.recordInput("%1", "\r", "keyboard", "")

	result := a.GetInputHistory()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	if result[0].Input != "ls" {
		t.Errorf("input = %q, want %q", result[0].Input, "ls")
	}
}

func TestRecordInput_BackspaceOnEmptyBuffer(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	// Backspace on empty buffer should not panic.
	a.recordInput("%1", "\x08", "keyboard", "")
	a.recordInput("%1", "\x7f", "keyboard", "")

	result := a.GetInputHistory()
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

func TestRecordInput_CtrlC(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)
	a.initInputHistory(a.configState.ConfigPath())
	defer a.closeInputHistory()

	// Type partial command then Ctrl+C.
	a.recordInput("%1", "some-cmd", "keyboard", "")
	a.recordInput("%1", "\x03", "keyboard", "")

	result := a.GetInputHistory()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry (^C), got %d", len(result))
	}
	if result[0].Input != "^C" {
		t.Errorf("input = %q, want %q", result[0].Input, "^C")
	}
}

func TestRecordInput_CtrlCDiscardsBuffer(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)
	a.initInputHistory(a.configState.ConfigPath())
	defer a.closeInputHistory()

	// Type partial command, Ctrl+C, then new command + Enter.
	a.recordInput("%1", "bad-cmd\x03new-cmd\r", "keyboard", "")

	result := a.GetInputHistory()
	if len(result) != 2 {
		t.Fatalf("expected 2 entries (^C then new-cmd), got %d", len(result))
	}
	if result[0].Input != "^C" {
		t.Errorf("entry[0].Input = %q, want ^C", result[0].Input)
	}
	if result[1].Input != "new-cmd" {
		t.Errorf("entry[1].Input = %q, want new-cmd", result[1].Input)
	}
}

func TestRecordInput_CtrlD(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantInput string
	}{
		{name: "Ctrl+D on empty buffer", input: "\x04", wantInput: "^D"},
		{name: "Ctrl+D with buffer content", input: "exit\x04", wantInput: "exit (^D)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
			}
			stubRuntimeEventsEmit(t)
			a.initInputHistory(a.configState.ConfigPath())
			defer a.closeInputHistory()

			a.recordInput("%1", tt.input, "keyboard", "")

			result := a.GetInputHistory()
			if len(result) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(result))
			}
			if result[0].Input != tt.wantInput {
				t.Errorf("input = %q, want %q", result[0].Input, tt.wantInput)
			}
		})
	}
}

func TestRecordInput_MaxInputLen(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	// Fill buffer to max, then one more character should be silently dropped.
	a.recordInput("%1", strings.Repeat("あ", inputHistoryMaxInputLen), "keyboard", "")
	a.recordInput("%1", "X", "keyboard", "") // should be truncated
	a.recordInput("%1", "\r", "keyboard", "")

	result := a.GetInputHistory()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}
	runeCount := len([]rune(result[0].Input))
	if runeCount != inputHistoryMaxInputLen {
		t.Errorf("rune count = %d, want %d", runeCount, inputHistoryMaxInputLen)
	}
}

func TestRecordInput_DifferentPanesSeparateBuffers(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)
	a.initInputHistory(a.configState.ConfigPath())
	defer a.closeInputHistory()

	a.recordInput("%1", "abc\r", "keyboard", "")
	a.recordInput("%2", "xyz\r", "keyboard", "")

	result := a.GetInputHistory()
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}

	paneInputs := map[string]string{}
	for _, e := range result {
		paneInputs[e.PaneID] = e.Input
	}
	if paneInputs["%1"] != "abc" {
		t.Errorf("pane %%1 input = %q, want %q", paneInputs["%1"], "abc")
	}
	if paneInputs["%2"] != "xyz" {
		t.Errorf("pane %%2 input = %q, want %q", paneInputs["%2"], "xyz")
	}
}

func TestRecordInput_TimeoutFlush(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)
	a.initInputHistory(a.configState.ConfigPath())
	defer a.closeInputHistory()

	// Type without pressing Enter - should flush via flushLineBuffer (simulated).
	a.recordInput("%1", "hello", "keyboard", "")

	// Directly invoke the timeout flush (avoids 5s wait in tests).
	// shutdownFlushSentinel bypasses the generation check.
	a.flushLineBuffer("%1", shutdownFlushSentinel)

	result := a.GetInputHistory()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry after timeout flush, got %d", len(result))
	}
	if result[0].Input != "hello" {
		t.Errorf("input = %q, want %q", result[0].Input, "hello")
	}
}

func TestRecordInput_RefreshesMetadataForTimeoutFlush(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)
	a.initInputHistory(a.configState.ConfigPath())
	defer a.closeInputHistory()

	a.recordInput("%1", "abc", "keyboard", "session-a")
	a.recordInput("%1", "def", "sync-input", "session-b")
	a.flushLineBuffer("%1", shutdownFlushSentinel)

	result := a.GetInputHistory()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry after timeout flush, got %d", len(result))
	}
	if result[0].Input != "abcdef" {
		t.Fatalf("input = %q, want %q", result[0].Input, "abcdef")
	}
	if result[0].Source != "sync-input" {
		t.Fatalf("source = %q, want %q", result[0].Source, "sync-input")
	}
	if result[0].Session != "session-b" {
		t.Fatalf("session = %q, want %q", result[0].Session, "session-b")
	}
}

func TestFlushLineBuffer_NoOpForMissingPane(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	// Should not panic when pane does not exist.
	a.flushLineBuffer("%nonexistent", shutdownFlushSentinel)

	result := a.GetInputHistory()
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

func TestFlushLineBuffer_NoOpForEmptyBuffer(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	// Build and then erase a line buffer so the pane exists with empty content.
	a.recordInput("%1", "abc", "keyboard", "")
	a.recordInput("%1", "\x7f\x7f\x7f", "keyboard", "")

	a.flushLineBuffer("%1", shutdownFlushSentinel)

	result := a.GetInputHistory()
	if len(result) != 0 {
		t.Errorf("expected 0 entries for empty buffer flush, got %d", len(result))
	}
}

func TestFlushAllLineBuffers_PersistsPendingBuffers(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)
	a.initInputHistory(a.configState.ConfigPath())
	defer a.closeInputHistory()

	// Record to multiple panes without pressing Enter.
	a.recordInput("%1", "cmd1", "keyboard", "session-a")
	a.recordInput("%2", "cmd2", "sync-input", "session-b")
	a.recordInput("%3", "cmd3", "keyboard", "session-c")

	// Flush all (as shutdown would do).
	a.flushAllLineBuffers()

	result := a.GetInputHistory()
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	entriesByPane := map[string]InputHistoryEntry{}
	for _, entry := range result {
		entriesByPane[entry.PaneID] = entry
	}
	tests := []struct {
		paneID  string
		input   string
		source  string
		session string
	}{
		{paneID: "%1", input: "cmd1", source: "keyboard", session: "session-a"},
		{paneID: "%2", input: "cmd2", source: "sync-input", session: "session-b"},
		{paneID: "%3", input: "cmd3", source: "keyboard", session: "session-c"},
	}
	for _, tt := range tests {
		entry, ok := entriesByPane[tt.paneID]
		if !ok {
			t.Fatalf("missing pane entry for %s", tt.paneID)
		}
		if entry.Input != tt.input {
			t.Fatalf("pane %s input = %q, want %q", tt.paneID, entry.Input, tt.input)
		}
		if entry.Source != tt.source {
			t.Fatalf("pane %s source = %q, want %q", tt.paneID, entry.Source, tt.source)
		}
		if entry.Session != tt.session {
			t.Fatalf("pane %s session = %q, want %q", tt.paneID, entry.Session, tt.session)
		}
	}

	// Verify all persisted to JSONL.
	content, err := os.ReadFile(a.GetInputHistoryFilePath())
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 JSONL lines, got %d", len(lines))
	}
	jsonlEntriesByPane := map[string]InputHistoryEntry{}
	for _, line := range lines {
		var entry InputHistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("failed to unmarshal JSONL line: %v", err)
		}
		jsonlEntriesByPane[entry.PaneID] = entry
	}
	for _, tt := range tests {
		entry, ok := jsonlEntriesByPane[tt.paneID]
		if !ok {
			t.Fatalf("missing JSONL pane entry for %s", tt.paneID)
		}
		if entry.Input != tt.input {
			t.Fatalf("JSONL pane %s input = %q, want %q", tt.paneID, entry.Input, tt.input)
		}
		if entry.Source != tt.source {
			t.Fatalf("JSONL pane %s source = %q, want %q", tt.paneID, entry.Source, tt.source)
		}
		if entry.Session != tt.session {
			t.Fatalf("JSONL pane %s session = %q, want %q", tt.paneID, entry.Session, tt.session)
		}
	}
}

func TestFlushAllLineBuffers_ClearsBufferMap(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	a.recordInput("%1", "test", "keyboard", "")

	a.flushAllLineBuffers()
	got := a.GetInputHistory()
	if len(got) != 1 || got[0].Input != "test" {
		t.Fatalf("expected one flushed entry after first flushAll, got %#v", got)
	}
	// Second flush should be a no-op (no stale buffers / duplicate writes).
	a.flushAllLineBuffers()
	got = a.GetInputHistory()
	if len(got) != 1 {
		t.Fatalf("expected no additional entries after second flushAll, got %d", len(got))
	}
}

func TestFlushAllLineBuffers_NoOpWhenNilMap(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	// Should not panic with nil map.
	a.flushAllLineBuffers()
}

func TestFlushLineBuffer_IgnoresTimerFlushDuringShutdown(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	a.recordInput("%1", "pending", "keyboard", "session-a")
	a.shuttingDown.Store(true)
	a.flushLineBuffer("%1", 1)

	if got := a.GetInputHistory(); len(got) != 0 {
		t.Fatalf("expected no entries from timer flush during shutdown, got %d", len(got))
	}
}

func TestFlushLineBuffer_ShutdownSentinelStillFlushes(t *testing.T) {
	a := &App{
		configState: newConfigStateForTest("config.yaml"),
	}
	stubRuntimeEventsEmit(t)

	a.recordInput("%1", "pending", "keyboard", "session-a")
	a.shuttingDown.Store(true)
	a.flushLineBuffer("%1", shutdownFlushSentinel)

	result := a.GetInputHistory()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry from shutdown sentinel flush, got %d", len(result))
	}
	if result[0].Input != "pending" {
		t.Fatalf("input = %q, want %q", result[0].Input, "pending")
	}
}

func TestRecordInput_ConcurrentSafety(t *testing.T) {
	tmpDir := t.TempDir()
	a := &App{
		configState: newConfigStateForTest(filepath.Join(tmpDir, "config.yaml")),
	}
	stubRuntimeEventsEmit(t)
	a.initInputHistory(a.configState.ConfigPath())
	defer func() {
		a.flushAllLineBuffers()
		a.closeInputHistory()
	}()

	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		paneID := fmt.Sprintf("%%%d", i)
		go func(pid string) {
			defer wg.Done()
			for j := range 5 {
				a.recordInput(pid, fmt.Sprintf("cmd%d\r", j), "keyboard", "")
			}
		}(paneID)
	}
	wg.Wait()

	result := a.GetInputHistory()
	// Each pane sends 5 commands with Enter -> 50 entries total.
	if len(result) != 50 {
		t.Errorf("expected 50 entries (10 panes x 5 commands), got %d", len(result))
	}

	// Verify all seq values are unique and monotonically increasing.
	for i := 1; i < len(result); i++ {
		if result[i].Seq <= result[i-1].Seq {
			t.Errorf("entry %d: seq %d not greater than %d", i, result[i].Seq, result[i-1].Seq)
		}
	}
}
