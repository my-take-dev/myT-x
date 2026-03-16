package main

import (
	"strings"
	"testing"
	"time"

	"myT-x/internal/ipc"
	"myT-x/internal/tmux"
)

func TestSendKeysLiteralPasteWithEnterOrder(t *testing.T) {
	// Stub executeRouterRequestFn to capture all send-keys calls in order.
	origExec := executeRouterRequestFn
	origSleep := sendKeysLiteralSleepFn
	t.Cleanup(func() {
		executeRouterRequestFn = origExec
		sendKeysLiteralSleepFn = origSleep
	})
	sendKeysLiteralSleepFn = func(time.Duration) {}

	var calls []string
	executeRouterRequestFn = func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
		if req.Command != "send-keys" {
			return ipc.TmuxResponse{ExitCode: 0}
		}
		args := strings.Join(req.Args, " ")
		_, isLiteral := req.Flags["-l"]
		if isLiteral {
			switch {
			case args == bracketedPasteStart:
				calls = append(calls, "paste-start")
			case args == bracketedPasteEnd:
				calls = append(calls, "paste-end")
			default:
				calls = append(calls, "text:"+args)
			}
		} else {
			calls = append(calls, "key:"+args)
		}
		return ipc.TmuxResponse{ExitCode: 0}
	}

	err := sendKeysLiteralPasteWithEnter(nil, "%1", "hello")
	if err != nil {
		t.Fatalf("sendKeysLiteralPasteWithEnter() error = %v", err)
	}

	// Expected order: paste-start -> text -> paste-end -> Enter (C-m)
	wantOrder := []string{"paste-start", "text:hello", "paste-end", "key:C-m"}
	if len(calls) != len(wantOrder) {
		t.Fatalf("got %d calls, want %d: %v", len(calls), len(wantOrder), calls)
	}
	for i, want := range wantOrder {
		if calls[i] != want {
			t.Fatalf("calls[%d] = %q, want %q\nfull calls: %v", i, calls[i], want, calls)
		}
	}
}
