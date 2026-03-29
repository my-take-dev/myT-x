package inputhistory

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
)

// --------------------------------------------------------------------
// Ring buffer tests (migrated from main package)
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
			rb.push(Entry{Seq: 1, Input: "hello"})

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
			wantFirst: "input-0",
			wantLast:  "input-2",
		},
		{
			name:      "at capacity",
			capacity:  5,
			pushCount: 5,
			wantCount: 5,
			wantFirst: "input-0",
			wantLast:  "input-4",
		},
		{
			name:      "overflow wraps around",
			capacity:  5,
			pushCount: 8,
			wantCount: 5,
			wantFirst: "input-3",
			wantLast:  "input-7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := newRingBuffer(tt.capacity)
			for i := 0; i < tt.pushCount; i++ {
				rb.push(Entry{Input: fmt.Sprintf("input-%d", i)})
			}

			snap := rb.snapshot()
			if len(snap) != tt.wantCount {
				t.Fatalf("snapshot length = %d, want %d", len(snap), tt.wantCount)
			}
			if snap[0].Input != tt.wantFirst {
				t.Errorf("first entry = %q, want %q", snap[0].Input, tt.wantFirst)
			}
			if snap[len(snap)-1].Input != tt.wantLast {
				t.Errorf("last entry = %q, want %q", snap[len(snap)-1].Input, tt.wantLast)
			}
		})
	}
}

func TestRingBuffer_SnapshotIndependence(t *testing.T) {
	rb := newRingBuffer(10)
	rb.push(Entry{Input: "original"})

	snap1 := rb.snapshot()
	snap2 := rb.snapshot()

	snap1[0].Input = "mutated"
	if snap2[0].Input == "mutated" {
		t.Error("mutating snapshot1 affected snapshot2 - copies are not independent")
	}
}

func TestRingBuffer_EmptySnapshot(t *testing.T) {
	rb := newRingBuffer(10)
	snap := rb.snapshot()
	if snap == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(snap) != 0 {
		t.Errorf("expected 0 entries, got %d", len(snap))
	}
}

// --------------------------------------------------------------------
// NewService tests
// --------------------------------------------------------------------

func TestNewService_NilShutdownFunc(t *testing.T) {
	svc := NewService(nil, nil)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	// isShuttingDown should default to returning false.
	if svc.isShuttingDown() {
		t.Error("expected isShuttingDown to return false with nil func")
	}
}

// --------------------------------------------------------------------
// WriteEntry tests
// --------------------------------------------------------------------

type mockEmitter struct {
	mu    sync.Mutex
	calls []string
}

func (m *mockEmitter) Emit(eventName string, _ any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, eventName)
}

func (m *mockEmitter) EmitWithContext(_ context.Context, eventName string, _ any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, eventName)
}

func TestWriteEntry_EmitsEvent(t *testing.T) {
	em := &mockEmitter{}
	svc := NewService(em, func() bool { return false })

	svc.WriteEntry(Entry{
		Timestamp: "20260223120000",
		PaneID:    "%1",
		Input:     "ls",
		Source:    "keyboard",
	})

	em.mu.Lock()
	defer em.mu.Unlock()
	if len(em.calls) != 1 {
		t.Fatalf("expected 1 emit call, got %d", len(em.calls))
	}
	if em.calls[0] != "app:input-history-updated" {
		t.Errorf("event name = %q, want %q", em.calls[0], "app:input-history-updated")
	}
}

func TestWriteEntry_ThrottlesEmission(t *testing.T) {
	em := &mockEmitter{}
	svc := NewService(em, func() bool { return false })

	// First write should emit.
	svc.WriteEntry(Entry{Input: "first"})

	// Rapid subsequent writes should be throttled (within emitMinInterval).
	for range 5 {
		svc.WriteEntry(Entry{Input: "throttled"})
	}

	em.mu.Lock()
	defer em.mu.Unlock()
	// Only the first write should have triggered an emit (the rest are within the interval).
	if len(em.calls) != 1 {
		t.Errorf("expected 1 emit call (throttled), got %d", len(em.calls))
	}
}

func TestWriteEntry_SeqMonotonicallyIncreasing(t *testing.T) {
	svc := NewService(nil, func() bool { return false })

	for range 20 {
		svc.WriteEntry(Entry{Input: "test"})
	}

	snap := svc.Snapshot()
	if len(snap) != 20 {
		t.Fatalf("expected 20 entries, got %d", len(snap))
	}
	for i := 1; i < len(snap); i++ {
		if snap[i].Seq != snap[i-1].Seq+1 {
			t.Errorf("entry %d: seq = %d, want %d", i, snap[i].Seq, snap[i-1].Seq+1)
		}
	}
}

func TestWriteEntry_WritesToFile(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(nil, func() bool { return false })
	svc.Init(filepath.Join(tmpDir, "config.yaml"))
	defer svc.Close()

	svc.WriteEntry(Entry{
		Timestamp: "20260223120000",
		PaneID:    "%5",
		Input:     "ls -la",
		Source:    "keyboard",
	})

	content, err := os.ReadFile(svc.FilePath())
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var parsed Entry
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if parsed.Seq == 0 {
		t.Error("expected seq > 0")
	}
	if parsed.Input != "ls -la" {
		t.Errorf("input = %q, want %q", parsed.Input, "ls -la")
	}
}

// --------------------------------------------------------------------
// parseFileName / sortFilesForCleanup tests
// --------------------------------------------------------------------

func TestParseFileName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantTS  string
		wantPID int
		wantOK  bool
	}{
		{
			name:    "valid filename",
			input:   "input-20260101-120000-1234.jsonl",
			wantTS:  "20260101-120000",
			wantPID: 1234,
			wantOK:  true,
		},
		{
			name:   "missing prefix",
			input:  "history-20260101-120000-1234.jsonl",
			wantOK: false,
		},
		{
			name:   "missing suffix",
			input:  "input-20260101-120000-1234.txt",
			wantOK: false,
		},
		{
			name:   "wrong part count",
			input:  "input-20260101-1234.jsonl",
			wantOK: false,
		},
		{
			name:   "non-numeric date",
			input:  "input-2026ABCD-120000-1234.jsonl",
			wantOK: false,
		},
		{
			name:   "non-numeric pid",
			input:  "input-20260101-120000-abc.jsonl",
			wantOK: false,
		},
		{
			name:   "empty string",
			input:  "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, pid, ok := parseFileName(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if ts != tt.wantTS {
				t.Errorf("timestamp = %q, want %q", ts, tt.wantTS)
			}
			if pid != tt.wantPID {
				t.Errorf("pid = %d, want %d", pid, tt.wantPID)
			}
		})
	}
}

func TestSortFilesForCleanup(t *testing.T) {
	files := []string{
		"input-20260101-000000-10.jsonl",
		"input-20260101-000000-9.jsonl",
		"input-20260102-000000-1.jsonl",
		"not-a-history-file.txt",
		"input-20260101-000000-9.jsonl",
	}

	sortFilesForCleanup(files)

	// Non-parseable files sort first, then by timestamp, then by numeric PID.
	expected := []string{
		"not-a-history-file.txt",
		"input-20260101-000000-9.jsonl",
		"input-20260101-000000-9.jsonl",
		"input-20260101-000000-10.jsonl",
		"input-20260102-000000-1.jsonl",
	}
	for i, got := range files {
		if got != expected[i] {
			t.Errorf("index %d: got %q, want %q", i, got, expected[i])
		}
	}
}

// --------------------------------------------------------------------
// Init tests
// --------------------------------------------------------------------

func TestInit_Reinitialization(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(nil, func() bool { return false })

	// First init.
	configPath := filepath.Join(tmpDir, "config.yaml")
	svc.Init(configPath)
	path1 := svc.FilePath()
	if path1 == "" {
		t.Fatal("expected non-empty path after first init")
	}

	// Small delay to ensure different filename (timestamp-based).
	time.Sleep(1100 * time.Millisecond)

	// Re-init should create a new file and close the old one.
	svc.Init(configPath)
	path2 := svc.FilePath()
	if path2 == "" {
		t.Fatal("expected non-empty path after re-init")
	}
	if path1 == path2 {
		t.Error("expected different file paths after re-init")
	}

	// Old file should be closed (removable).
	if err := os.Remove(path1); err != nil {
		t.Fatalf("expected old file to be removable after re-init: %v", err)
	}

	svc.Close()
}

// --------------------------------------------------------------------
// ProcessInputString tests (package-level)
// --------------------------------------------------------------------

func TestProcessInputString_CSI(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "arrow up removed", input: "\x1b[A", want: ""},
		{name: "CSI mid-text", input: "ls\x1b[0mfoo", want: "lsfoo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProcessInputString(tt.input); got != tt.want {
				t.Errorf("ProcessInputString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestProcessInputString_PreservesControlChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "CR preserved", input: "abc\r", want: "abc\r"},
		{name: "ctrl-C preserved", input: "\x03", want: "\x03"},
		{name: "ctrl-D preserved", input: "\x04", want: "\x04"},
		{name: "backspace preserved", input: "\x08", want: "\x08"},
		{name: "DEL preserved", input: "\x7f", want: "\x7f"},
		{name: "empty string", input: "", want: ""},
		{name: "multibyte runes", input: "日本語", want: "日本語"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ProcessInputString(tt.input); got != tt.want {
				t.Errorf("ProcessInputString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --------------------------------------------------------------------
// Concurrent safety test
// --------------------------------------------------------------------

func TestService_ConcurrentWriteEntry(t *testing.T) {
	svc := NewService(nil, func() bool { return false })

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			for range 5 {
				svc.WriteEntry(Entry{Input: "concurrent"})
			}
		})
	}
	wg.Wait()

	snap := svc.Snapshot()
	if len(snap) != 50 {
		t.Errorf("expected 50 entries, got %d", len(snap))
	}

	// All seq values must be unique and monotonically increasing.
	for i := 1; i < len(snap); i++ {
		if snap[i].Seq <= snap[i-1].Seq {
			t.Errorf("entry %d: seq %d not greater than %d", i, snap[i].Seq, snap[i-1].Seq)
		}
	}
}

// --------------------------------------------------------------------
// lineBuffer.stopTimer test
// --------------------------------------------------------------------

func TestLineBuffer_StopTimer_NilSafe(t *testing.T) {
	lb := &lineBuffer{}
	// Should not panic when timer is nil.
	lb.stopTimer()
	if lb.timer != nil {
		t.Error("expected timer to remain nil")
	}
}

func TestLineBuffer_StopTimer_StopsAndNils(t *testing.T) {
	lb := &lineBuffer{
		timer: time.AfterFunc(time.Hour, func() {}),
	}
	lb.stopTimer()
	if lb.timer != nil {
		t.Error("expected timer to be nil after stopTimer")
	}
}
