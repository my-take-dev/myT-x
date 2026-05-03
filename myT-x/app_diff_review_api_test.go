package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"myT-x/internal/tmux"
)

func TestSendDiffReview_EmptyPaneID(t *testing.T) {
	app := &App{}
	app.sessions = tmux.NewSessionManager()
	t.Cleanup(app.sessions.Close)

	err := app.SendDiffReview("", "some comment")
	if err == nil {
		t.Fatal("SendDiffReview with empty paneID should return error")
	}
}

func TestSendDiffReview_NilSessions(t *testing.T) {
	app := &App{}

	err := app.SendDiffReview("%0", "some comment")
	if err == nil {
		t.Fatal("SendDiffReview with nil sessions should return error")
	}
}

func TestSendDiffReview_NewlineOnlyText(t *testing.T) {
	mgr := tmux.NewSessionManager()
	t.Cleanup(mgr.Close)
	_, pane, err := mgr.CreateSession("test", "bash", 80, 24)
	if err != nil {
		t.Fatal(err)
	}

	var calls []string
	app := &App{
		sessions: mgr,
		router:   tmux.NewCommandRouter(mgr, nil, tmux.RouterOptions{}),
		sendKeys: callRecorder(&calls),
	}

	err = app.SendDiffReview(fmt.Sprintf("%%%d", pane.ID), "\r\n\r\n")
	if err != nil {
		t.Fatalf("newline-only text should succeed silently, got: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("no router calls expected for newline-only text after TrimRight, got: %v", calls)
	}
}

func TestSendDiffReview_PreservesLiteralWhitespaceWithoutTrailingNewlines(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{name: "spaces", text: "   "},
		{name: "tab", text: "\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := tmux.NewSessionManager()
			t.Cleanup(mgr.Close)
			_, pane, err := mgr.CreateSession("test", "bash", 80, 24)
			if err != nil {
				t.Fatal(err)
			}

			var calls []string
			app := &App{
				sessions: mgr,
				router:   tmux.NewCommandRouter(mgr, nil, tmux.RouterOptions{}),
				sendKeys: callRecorder(&calls),
			}

			err = app.SendDiffReview(fmt.Sprintf("%%%d", pane.ID), tt.text)
			if err != nil {
				t.Fatalf("SendDiffReview(%q) returned error: %v", tt.text, err)
			}
			foundLiteralText := slices.Contains(calls, "text:"+tt.text)
			if !foundLiteralText {
				t.Fatalf("expected literal whitespace to be sent, got calls: %v", calls)
			}
		})
	}
}

func TestSendDiffReview_SendKeysFailureDoesNotRecordInput(t *testing.T) {
	mgr := tmux.NewSessionManager()
	t.Cleanup(mgr.Close)
	_, pane, err := mgr.CreateSession("test", "bash", 80, 24)
	if err != nil {
		t.Fatal(err)
	}

	app := &App{
		sessions: mgr,
		router:   tmux.NewCommandRouter(mgr, nil, tmux.RouterOptions{}),
		sendKeys: failOnCommand("send-keys"),
	}

	err = app.SendDiffReview(fmt.Sprintf("%%%d", pane.ID), "# Review\n\n> fix this")
	if err == nil {
		t.Fatal("SendDiffReview should return an error when send-keys fails")
	}
	if got := app.GetInputHistory(); len(got) != 0 {
		t.Fatalf("send failure must not record input history, got %d entries", len(got))
	}
}

func TestSendDiffReview_RouterNotInitialized(t *testing.T) {
	mgr := tmux.NewSessionManager()
	t.Cleanup(mgr.Close)
	_, pane, err := mgr.CreateSession("test", "bash", 80, 24)
	if err != nil {
		t.Fatal(err)
	}

	app := &App{
		sessions: mgr,
		sendKeys: callRecorder(&[]string{}),
	}

	err = app.SendDiffReview(fmt.Sprintf("%%%d", pane.ID), "# Review")
	if !errors.Is(err, errRouterNotInitialized) {
		t.Fatalf("SendDiffReview should fail with errRouterNotInitialized, got: %v", err)
	}
	if got := app.GetInputHistory(); len(got) != 0 {
		t.Fatalf("router failure must not record input history, got %d entries", len(got))
	}
}

func TestSendDiffReview_EndToEnd(t *testing.T) {
	stubRuntimeEventsEmit(t)
	mgr := tmux.NewSessionManager()
	t.Cleanup(mgr.Close)
	_, pane, err := mgr.CreateSession("test", "bash", 80, 24)
	if err != nil {
		t.Fatal(err)
	}

	var calls []string
	app := &App{
		sessions: mgr,
		router:   tmux.NewCommandRouter(mgr, nil, tmux.RouterOptions{}),
		sendKeys: callRecorder(&calls),
	}
	app.setRuntimeContext(context.Background())
	t.Cleanup(func() {
		if app.inputHistoryService == nil {
			return
		}
		app.flushAllLineBuffers()
		app.closeInputHistory()
	})

	err = app.SendDiffReview(fmt.Sprintf("%%%d", pane.ID), "# Review\n\n> fix this")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The expected text call is transport-compensated. Input history below
	// remains in the logical user-authored order.
	wantCalls := []string{"select-pane:%0", "text:> fix this\n\n# Review", "key:C-m"}
	if !slices.Equal(calls, wantCalls) {
		t.Fatalf("send call sequence = %v, want %v", calls, wantCalls)
	}

	hasText := false
	hasEnter := false
	for _, c := range calls {
		switch {
		case c == "key:C-m":
			hasEnter = true
		case strings.HasPrefix(c, "text:"):
			hasText = true
		}
	}
	if !hasText {
		t.Error("missing text chunk in call sequence")
	}
	if !hasEnter {
		t.Error("missing Enter (C-m) in call sequence")
	}

	app.flushAllLineBuffers()
	history := app.GetInputHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 input history entry, got %d", len(history))
	}
	if history[0].Input != "# Review> fix this" {
		t.Errorf("input history text = %q, want %q", history[0].Input, "# Review> fix this")
	}
	if history[0].Source != "diff-review" {
		t.Errorf("input history source = %q, want %q", history[0].Source, "diff-review")
	}
	if history[0].Session != "test" {
		t.Errorf("input history session = %q, want %q", history[0].Session, "test")
	}
}

func TestReverseLinesForLiteralSendKeysWorkaround(t *testing.T) {
	// These expectations describe the Diff Review transport workaround, not
	// the logical Markdown document. If the receiving TUI changes its literal
	// multiline display behavior, this contract and the manual verification
	// note in knowledge/diff-review-pane-send-order.md must be revisited.
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "single line unchanged",
			text: "# Overall Comment",
			want: "# Overall Comment",
		},
		{
			name: "multiline markdown reversed for terminal display compensation",
			text: strings.Join([]string{
				"# Overall Comment",
				"",
				"ping 8.8.8.8",
				"",
				"---",
				"",
				"# Code Review Comments",
				"",
				"---",
				"",
				"te",
				"",
				"---",
				"",
				"## `readme.md` (L+4 to L+6)",
				"",
				"```md",
				"ハロー7",
				"ハロー6",
				"Hello 5",
				"```",
			}, "\n"),
			want: strings.Join([]string{
				"```",
				"Hello 5",
				"ハロー6",
				"ハロー7",
				"```md",
				"",
				"## `readme.md` (L+4 to L+6)",
				"",
				"---",
				"",
				"te",
				"",
				"---",
				"",
				"# Code Review Comments",
				"",
				"---",
				"",
				"ping 8.8.8.8",
				"",
				"# Overall Comment",
			}, "\n"),
		},
		{
			name: "normalizes CRLF before reversing",
			text: "one\r\ntwo\r\nthree",
			want: "three\ntwo\none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reverseLinesForLiteralSendKeysWorkaround(tt.text); got != tt.want {
				t.Fatalf("reverseLinesForLiteralSendKeysWorkaround() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReverseLinesForLiteralSendKeysWorkaround_RestoresObservedMarkdownDisplay(t *testing.T) {
	logicalMarkdown := strings.Join([]string{
		"# Overall Comment",
		"",
		"Please check the fenced block.",
		"",
		"---",
		"",
		"# Code Review Comments",
		"",
		"## `README.md` (L+4 to L+6)",
		"```md",
		"Hello 5",
		"ハロー6",
		"ハロー7",
		"```",
		"> Keep this order.",
	}, "\n")

	transportText := reverseLinesForLiteralSendKeysWorkaround(logicalMarkdown)
	if strings.Contains(transportText, "```md\nHello 5\nハロー6\nハロー7\n```") {
		t.Fatal("transport text should expose the fenced block in reversed order until the TUI display compensation is applied")
	}
	observedDisplay := reverseLinesForLiteralSendKeysWorkaround(transportText)
	if observedDisplay != logicalMarkdown {
		t.Fatalf("observed display = %q, want logical Markdown %q", observedDisplay, logicalMarkdown)
	}
}

func TestSendDiffReview_WorkaroundRunsBeforeChunking(t *testing.T) {
	mgr := tmux.NewSessionManager()
	t.Cleanup(mgr.Close)
	_, pane, err := mgr.CreateSession("test", "bash", 80, 24)
	if err != nil {
		t.Fatal(err)
	}

	var calls []string
	app := &App{
		sessions: mgr,
		router:   tmux.NewCommandRouter(mgr, nil, tmux.RouterOptions{}),
		sendKeys: callRecorder(&calls),
	}
	text := strings.Repeat("a", sendKeysLiteralChunkSize-1) + "\n" + "tail"
	wantSentText := reverseLinesForLiteralSendKeysWorkaround(text)

	if err := app.SendDiffReview(fmt.Sprintf("%%%d", pane.ID), text); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var chunks []string
	for _, call := range calls {
		if after, ok := strings.CutPrefix(call, "text:"); ok {
			chunks = append(chunks, after)
		}
	}
	if len(chunks) < 2 {
		t.Fatalf("expected chunked text calls, got calls: %v", calls)
	}
	if got := strings.Join(chunks, ""); got != wantSentText {
		t.Fatalf("sent chunks joined = %q, want %q", got, wantSentText)
	}
}
