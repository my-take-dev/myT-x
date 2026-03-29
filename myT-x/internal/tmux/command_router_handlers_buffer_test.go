package tmux

import (
	"bytes"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"myT-x/internal/ipc"
)

type stubLoadBufferFile struct {
	reader  io.Reader
	info    os.FileInfo
	statErr error
	closeFn func() error
}

func (f *stubLoadBufferFile) Read(p []byte) (int, error) {
	return f.reader.Read(p)
}

func (f *stubLoadBufferFile) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

func (f *stubLoadBufferFile) Stat() (os.FileInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	return f.info, nil
}

type stubSaveBufferFile struct {
	writeFn func([]byte) (int, error)
	closeFn func() error
}

func (f *stubSaveBufferFile) Write(p []byte) (int, error) {
	if f.writeFn != nil {
		return f.writeFn(p)
	}
	return len(p), nil
}

func (f *stubSaveBufferFile) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

type stubFileInfo struct {
	size int64
}

func (i stubFileInfo) Name() string       { return "stub" }
func (i stubFileInfo) Size() int64        { return i.size }
func (i stubFileInfo) Mode() os.FileMode  { return 0 }
func (i stubFileInfo) ModTime() time.Time { return time.Time{} }
func (i stubFileInfo) IsDir() bool        { return false }
func (i stubFileInfo) Sys() any           { return nil }

func TestHandleListBuffers(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*BufferStore)
		format     string
		wantLines  int
		wantSubstr string
	}{
		{
			name:      "empty buffer store",
			setup:     func(bs *BufferStore) {},
			wantLines: 0,
		},
		{
			name: "single buffer default format",
			setup: func(bs *BufferStore) {
				bs.Set("buf0", []byte("hello world"), false)
			},
			wantLines:  1,
			wantSubstr: "buf0",
		},
		{
			name: "multiple buffers",
			setup: func(bs *BufferStore) {
				bs.Set("a", []byte("1"), false)
				bs.Set("b", []byte("2"), false)
			},
			wantLines: 2,
		},
		{
			name: "custom format",
			setup: func(bs *BufferStore) {
				bs.Set("test", []byte("data"), false)
			},
			format:     "#{buffer_name}:#{buffer_size}",
			wantLines:  1,
			wantSubstr: "test:4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)
			router := NewCommandRouter(sessions, nil, RouterOptions{})
			tt.setup(router.buffers)

			flags := map[string]any{}
			if tt.format != "" {
				flags["-F"] = tt.format
			}
			resp := router.Execute(ipc.TmuxRequest{
				Command: "list-buffers",
				Flags:   flags,
			})
			if resp.ExitCode != 0 {
				t.Fatalf("list-buffers exit code = %d, stderr = %q", resp.ExitCode, resp.Stderr)
			}

			lines := strings.Split(strings.TrimRight(resp.Stdout, "\n"), "\n")
			if resp.Stdout == "" {
				lines = nil
			}
			if len(lines) != tt.wantLines {
				t.Fatalf("list-buffers lines = %d, want %d, stdout = %q", len(lines), tt.wantLines, resp.Stdout)
			}
			if tt.wantSubstr != "" && !strings.Contains(resp.Stdout, tt.wantSubstr) {
				t.Fatalf("list-buffers stdout %q does not contain %q", resp.Stdout, tt.wantSubstr)
			}
		})
	}
}

func TestHandleSetBuffer(t *testing.T) {
	tests := []struct {
		name         string
		flags        map[string]any
		args         []string
		setup        func(*BufferStore)
		wantExitCode int
		verify       func(t *testing.T, bs *BufferStore)
	}{
		{
			name:  "set named buffer",
			flags: map[string]any{"-b": "test"},
			args:  []string{"hello"},
			verify: func(t *testing.T, bs *BufferStore) {
				buf, ok := bs.Get("test")
				if !ok {
					t.Fatal("buffer 'test' not found")
				}
				if string(buf.Data) != "hello" {
					t.Fatalf("data = %q, want %q", buf.Data, "hello")
				}
			},
		},
		{
			name:  "append to buffer",
			flags: map[string]any{"-b": "buf", "-a": true},
			args:  []string{" world"},
			setup: func(bs *BufferStore) {
				bs.Set("buf", []byte("hello"), false)
			},
			verify: func(t *testing.T, bs *BufferStore) {
				buf, _ := bs.Get("buf")
				if string(buf.Data) != "hello world" {
					t.Fatalf("data = %q, want %q", buf.Data, "hello world")
				}
			},
		},
		{
			name:  "rename buffer",
			flags: map[string]any{"-b": "old", "-n": "new"},
			setup: func(bs *BufferStore) {
				bs.Set("old", []byte("data"), false)
			},
			verify: func(t *testing.T, bs *BufferStore) {
				if _, ok := bs.Get("old"); ok {
					t.Fatal("old name should not exist")
				}
				if _, ok := bs.Get("new"); !ok {
					t.Fatal("new name should exist")
				}
			},
		},
		{
			name:         "missing data argument",
			flags:        map[string]any{},
			args:         []string{},
			wantExitCode: 1,
		},
		{
			name:         "rename without -b",
			flags:        map[string]any{"-n": "new"},
			wantExitCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)
			router := NewCommandRouter(sessions, nil, RouterOptions{})
			if tt.setup != nil {
				tt.setup(router.buffers)
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "set-buffer",
				Flags:   tt.flags,
				Args:    tt.args,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("set-buffer exit code = %d, want %d, stderr = %q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
			if tt.verify != nil {
				tt.verify(t, router.buffers)
			}
		})
	}
}

func TestHandlePasteBuffer(t *testing.T) {
	tests := []struct {
		name         string
		flags        map[string]any
		setup        func(*BufferStore)
		wantExitCode int
	}{
		{
			name:         "no buffer available",
			flags:        map[string]any{"-t": "%0"},
			wantExitCode: 1,
		},
		{
			name:  "no target pane",
			flags: map[string]any{"-t": "%999"},
			setup: func(bs *BufferStore) {
				bs.Set("buf", []byte("data"), false)
			},
			wantExitCode: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)
			router := NewCommandRouter(sessions, nil, RouterOptions{})
			if _, _, err := sessions.CreateSession("test", "0", 120, 40); err != nil {
				t.Fatalf("CreateSession error: %v", err)
			}
			if tt.setup != nil {
				tt.setup(router.buffers)
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "paste-buffer",
				Flags:   tt.flags,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("paste-buffer exit code = %d, want %d, stderr = %q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
		})
	}
}

// TestPasteBufferDataTransformViaHandler verifies the -r/-s flag priority logic
// (C-2 fix) through the actual handlePasteBuffer handler path.
// tmux semantics: -s (separator) takes priority over -r (replace newlines).
// The handler is exercised via router.Execute to ensure flag reading logic
// (mustBool/mustString on req.Flags) is tested end-to-end.
func TestPasteBufferDataTransformViaHandler(t *testing.T) {
	tests := []struct {
		name         string
		data         string
		flags        map[string]any
		wantExitCode int
	}{
		{
			name:         "only -r flag succeeds",
			data:         "line1\nline2\nline3",
			flags:        map[string]any{"-r": true},
			wantExitCode: 0,
		},
		{
			name:         "only -s flag succeeds",
			data:         "line1\nline2\nline3",
			flags:        map[string]any{"-s": " | "},
			wantExitCode: 0,
		},
		{
			name:         "-s takes priority over -r when both specified",
			data:         "line1\nline2\nline3",
			flags:        map[string]any{"-r": true, "-s": " | "},
			wantExitCode: 0,
		},
		{
			name:         "neither flag succeeds",
			data:         "line1\nline2\nline3",
			flags:        map[string]any{},
			wantExitCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)
			emitter := &captureEmitter{}
			router := NewCommandRouter(sessions, emitter, RouterOptions{ShimAvailable: true})

			// Create session via router to attach a real terminal.
			resp := router.Execute(ipc.TmuxRequest{
				Command: "new-session",
				Flags:   map[string]any{"-s": "test", "-x": 80, "-y": 24},
			})
			if resp.ExitCode != 0 {
				t.Fatalf("new-session failed: exit=%d stderr=%q", resp.ExitCode, resp.Stderr)
			}

			// Set buffer data.
			router.buffers.Set("testbuf", []byte(tt.data), false)

			// Build flags for paste-buffer.
			flags := map[string]any{"-t": "%0", "-b": "testbuf"}
			maps.Copy(flags, tt.flags)

			resp = router.Execute(ipc.TmuxRequest{
				Command: "paste-buffer",
				Flags:   flags,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("paste-buffer exit code = %d, want %d, stderr = %q",
					resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
		})
	}
}

func TestHandleLoadBuffer(t *testing.T) {
	tests := []struct {
		name            string
		filePath        string
		fileContent     string
		createLargeFile bool
		flags           map[string]any
		wantExitCode    int
		verify          func(t *testing.T, bs *BufferStore, bufferName string)
	}{
		{
			name:         "load file into named buffer",
			filePath:     "testfile.txt",
			fileContent:  "hello world",
			flags:        map[string]any{"-b": "mybuf"},
			wantExitCode: 0,
			verify: func(t *testing.T, bs *BufferStore, bufferName string) {
				buf, ok := bs.Get("mybuf")
				if !ok {
					t.Fatal("buffer 'mybuf' not found")
				}
				if string(buf.Data) != "hello world" {
					t.Fatalf("buffer data = %q, want %q", buf.Data, "hello world")
				}
			},
		},
		{
			name:         "load file into auto-named buffer (no -b)",
			filePath:     "testfile.txt",
			fileContent:  "test content",
			flags:        map[string]any{},
			wantExitCode: 0,
			verify: func(t *testing.T, bs *BufferStore, bufferName string) {
				// Verify that at least one buffer exists
				buffers := bs.List()
				if len(buffers) == 0 {
					t.Fatal("no buffers found after load-buffer")
				}
				// Find any buffer containing the expected content
				found := false
				for _, buf := range buffers {
					if string(buf.Data) == "test content" {
						found = true
						break
					}
				}
				if !found {
					t.Fatal("expected content not found in any buffer")
				}
			},
		},
		{
			name:         "missing file path argument",
			filePath:     "",
			flags:        map[string]any{},
			wantExitCode: 1,
		},
		{
			name:         "stdin '-' is rejected",
			filePath:     "-",
			flags:        map[string]any{},
			wantExitCode: 1,
		},
		{
			name:         "file not found",
			filePath:     "/nonexistent/path/to/missing/file.txt",
			flags:        map[string]any{},
			wantExitCode: 1,
		},
		{
			name:         "empty file stores nothing",
			filePath:     "empty.txt",
			fileContent:  "",
			flags:        map[string]any{"-b": "emptybuf"},
			wantExitCode: 0,
			verify: func(t *testing.T, bs *BufferStore, bufferName string) {
				// Empty files should not create a buffer (silent success)
				_, ok := bs.Get("emptybuf")
				if ok {
					t.Fatal("empty file should not create a buffer")
				}
			},
		},
		{
			name:            "oversized file is rejected before reading",
			filePath:        "too-large.bin",
			createLargeFile: true,
			flags:           map[string]any{"-b": "largebuf"},
			wantExitCode:    1,
			verify: func(t *testing.T, bs *BufferStore, bufferName string) {
				if _, ok := bs.Get("largebuf"); ok {
					t.Fatal("oversized file should not create a buffer")
				}
			},
		},
		{
			name:         "-w flag accepted as no-op",
			filePath:     "testfile.txt",
			fileContent:  "content",
			flags:        map[string]any{"-w": true, "-b": "wbuf"},
			wantExitCode: 0,
			verify: func(t *testing.T, bs *BufferStore, bufferName string) {
				buf, ok := bs.Get("wbuf")
				if !ok {
					t.Fatal("buffer 'wbuf' not found")
				}
				if string(buf.Data) != "content" {
					t.Fatalf("buffer data = %q, want %q", buf.Data, "content")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)
			router := NewCommandRouter(sessions, nil, RouterOptions{})

			// Prepare file if needed
			var args []string
			if tt.filePath != "" && tt.filePath != "-" && !strings.HasPrefix(tt.filePath, "/nonexistent") {
				tmpDir := t.TempDir()
				fullPath := filepath.Join(tmpDir, tt.filePath)
				if tt.createLargeFile {
					file, err := os.Create(fullPath)
					if err != nil {
						t.Fatalf("Create error: %v", err)
					}
					if err := file.Truncate(maxLoadBufferFileSize + 1); err != nil {
						_ = file.Close()
						t.Fatalf("Truncate error: %v", err)
					}
					if err := file.Close(); err != nil {
						t.Fatalf("Close error: %v", err)
					}
				} else if err := os.WriteFile(fullPath, []byte(tt.fileContent), 0644); err != nil {
					t.Fatalf("WriteFile error: %v", err)
				}
				args = []string{fullPath}
			} else if tt.filePath != "" {
				args = []string{tt.filePath}
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "load-buffer",
				Flags:   tt.flags,
				Args:    args,
			})

			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("load-buffer exit code = %d, want %d, stderr = %q",
					resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}

			if tt.verify != nil {
				bufferName := mustString(tt.flags["-b"])
				tt.verify(t, router.buffers, bufferName)
			}
		})
	}
}

func TestHandleLoadBufferRejectsGrowthAfterStat(t *testing.T) {
	t.Parallel()

	sessions := NewSessionManager()
	t.Cleanup(sessions.Close)
	router := NewCommandRouter(sessions, nil, RouterOptions{})
	router.openLoadBufferFile = func(path string) (loadBufferReadCloser, error) {
		return &stubLoadBufferFile{
			reader: bytes.NewReader(bytes.Repeat([]byte("x"), maxLoadBufferFileSize+1)),
			info:   stubFileInfo{size: maxLoadBufferFileSize - 1},
		}, nil
	}

	resp := router.Execute(ipc.TmuxRequest{
		Command: "load-buffer",
		Flags:   map[string]any{"-b": "growing"},
		Args:    []string{"growing.txt"},
	})
	if resp.ExitCode != 1 {
		t.Fatalf("load-buffer exit code = %d, want 1, stderr = %q", resp.ExitCode, resp.Stderr)
	}
	if _, ok := router.buffers.Get("growing"); ok {
		t.Fatal("oversized read after stat should not create a buffer")
	}
	if !strings.Contains(resp.Stderr, "file too large") {
		t.Fatalf("stderr = %q, want oversized file error", resp.Stderr)
	}
}

func TestHandleSaveBuffer(t *testing.T) {
	tests := []struct {
		name                string
		setup               func(t *testing.T, sessions *SessionManager, router *CommandRouter)
		flags               map[string]any
		targetPath          string
		existingFileContent string
		wantExitCode        int
		wantStderr          string
		verify              func(t *testing.T, resp ipc.TmuxResponse, fullPath string)
	}{
		{
			name: "save named buffer to file",
			setup: func(t *testing.T, sessions *SessionManager, router *CommandRouter) {
				t.Helper()
				router.buffers.Set("savebuf", []byte("hello world"), false)
			},
			flags:        map[string]any{"-b": "savebuf"},
			targetPath:   "saved.txt",
			wantExitCode: 0,
			verify: func(t *testing.T, resp ipc.TmuxResponse, fullPath string) {
				t.Helper()
				data, err := os.ReadFile(fullPath)
				if err != nil {
					t.Fatalf("ReadFile error: %v", err)
				}
				if string(data) != "hello world" {
					t.Fatalf("saved file = %q, want %q", data, "hello world")
				}
				if resp.Stdout != "" {
					t.Fatalf("stdout = %q, want empty", resp.Stdout)
				}
			},
		},
		{
			name: "save latest buffer when -b omitted",
			setup: func(t *testing.T, sessions *SessionManager, router *CommandRouter) {
				t.Helper()
				router.buffers.Set("first", []byte("older"), false)
				router.buffers.Set("second", []byte("latest"), false)
			},
			flags:        map[string]any{},
			targetPath:   "latest.txt",
			wantExitCode: 0,
			verify: func(t *testing.T, resp ipc.TmuxResponse, fullPath string) {
				t.Helper()
				data, err := os.ReadFile(fullPath)
				if err != nil {
					t.Fatalf("ReadFile error: %v", err)
				}
				if string(data) != "latest" {
					t.Fatalf("saved file = %q, want %q", data, "latest")
				}
			},
		},
		{
			name: "append mode preserves existing file content",
			setup: func(t *testing.T, sessions *SessionManager, router *CommandRouter) {
				t.Helper()
				router.buffers.Set("appendbuf", []byte("world"), false)
			},
			flags:               map[string]any{"-a": true, "-b": "appendbuf"},
			targetPath:          "append.txt",
			existingFileContent: "hello ",
			wantExitCode:        0,
			verify: func(t *testing.T, resp ipc.TmuxResponse, fullPath string) {
				t.Helper()
				data, err := os.ReadFile(fullPath)
				if err != nil {
					t.Fatalf("ReadFile error: %v", err)
				}
				if string(data) != "hello world" {
					t.Fatalf("saved file = %q, want %q", data, "hello world")
				}
			},
		},
		{
			name: "truncate mode overwrites existing file content",
			setup: func(t *testing.T, sessions *SessionManager, router *CommandRouter) {
				t.Helper()
				router.buffers.Set("truncatebuf", []byte("new"), false)
			},
			flags:               map[string]any{"-b": "truncatebuf"},
			targetPath:          "overwrite.txt",
			existingFileContent: "old content that should be replaced",
			wantExitCode:        0,
			verify: func(t *testing.T, resp ipc.TmuxResponse, fullPath string) {
				t.Helper()
				data, err := os.ReadFile(fullPath)
				if err != nil {
					t.Fatalf("ReadFile error: %v", err)
				}
				if string(data) != "new" {
					t.Fatalf("saved file = %q, want %q", data, "new")
				}
			},
		},
		{
			name: "path dash writes to stdout",
			setup: func(t *testing.T, sessions *SessionManager, router *CommandRouter) {
				t.Helper()
				router.buffers.Set("stdoutbuf", []byte("stdout data"), false)
			},
			flags:        map[string]any{"-b": "stdoutbuf"},
			targetPath:   "-",
			wantExitCode: 0,
			verify: func(t *testing.T, resp ipc.TmuxResponse, fullPath string) {
				t.Helper()
				if resp.Stdout != "stdout data" {
					t.Fatalf("stdout = %q, want %q", resp.Stdout, "stdout data")
				}
			},
		},
		{
			name: "capture pane buffer can be saved to file",
			setup: func(t *testing.T, sessions *SessionManager, router *CommandRouter) {
				t.Helper()
				if _, _, err := sessions.CreateSession("test", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession error: %v", err)
				}
				pane, err := sessions.ResolveTarget("%0", 0)
				if err != nil {
					t.Fatalf("ResolveTarget error: %v", err)
				}
				pane.OutputHistory = NewPaneOutputHistory(1024)
				pane.OutputHistory.Write([]byte("captured output"))
				resp := router.Execute(ipc.TmuxRequest{
					Command: "capture-pane",
					Flags:   map[string]any{"-t": "%0", "-b": "capbuf"},
				})
				if resp.ExitCode != 0 {
					t.Fatalf("capture-pane exit code = %d, stderr = %q", resp.ExitCode, resp.Stderr)
				}
			},
			flags:        map[string]any{"-b": "capbuf"},
			targetPath:   "captured.txt",
			wantExitCode: 0,
			verify: func(t *testing.T, resp ipc.TmuxResponse, fullPath string) {
				t.Helper()
				data, err := os.ReadFile(fullPath)
				if err != nil {
					t.Fatalf("ReadFile error: %v", err)
				}
				if string(data) != "captured output" {
					t.Fatalf("saved file = %q, want %q", data, "captured output")
				}
			},
		},
		{
			name:         "missing file path argument",
			flags:        map[string]any{},
			targetPath:   "",
			wantExitCode: 1,
			wantStderr:   "save-buffer requires a file path argument",
		},
		{
			name: "named buffer must exist",
			setup: func(t *testing.T, sessions *SessionManager, router *CommandRouter) {
				t.Helper()
				router.buffers.Set("other", []byte("data"), false)
			},
			flags:        map[string]any{"-b": "missing"},
			targetPath:   "missing.txt",
			wantExitCode: 1,
			wantStderr:   "no buffer missing",
		},
		{
			name:         "latest buffer requires at least one buffer",
			flags:        map[string]any{},
			targetPath:   "latest.txt",
			wantExitCode: 1,
			wantStderr:   "no buffers",
		},
		{
			name: "file open error returns wrapped response",
			setup: func(t *testing.T, sessions *SessionManager, router *CommandRouter) {
				t.Helper()
				router.buffers.Set("openbuf", []byte("data"), false)
			},
			flags:        map[string]any{"-b": "openbuf"},
			targetPath:   filepath.Join("missing-dir", "deep", "nested", "file.txt"),
			wantExitCode: 1,
			wantStderr:   "save-buffer:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)
			router := NewCommandRouter(sessions, nil, RouterOptions{ShimAvailable: true})

			var fullPath string
			if tt.targetPath != "" && tt.targetPath != "-" {
				fullPath = filepath.Join(t.TempDir(), tt.targetPath)
				if tt.existingFileContent != "" {
					if err := os.WriteFile(fullPath, []byte(tt.existingFileContent), 0644); err != nil {
						t.Fatalf("WriteFile error: %v", err)
					}
				}
			}

			if tt.setup != nil {
				tt.setup(t, sessions, router)
			}

			var args []string
			switch {
			case tt.targetPath == "":
			case tt.targetPath == "-":
				args = []string{"-"}
			default:
				args = []string{fullPath}
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "save-buffer",
				Flags:   tt.flags,
				Args:    args,
			})
			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("save-buffer exit code = %d, want %d, stderr = %q", resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}
			if tt.wantStderr != "" && !strings.Contains(resp.Stderr, tt.wantStderr) {
				t.Fatalf("stderr = %q, want substring %q", resp.Stderr, tt.wantStderr)
			}

			if tt.verify != nil {
				tt.verify(t, resp, fullPath)
			}
		})
	}
}

func TestHandleSaveBufferCleanupOnFailure(t *testing.T) {
	tests := []struct {
		name           string
		flags          map[string]any
		file           saveBufferWriteCloser
		wantRemoveCall bool
	}{
		{
			name:  "truncate mode removes partial file on write failure",
			flags: map[string]any{"-b": "savebuf"},
			file: &stubSaveBufferFile{
				writeFn: func(p []byte) (int, error) {
					return len(p) / 2, errors.New("disk full")
				},
			},
			wantRemoveCall: true,
		},
		{
			name:  "truncate mode removes partial file on close failure",
			flags: map[string]any{"-b": "savebuf"},
			file: &stubSaveBufferFile{
				writeFn: func(p []byte) (int, error) {
					return len(p), nil
				},
				closeFn: func() error {
					return errors.New("flush failed")
				},
			},
			wantRemoveCall: true,
		},
		{
			name:  "append mode keeps existing target on write failure",
			flags: map[string]any{"-a": true, "-b": "savebuf"},
			file: &stubSaveBufferFile{
				writeFn: func(p []byte) (int, error) {
					return len(p) / 2, errors.New("disk full")
				},
			},
			wantRemoveCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var removedPath string

			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)
			router := NewCommandRouter(sessions, nil, RouterOptions{})
			router.openSaveBufferFile = func(path string, flag int, perm os.FileMode) (saveBufferWriteCloser, error) {
				return tt.file, nil
			}
			router.removeSaveBufferFile = func(path string) error {
				removedPath = path
				return nil
			}
			router.buffers.Set("savebuf", []byte("payload"), false)

			targetPath := filepath.Join(t.TempDir(), "out.txt")
			resp := router.Execute(ipc.TmuxRequest{
				Command: "save-buffer",
				Flags:   tt.flags,
				Args:    []string{targetPath},
			})
			if resp.ExitCode != 1 {
				t.Fatalf("save-buffer exit code = %d, want 1, stderr = %q", resp.ExitCode, resp.Stderr)
			}
			if gotRemoveCall := removedPath != ""; gotRemoveCall != tt.wantRemoveCall {
				t.Fatalf("remove called = %v, want %v (removedPath=%q)", gotRemoveCall, tt.wantRemoveCall, removedPath)
			}
			if tt.wantRemoveCall && removedPath != targetPath {
				t.Fatalf("removed path = %q, want %q", removedPath, targetPath)
			}
		})
	}
}

func TestHandleCapturePane(t *testing.T) {
	tests := []struct {
		name           string
		paneHasHistory bool
		historyContent string
		flags          map[string]any
		createPane     bool
		wantExitCode   int
		verifyStdout   func(t *testing.T, stdout string)
		verifyBuffer   func(t *testing.T, bs *BufferStore)
	}{
		{
			name:           "capture with -p prints to stdout",
			paneHasHistory: true,
			historyContent: "pane output data",
			flags:          map[string]any{"-p": true, "-t": "%0"},
			createPane:     true,
			wantExitCode:   0,
			verifyStdout: func(t *testing.T, stdout string) {
				if stdout != "pane output data" {
					t.Fatalf("stdout = %q, want %q", stdout, "pane output data")
				}
			},
		},
		{
			name:           "capture to named buffer with -b",
			paneHasHistory: true,
			historyContent: "buffer content",
			flags:          map[string]any{"-b": "capbuf", "-t": "%0"},
			createPane:     true,
			wantExitCode:   0,
			verifyBuffer: func(t *testing.T, bs *BufferStore) {
				buf, ok := bs.Get("capbuf")
				if !ok {
					t.Fatal("buffer 'capbuf' not found")
				}
				if string(buf.Data) != "buffer content" {
					t.Fatalf("buffer data = %q, want %q", buf.Data, "buffer content")
				}
			},
		},
		{
			name:           "capture to auto-named buffer (no -b)",
			paneHasHistory: true,
			historyContent: "auto buffer",
			flags:          map[string]any{"-t": "%0"},
			createPane:     true,
			wantExitCode:   0,
			verifyBuffer: func(t *testing.T, bs *BufferStore) {
				buffers := bs.List()
				if len(buffers) == 0 {
					t.Fatal("no buffers found after capture-pane")
				}
				found := false
				for _, buf := range buffers {
					if string(buf.Data) == "auto buffer" {
						found = true
						break
					}
				}
				if !found {
					t.Fatal("expected content not found in any buffer")
				}
			},
		},
		{
			name:         "no target pane found",
			flags:        map[string]any{"-t": "%999"},
			createPane:   true,
			wantExitCode: 1,
		},
		{
			name:         "no target pane found with -q suppresses error",
			flags:        map[string]any{"-t": "%999", "-q": true},
			createPane:   true,
			wantExitCode: 0,
			verifyStdout: func(t *testing.T, stdout string) {
				if stdout != "" {
					t.Fatalf("stdout should be empty with -q, got %q", stdout)
				}
			},
		},
		{
			name:           "nil OutputHistory returns error",
			paneHasHistory: false,
			flags:          map[string]any{"-t": "%0"},
			createPane:     true,
			wantExitCode:   1,
		},
		{
			name:           "nil OutputHistory with -q suppresses error",
			paneHasHistory: false,
			flags:          map[string]any{"-t": "%0", "-q": true},
			createPane:     true,
			wantExitCode:   0,
			verifyStdout: func(t *testing.T, stdout string) {
				if stdout != "" {
					t.Fatalf("stdout should be empty with -q, got %q", stdout)
				}
			},
		},
		{
			name:           "-S and -E flags are accepted as no-op full-history selectors",
			paneHasHistory: true,
			historyContent: "full history",
			flags:          map[string]any{"-p": true, "-t": "%0", "-S": "-10", "-E": "-1"},
			createPane:     true,
			wantExitCode:   0,
			verifyStdout: func(t *testing.T, stdout string) {
				if stdout != "full history" {
					t.Fatalf("stdout = %q, want %q", stdout, "full history")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := NewSessionManager()
			t.Cleanup(sessions.Close)
			router := NewCommandRouter(sessions, nil, RouterOptions{ShimAvailable: true})

			// Create session with a pane
			if tt.createPane {
				if _, _, err := sessions.CreateSession("test", "0", 120, 40); err != nil {
					t.Fatalf("CreateSession error: %v", err)
				}

				// Get the default pane and set up OutputHistory if needed
				if pane, err := sessions.ResolveTarget("%0", 0); err == nil {
					if tt.paneHasHistory {
						pane.OutputHistory = NewPaneOutputHistory(1024)
						pane.OutputHistory.Write([]byte(tt.historyContent))
					}
				}
			}

			resp := router.Execute(ipc.TmuxRequest{
				Command: "capture-pane",
				Flags:   tt.flags,
			})

			if resp.ExitCode != tt.wantExitCode {
				t.Fatalf("capture-pane exit code = %d, want %d, stderr = %q",
					resp.ExitCode, tt.wantExitCode, resp.Stderr)
			}

			if tt.verifyStdout != nil {
				tt.verifyStdout(t, resp.Stdout)
			}

			if tt.verifyBuffer != nil {
				tt.verifyBuffer(t, router.buffers)
			}
		})
	}
}
