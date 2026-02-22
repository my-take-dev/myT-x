package tmux

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

// newTestFixture creates a fully wired session/window/pane graph for format tests.
func newTestFixture() (*TmuxSession, *TmuxWindow, *TmuxPane) {
	session := &TmuxSession{
		ID:             0,
		Name:           "claude-swarm",
		CreatedAt:      time.Unix(1706745600, 0),
		ActiveWindowID: 0,
		Env:            map[string]string{},
	}
	window := &TmuxWindow{
		ID:       0,
		Name:     "main",
		Session:  session,
		ActivePN: 0,
	}
	pane := &TmuxPane{
		ID:     3,
		Index:  1,
		Width:  120,
		Height: 30,
		Active: true,
		Window: window,
		Env:    map[string]string{},
	}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}
	return session, window, pane
}

func TestExpandFormatPaneVars(t *testing.T) {
	_, _, pane := newTestFixture()

	got := expandFormat("#{session_name} #{pane_id} #{pane_tty} #{pane_active}", pane)
	if !strings.Contains(got, "claude-swarm %3") {
		t.Fatalf("unexpected expanded format: %q", got)
	}
	if !strings.Contains(got, `\\.\conpty\%3`) {
		t.Fatalf("missing pane_tty: %q", got)
	}
	if !strings.HasSuffix(got, "1") {
		t.Fatalf("missing active flag: %q", got)
	}
}

func TestLookupFormatVariableAllVariables(t *testing.T) {
	session := &TmuxSession{
		ID:             10,
		Name:           "test-session",
		CreatedAt:      time.Unix(1706745600, 0),
		ActiveWindowID: 5,
		Env:            map[string]string{},
	}
	window := &TmuxWindow{
		ID:       5,
		Name:     "editor",
		Session:  session,
		ActivePN: 0,
	}
	pane := &TmuxPane{
		ID:     7,
		Index:  2,
		Width:  200,
		Height: 50,
		Active: true,
		Window: window,
		Env:    map[string]string{},
	}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}

	tests := []struct {
		name     string
		variable string
		want     string
	}{
		// --- pane variables ---
		{name: "pane_id", variable: "pane_id", want: "%7"},
		{name: "pane_index", variable: "pane_index", want: "2"},
		{name: "pane_width", variable: "pane_width", want: "200"},
		{name: "pane_height", variable: "pane_height", want: "50"},
		{name: "pane_active when active", variable: "pane_active", want: "1"},
		{name: "pane_tty", variable: "pane_tty", want: `\\.\conpty\%7`},
		{name: "pane_active_suffix when active", variable: "pane_active_suffix", want: " (active)"},
		// --- window variables ---
		{name: "window_index returns stable window ID", variable: "window_index", want: "5"},
		{name: "window_name", variable: "window_name", want: "editor"},
		{name: "window_panes", variable: "window_panes", want: "1"},
		{name: "window_active when active", variable: "window_active", want: "1"},
		// --- session variables ---
		{name: "session_name", variable: "session_name", want: "test-session"},
		{name: "session_windows", variable: "session_windows", want: "1"},
		{name: "session_created", variable: "session_created", want: strconv.FormatInt(session.CreatedAt.Unix(), 10)},
		{name: "session_created_human", variable: "session_created_human", want: session.CreatedAt.Format("Mon Jan _2 15:04:05 2006")},
		// --- unknown variable ---
		{name: "unknown_var returns empty string", variable: "unknown_var", want: ""},
		{name: "completely_bogus returns empty string", variable: "completely_bogus", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lookupFormatVariable(tt.variable, pane)
			if got != tt.want {
				t.Fatalf("lookupFormatVariable(%q) = %q, want %q", tt.variable, got, tt.want)
			}
		})
	}
}

func TestLookupFormatVariableInactivePane(t *testing.T) {
	session := &TmuxSession{
		ID:             10,
		Name:           "demo",
		ActiveWindowID: 5,
		Env:            map[string]string{},
	}
	window := &TmuxWindow{
		ID:       5,
		Name:     "main",
		Session:  session,
		ActivePN: 0,
	}
	pane := &TmuxPane{
		ID:     7,
		Index:  0,
		Width:  80,
		Height: 24,
		Active: false,
		Window: window,
		Env:    map[string]string{},
	}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}

	tests := []struct {
		name     string
		variable string
		want     string
	}{
		{name: "pane_active when inactive", variable: "pane_active", want: "0"},
		{name: "pane_active_suffix when inactive", variable: "pane_active_suffix", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lookupFormatVariable(tt.variable, pane)
			if got != tt.want {
				t.Fatalf("lookupFormatVariable(%q) = %q, want %q", tt.variable, got, tt.want)
			}
		})
	}
}

func TestLookupFormatVariableWindowActive(t *testing.T) {
	session := &TmuxSession{ID: 10, Name: "demo", ActiveWindowID: 2}
	windowA := &TmuxWindow{ID: 1, Name: "alpha", Session: session, ActivePN: 0}
	windowB := &TmuxWindow{ID: 2, Name: "beta", Session: session, ActivePN: 0}
	paneA := &TmuxPane{ID: 11, Index: 0, Active: true, Window: windowA}
	paneB := &TmuxPane{ID: 12, Index: 0, Active: true, Window: windowB}
	windowA.Panes = []*TmuxPane{paneA}
	windowB.Panes = []*TmuxPane{paneB}
	session.Windows = []*TmuxWindow{windowA, windowB}

	if got := lookupFormatVariable("window_active", paneA); got != "0" {
		t.Fatalf("window_active(alpha) = %q, want %q", got, "0")
	}
	if got := lookupFormatVariable("window_active", paneB); got != "1" {
		t.Fatalf("window_active(beta) = %q, want %q", got, "1")
	}
}

func TestLookupFormatVariableNilPane(t *testing.T) {
	// When pane is nil, string variables return "" and numeric variables return "0".
	tests := []struct {
		name     string
		variable string
		want     string
	}{
		{name: "session_name", variable: "session_name", want: ""},
		{name: "window_name", variable: "window_name", want: ""},
		{name: "pane_id", variable: "pane_id", want: ""},
		{name: "pane_tty", variable: "pane_tty", want: ""},
		{name: "session_windows", variable: "session_windows", want: "0"},
		{name: "window_index", variable: "window_index", want: "0"},
		{name: "window_panes", variable: "window_panes", want: "0"},
		{name: "window_active", variable: "window_active", want: "0"},
		{name: "pane_index", variable: "pane_index", want: "0"},
		{name: "pane_width", variable: "pane_width", want: "0"},
		{name: "pane_height", variable: "pane_height", want: "0"},
		{name: "pane_active", variable: "pane_active", want: "0"},
		{name: "session_created", variable: "session_created", want: "0"},
		{name: "pane_active_suffix", variable: "pane_active_suffix", want: ""},
		{name: "unknown_var", variable: "unknown_var", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lookupFormatVariable(tt.variable, nil)
			if got != tt.want {
				t.Fatalf("lookupFormatVariable(%q, nil) = %q, want %q", tt.variable, got, tt.want)
			}
		})
	}
}

func TestLookupFormatVariableNilWindow(t *testing.T) {
	// Pane with nil Window: window-related variables return defaults.
	pane := &TmuxPane{
		ID:     1,
		Index:  0,
		Width:  80,
		Height: 24,
		Active: true,
		Window: nil,
		Env:    map[string]string{},
	}

	tests := []struct {
		name     string
		variable string
		want     string
	}{
		{name: "window_index nil window", variable: "window_index", want: "0"},
		{name: "window_name nil window", variable: "window_name", want: ""},
		{name: "window_panes nil window", variable: "window_panes", want: "0"},
		{name: "window_active nil window", variable: "window_active", want: "0"},
		{name: "session_name nil window", variable: "session_name", want: ""},
		{name: "session_windows nil window", variable: "session_windows", want: "0"},
		{name: "session_created nil window", variable: "session_created", want: "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lookupFormatVariable(tt.variable, pane)
			if got != tt.want {
				t.Fatalf("lookupFormatVariable(%q, pane-with-nil-window) = %q, want %q", tt.variable, got, tt.want)
			}
		})
	}
}

func TestLookupFormatVariableNilSession(t *testing.T) {
	// Window with nil Session: session-related variables return defaults.
	window := &TmuxWindow{
		ID:       1,
		Name:     "main",
		Session:  nil,
		ActivePN: 0,
	}
	pane := &TmuxPane{
		ID:     0,
		Index:  0,
		Width:  80,
		Height: 24,
		Active: true,
		Window: window,
		Env:    map[string]string{},
	}
	window.Panes = []*TmuxPane{pane}

	tests := []struct {
		name     string
		variable string
		want     string
	}{
		{name: "session_name nil session", variable: "session_name", want: ""},
		{name: "session_windows nil session", variable: "session_windows", want: "0"},
		{name: "session_created nil session", variable: "session_created", want: "0"},
		{name: "window_active nil session", variable: "window_active", want: "0"},
		{name: "window_name with nil session", variable: "window_name", want: "main"},
		{name: "window_panes with nil session", variable: "window_panes", want: "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lookupFormatVariable(tt.variable, pane)
			if got != tt.want {
				t.Fatalf("lookupFormatVariable(%q, pane-with-nil-session) = %q, want %q", tt.variable, got, tt.want)
			}
		})
	}
}

func TestLookupFormatVariableSessionCreatedHumanNilSession(t *testing.T) {
	// Ensure session_created_human returns epoch time format for nil session.
	window := &TmuxWindow{ID: 0, Name: "w", Session: nil}
	pane := &TmuxPane{ID: 0, Window: window}

	got := lookupFormatVariable("session_created_human", pane)
	want := time.Unix(0, 0).Format("Mon Jan _2 15:04:05 2006")
	if got != want {
		t.Fatalf("session_created_human(nil session) = %q, want %q", got, want)
	}
}

func TestExpandFormatMultiplePlaceholders(t *testing.T) {
	_, _, pane := newTestFixture()

	tests := []struct {
		name   string
		format string
		want   string
	}{
		{
			name:   "two placeholders",
			format: "#{session_name} #{window_name}",
			want:   "claude-swarm main",
		},
		{
			name:   "three placeholders with separators",
			format: "#{session_name}:#{window_index}.#{pane_index}",
			want:   "claude-swarm:0.1",
		},
		{
			name:   "repeated same placeholder",
			format: "#{pane_id} #{pane_id}",
			want:   "%3 %3",
		},
		{
			name:   "placeholder in the middle of text",
			format: "prefix-#{window_name}-suffix",
			want:   "prefix-main-suffix",
		},
		{
			name:   "all pane variables in one string",
			format: "#{pane_id} #{pane_index} #{pane_width}x#{pane_height} #{pane_active}",
			want:   "%3 1 120x30 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandFormat(tt.format, pane)
			if got != tt.want {
				t.Fatalf("expandFormat(%q) = %q, want %q", tt.format, got, tt.want)
			}
		})
	}
}

func TestExpandFormatEmptyString(t *testing.T) {
	_, _, pane := newTestFixture()

	got := expandFormat("", pane)
	if got != "" {
		t.Fatalf("expandFormat(\"\") = %q, want empty string", got)
	}
}

func TestExpandFormatNoPlaceholders(t *testing.T) {
	_, _, pane := newTestFixture()

	got := expandFormat("plain text no vars", pane)
	if got != "plain text no vars" {
		t.Fatalf("expandFormat(plain) = %q, want %q", got, "plain text no vars")
	}
}

func TestExpandFormatUnknownVariable(t *testing.T) {
	_, _, pane := newTestFixture()

	// Unknown variables should be replaced with empty string.
	got := expandFormat("before-#{unknown_var}-after", pane)
	if got != "before--after" {
		t.Fatalf("expandFormat(unknown_var) = %q, want %q", got, "before--after")
	}
}

func TestExpandFormatNilPane(t *testing.T) {
	got := expandFormat("#{session_name}:#{window_index}.#{pane_index}", nil)
	if got != ":0.0" {
		t.Fatalf("expandFormat(nil pane) = %q, want %q", got, ":0.0")
	}
}

func TestExpandFormatWindowIndex(t *testing.T) {
	// #{window_index} returns the stable window ID, not a positional index.
	session := &TmuxSession{
		ID:             0,
		Name:           "demo",
		ActiveWindowID: 42,
		Env:            map[string]string{},
	}
	window := &TmuxWindow{
		ID:       42,
		Name:     "main",
		Session:  session,
		ActivePN: 0,
	}
	pane := &TmuxPane{
		ID:     0,
		Index:  0,
		Width:  80,
		Height: 24,
		Active: true,
		Window: window,
		Env:    map[string]string{},
	}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}

	got := expandFormat("#{window_index}", pane)
	if got != "42" {
		t.Fatalf("#{window_index} should return stable window ID: got %q, want %q", got, "42")
	}
}

func TestExpandFormatMultipleWindows(t *testing.T) {
	session := &TmuxSession{
		ID:             0,
		Name:           "multi",
		ActiveWindowID: 1,
		Env:            map[string]string{},
	}
	w0 := &TmuxWindow{ID: 0, Name: "first", Session: session, ActivePN: 0}
	w1 := &TmuxWindow{ID: 1, Name: "second", Session: session, ActivePN: 0}
	p0 := &TmuxPane{ID: 0, Index: 0, Active: true, Window: w0, Env: map[string]string{}}
	p1 := &TmuxPane{ID: 1, Index: 0, Active: true, Window: w1, Env: map[string]string{}}
	w0.Panes = []*TmuxPane{p0}
	w1.Panes = []*TmuxPane{p1}
	session.Windows = []*TmuxWindow{w0, w1}

	// window_panes should count panes in the pane's own window, not session-wide.
	got := expandFormat("#{session_windows} #{window_panes}", p0)
	if got != "2 1" {
		t.Fatalf("session_windows/window_panes mismatch: got %q, want %q", got, "2 1")
	}
}

func TestFormatWindowLineUsesActivePaneIndex(t *testing.T) {
	session := &TmuxSession{ID: 20, Name: "demo", ActiveWindowID: 3}
	window := &TmuxWindow{ID: 3, Name: "main", Session: session, ActivePN: 1}
	pane0 := &TmuxPane{ID: 21, Index: 0, Window: window}
	pane1 := &TmuxPane{ID: 22, Index: 1, Window: window}
	window.Panes = []*TmuxPane{pane0, pane1}
	session.Windows = []*TmuxWindow{window}

	got := formatWindowLine(window, "#{window_name} #{pane_id}")
	if got != "main %22" {
		t.Fatalf("formatWindowLine() = %q, want %q", got, "main %22")
	}
}

func TestFormatWindowLineDefaultFormat(t *testing.T) {
	session := &TmuxSession{ID: 0, Name: "demo", ActiveWindowID: 0}
	window := &TmuxWindow{ID: 0, Name: "main", Session: session, ActivePN: 0}
	pane := &TmuxPane{ID: 0, Index: 0, Active: true, Window: window, Width: 80, Height: 24, Env: map[string]string{}}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}

	// Empty custom format should use defaultWindowListFormat.
	got := formatWindowLine(window, "")
	if !strings.Contains(got, "main") {
		t.Fatalf("default format should contain window name 'main': got %q", got)
	}
	if !strings.Contains(got, "1 panes") {
		t.Fatalf("default format should contain '1 panes': got %q", got)
	}
}

func TestFormatWindowLineNilWindow(t *testing.T) {
	got := formatWindowLine(nil, "#{window_name}")
	if got != "" {
		t.Fatalf("formatWindowLine(nil) = %q, want empty", got)
	}
}

func TestFormatWindowLineOutOfBoundsActivePNFallsBackToFirstPane(t *testing.T) {
	session := &TmuxSession{ID: 0, Name: "demo", ActiveWindowID: 0}
	window := &TmuxWindow{ID: 0, Name: "main", Session: session, ActivePN: 99}
	pane := &TmuxPane{ID: 5, Index: 0, Active: true, Window: window, Env: map[string]string{}}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}

	got := formatWindowLine(window, "#{pane_id}")
	if got != "%5" {
		t.Fatalf("formatWindowLine with out-of-bounds ActivePN = %q, want %%5 (first pane fallback)", got)
	}
}

func TestFormatSessionLineDefaultFormat(t *testing.T) {
	session := &TmuxSession{
		ID:             0,
		Name:           "work",
		CreatedAt:      time.Unix(1706745600, 0),
		ActiveWindowID: 0,
		Env:            map[string]string{},
	}
	window := &TmuxWindow{ID: 0, Name: "main", Session: session, ActivePN: 0}
	pane := &TmuxPane{ID: 0, Index: 0, Active: true, Window: window, Width: 80, Height: 24, Env: map[string]string{}}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}

	got := formatSessionLine(session, "")
	if !strings.Contains(got, "work") {
		t.Fatalf("default session format should contain session name 'work': got %q", got)
	}
	if !strings.Contains(got, "1 windows") {
		t.Fatalf("default session format should contain '1 windows': got %q", got)
	}
}

func TestFormatSessionLineCustomFormat(t *testing.T) {
	session := &TmuxSession{
		ID:             5,
		Name:           "custom",
		CreatedAt:      time.Unix(1706745600, 0),
		ActiveWindowID: 0,
		Env:            map[string]string{},
	}
	window := &TmuxWindow{ID: 0, Name: "main", Session: session, ActivePN: 0}
	pane := &TmuxPane{ID: 0, Index: 0, Active: true, Window: window, Width: 80, Height: 24, Env: map[string]string{}}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}

	got := formatSessionLine(session, "#{session_name}|#{session_windows}")
	if got != "custom|1" {
		t.Fatalf("formatSessionLine custom format = %q, want %q", got, "custom|1")
	}
}

func TestFormatPaneLineDefaultFormat(t *testing.T) {
	session := &TmuxSession{ID: 0, Name: "demo", ActiveWindowID: 0, Env: map[string]string{}}
	window := &TmuxWindow{ID: 0, Name: "main", Session: session, ActivePN: 0}
	pane := &TmuxPane{ID: 2, Index: 0, Active: true, Window: window, Width: 100, Height: 35, Env: map[string]string{}}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}

	got := formatPaneLine(pane, "")
	if !strings.Contains(got, "100x35") {
		t.Fatalf("default pane format should contain '100x35': got %q", got)
	}
	if !strings.Contains(got, "%2") {
		t.Fatalf("default pane format should contain pane id '%%2': got %q", got)
	}
}

func TestFormatPaneLineCustomFormat(t *testing.T) {
	session := &TmuxSession{ID: 0, Name: "demo", ActiveWindowID: 0, Env: map[string]string{}}
	window := &TmuxWindow{ID: 0, Name: "main", Session: session, ActivePN: 0}
	pane := &TmuxPane{ID: 0, Index: 0, Active: true, Window: window, Width: 80, Height: 24, Env: map[string]string{}}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}

	got := formatPaneLine(pane, "#{pane_id}:#{pane_active}")
	if got != "%0:1" {
		t.Fatalf("formatPaneLine custom = %q, want %q", got, "%0:1")
	}
}

func TestFormatPaneLineWhitespaceOnlyCustomFormat(t *testing.T) {
	session := &TmuxSession{ID: 0, Name: "demo", ActiveWindowID: 0, Env: map[string]string{}}
	window := &TmuxWindow{ID: 0, Name: "main", Session: session, ActivePN: 0}
	pane := &TmuxPane{ID: 0, Index: 0, Active: true, Window: window, Width: 80, Height: 24, Env: map[string]string{}}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}

	// Whitespace-only custom format should fall back to default.
	got := formatPaneLine(pane, "   ")
	if !strings.Contains(got, "%0") {
		t.Fatalf("whitespace-only format should use default (contain pane id): got %q", got)
	}
}

func TestJoinLines(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{name: "empty", lines: []string{}, want: ""},
		{name: "single line", lines: []string{"hello"}, want: "hello\n"},
		{name: "two lines", lines: []string{"a", "b"}, want: "a\nb\n"},
		{name: "three lines", lines: []string{"x", "y", "z"}, want: "x\ny\nz\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinLines(tt.lines)
			if got != tt.want {
				t.Fatalf("joinLines(%v) = %q, want %q", tt.lines, got, tt.want)
			}
		})
	}
}

func TestExpandFormatSessionCreatedHumanInline(t *testing.T) {
	// Verify formatSessionLine correctly resolves #{session_created_human}
	// through the unified expandFormat/lookupFormatVariable path.
	created := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	session := &TmuxSession{
		ID:             0,
		Name:           "demo",
		CreatedAt:      created,
		ActiveWindowID: 0,
		Env:            map[string]string{},
	}
	window := &TmuxWindow{ID: 0, Name: "main", Session: session, ActivePN: 0}
	pane := &TmuxPane{ID: 0, Index: 0, Active: true, Window: window, Width: 80, Height: 24, Env: map[string]string{}}
	window.Panes = []*TmuxPane{pane}
	session.Windows = []*TmuxWindow{window}

	got := formatSessionLine(session, "#{session_created_human}")
	want := created.Format("Mon Jan _2 15:04:05 2006")
	if got != want {
		t.Fatalf("formatSessionLine(session_created_human) = %q, want %q", got, want)
	}
}

func TestExpandFormatPaneIDZero(t *testing.T) {
	// Ensure pane ID 0 is correctly formatted as "%0", not omitted.
	window := &TmuxWindow{ID: 0, Name: "main"}
	pane := &TmuxPane{ID: 0, Index: 0, Window: window, Env: map[string]string{}}
	window.Panes = []*TmuxPane{pane}

	got := expandFormat("#{pane_id}", pane)
	if got != "%0" {
		t.Fatalf("pane ID 0 should format as %%0: got %q", got)
	}
}

func TestExpandFormatAdjacentPlaceholders(t *testing.T) {
	_, _, pane := newTestFixture()

	// Two placeholders with no separator between them.
	got := expandFormat("#{pane_active}#{pane_index}", pane)
	if got != "11" {
		t.Fatalf("adjacent placeholders = %q, want %q", got, "11")
	}
}

func TestLookupFormatVariableWindowPanesMultiplePanes(t *testing.T) {
	session := &TmuxSession{ID: 0, Name: "demo", ActiveWindowID: 0, Env: map[string]string{}}
	window := &TmuxWindow{ID: 0, Name: "main", Session: session, ActivePN: 0}
	panes := make([]*TmuxPane, 5)
	for i := range panes {
		panes[i] = &TmuxPane{ID: i, Index: i, Window: window, Env: map[string]string{}}
	}
	window.Panes = panes
	session.Windows = []*TmuxWindow{window}

	got := lookupFormatVariable("window_panes", panes[0])
	want := fmt.Sprintf("%d", len(panes))
	if got != want {
		t.Fatalf("window_panes with %d panes = %q, want %q", len(panes), got, want)
	}
}
