package tmux

import "testing"

// TestSelectCapturePaneLines_ReturnsErrorForInvalidFlag_ByDesign locks in the
// contract that selectCapturePaneLines returns a non-nil error when -S/-E
// cannot be parsed. The caller (handleCapturePane) RELIES on this error to
// trigger the quiet (-q) swallow branch that returns empty output.
//
// If this test fails (e.g. because the function begins returning nil data
// silently on bad input), handleCapturePane's quiet branch will no longer be
// exercised and tmux-shim's "silent empty on error" policy
// (project CLAUDE.md §tmux-shim について) will break.
//
// Design decision: /ACCEPTED_DESIGN_DECISIONS.md AD-003.
// DO NOT change this behavior without updating AD-003 and the tmux-shim
// section of the project CLAUDE.md in the same PR.
func TestSelectCapturePaneLines_ReturnsErrorForInvalidFlag_ByDesign(t *testing.T) {
	data := []byte("line1\nline2\nline3\n")

	cases := []struct {
		name      string
		startFlag any
		endFlag   any
	}{
		{"invalid start string flag", "not-a-number", nil},
		{"invalid end string flag", nil, "abc"},
		{"both flags invalid", "xx", "yy"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			out, err := selectCapturePaneLines(data, tt.startFlag, tt.endFlag)
			if err == nil {
				t.Fatalf("expected non-nil error for invalid flag to preserve the quiet-swallow contract (AD-003); got out=%q err=nil", string(out))
			}
		})
	}
}

// TestSelectCapturePaneLines_EmptyDataReturnsNoError documents the accepted
// boundary: empty input is a non-error condition. Combined with the test
// above, this asserts that errors come only from invalid flags, not from
// benign empty input — preserving the shape that handleCapturePane relies on.
func TestSelectCapturePaneLines_EmptyDataReturnsNoError(t *testing.T) {
	out, err := selectCapturePaneLines(nil, nil, nil)
	if err != nil {
		t.Fatalf("empty data must not error; got %v", err)
	}
	if out != nil {
		t.Fatalf("empty data must return nil slice; got %q", string(out))
	}
}
