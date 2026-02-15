//go:build windows

package terminal

import (
	"bytes"
	"os"
	"sync"
	"testing"
	"time"
)

func TestStartUsesConPtyWithEnv(t *testing.T) {
	if !IsConPtyAvailable() {
		t.Skip("ConPTY is unavailable on this Windows version")
	}
	t.Setenv("GO_TMUX_ENABLE_CONPTY", "1")
	t.Setenv("GO_TMUX_DISABLE_CONPTY", "")

	term, err := Start(Config{
		Shell:   "cmd.exe",
		Columns: 80,
		Rows:    24,
		Env:     os.Environ(),
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer term.Close()

	if term.pty == nil {
		t.Fatalf("expected ConPTY backend, got fallback mode")
	}
}

func TestConPtyCmdEchoAcceptsInput(t *testing.T) {
	if !IsConPtyAvailable() {
		t.Skip("ConPTY is unavailable on this Windows version")
	}
	t.Setenv("GO_TMUX_ENABLE_CONPTY", "1")
	t.Setenv("GO_TMUX_DISABLE_CONPTY", "")

	term, err := Start(Config{
		Shell:   "cmd.exe",
		Columns: 80,
		Rows:    24,
		Env:     os.Environ(),
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer term.Close()

	if term.pty == nil {
		t.Fatalf("expected ConPTY backend, got fallback mode")
	}

	var output bytes.Buffer
	var outputMu sync.Mutex
	done := make(chan struct{})
	go func() {
		term.ReadLoop(func(chunk []byte) {
			outputMu.Lock()
			output.Write(chunk)
			outputMu.Unlock()
		})
		close(done)
	}()

	time.Sleep(1200 * time.Millisecond)
	_, _ = term.Write([]byte("echo __GO_TMUX_CONPTY_MARKER__\r"))

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		outputMu.Lock()
		found := bytes.Contains(output.Bytes(), []byte("__GO_TMUX_CONPTY_MARKER__"))
		outputMu.Unlock()
		if found {
			_, _ = term.Write([]byte("exit\r"))
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	_, _ = term.Write([]byte("exit\r"))
	_ = term.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	outputMu.Lock()
	defer outputMu.Unlock()
	t.Fatalf("conpty marker not found in output tail: %q", tailBytes(output.Bytes(), 600))
}

func TestConPtyPowerShellAcceptsInput(t *testing.T) {
	if !IsConPtyAvailable() {
		t.Skip("ConPTY is unavailable on this Windows version")
	}
	t.Setenv("GO_TMUX_ENABLE_CONPTY", "1")
	t.Setenv("GO_TMUX_DISABLE_CONPTY", "")

	term, err := Start(Config{
		Shell:   "powershell.exe",
		Columns: 120,
		Rows:    40,
		Env:     os.Environ(),
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer term.Close()

	if term.pty == nil {
		t.Fatalf("expected ConPTY backend, got fallback mode")
	}

	var output bytes.Buffer
	var outputMu sync.Mutex
	done := make(chan struct{})
	go func() {
		term.ReadLoop(func(chunk []byte) {
			outputMu.Lock()
			output.Write(chunk)
			outputMu.Unlock()
		})
		close(done)
	}()

	time.Sleep(1200 * time.Millisecond)
	_, _ = term.Write([]byte("Write-Output __GO_TMUX_PS_MARKER__\r"))

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		outputMu.Lock()
		found := bytes.Contains(output.Bytes(), []byte("__GO_TMUX_PS_MARKER__"))
		outputMu.Unlock()
		if found {
			_, _ = term.Write([]byte("exit\r"))
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	_, _ = term.Write([]byte("exit\r"))
	_ = term.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	outputMu.Lock()
	defer outputMu.Unlock()
	t.Fatalf("powershell marker not found in output tail: %q", tailBytes(output.Bytes(), 600))
}

func TestStartCanForcePipeMode(t *testing.T) {
	t.Setenv("GO_TMUX_ENABLE_CONPTY", "")
	t.Setenv("GO_TMUX_DISABLE_CONPTY", "1")

	term, err := Start(Config{
		Shell: "cmd.exe",
		Env:   os.Environ(),
	})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer term.Close()

	if term.pty != nil {
		t.Fatalf("expected pipe-mode backend when ConPTY is disabled")
	}
}

func TestPipeModePowershellToCmdAcceptsInput(t *testing.T) {
	term, err := startPipeMode(Config{
		Shell: "powershell.exe",
		Env:   os.Environ(),
	})
	if err != nil {
		t.Fatalf("startPipeMode() error = %v", err)
	}
	defer term.Close()

	var output bytes.Buffer
	var outputMu sync.Mutex
	done := make(chan struct{})
	go func() {
		term.ReadLoop(func(chunk []byte) {
			outputMu.Lock()
			output.Write(chunk)
			outputMu.Unlock()
		})
		close(done)
	}()

	time.Sleep(1200 * time.Millisecond)
	_, _ = term.Write([]byte("cmd\r"))
	time.Sleep(500 * time.Millisecond)
	_, _ = term.Write([]byte("echo __GO_TMUX_PIPE_MARKER__\r"))

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		outputMu.Lock()
		found := bytes.Contains(output.Bytes(), []byte("__GO_TMUX_PIPE_MARKER__"))
		outputMu.Unlock()
		if found {
			_, _ = term.Write([]byte("exit\r"))
			_, _ = term.Write([]byte("exit\r"))
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	_, _ = term.Write([]byte("exit\r"))
	_, _ = term.Write([]byte("exit\r"))
	_ = term.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	outputMu.Lock()
	defer outputMu.Unlock()
	t.Fatalf("pipe-mode marker not found in output tail: %q", tailBytes(output.Bytes(), 600))
}

func tailBytes(data []byte, n int) []byte {
	if n <= 0 || len(data) <= n {
		return data
	}
	return data[len(data)-n:]
}

func TestStartPipeModeHidesWindow(t *testing.T) {
	t.Setenv("GO_TMUX_ENABLE_CONPTY", "")
	t.Setenv("GO_TMUX_DISABLE_CONPTY", "1")

	term, err := startPipeMode(Config{
		Shell: "cmd.exe",
		Env:   os.Environ(),
	})
	if err != nil {
		t.Fatalf("startPipeMode() error = %v", err)
	}
	defer term.Close()

	if term.cmd == nil {
		t.Fatal("expected cmd to be set in pipe mode")
	}
	if term.cmd.SysProcAttr == nil {
		t.Fatal("expected SysProcAttr to be set by HideWindow")
	}
	if !term.cmd.SysProcAttr.HideWindow {
		t.Fatal("expected SysProcAttr.HideWindow = true")
	}
}

func TestBuildCommandLine(t *testing.T) {
	tests := []struct {
		name  string
		shell string
		args  []string
		want  string
	}{
		{
			name:  "shell only",
			shell: "cmd.exe",
			args:  nil,
			want:  "cmd.exe",
		},
		{
			name:  "shell with args",
			shell: "powershell.exe",
			args:  []string{"-NoProfile", "-Command", "echo hello"},
			want:  "powershell.exe -NoProfile -Command \"echo hello\"",
		},
		{
			name:  "shell path with spaces",
			shell: `C:\Program Files\PowerShell\7\pwsh.exe`,
			args:  []string{"-NoProfile"},
			want:  `"C:\Program Files\PowerShell\7\pwsh.exe" -NoProfile`,
		},
		{
			name:  "args with spaces",
			shell: "cmd.exe",
			args:  []string{"/c", "echo hello world"},
			want:  `cmd.exe /c "echo hello world"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCommandLine(tt.shell, tt.args)
			if got != tt.want {
				t.Errorf("buildCommandLine(%q, %v) = %q, want %q", tt.shell, tt.args, got, tt.want)
			}
		})
	}
}
