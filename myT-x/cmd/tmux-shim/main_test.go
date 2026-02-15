package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"myT-x/internal/config"
	"myT-x/internal/ipc"
)

// NOT safe for t.Parallel(): this helper temporarily replaces os.Stderr.
func captureStderr(t *testing.T, run func()) string {
	t.Helper()

	original := os.Stderr
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}

	os.Stderr = writePipe
	t.Cleanup(func() {
		os.Stderr = original
		_ = writePipe.Close()
		_ = readPipe.Close()
	})

	run()
	_ = writePipe.Close()

	output, readErr := io.ReadAll(readPipe)
	if readErr != nil {
		t.Fatalf("ReadAll(stderr pipe) error = %v", readErr)
	}
	return string(output)
}

func resetDebugLogFallbackState() {
	debugLogFallbackMu.Lock()
	debugLogFallbackLogged = false
	debugLogFallbackMessageCount = 0
	debugLogFallbackMu.Unlock()
	pruneCountByDirMu.Lock()
	pruneCountByDir = map[string]int{}
	pruneCountByDirMu.Unlock()
}

func prepareDebugLogFallbackState(t *testing.T) {
	t.Helper()
	resetDebugLogFallbackState()
	t.Cleanup(resetDebugLogFallbackState)
}

func applyShellTransformSafeWith(req *ipc.TmuxRequest, transform func(*ipc.TmuxRequest) bool) (changed bool, err error) {
	if transform == nil {
		return false, errors.New("shell transform function is nil")
	}
	return runTransformSafe("shell", req, func() (bool, error) {
		return transform(req), nil
	})
}

func applyModelTransformSafeWith(req *ipc.TmuxRequest, load modelConfigLoader, transform func(*ipc.TmuxRequest, modelConfigLoader) (bool, error)) (changed bool, err error) {
	if transform == nil {
		return false, errors.New("model transform function is nil")
	}
	return runTransformSafe("model", req, func() (bool, error) {
		return transform(req, load)
	})
}

func TestParseCommandSplitWindow(t *testing.T) {
	req, err := parseCommand([]string{
		"split-window",
		"-h",
		"-t",
		"demo:0.0",
		"-e",
		"CLAUDE_CODE_AGENT_ID=researcher-1",
		"claude",
		"--agent-mode",
	})
	if err != nil {
		t.Fatalf("parseCommand() error = %v", err)
	}
	if req.Command != "split-window" {
		t.Fatalf("command mismatch: %s", req.Command)
	}
	if v, ok := req.Flags["-h"].(bool); !ok || !v {
		t.Fatalf("-h flag not parsed: %#v", req.Flags["-h"])
	}
	if target, ok := req.Flags["-t"].(string); !ok || target != "demo:0.0" {
		t.Fatalf("-t flag mismatch: %#v", req.Flags["-t"])
	}
	if req.Env["CLAUDE_CODE_AGENT_ID"] != "researcher-1" {
		t.Fatalf("env mismatch: %#v", req.Env)
	}
	if len(req.Args) != 2 || req.Args[0] != "claude" || req.Args[1] != "--agent-mode" {
		t.Fatalf("args mismatch: %#v", req.Args)
	}
}

func TestParseCommandRequiresTarget(t *testing.T) {
	_, err := parseCommand([]string{"has-session"})
	if err == nil {
		t.Fatal("expected error for missing -t")
	}
}

func TestParseCommandCombinedBoolFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantFlags map[string]bool
		wantErr   bool
	}{
		{
			name: "split-window -dPh combined",
			args: []string{"split-window", "-dPh", "-t", "demo:0"},
			wantFlags: map[string]bool{
				"-d": true,
				"-P": true,
				"-h": true,
			},
		},
		{
			name: "split-window -dP separate -F",
			args: []string{"split-window", "-dP", "-F", "#{pane_id}", "-t", "%0"},
			wantFlags: map[string]bool{
				"-d": true,
				"-P": true,
			},
		},
		{
			name:    "invalid combined flag with non-bool",
			args:    []string{"split-window", "-dF"},
			wantErr: true,
		},
		{
			name:    "unknown combined flag char",
			args:    []string{"split-window", "-dZ"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := parseCommand(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for flag, want := range tt.wantFlags {
				got, ok := req.Flags[flag].(bool)
				if !ok || got != want {
					t.Errorf("flag %s = %v, want %v", flag, req.Flags[flag], want)
				}
			}
		})
	}
}

func TestParseCommandSplitWindowWithDPF(t *testing.T) {
	// Claude Code Agent Teams pattern:
	// tmux split-window -dP -F '#{pane_id}' -h -t %0 -e KEY=VAL -- claude --resume abc
	req, err := parseCommand([]string{
		"split-window",
		"-dP",
		"-F", "#{pane_id}",
		"-h",
		"-t", "%0",
		"-e", "CLAUDE_CODE_AGENT_TYPE=teammate",
		"--",
		"claude", "--resume", "abc-123",
	})
	if err != nil {
		t.Fatalf("parseCommand() error = %v", err)
	}
	if !asBool(req.Flags["-d"]) {
		t.Error("-d flag not set")
	}
	if !asBool(req.Flags["-P"]) {
		t.Error("-P flag not set")
	}
	if asString(req.Flags["-F"]) != "#{pane_id}" {
		t.Errorf("-F = %q, want #{pane_id}", asString(req.Flags["-F"]))
	}
	if !asBool(req.Flags["-h"]) {
		t.Error("-h flag not set")
	}
	if asString(req.Flags["-t"]) != "%0" {
		t.Errorf("-t = %q, want %%0", asString(req.Flags["-t"]))
	}
	if req.Env["CLAUDE_CODE_AGENT_TYPE"] != "teammate" {
		t.Errorf("env = %v", req.Env)
	}
	if len(req.Args) != 3 || req.Args[0] != "claude" || req.Args[2] != "abc-123" {
		t.Errorf("args = %v", req.Args)
	}
}

func TestParseCommandSplitWindowSizeFlag(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantL   string
		wantErr bool
	}{
		{
			name:  "integer value",
			args:  []string{"split-window", "-l", "30", "-t", "%0"},
			wantL: "30",
		},
		{
			name:  "percentage value",
			args:  []string{"split-window", "-l", "70%", "-t", "%0"},
			wantL: "70%",
		},
		{
			name:  "percentage with combined flags",
			args:  []string{"split-window", "-dPh", "-l", "50%", "-t", "%0"},
			wantL: "50%",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := parseCommand(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := asString(req.Flags["-l"]); got != tt.wantL {
				t.Errorf("-l = %q, want %q", got, tt.wantL)
			}
		})
	}
}

func TestParseCommandNewSessionWithPF(t *testing.T) {
	req, err := parseCommand([]string{
		"new-session",
		"-dP",
		"-F", "#{session_name}:#{pane_id}",
		"-s", "claude-swarm",
		"-c", "/tmp/work",
	})
	if err != nil {
		t.Fatalf("parseCommand() error = %v", err)
	}
	if !asBool(req.Flags["-d"]) {
		t.Error("-d flag not set")
	}
	if !asBool(req.Flags["-P"]) {
		t.Error("-P flag not set")
	}
	if asString(req.Flags["-F"]) != "#{session_name}:#{pane_id}" {
		t.Errorf("-F = %q", asString(req.Flags["-F"]))
	}
	if asString(req.Flags["-s"]) != "claude-swarm" {
		t.Errorf("-s = %q", asString(req.Flags["-s"]))
	}
}

func TestParseCommandKillSessionRequiresTarget(t *testing.T) {
	if _, err := parseCommand([]string{"kill-session"}); err == nil {
		t.Fatal("parseCommand(kill-session) expected missing -t error")
	}

	req, err := parseCommand([]string{"kill-session", "-t", "demo"})
	if err != nil {
		t.Fatalf("parseCommand(kill-session -t demo) error = %v", err)
	}
	if req.Command != "kill-session" {
		t.Fatalf("command = %q, want kill-session", req.Command)
	}
	if asString(req.Flags["-t"]) != "demo" {
		t.Fatalf("-t = %q, want demo", asString(req.Flags["-t"]))
	}
}

func TestParseCommandSelectPaneDirectionalFlags(t *testing.T) {
	req, err := parseCommand([]string{"select-pane", "-L"})
	if err != nil {
		t.Fatalf("parseCommand(select-pane -L) error = %v", err)
	}
	if !asBool(req.Flags["-L"]) {
		t.Fatalf("-L flag = %v, want true", req.Flags["-L"])
	}
}

func TestParseCommandListPanesWithSessionScope(t *testing.T) {
	req, err := parseCommand([]string{"list-panes", "-s", "-t", "demo:0", "-F", "#{pane_id}"})
	if err != nil {
		t.Fatalf("parseCommand(list-panes) error = %v", err)
	}
	if !asBool(req.Flags["-s"]) {
		t.Fatalf("-s flag = %v, want true", req.Flags["-s"])
	}
	if asString(req.Flags["-t"]) != "demo:0" {
		t.Fatalf("-t = %q, want demo:0", asString(req.Flags["-t"]))
	}
	if asString(req.Flags["-F"]) != "#{pane_id}" {
		t.Fatalf("-F = %q, want #{pane_id}", asString(req.Flags["-F"]))
	}
}

func TestParseCommandDisplayMessageRequiresPrintFlag(t *testing.T) {
	if _, err := parseCommand([]string{"display-message"}); err == nil {
		t.Fatal("parseCommand(display-message) expected missing -p error")
	}

	req, err := parseCommand([]string{"display-message", "-p", "#{pane_id}"})
	if err != nil {
		t.Fatalf("parseCommand(display-message -p) error = %v", err)
	}
	if !asBool(req.Flags["-p"]) {
		t.Fatalf("-p flag = %v, want true", req.Flags["-p"])
	}
	if len(req.Args) != 1 || req.Args[0] != "#{pane_id}" {
		t.Fatalf("args = %#v, want [#{pane_id}]", req.Args)
	}
}

func TestParseCommandFlagIntRejectsNonInteger(t *testing.T) {
	_, err := parseCommand([]string{"new-session", "-x", "not-int"})
	if err == nil {
		t.Fatal("parseCommand(new-session -x not-int) expected integer validation error")
	}
	if !strings.Contains(err.Error(), "expects integer") {
		t.Fatalf("error = %v, want integer validation message", err)
	}
}

func TestNextRotatedShimDebugLogPathIncrementsOnCollision(t *testing.T) {
	logDir := t.TempDir()
	startUnix := int64(1700000000)

	collided0 := filepath.Join(logDir, "shim-debug-1700000000.log")
	collided1 := filepath.Join(logDir, "shim-debug-1700000001.log")
	if err := os.WriteFile(collided0, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to create collision file 0: %v", err)
	}
	if err := os.WriteFile(collided1, []byte("x"), 0o644); err != nil {
		t.Fatalf("failed to create collision file 1: %v", err)
	}

	nextPath, err := nextRotatedShimDebugLogPath(logDir, startUnix)
	if err != nil {
		t.Fatalf("nextRotatedShimDebugLogPath() error = %v", err)
	}
	want := filepath.Join(logDir, "shim-debug-1700000002.log")
	if nextPath != want {
		t.Fatalf("next path = %q, want %q", nextPath, want)
	}
}

func TestRotateShimDebugLogIfNeededScenarios(t *testing.T) {
	originalRename := renameFileFn
	originalRemove := removeFileFn
	t.Cleanup(func() {
		renameFileFn = originalRename
		removeFileFn = originalRemove
	})

	tests := []struct {
		name          string
		unixTime      int64
		basePayload   []byte
		wantBase      bool
		wantRotatedAt int64
	}{
		{
			name:          "rotates at size limit",
			unixTime:      1700000100,
			basePayload:   bytes.Repeat([]byte("a"), shimDebugLogMaxBytes),
			wantBase:      false,
			wantRotatedAt: 1700000100,
		},
		{
			name:          "no-op below size limit",
			unixTime:      1700000200,
			basePayload:   bytes.Repeat([]byte("a"), shimDebugLogMaxBytes-1),
			wantBase:      true,
			wantRotatedAt: 0,
		},
		{
			name:          "rotates above size limit",
			unixTime:      1700000250,
			basePayload:   bytes.Repeat([]byte("a"), shimDebugLogMaxBytes+1),
			wantBase:      false,
			wantRotatedAt: 1700000250,
		},
		{
			name:          "no-op when base file missing",
			unixTime:      1700000300,
			basePayload:   nil,
			wantBase:      false,
			wantRotatedAt: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logDir := t.TempDir()
			basePath := filepath.Join(logDir, shimDebugLogFileName)
			if tt.basePayload != nil {
				if err := os.WriteFile(basePath, tt.basePayload, 0o644); err != nil {
					t.Fatalf("failed to create base log: %v", err)
				}
			}

			if err := rotateShimDebugLogIfNeeded(basePath, shimDebugLogMaxBytes, tt.unixTime); err != nil {
				t.Fatalf("rotateShimDebugLogIfNeeded() error = %v", err)
			}

			_, baseErr := os.Stat(basePath)
			if tt.wantBase {
				if baseErr != nil {
					t.Fatalf("base log should remain, stat err = %v", baseErr)
				}
			} else if !os.IsNotExist(baseErr) {
				t.Fatalf("base log should be absent, stat err = %v", baseErr)
			}

			rotatedPath := filepath.Join(logDir, fmt.Sprintf("shim-debug-%d.log", tt.unixTime))
			_, rotatedErr := os.Stat(rotatedPath)
			if tt.wantRotatedAt > 0 {
				if rotatedErr != nil {
					t.Fatalf("rotated log missing: %v", rotatedErr)
				}
			} else if !os.IsNotExist(rotatedErr) {
				t.Fatalf("rotated log should not exist, stat err = %v", rotatedErr)
			}
		})
	}
}

func TestRotateShimDebugLogIfNeededRetriesOnRenameCollision(t *testing.T) {
	originalRename := renameFileFn
	t.Cleanup(func() {
		renameFileFn = originalRename
	})

	logDir := t.TempDir()
	basePath := filepath.Join(logDir, shimDebugLogFileName)
	if err := os.WriteFile(basePath, bytes.Repeat([]byte("a"), shimDebugLogMaxBytes), 0o644); err != nil {
		t.Fatalf("failed to create base log: %v", err)
	}

	renameCalls := 0
	renameFileFn = func(oldPath, newPath string) error {
		renameCalls++
		if renameCalls < 3 {
			return os.ErrExist
		}
		return os.Rename(oldPath, newPath)
	}

	const unixTime = int64(1700002100)
	if err := rotateShimDebugLogIfNeeded(basePath, shimDebugLogMaxBytes, unixTime); err != nil {
		t.Fatalf("rotateShimDebugLogIfNeeded() error = %v", err)
	}
	if renameCalls != 3 {
		t.Fatalf("rename call count = %d, want 3", renameCalls)
	}

	wantRotated := filepath.Join(logDir, "shim-debug-1700002102.log")
	if _, err := os.Stat(wantRotated); err != nil {
		t.Fatalf("expected rotated log %q, stat err = %v", wantRotated, err)
	}
}

func TestRotateShimDebugLogIfNeededFailsAfterMaxRenameRetries(t *testing.T) {
	originalRename := renameFileFn
	t.Cleanup(func() {
		renameFileFn = originalRename
	})

	logDir := t.TempDir()
	basePath := filepath.Join(logDir, shimDebugLogFileName)
	if err := os.WriteFile(basePath, bytes.Repeat([]byte("a"), shimDebugLogMaxBytes), 0o644); err != nil {
		t.Fatalf("failed to create base log: %v", err)
	}

	renameCalls := 0
	renameFileFn = func(_, _ string) error {
		renameCalls++
		return os.ErrExist
	}

	err := rotateShimDebugLogIfNeeded(basePath, shimDebugLogMaxBytes, 1700002150)
	if err == nil {
		t.Fatal("rotateShimDebugLogIfNeeded() expected retry exhaustion error")
	}
	if renameCalls != 4 {
		t.Fatalf("rename call count = %d, want 4", renameCalls)
	}
}

func TestRotateShimDebugLogIfNeededPrunesOldGenerations(t *testing.T) {
	logDir := t.TempDir()
	basePath := filepath.Join(logDir, shimDebugLogFileName)
	payload := bytes.Repeat([]byte("a"), shimDebugLogMaxBytes)
	if err := os.WriteFile(basePath, payload, 0o644); err != nil {
		t.Fatalf("failed to create base log: %v", err)
	}

	for ts := int64(1700001000); ts < 1700001048; ts++ {
		path := filepath.Join(logDir, fmt.Sprintf("shim-debug-%d.log", ts))
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatalf("failed to create rotated log %s: %v", path, err)
		}
	}

	if err := rotateShimDebugLogIfNeeded(basePath, shimDebugLogMaxBytes, 1700002000); err != nil {
		t.Fatalf("rotateShimDebugLogIfNeeded() error = %v", err)
	}

	rotated, err := filepath.Glob(filepath.Join(logDir, "shim-debug-*.log"))
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(rotated) != shimDebugLogKeepGenerations {
		t.Fatalf("rotated log count = %d, want %d", len(rotated), shimDebugLogKeepGenerations)
	}

	newest := filepath.Join(logDir, "shim-debug-1700002000.log")
	if _, statErr := os.Stat(newest); statErr != nil {
		t.Fatalf("newest rotated log missing: %v", statErr)
	}
}

func TestPruneRotatedShimDebugLogsContinuesAfterRemoveError(t *testing.T) {
	originalRemove := removeFileFn
	t.Cleanup(func() {
		removeFileFn = originalRemove
	})

	logDir := t.TempDir()
	log1 := filepath.Join(logDir, "shim-debug-1.log")
	log2 := filepath.Join(logDir, "shim-debug-2.log")
	log3 := filepath.Join(logDir, "shim-debug-3.log")
	for _, path := range []string{log1, log2, log3} {
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatalf("failed to create rotated log %s: %v", path, err)
		}
	}

	var removed []string
	removeFileFn = func(path string) error {
		removed = append(removed, filepath.Base(path))
		if strings.HasSuffix(path, "shim-debug-2.log") {
			return errors.New("simulated remove failure")
		}
		return os.Remove(path)
	}

	err := pruneRotatedShimDebugLogs(logDir, 1)
	if err == nil {
		t.Fatal("pruneRotatedShimDebugLogs() expected aggregated remove error")
	}
	if len(removed) != 2 {
		t.Fatalf("remove calls = %v, want 2 files", removed)
	}

	if _, statErr := os.Stat(log2); os.IsNotExist(statErr) {
		t.Fatalf("failed file should remain: %s", log2)
	}
	if _, statErr := os.Stat(log1); !os.IsNotExist(statErr) {
		t.Fatalf("other old file should still be pruned, stat err = %v", statErr)
	}
}

func TestPruneRotatedShimDebugLogsNoopWhenKeepIsNonPositive(t *testing.T) {
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "shim-debug-1.log")
	if err := os.WriteFile(logPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("failed to create rotated log: %v", err)
	}

	if err := pruneRotatedShimDebugLogs(logDir, 0); err != nil {
		t.Fatalf("pruneRotatedShimDebugLogs() error = %v", err)
	}
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("rotated log should remain for keep<=0: %v", err)
	}
}

func TestShouldPruneRotatedShimDebugLogsSkipsBelowLimit(t *testing.T) {
	prepareDebugLogFallbackState(t)

	logDir := t.TempDir()
	rotatedPath := filepath.Join(logDir, "shim-debug-1700001001.log")
	if err := os.WriteFile(rotatedPath, []byte("new"), 0o644); err != nil {
		t.Fatalf("failed to create rotated log: %v", err)
	}

	shouldPrune := shouldPruneRotatedShimDebugLogs(logDir, 32)
	if shouldPrune {
		t.Fatal("shouldPruneRotatedShimDebugLogs() = true, want false below keep limit")
	}
}

func TestShouldPruneRotatedShimDebugLogsUsesCachedCountPerDirectory(t *testing.T) {
	prepareDebugLogFallbackState(t)

	logDir := t.TempDir()
	path1 := filepath.Join(logDir, "shim-debug-1700001001.log")
	path2 := filepath.Join(logDir, "shim-debug-1700001002.log")
	path3 := filepath.Join(logDir, "shim-debug-1700001003.log")

	if err := os.WriteFile(path1, []byte("new"), 0o644); err != nil {
		t.Fatalf("failed to create rotated log %s: %v", path1, err)
	}

	if shouldPruneRotatedShimDebugLogs(logDir, 2) {
		t.Fatal("first check should not prune at keep limit")
	}
	if err := os.WriteFile(path2, []byte("new"), 0o644); err != nil {
		t.Fatalf("failed to create rotated log %s: %v", path2, err)
	}
	if shouldPruneRotatedShimDebugLogs(logDir, 2) {
		t.Fatal("second check should not prune at keep limit")
	}
	if err := os.WriteFile(path3, []byte("new"), 0o644); err != nil {
		t.Fatalf("failed to create rotated log %s: %v", path3, err)
	}
	if !shouldPruneRotatedShimDebugLogs(logDir, 2) {
		t.Fatal("third check should prune when cached count exceeds keep")
	}
}

func TestNextRotatedShimDebugLogPathFailsWhenAttemptsExhausted(t *testing.T) {
	logDir := t.TempDir()
	startUnix := int64(1700003000)
	for ts := startUnix; ts < startUnix+64; ts++ {
		path := filepath.Join(logDir, fmt.Sprintf("shim-debug-%d.log", ts))
		if err := os.WriteFile(path, []byte("occupied"), 0o644); err != nil {
			t.Fatalf("failed to create occupied path %s: %v", path, err)
		}
	}

	if _, err := nextRotatedShimDebugLogPath(logDir, startUnix); err == nil {
		t.Fatal("nextRotatedShimDebugLogPath() expected exhaustion error")
	}
}

func TestNextRotatedShimDebugLogPathReturnsErrorForInvalidLogDir(t *testing.T) {
	if _, err := nextRotatedShimDebugLogPath(string([]byte{0}), 1700004000); err == nil {
		t.Fatal("nextRotatedShimDebugLogPath() expected stat error")
	}
}

func TestParseRotatedShimDebugLogUnix(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantOK    bool
		wantValue int64
	}{
		{
			name:      "valid filename",
			path:      "shim-debug-1700000123.log",
			wantOK:    true,
			wantValue: 1700000123,
		},
		{
			name:      "valid path with directory",
			path:      filepath.Join("C:\\logs", "shim-debug-1700000456.log"),
			wantOK:    true,
			wantValue: 1700000456,
		},
		{
			name:   "invalid prefix",
			path:   "debug-1700000123.log",
			wantOK: false,
		},
		{
			name:   "invalid suffix",
			path:   "shim-debug-1700000123.txt",
			wantOK: false,
		},
		{
			name:   "missing timestamp",
			path:   "shim-debug-.log",
			wantOK: false,
		},
		{
			name:   "non numeric timestamp",
			path:   "shim-debug-abc.log",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, gotOK := parseRotatedShimDebugLogUnix(tt.path)
			if gotOK != tt.wantOK {
				t.Fatalf("parseRotatedShimDebugLogUnix(%q) ok = %v, want %v", tt.path, gotOK, tt.wantOK)
			}
			if gotValue != tt.wantValue {
				t.Fatalf("parseRotatedShimDebugLogUnix(%q) value = %d, want %d", tt.path, gotValue, tt.wantValue)
			}
		})
	}
}

func TestDebugLogFallbackIncludesOriginalMessage(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")
	prepareDebugLogFallbackState(t)

	output := captureStderr(t, func() {
		debugLog("fallback message %s", "body")
	})

	if !strings.Contains(output, "logging unavailable") {
		t.Fatalf("stderr output = %q, want fallback reason", output)
	}
	if !strings.Contains(output, "fallback message body") {
		t.Fatalf("stderr output = %q, want original message", output)
	}
}

func TestDebugLogFallbackMessageEmitsOnlyFirstNMessages(t *testing.T) {
	prepareDebugLogFallbackState(t)
	output := captureStderr(t, func() {
		debugLogFallbackMessage("first fallback message")
		debugLogFallbackMessage("second fallback message")
		debugLogFallbackMessage("third fallback message")
		debugLogFallbackMessage("fourth fallback message")
	})

	if !strings.Contains(output, "first fallback message") {
		t.Fatalf("stderr output = %q, want first fallback message", output)
	}
	if !strings.Contains(output, "second fallback message") {
		t.Fatalf("stderr output = %q, want second fallback message", output)
	}
	if !strings.Contains(output, "third fallback message") {
		t.Fatalf("stderr output = %q, want third fallback message", output)
	}
	if strings.Contains(output, "fourth fallback message") {
		t.Fatalf("stderr output should suppress messages after first %d entries, got %q", debugLogFallbackMaxMessages, output)
	}
}

func TestFlushDebugLogFallbackSummaryNoopWithoutSuppressedMessages(t *testing.T) {
	prepareDebugLogFallbackState(t)
	output := captureStderr(t, func() {
		flushDebugLogFallbackSummary()
	})
	if output != "" {
		t.Fatalf("stderr output = %q, want empty when no suppressed messages", output)
	}
}

func TestDebugLogFallbackMessageIgnoresWhitespaceInput(t *testing.T) {
	prepareDebugLogFallbackState(t)
	output := captureStderr(t, func() {
		debugLogFallbackMessage("   \n\t")
	})
	if output != "" {
		t.Fatalf("stderr output = %q, want empty for whitespace-only input", output)
	}
}

func TestApplyModelTransformSafeOnLoaderError(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "split-window",
		Args:    []string{"--model claude-opus-4-6"},
	}
	before := append([]string(nil), req.Args...)

	changed, err := applyModelTransformSafeWith(&req, func() (*config.AgentModel, error) {
		return nil, errors.New("load failed")
	}, applyModelTransform)
	if err == nil {
		t.Fatal("expected error")
	}
	if changed {
		t.Fatal("changed should be false when loader fails")
	}
	if req.Args[0] != before[0] {
		t.Fatalf("args changed on loader error: got %q, want %q", req.Args[0], before[0])
	}
}

func TestApplyModelTransformSafeWithUsesLoader(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "split-window",
		Args:    []string{"claude --model claude-opus-4-6 --agent-name reviewer"},
	}

	changed, err := applyModelTransformSafeWith(&req, func() (*config.AgentModel, error) {
		return &config.AgentModel{
			From: "claude-opus-4-6",
			To:   "claude-sonnet-4-5",
		}, nil
	}, applyModelTransform)
	if err != nil {
		t.Fatalf("applyModelTransformSafe() error = %v", err)
	}
	if !changed {
		t.Fatal("changed should be true when model replacement is applied")
	}
	if !strings.Contains(req.Args[0], "claude-sonnet-4-5") {
		t.Fatalf("args[0] = %q, want replacement model", req.Args[0])
	}
}

func TestApplyModelTransformSafeOnPanic(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "split-window",
		Args:    []string{"--model claude-opus-4-6"},
	}
	before := append([]string(nil), req.Args...)

	changed, err := applyModelTransformSafeWith(&req, func() (*config.AgentModel, error) {
		panic("boom")
	}, applyModelTransform)
	if err == nil {
		t.Fatal("expected panic to be converted to error")
	}
	if changed {
		t.Fatal("changed should be false when panic occurs")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error should include recovered value, got: %v", err)
	}
	if req.Args[0] != before[0] {
		t.Fatalf("args changed on panic: got %q, want %q", req.Args[0], before[0])
	}
}

func TestApplyModelTransformSafeWithRestoresRequestOnErrorAfterPartialMutation(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "split-window",
		Flags:   map[string]any{"-t": "before-target"},
		Env:     map[string]string{"MODE": "before"},
		Args:    []string{"--model", "before"},
	}
	before := req
	before.Flags = map[string]any{"-t": "before-target"}
	before.Env = map[string]string{"MODE": "before"}
	before.Args = []string{"--model", "before"}

	changed, err := applyModelTransformSafeWith(&req, nil, func(r *ipc.TmuxRequest, _ modelConfigLoader) (bool, error) {
		r.Args[1] = "after"
		r.Env["MODE"] = "after"
		return true, errors.New("transform failed after mutation")
	})
	if err == nil {
		t.Fatal("expected transform error")
	}
	if changed {
		t.Fatal("changed should be false when transform returns error")
	}
	if req.Args[1] != before.Args[1] {
		t.Fatalf("args[1] = %q, want %q", req.Args[1], before.Args[1])
	}
	if req.Env["MODE"] != before.Env["MODE"] {
		t.Fatalf("env MODE = %q, want %q", req.Env["MODE"], before.Env["MODE"])
	}
	if asString(req.Flags["-t"]) != asString(before.Flags["-t"]) {
		t.Fatalf("flag -t = %q, want %q", asString(req.Flags["-t"]), asString(before.Flags["-t"]))
	}
}

func TestApplyShellTransformSafeOnPanic(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "split-window",
		Args:    []string{"pwsh -NoLogo"},
	}
	before := append([]string(nil), req.Args...)

	changed, err := applyShellTransformSafeWith(&req, func(*ipc.TmuxRequest) bool {
		panic("boom")
	})
	if err == nil {
		t.Fatal("expected panic to be converted to error")
	}
	if changed {
		t.Fatal("changed should be false when panic occurs")
	}
	if !strings.Contains(err.Error(), "panic") {
		t.Fatalf("error should mention panic, got: %v", err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error should include recovered value, got: %v", err)
	}
	if req.Args[0] != before[0] {
		t.Fatalf("args changed on panic: got %q, want %q", req.Args[0], before[0])
	}
}

func TestApplyShellTransformSafeWithNilTransform(t *testing.T) {
	req := ipc.TmuxRequest{Command: "split-window"}

	changed, err := applyShellTransformSafeWith(&req, nil)
	if err == nil {
		t.Fatal("expected error for nil transform")
	}
	if changed {
		t.Fatal("changed should be false for nil transform")
	}
}

func TestApplyModelTransformSafeWithNilTransform(t *testing.T) {
	req := ipc.TmuxRequest{Command: "split-window"}

	changed, err := applyModelTransformSafeWith(&req, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil transform")
	}
	if changed {
		t.Fatal("changed should be false for nil transform")
	}
}

func TestApplyShellTransformSafeWithNilRequest(t *testing.T) {
	changed, err := applyShellTransformSafeWith(nil, func(*ipc.TmuxRequest) bool {
		return true
	})
	if err == nil {
		t.Fatal("expected error for nil request")
	}
	if changed {
		t.Fatal("changed should be false for nil request")
	}
	if !strings.Contains(err.Error(), "tmux request is nil") {
		t.Fatalf("error should mention nil request, got: %v", err)
	}
}

func TestApplyModelTransformSafeWithNilRequest(t *testing.T) {
	changed, err := applyModelTransformSafeWith(nil, nil, applyModelTransform)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
	if changed {
		t.Fatal("changed should be false for nil request")
	}
	if !strings.Contains(err.Error(), "tmux request is nil") {
		t.Fatalf("error should mention nil request, got: %v", err)
	}
}

func TestApplyShellTransformSafeWithDelegatesChangedState(t *testing.T) {
	t.Run("changed true", func(t *testing.T) {
		req := ipc.TmuxRequest{Command: "split-window", Args: []string{"initial"}}
		changed, err := applyShellTransformSafeWith(&req, func(r *ipc.TmuxRequest) bool {
			r.Args[0] = "updated"
			return true
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Fatal("changed should be true")
		}
		if req.Args[0] != "updated" {
			t.Fatalf("args[0] = %q, want updated", req.Args[0])
		}
	})

	t.Run("changed false", func(t *testing.T) {
		req := ipc.TmuxRequest{Command: "split-window", Args: []string{"initial"}}
		changed, err := applyShellTransformSafeWith(&req, func(*ipc.TmuxRequest) bool {
			return false
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if changed {
			t.Fatal("changed should be false")
		}
		if req.Args[0] != "initial" {
			t.Fatalf("args[0] = %q, want initial", req.Args[0])
		}
	})
}

func TestApplyModelTransformSafeWithDelegatesChangedState(t *testing.T) {
	t.Run("changed true", func(t *testing.T) {
		req := ipc.TmuxRequest{Command: "split-window", Args: []string{"--model", "before"}}
		changed, err := applyModelTransformSafeWith(&req, nil, func(r *ipc.TmuxRequest, _ modelConfigLoader) (bool, error) {
			r.Args[1] = "after"
			return true, nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !changed {
			t.Fatal("changed should be true")
		}
		if req.Args[1] != "after" {
			t.Fatalf("args[1] = %q, want after", req.Args[1])
		}
	})

	t.Run("changed false", func(t *testing.T) {
		req := ipc.TmuxRequest{Command: "split-window", Args: []string{"--model", "before"}}
		changed, err := applyModelTransformSafeWith(&req, nil, func(*ipc.TmuxRequest, modelConfigLoader) (bool, error) {
			return false, nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if changed {
			t.Fatal("changed should be false")
		}
		if req.Args[1] != "before" {
			t.Fatalf("args[1] = %q, want before", req.Args[1])
		}
	})
}

func TestApplyShellTransformSafeWithRestoresRequestOnPanicAfterPartialMutation(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "split-window",
		Flags:   map[string]any{"-t": "before-target"},
		Env:     map[string]string{"MODE": "before"},
		Args:    []string{"before"},
	}
	before := req
	before.Flags = map[string]any{"-t": "before-target"}
	before.Env = map[string]string{"MODE": "before"}
	before.Args = []string{"before"}

	changed, err := applyShellTransformSafeWith(&req, func(r *ipc.TmuxRequest) bool {
		r.Args[0] = "after"
		r.Env["MODE"] = "after"
		panic("shell exploded")
	})
	if err == nil {
		t.Fatal("expected panic to be converted to error")
	}
	if changed {
		t.Fatal("changed should be false when panic occurs")
	}
	if req.Args[0] != before.Args[0] {
		t.Fatalf("args[0] = %q, want %q", req.Args[0], before.Args[0])
	}
	if req.Env["MODE"] != before.Env["MODE"] {
		t.Fatalf("env MODE = %q, want %q", req.Env["MODE"], before.Env["MODE"])
	}
	if asString(req.Flags["-t"]) != asString(before.Flags["-t"]) {
		t.Fatalf("flag -t = %q, want %q", asString(req.Flags["-t"]), asString(before.Flags["-t"]))
	}
}

func TestApplyModelTransformSafeWithRestoresRequestOnPanicAfterPartialMutation(t *testing.T) {
	req := ipc.TmuxRequest{
		Command: "split-window",
		Flags:   map[string]any{"-t": "before-target"},
		Env:     map[string]string{"MODE": "before"},
		Args:    []string{"--model", "before"},
	}
	before := req
	before.Flags = map[string]any{"-t": "before-target"}
	before.Env = map[string]string{"MODE": "before"}
	before.Args = []string{"--model", "before"}

	changed, err := applyModelTransformSafeWith(&req, nil, func(r *ipc.TmuxRequest, _ modelConfigLoader) (bool, error) {
		r.Args[1] = "after"
		r.Env["MODE"] = "after"
		panic("model exploded")
	})
	if err == nil {
		t.Fatal("expected panic to be converted to error")
	}
	if changed {
		t.Fatal("changed should be false when panic occurs")
	}
	if req.Args[1] != before.Args[1] {
		t.Fatalf("args[1] = %q, want %q", req.Args[1], before.Args[1])
	}
	if req.Env["MODE"] != before.Env["MODE"] {
		t.Fatalf("env MODE = %q, want %q", req.Env["MODE"], before.Env["MODE"])
	}
	if asString(req.Flags["-t"]) != asString(before.Flags["-t"]) {
		t.Fatalf("flag -t = %q, want %q", asString(req.Flags["-t"]), asString(before.Flags["-t"]))
	}
}

func TestCloneTransformRequestCreatesIndependentCopies(t *testing.T) {
	original := &ipc.TmuxRequest{
		Command: "split-window",
		Flags: map[string]any{
			"-t": "demo:0.0",
			"-h": true,
		},
		Args: []string{"claude", "--resume", "123"},
		Env: map[string]string{
			"MODE": "before",
		},
		CallerPane: "%1",
	}

	cloned := cloneTransformRequest(original)
	cloned.Flags["-t"] = "demo:0.1"
	cloned.Args[1] = "--model"
	cloned.Env["MODE"] = "after"
	cloned.CallerPane = "%2"

	if asString(original.Flags["-t"]) != "demo:0.0" {
		t.Fatalf("original flags were mutated: got %v", original.Flags)
	}
	if original.Args[1] != "--resume" {
		t.Fatalf("original args were mutated: got %v", original.Args)
	}
	if original.Env["MODE"] != "before" {
		t.Fatalf("original env was mutated: got %v", original.Env)
	}
	if original.CallerPane != "%1" {
		t.Fatalf("original caller pane was mutated: got %q", original.CallerPane)
	}
}

func TestCloneTransformRequestPreservesNilCollections(t *testing.T) {
	original := &ipc.TmuxRequest{Command: "list-sessions"}
	cloned := cloneTransformRequest(original)

	if cloned.Flags != nil {
		t.Fatalf("Flags should remain nil, got: %#v", cloned.Flags)
	}
	if cloned.Env != nil {
		t.Fatalf("Env should remain nil, got: %#v", cloned.Env)
	}
	if cloned.Args != nil {
		t.Fatalf("Args should remain nil, got: %#v", cloned.Args)
	}
}

func TestCloneTransformRequestNilInputReturnsZeroValue(t *testing.T) {
	cloned := cloneTransformRequest(nil)

	if cloned.Command != "" {
		t.Fatalf("Command = %q, want empty", cloned.Command)
	}
	if cloned.Flags != nil {
		t.Fatalf("Flags should be nil, got: %#v", cloned.Flags)
	}
	if cloned.Env != nil {
		t.Fatalf("Env should be nil, got: %#v", cloned.Env)
	}
	if cloned.Args != nil {
		t.Fatalf("Args should be nil, got: %#v", cloned.Args)
	}
	if cloned.CallerPane != "" {
		t.Fatalf("CallerPane = %q, want empty", cloned.CallerPane)
	}
}

func TestParseCommandAdditionalCoverage(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantCmd   string
		wantFlags map[string]any
		wantArgs  []string
		wantErr   bool
	}{
		// kill-session: session:window target
		{
			name:      "kill-session with session:window target",
			args:      []string{"kill-session", "-t", "main:0"},
			wantCmd:   "kill-session",
			wantFlags: map[string]any{"-t": "main:0"},
		},
		// select-pane: all directional flags
		{
			name:      "select-pane -R",
			args:      []string{"select-pane", "-R"},
			wantCmd:   "select-pane",
			wantFlags: map[string]any{"-R": true},
		},
		{
			name:      "select-pane -U",
			args:      []string{"select-pane", "-U"},
			wantCmd:   "select-pane",
			wantFlags: map[string]any{"-U": true},
		},
		{
			name:      "select-pane -D",
			args:      []string{"select-pane", "-D"},
			wantCmd:   "select-pane",
			wantFlags: map[string]any{"-D": true},
		},
		{
			name:      "select-pane with -t target",
			args:      []string{"select-pane", "-t", "%1"},
			wantCmd:   "select-pane",
			wantFlags: map[string]any{"-t": "%1"},
		},
		// list-panes: without -s flag
		{
			name:      "list-panes without -s",
			args:      []string{"list-panes", "-t", "demo:0", "-F", "#{pane_id}"},
			wantCmd:   "list-panes",
			wantFlags: map[string]any{"-t": "demo:0", "-F": "#{pane_id}"},
		},
		// display-message: with -t and format args
		{
			name:      "display-message with -t and format",
			args:      []string{"display-message", "-p", "-t", "%0", "#{session_name}"},
			wantCmd:   "display-message",
			wantFlags: map[string]any{"-p": true, "-t": "%0"},
			wantArgs:  []string{"#{session_name}"},
		},
		// flagInt: valid integers
		{
			name:      "new-session with valid -x integer",
			args:      []string{"new-session", "-s", "test", "-x", "120"},
			wantCmd:   "new-session",
			wantFlags: map[string]any{"-s": "test", "-x": 120},
		},
		{
			name:      "new-session with valid -y integer",
			args:      []string{"new-session", "-s", "test", "-y", "40"},
			wantCmd:   "new-session",
			wantFlags: map[string]any{"-s": "test", "-y": 40},
		},
		// flagInt: error on non-integer -y
		{
			name:    "new-session -y non-integer",
			args:    []string{"new-session", "-y", "abc"},
			wantErr: true,
		},
		// send-keys: basic parsing
		{
			name:      "send-keys with target and args",
			args:      []string{"send-keys", "-t", "%0", "ls", "-la"},
			wantCmd:   "send-keys",
			wantFlags: map[string]any{"-t": "%0"},
			wantArgs:  []string{"ls", "-la"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := parseCommand(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.Command != tt.wantCmd {
				t.Fatalf("command = %q, want %q", req.Command, tt.wantCmd)
			}
			for flag, want := range tt.wantFlags {
				got := req.Flags[flag]
				if !reflect.DeepEqual(got, want) {
					t.Errorf("flag %s = %v (%T), want %v (%T)", flag, got, got, want, want)
				}
			}
			if tt.wantArgs != nil {
				if !reflect.DeepEqual(req.Args, tt.wantArgs) {
					t.Errorf("args = %v, want %v", req.Args, tt.wantArgs)
				}
			}
		})
	}
}

func TestTmuxRequestStructFieldCountForCloneTransformRequest(t *testing.T) {
	if got := reflect.TypeOf(ipc.TmuxRequest{}).NumField(); got != 5 {
		t.Fatalf("ipc.TmuxRequest field count = %d, want 5 (command, flags, args, env, caller_pane)", got)
	}
}
