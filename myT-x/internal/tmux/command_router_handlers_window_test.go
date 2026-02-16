package tmux

import (
	"sync"
	"testing"

	"myT-x/internal/ipc"
)

type captureEmitter struct {
	mu     sync.Mutex
	events []capturedEvent
}

type capturedEvent struct {
	name    string
	payload any
}

func (e *captureEmitter) Emit(name string, payload any) {
	e.mu.Lock()
	e.events = append(e.events, capturedEvent{name: name, payload: payload})
	e.mu.Unlock()
}

func (e *captureEmitter) Events() []capturedEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := make([]capturedEvent, len(e.events))
	copy(cp, e.events)
	return cp
}

func TestHandleActivateWindow(t *testing.T) {
	tests := []struct {
		name          string
		wantExitCode  int
		wantStdout    string
		wantEventName string
	}{
		{
			name:          "returns exit code 0 and emits event",
			wantExitCode:  0,
			wantStdout:    "ok\n",
			wantEventName: "app:activate-window",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emitter := &captureEmitter{}
			router := NewCommandRouter(NewSessionManager(), emitter, RouterOptions{
				DefaultShell: "cmd.exe",
			})

			resp := router.Execute(ipc.TmuxRequest{Command: "activate-window"})

			if resp.ExitCode != tt.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", resp.ExitCode, tt.wantExitCode)
			}
			if resp.Stdout != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", resp.Stdout, tt.wantStdout)
			}

			events := emitter.Events()
			if len(events) != 1 {
				t.Fatalf("emitted %d events, want 1", len(events))
			}
			if events[0].name != tt.wantEventName {
				t.Errorf("event name = %q, want %q", events[0].name, tt.wantEventName)
			}
			if events[0].payload != nil {
				t.Errorf("event payload = %#v, want nil", events[0].payload)
			}
		})
	}
}

func TestHandleActivateWindowWithNilEmitter(t *testing.T) {
	router := NewCommandRouter(NewSessionManager(), nil, RouterOptions{
		DefaultShell: "cmd.exe",
	})

	resp := router.Execute(ipc.TmuxRequest{Command: "activate-window"})

	if resp.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", resp.ExitCode)
	}
	if resp.Stdout != "ok\n" {
		t.Fatalf("Stdout = %q, want %q", resp.Stdout, "ok\n")
	}
}
