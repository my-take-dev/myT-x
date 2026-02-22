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
	"time"
)

func TestInitSessionLog_CreatesDirectory(t *testing.T) {
	tests := []struct {
		name      string
		configDir string
		wantDir   string
	}{
		{
			name:      "creates session-logs directory in config parent",
			configDir: "test_config.yaml",
			wantDir:   "session-logs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configPath:        filepath.Join(tmpDir, tt.configDir),
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog()

			expectedDir := filepath.Join(tmpDir, tt.wantDir)
			info, err := os.Stat(expectedDir)
			if err != nil {
				t.Fatalf("expected session-logs directory to exist at %s, got error: %v", expectedDir, err)
			}
			if !info.IsDir() {
				t.Fatalf("expected %s to be a directory, but it is not", expectedDir)
			}

			// Cleanup
			if a.sessionLogFile != nil {
				a.sessionLogFile.Close()
			}
		})
	}
}

func TestInitSessionLog_CreatesJSONLFile(t *testing.T) {
	tests := []struct {
		name              string
		configDir         string
		expectedFileRegex string
	}{
		{
			name:              "creates session-*.jsonl file",
			configDir:         "config.yaml",
			expectedFileRegex: "session-.*\\.jsonl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configPath:        filepath.Join(tmpDir, tt.configDir),
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog()

			if a.sessionLogFile == nil {
				t.Fatal("expected sessionLogFile to be opened, got nil")
			}

			// Verify the file path is set and file exists
			if a.sessionLogPath == "" {
				t.Fatal("expected sessionLogPath to be set, got empty string")
			}

			info, err := os.Stat(a.sessionLogPath)
			if err != nil {
				t.Fatalf("expected session log file to exist at %s, got error: %v", a.sessionLogPath, err)
			}

			// Verify it's a file (not directory)
			if info.IsDir() {
				t.Fatalf("expected %s to be a file, but it is a directory", a.sessionLogPath)
			}

			// Verify filename matches pattern: session-YYYYMMDD-HHMMSS-PID.jsonl (A-2)
			filename := filepath.Base(a.sessionLogPath)
			if !strings.HasPrefix(filename, "session-") || !strings.HasSuffix(filename, ".jsonl") {
				t.Fatalf("expected filename matching session-*.jsonl pattern, got %s", filename)
			}
			// Verify PID is present in the filename (A-2: collision prevention).
			pidPattern := regexp.MustCompile(`^session-\d{8}-\d{6}-\d+\.jsonl$`)
			if !pidPattern.MatchString(filename) {
				t.Fatalf("expected filename matching session-YYYYMMDD-HHMMSS-PID.jsonl pattern, got %s", filename)
			}

			// Cleanup
			if a.sessionLogFile != nil {
				a.sessionLogFile.Close()
			}
		})
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
				configPath:        filepath.Join(tmpDir, "config.yaml"),
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog()
			defer func() {
				if a.sessionLogFile != nil {
					a.sessionLogFile.Close()
				}
			}()

			a.writeSessionLogEntry(tt.entry)

			// Read the file and verify JSONL format
			content, err := os.ReadFile(a.sessionLogPath)
			if err != nil {
				t.Fatalf("failed to read log file: %v", err)
			}

			lines := strings.Split(strings.TrimSpace(string(content)), "\n")
			if len(lines) == 0 {
				t.Fatal("expected at least one line in the log file")
			}

			// Parse the first (and should be only) line as JSON
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
			// Verify seq field is assigned (A-seq: must be > 0 after write).
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
				configPath:        filepath.Join(tmpDir, "config.yaml"),
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog()
			defer func() {
				if a.sessionLogFile != nil {
					a.sessionLogFile.Close()
				}
			}()

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
		maxEntries     int
		expectedCount  int
	}{
		{
			name:           "keeps only the newest entries when exceeding max",
			entriesToWrite: sessionLogMaxEntries + 100,
			maxEntries:     sessionLogMaxEntries,
			expectedCount:  sessionLogMaxEntries,
		},
		{
			name:           "does not cap when below max",
			entriesToWrite: sessionLogMaxEntries - 100,
			maxEntries:     sessionLogMaxEntries,
			expectedCount:  sessionLogMaxEntries - 100,
		},
		{
			name:           "handles exact max entries",
			entriesToWrite: sessionLogMaxEntries,
			maxEntries:     sessionLogMaxEntries,
			expectedCount:  sessionLogMaxEntries,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configPath:        filepath.Join(tmpDir, "config.yaml"),
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog()
			defer func() {
				if a.sessionLogFile != nil {
					a.sessionLogFile.Close()
				}
			}()

			// Write entries
			for i := 0; i < tt.entriesToWrite; i++ {
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

			// Verify the newest entries are retained (message numbers should be sequential at the end)
			if len(result) > 0 {
				// The last entry should have a message with a high index
				lastMessage := result[len(result)-1].Message
				if !strings.HasPrefix(lastMessage, "entry") {
					t.Errorf("expected last message to start with 'entry', got %s", lastMessage)
				}
			}
		})
	}
}

func TestCleanupOldSessionLogs_KeepsMaxFiles(t *testing.T) {
	tests := []struct {
		name               string
		filesToCreate      int
		maxFilesToKeep     int
		expectedFilesAfter int
	}{
		{
			name:               "deletes oldest files when count exceeds max",
			filesToCreate:      110,
			maxFilesToKeep:     sessionLogMaxFiles,
			expectedFilesAfter: sessionLogMaxFiles,
		},
		{
			name:               "keeps all files when below max",
			filesToCreate:      50,
			maxFilesToKeep:     sessionLogMaxFiles,
			expectedFilesAfter: 50,
		},
		{
			name:               "handles exact max files",
			filesToCreate:      sessionLogMaxFiles,
			maxFilesToKeep:     sessionLogMaxFiles,
			expectedFilesAfter: sessionLogMaxFiles,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logDir := filepath.Join(tmpDir, "session-logs")
			if err := os.MkdirAll(logDir, 0o700); err != nil {
				t.Fatalf("failed to create log directory: %v", err)
			}

			// Create dummy log files with sequential timestamps
			for i := 0; i < tt.filesToCreate; i++ {
				filename := "session-" + formatIndexAsTimestamp(i) + ".jsonl"
				filePath := filepath.Join(logDir, filename)
				file, err := os.Create(filePath)
				if err != nil {
					t.Fatalf("failed to create dummy log file: %v", err)
				}
				file.Close()
			}

			// Set up App and call cleanup
			a := &App{
				sessionLogPath: filepath.Join(logDir, "session-current.jsonl"),
			}

			stubRuntimeEventsEmit(t)

			a.cleanupOldSessionLogs()

			// Count remaining files
			entries, err := os.ReadDir(logDir)
			if err != nil {
				t.Fatalf("failed to read log directory: %v", err)
			}

			var logFiles []string
			for _, entry := range entries {
				name := entry.Name()
				if !entry.IsDir() && strings.HasPrefix(name, "session-") && strings.HasSuffix(name, ".jsonl") {
					logFiles = append(logFiles, name)
				}
			}

			if len(logFiles) != tt.expectedFilesAfter {
				t.Errorf("expected %d files after cleanup, got %d", tt.expectedFilesAfter, len(logFiles))
			}
		})
	}
}

func TestCleanupOldSessionLogs_DeletesOldest(t *testing.T) {
	tests := []struct {
		name          string
		filesToCreate int
		expectDeleted []int // indices that should be deleted
	}{
		{
			name:          "deletes oldest files first when exceeding max",
			filesToCreate: sessionLogMaxFiles + 10,
			expectDeleted: func() []int {
				indices := make([]int, 10)
				for i := range indices {
					indices[i] = i
				}
				return indices
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			logDir := filepath.Join(tmpDir, "session-logs")
			if err := os.MkdirAll(logDir, 0o700); err != nil {
				t.Fatalf("failed to create log directory: %v", err)
			}

			// Create dummy log files with sequential timestamps
			createdFiles := make([]string, tt.filesToCreate)
			for i := 0; i < tt.filesToCreate; i++ {
				filename := "session-" + formatIndexAsTimestamp(i) + ".jsonl"
				createdFiles[i] = filename
				filePath := filepath.Join(logDir, filename)
				file, err := os.Create(filePath)
				if err != nil {
					t.Fatalf("failed to create dummy log file: %v", err)
				}
				file.Close()
			}

			// Set up App and call cleanup
			a := &App{
				sessionLogPath: filepath.Join(logDir, "session-current.jsonl"),
			}

			stubRuntimeEventsEmit(t)

			a.cleanupOldSessionLogs()

			// Verify that the oldest files are deleted
			for _, idx := range tt.expectDeleted {
				deletedFile := filepath.Join(logDir, createdFiles[idx])
				if _, err := os.Stat(deletedFile); err == nil {
					t.Errorf("expected file %s to be deleted, but it still exists", createdFiles[idx])
				}
			}

			// Verify that newer files still exist
			for i := len(tt.expectDeleted); i < tt.filesToCreate; i++ {
				remainingFile := filepath.Join(logDir, createdFiles[i])
				if _, err := os.Stat(remainingFile); err != nil {
					t.Errorf("expected file %s to exist after cleanup, but it was deleted or not found", createdFiles[i])
				}
			}
		})
	}
}

func TestCleanupOldSessionLogs_PreservesCurrentFile(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "session-logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatalf("failed to create log directory: %v", err)
	}

	currentName := "session-20260222-152200-4242.jsonl"
	currentPath := filepath.Join(logDir, currentName)
	if err := os.WriteFile(currentPath, []byte("current\n"), 0o600); err != nil {
		t.Fatalf("failed to create current log file: %v", err)
	}

	// Create enough files to trigger cleanup.
	for i := 0; i < sessionLogMaxFiles+20; i++ {
		name := fmt.Sprintf("session-%s-%04d.jsonl", formatIndexAsTimestamp(i), i)
		if err := os.WriteFile(filepath.Join(logDir, name), []byte("old\n"), 0o600); err != nil {
			t.Fatalf("failed to create old log file: %v", err)
		}
	}

	a := &App{sessionLogPath: currentPath}
	a.cleanupOldSessionLogs()

	if _, err := os.Stat(currentPath); err != nil {
		t.Fatalf("current log file should not be deleted: %v", err)
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
				configPath:        filepath.Join(tmpDir, "config.yaml"),
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog()
			defer func() {
				if a.sessionLogFile != nil {
					a.sessionLogFile.Close()
				}
			}()

			if tt.writeEntry {
				entry := SessionLogEntry{
					Timestamp: "20060102150405",
					Level:     "error",
					Message:   "test",
					Source:    "test",
				}
				a.writeSessionLogEntry(entry)
			}

			result := a.GetSessionErrorLog()
			if len(result) != tt.expectCount {
				t.Errorf("expected %d entries, got %d", tt.expectCount, len(result))
			}

			// Verify that empty result is a slice (not nil), per contract
			if result == nil {
				t.Error("expected non-nil empty slice for empty log, got nil")
			}

			// Verify slice is empty
			if cap(result) > 0 && len(result) == 0 {
				// This is allowed - an empty slice with capacity
				return
			}
		})
	}
}

func TestGetSessionErrorLog_ReturnsIndependentCopy(t *testing.T) {
	tests := []struct {
		name    string
		entries int
	}{
		{
			name:    "returned slice is independent copy",
			entries: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configPath:        filepath.Join(tmpDir, "config.yaml"),
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog()
			defer func() {
				if a.sessionLogFile != nil {
					a.sessionLogFile.Close()
				}
			}()

			// Write entries
			for i := 0; i < tt.entries; i++ {
				entry := SessionLogEntry{
					Timestamp: "20060102150405",
					Level:     "error",
					Message:   fmt.Sprintf("entry %d", i),
					Source:    "test",
				}
				a.writeSessionLogEntry(entry)
			}

			result1 := a.GetSessionErrorLog()
			result2 := a.GetSessionErrorLog()

			// Verify copies are equal but different slices
			if len(result1) != len(result2) {
				t.Errorf("expected same length, got %d and %d", len(result1), len(result2))
			}

			// Modifying one copy should not affect the other
			if len(result1) > 0 {
				originalMessage := result1[0].Message
				result1[0].Message = "modified"
				if result2[0].Message == "modified" {
					t.Error("modifying one copy affected the other - copies are not independent")
				}
				result1[0].Message = originalMessage // restore
			}

			// Verify they still match after restore
			for i := range result1 {
				if result1[i] != result2[i] {
					t.Errorf("entry %d mismatch after modification/restore", i)
				}
			}
		})
	}
}

func TestCloseSessionLog_ClosesFile(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "closeSessionLog sets file handle to nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configPath:        filepath.Join(tmpDir, "config.yaml"),
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog()

			if a.sessionLogFile == nil {
				t.Fatal("expected sessionLogFile to be non-nil after initSessionLog()")
			}

			a.closeSessionLog()

			if a.sessionLogFile != nil {
				t.Error("expected sessionLogFile to be nil after closeSessionLog()")
			}
		})
	}
}

func TestGetSessionLogFilePath_ReturnsCorrectPath(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "returns the current session log file path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configPath:        filepath.Join(tmpDir, "config.yaml"),
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog()
			defer func() {
				if a.sessionLogFile != nil {
					a.sessionLogFile.Close()
				}
			}()

			result := a.GetSessionLogFilePath()

			if result == "" {
				t.Error("expected non-empty path, got empty string")
			}

			if result != a.sessionLogPath {
				t.Errorf("expected path %s, got %s", a.sessionLogPath, result)
			}

			// Verify the file exists
			if _, err := os.Stat(result); err != nil {
				t.Errorf("expected file to exist at %s, got error: %v", result, err)
			}
		})
	}
}

func TestWriteSessionLogEntry_WithoutInitializedFile(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "handles write when sessionLogFile is nil gracefully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &App{
				configPath:        "config.yaml",
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
				// Intentionally not calling initSessionLog()
			}

			stubRuntimeEventsEmit(t)

			entry := SessionLogEntry{
				Timestamp: "20060102150405",
				Level:     "error",
				Message:   "test",
				Source:    "test",
			}

			// Should not panic even though file is nil
			a.writeSessionLogEntry(entry)

			// Entry should be in memory even without a file
			result := a.GetSessionErrorLog()
			if len(result) != 1 {
				t.Errorf("expected 1 entry in memory, got %d", len(result))
			}

			// Verify the entry was properly stored
			if result[0].Message != entry.Message {
				t.Errorf("entry message mismatch: got %s, want %s", result[0].Message, entry.Message)
			}
		})
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
				configPath:        "config.yaml",
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
			}

			stubRuntimeEventsEmit(t)

			for i := 0; i < tt.entryCount; i++ {
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

			// Verify first seq is 1.
			if result[0].Seq != 1 {
				t.Errorf("expected first seq to be 1, got %d", result[0].Seq)
			}

			// Verify strict monotonic increase.
			for i := 1; i < len(result); i++ {
				if result[i].Seq != result[i-1].Seq+1 {
					t.Errorf("entry %d: expected seq %d, got %d", i, result[i-1].Seq+1, result[i].Seq)
				}
			}
		})
	}
}

func TestWriteSessionLogEntry_EmitsCorrectEventName(t *testing.T) {
	tests := []struct {
		name              string
		expectedEventName string
	}{
		{
			name:              "emits app:session-log-updated ping event (A-0)",
			expectedEventName: "app:session-log-updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &App{
				configPath:        "config.yaml",
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
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

			if capturedEventName != tt.expectedEventName {
				t.Errorf("expected event name %q, got %q", tt.expectedEventName, capturedEventName)
			}

			// Verify payload is nil (ping-only, no entry data).
			if capturedPayload != nil {
				t.Errorf("expected nil payload for ping event, got %v", capturedPayload)
			}
		})
	}
}

func TestInitSessionLog_FilenameContainsPID(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "filename includes current process PID (A-2)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			a := &App{
				configPath:        filepath.Join(tmpDir, "config.yaml"),
				sessionLogEntries: newSessionLogRingBuffer(sessionLogMaxEntries),
			}

			stubRuntimeEventsEmit(t)

			a.initSessionLog()
			defer func() {
				if a.sessionLogFile != nil {
					a.sessionLogFile.Close()
				}
			}()

			filename := filepath.Base(a.sessionLogPath)
			expectedPIDSuffix := fmt.Sprintf("-%d.jsonl", os.Getpid())
			if !strings.HasSuffix(filename, expectedPIDSuffix) {
				t.Errorf("expected filename to end with %q, got %q", expectedPIDSuffix, filename)
			}
		})
	}
}

func TestNewSessionLogRingBuffer_ClampsNonPositiveCapacity(t *testing.T) {
	tests := []struct {
		name     string
		capacity int
	}{
		{name: "zero capacity clamped to 1", capacity: 0},
		{name: "negative capacity clamped to 1", capacity: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := newSessionLogRingBuffer(tt.capacity)
			rb.push(SessionLogEntry{
				Seq:       1,
				Timestamp: "20060102150405",
				Level:     "error",
				Message:   "first",
				Source:    "test",
			})

			entries := rb.snapshot()
			if len(entries) != 1 {
				t.Fatalf("snapshot length = %d, want 1", len(entries))
			}
			if entries[0].Seq != 1 {
				t.Fatalf("snapshot[0].Seq = %d, want 1", entries[0].Seq)
			}
		})
	}

	// Verify autoincrement via writeSessionLogEntry (not manual Seq assignment).
	t.Run("autoincrement assigns sequential Seq via writeSessionLogEntry", func(t *testing.T) {
		a := &App{
			configPath:        "config.yaml",
			sessionLogEntries: newSessionLogRingBuffer(0), // clamped to 1
		}
		stubRuntimeEventsEmit(t)

		a.writeSessionLogEntry(SessionLogEntry{
			Timestamp: "20060102150405",
			Level:     "error",
			Message:   "auto-seq",
			Source:    "test",
		})

		result := a.GetSessionErrorLog()
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result[0].Seq != 1 {
			t.Fatalf("expected auto-assigned seq 1, got %d", result[0].Seq)
		}
	})
}

// formatIndexAsTimestamp formats an index as a timestamp for testing purposes.
// The format mimics session-YYYYMMDD-HHMMSS.jsonl
// For index 0, generates "20060101-000000"
// For index 1, generates "20060101-000001", etc.
// Uses time.Date for correct month/year boundary handling.
func formatIndexAsTimestamp(index int) string {
	base := time.Date(2006, 1, 1, 0, 0, 0, 0, time.UTC)
	ts := base.Add(time.Duration(index) * time.Second)
	return ts.Format("20060102-150405")
}
