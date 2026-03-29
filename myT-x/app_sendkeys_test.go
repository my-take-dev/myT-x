package main

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"myT-x/internal/ipc"
	"myT-x/internal/tmux"
)

// callRecorder builds a sendKeysIO that records all router calls as "cmd:detail" strings.
// select-pane → "select-pane:%1", send-keys -l text → "text:hello",
// send-keys C-m → "key:C-m", paste-start/end → "paste-start"/"paste-end".
func callRecorder(calls *[]string) sendKeysIO {
	return sendKeysIO{
		executeRequest: func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
			switch req.Command {
			case "select-pane":
				target, _ := req.Flags["-t"].(string)
				*calls = append(*calls, "select-pane:"+target)
			case "send-keys":
				args := strings.Join(req.Args, " ")
				_, isLiteral := req.Flags["-l"]
				if isLiteral {
					switch {
					case args == bracketedPasteStart:
						*calls = append(*calls, "paste-start")
					case args == bracketedPasteEnd:
						*calls = append(*calls, "paste-end")
					default:
						*calls = append(*calls, "text:"+args)
					}
				} else {
					*calls = append(*calls, "key:"+args)
				}
			}
			return ipc.TmuxResponse{ExitCode: 0}
		},
		sleep: func(time.Duration) {},
	}
}

// failOnCommand returns a sendKeysIO that returns ExitCode=1 when command matches.
func failOnCommand(failCmd string) sendKeysIO {
	return sendKeysIO{
		executeRequest: func(_ *tmux.CommandRouter, req ipc.TmuxRequest) ipc.TmuxResponse {
			if req.Command == failCmd {
				return ipc.TmuxResponse{ExitCode: 1, Stderr: failCmd + " error"}
			}
			return ipc.TmuxResponse{ExitCode: 0}
		},
		sleep: func(time.Duration) {},
	}
}

func TestSendKeysLiteralPasteWithEnterOrder(t *testing.T) {
	var calls []string
	sk := callRecorder(&calls)

	err := sk.sendKeysLiteralPasteWithEnter(nil, "%1", "hello")
	if err != nil {
		t.Fatalf("sendKeysLiteralPasteWithEnter() error = %v", err)
	}

	// Expected order: select-pane -> paste-start -> text -> paste-end -> Enter (C-m)
	wantOrder := []string{"select-pane:%1", "paste-start", "text:hello", "paste-end", "key:C-m"}
	if len(calls) != len(wantOrder) {
		t.Fatalf("got %d calls, want %d: %v", len(calls), len(wantOrder), calls)
	}
	for i, want := range wantOrder {
		if calls[i] != want {
			t.Fatalf("calls[%d] = %q, want %q\nfull calls: %v", i, calls[i], want, calls)
		}
	}
}

func TestSendKeysLiteralWithEnterOrder(t *testing.T) {
	var calls []string
	sk := callRecorder(&calls)

	err := sk.sendKeysLiteralWithEnter(nil, "%2", "world")
	if err != nil {
		t.Fatalf("sendKeysLiteralWithEnter() error = %v", err)
	}

	wantOrder := []string{"select-pane:%2", "text:world", "key:C-m"}
	if len(calls) != len(wantOrder) {
		t.Fatalf("got %d calls, want %d: %v", len(calls), len(wantOrder), calls)
	}
	for i, want := range wantOrder {
		if calls[i] != want {
			t.Fatalf("calls[%d] = %q, want %q\nfull calls: %v", i, calls[i], want, calls)
		}
	}
}

func TestSendKeysTrimRight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantText string // the text argument recorded by callRecorder
	}{
		{
			name:     "no trailing newline",
			input:    "hello",
			wantText: "hello",
		},
		{
			name:     "trailing LF",
			input:    "hello\n",
			wantText: "hello",
		},
		{
			name:     "trailing CR",
			input:    "hello\r",
			wantText: "hello",
		},
		{
			name:     "trailing CRLF",
			input:    "hello\r\n",
			wantText: "hello",
		},
		{
			name:     "multiple trailing newlines",
			input:    "hello\n\n\n",
			wantText: "hello",
		},
		{
			name:     "embedded newlines preserved",
			input:    "line1\nline2\n",
			wantText: "line1\nline2",
		},
		{
			name:     "only newlines becomes empty",
			input:    "\n\r\n",
			wantText: "",
		},
	}

	for _, tt := range tests {
		t.Run("Literal/"+tt.name, func(t *testing.T) {
			t.Parallel()
			var calls []string
			sk := callRecorder(&calls)
			if err := sk.sendKeysLiteralWithEnter(nil, "%1", tt.input); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// Find the text call (skip select-pane).
			var got string
			for _, c := range calls {
				if after, ok := strings.CutPrefix(c, "text:"); ok {
					got = after
					break
				}
			}
			if got != tt.wantText {
				t.Errorf("sent text = %q, want %q", got, tt.wantText)
			}
		})

		t.Run("Paste/"+tt.name, func(t *testing.T) {
			t.Parallel()
			var calls []string
			sk := callRecorder(&calls)
			if err := sk.sendKeysLiteralPasteWithEnter(nil, "%1", tt.input); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var got string
			for _, c := range calls {
				if after, ok := strings.CutPrefix(c, "text:"); ok {
					got = after
					break
				}
			}
			if got != tt.wantText {
				t.Errorf("sent text = %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestSendKeysSelectPaneFailure(t *testing.T) {
	t.Parallel()

	sk := failOnCommand("select-pane")

	t.Run("LiteralWithEnter", func(t *testing.T) {
		t.Parallel()
		err := sk.sendKeysLiteralWithEnter(nil, "%1", "hello")
		if err == nil {
			t.Fatal("expected error from select-pane failure")
		}
		if !strings.Contains(err.Error(), "select-pane failed") {
			t.Errorf("error = %q, want containing %q", err.Error(), "select-pane failed")
		}
	})

	t.Run("PasteWithEnter", func(t *testing.T) {
		t.Parallel()
		err := sk.sendKeysLiteralPasteWithEnter(nil, "%1", "hello")
		if err == nil {
			t.Fatal("expected error from select-pane failure")
		}
		if !strings.Contains(err.Error(), "select-pane failed") {
			t.Errorf("error = %q, want containing %q", err.Error(), "select-pane failed")
		}
	})
}

func TestSelectPane(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		var calls []string
		sk := callRecorder(&calls)
		if err := sk.selectPane(nil, "%5"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(calls) != 1 || calls[0] != "select-pane:%5" {
			t.Errorf("calls = %v, want [select-pane:%%5]", calls)
		}
	})

	t.Run("failure", func(t *testing.T) {
		t.Parallel()
		sk := failOnCommand("select-pane")
		err := sk.selectPane(nil, "%1")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "select-pane failed") {
			t.Errorf("error = %q, want containing 'select-pane failed'", err.Error())
		}
	})
}

func TestSendKeysIOFieldCount(t *testing.T) {
	t.Parallel()
	const expectedFields = 2
	actual := reflect.TypeFor[sendKeysIO]().NumField()
	if actual != expectedFields {
		t.Fatalf("sendKeysIO has %d fields, expected %d — update defaultSendKeysIO and tests for new fields", actual, expectedFields)
	}
}

func TestDefaultSendKeysIOAllFieldsNonNil(t *testing.T) {
	t.Parallel()
	d := defaultSendKeysIO()
	v := reflect.ValueOf(d)
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Kind() == reflect.Func && f.IsNil() {
			t.Errorf("defaultSendKeysIO().%s is nil", v.Type().Field(i).Name)
		}
	}
}
