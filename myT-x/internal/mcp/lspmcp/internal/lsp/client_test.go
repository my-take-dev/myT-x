package lsp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClientAppliesDefaults(t *testing.T) {
	client := NewClient(Config{Command: "dummy"})
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.cfg.RequestTimeout <= 0 {
		t.Errorf("RequestTimeout should be positive, got %v", client.cfg.RequestTimeout)
	}
	if client.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestClientRequestWithMockLSP(t *testing.T) {
	// mock_lsp を go run で起動。cwd はモジュールルート（go test 実行ディレクトリ）を想定。
	wd, err := os.Getwd()
	if err != nil {
		t.Skipf("Getwd failed: %v", err)
	}
	mockDir := filepath.Join(wd, "internal", "lsp", "testdata", "mock_lsp")
	if _, err := os.Stat(filepath.Join(mockDir, "main.go")); err != nil {
		// internal/lsp から実行された場合
		mockDir = filepath.Join(wd, "testdata", "mock_lsp")
		if _, err := os.Stat(filepath.Join(mockDir, "main.go")); err != nil {
			t.Skipf("mock_lsp not found: %v", err)
		}
	}

	client := NewClient(Config{
		Command:        "go",
		Args:           []string{"run", "."},
		RootDir:        mockDir,
		RequestTimeout: 5 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer func() {
		if err := client.Close(context.Background()); err != nil {
			t.Logf("Close error: %v", err)
		}
	}()

	raw, err := client.Request(ctx, "initialize", map[string]any{
		"processId": nil,
		"rootUri":   "file:///tmp",
	})
	if err != nil {
		t.Fatalf("Request(initialize) failed: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected non-empty result")
	}
}

func TestClientCloseBeforeStartIsNoop(t *testing.T) {
	client := NewClient(Config{Command: "dummy"})
	if err := client.Close(context.Background()); err != nil {
		t.Fatalf("Close before Start should be noop, got: %v", err)
	}
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

type countingWriteCloser struct {
	closed atomic.Int32
}

func (c *countingWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (c *countingWriteCloser) Close() error {
	c.closed.Add(1)
	return nil
}

type countingReadCloser struct {
	closed atomic.Int32
}

func (c *countingReadCloser) Read(_ []byte) (int, error) { return 0, io.EOF }
func (c *countingReadCloser) Close() error {
	c.closed.Add(1)
	return nil
}

func TestClientClose_DoneWaitTimeoutDoesNotHang(t *testing.T) {
	client := NewClient(Config{
		Command:        "dummy",
		RequestTimeout: 10 * time.Millisecond,
	})
	client.cmd = &exec.Cmd{}
	client.stdin = nopWriteCloser{Writer: &bytes.Buffer{}}
	client.stdout = io.NopCloser(strings.NewReader(""))
	client.stderr = io.NopCloser(strings.NewReader(""))
	client.readLoopDone = make(chan struct{})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := client.Close(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Close should return timeout error when read-loop completion is never signaled")
	}
	if !strings.Contains(err.Error(), "timed out waiting for read loop") {
		t.Fatalf("Close error = %v, want timed out waiting for read loop", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Close elapsed = %v, should not hang", elapsed)
	}
}

func TestClientClose_WaitsForDoneUsingCallerContext(t *testing.T) {
	prevShutdownTimeout := lspClientShutdownTimeout
	lspClientShutdownTimeout = 10 * time.Millisecond
	t.Cleanup(func() {
		lspClientShutdownTimeout = prevShutdownTimeout
	})

	client := NewClient(Config{
		Command:        "dummy",
		RequestTimeout: 10 * time.Millisecond,
	})
	client.cmd = &exec.Cmd{}
	client.stdin = nopWriteCloser{Writer: &bytes.Buffer{}}
	client.stdout = io.NopCloser(strings.NewReader(""))
	client.stderr = io.NopCloser(strings.NewReader(""))
	client.readLoopDone = make(chan struct{})

	time.AfterFunc(50*time.Millisecond, func() {
		close(client.readLoopDone)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := client.Close(ctx); err != nil {
		t.Fatalf("Close error = %v, want nil", err)
	}
}

func TestClientClose_ConcurrentCallsShareSingleCleanup(t *testing.T) {
	client := NewClient(Config{
		Command:        "dummy",
		RequestTimeout: 10 * time.Millisecond,
	})
	stdin := &countingWriteCloser{}
	stdout := &countingReadCloser{}
	stderr := &countingReadCloser{}
	client.cmd = &exec.Cmd{}
	client.stdin = stdin
	client.stdout = stdout
	client.stderr = stderr
	close(client.readLoopDone)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Go(func() {
			errs <- client.Close(context.Background())
		})
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}
	if got := stdin.closed.Load(); got != 1 {
		t.Fatalf("stdin Close count = %d, want 1", got)
	}
	if got := stdout.closed.Load(); got != 1 {
		t.Fatalf("stdout Close count = %d, want 1", got)
	}
	if got := stderr.closed.Load(); got != 1 {
		t.Fatalf("stderr Close count = %d, want 1", got)
	}
}

func TestClientNotifyAfterCloseReturnsClosedError(t *testing.T) {
	client := NewClient(Config{
		Command:        "dummy",
		RequestTimeout: time.Millisecond,
	})
	client.cmd = &exec.Cmd{}
	client.stdin = nopWriteCloser{Writer: &bytes.Buffer{}}
	client.stdout = io.NopCloser(strings.NewReader(""))
	client.stderr = io.NopCloser(strings.NewReader(""))
	close(client.readLoopDone)

	if err := client.Close(context.Background()); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	err := client.Notify("exit", nil)
	if !errors.Is(err, errLSPClientClosed) {
		t.Fatalf("Notify after Close error = %v, want %v", err, errLSPClientClosed)
	}
}

func TestExtractIDFromRawJSON(t *testing.T) {
	tests := []struct {
		payload string
		want    string
	}{
		{`{"id":1,"result":{}}`, "1"},
		{`{"id":"abc","result":null}`, `"abc"`}, // JSON 文字列は引用符付きで格納される
		{`{"id":123}`, "123"},
		{`{"method":"ping"}`, ""},
		{`invalid json`, ""},
		{`{"result":{}}`, ""},
	}
	for _, tt := range tests {
		got := extractIDFromRawJSON([]byte(tt.payload))
		if got != tt.want {
			t.Errorf("extractIDFromRawJSON(%q) = %q, want %q", tt.payload, got, tt.want)
		}
	}
}

func TestShouldWrapWindowsBatchCommand(t *testing.T) {
	prevGOOS := lspClientGOOS
	prevLookPath := lspClientLookPath
	t.Cleanup(func() {
		lspClientGOOS = prevGOOS
		lspClientLookPath = prevLookPath
	})

	lspClientGOOS = "windows"

	if !shouldWrapWindowsBatchCommand(`C:\tools\server.cmd`) {
		t.Fatal("expected .cmd command to require wrapping")
	}

	if !shouldWrapWindowsBatchCommand(`C:\tools\server.bat`) {
		t.Fatal("expected .bat command to require wrapping")
	}

	lspClientLookPath = func(file string) (string, error) {
		if file == "npx" {
			return `C:\Program Files\nodejs\npx.cmd`, nil
		}
		return "", errors.New("not found")
	}
	if !shouldWrapWindowsBatchCommand("npx") {
		t.Fatal("expected command resolved to .cmd via PATH to require wrapping")
	}

	lspClientLookPath = func(string) (string, error) {
		return `C:\tools\gopls.exe`, nil
	}
	if shouldWrapWindowsBatchCommand("gopls") {
		t.Fatal("did not expect .exe command to require wrapping")
	}

	lspClientGOOS = "linux"
	if shouldWrapWindowsBatchCommand(`C:\tools\server.cmd`) {
		t.Fatal("did not expect wrapping outside windows")
	}
}

func TestBuildLSPExecCommand_WindowsBatchWrap(t *testing.T) {
	prevGOOS := lspClientGOOS
	prevLookPath := lspClientLookPath
	t.Cleanup(func() {
		lspClientGOOS = prevGOOS
		lspClientLookPath = prevLookPath
	})

	lspClientGOOS = "windows"
	lspClientLookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	cmd := buildLSPExecCommand(context.Background(), `C:\tools\server.cmd`, []string{"--stdio"})
	if filepath.Base(strings.ToLower(cmd.Path)) != "cmd.exe" {
		t.Fatalf("wrapped command path = %q, want cmd.exe", cmd.Path)
	}
	if len(cmd.Args) < 5 {
		t.Fatalf("wrapped args too short: %v", cmd.Args)
	}
	if cmd.Args[1] != "/d" || cmd.Args[2] != "/s" || cmd.Args[3] != "/c" {
		t.Fatalf("unexpected wrapper args: %v", cmd.Args)
	}
	if cmd.Args[4] != `C:\tools\server.cmd` {
		t.Fatalf("wrapped command target = %q, want original command", cmd.Args[4])
	}
	wantHidden := runtime.GOOS == "windows"
	if hidden := testHideWindowEnabled(cmd); hidden != wantHidden {
		t.Fatalf("hideWindow = %v, want %v", hidden, wantHidden)
	}
}

func TestBuildLSPExecCommand_AppliesPlatformOptionsToDirectCommand(t *testing.T) {
	prevGOOS := lspClientGOOS
	t.Cleanup(func() {
		lspClientGOOS = prevGOOS
	})

	// Force the non-wrapper path and assert OS-specific SysProcAttr behavior.
	lspClientGOOS = "linux"

	cmd := buildLSPExecCommand(context.Background(), "gopls", []string{"serve"})
	wantHidden := runtime.GOOS == "windows"
	if hidden := testHideWindowEnabled(cmd); hidden != wantHidden {
		t.Fatalf("hideWindow = %v, want %v", hidden, wantHidden)
	}
}
