package tmux

import (
	"testing"
)

func TestSplitTmuxCommands(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single command",
			input: "list-sessions",
			want:  []string{"list-sessions"},
		},
		{
			name:  "two commands",
			input: "list-sessions; list-windows",
			want:  []string{"list-sessions", " list-windows"},
		},
		{
			name:  "semicolon inside double quotes",
			input: `send-keys "echo a;b"`,
			want:  []string{`send-keys "echo a;b"`},
		},
		{
			name:  "semicolon inside single quotes",
			input: "send-keys 'echo a;b'",
			want:  []string{"send-keys 'echo a;b'"},
		},
		{
			name:  "mixed quoted and unquoted semicolons",
			input: `send-keys "a;b"; list-sessions`,
			want:  []string{`send-keys "a;b"`, " list-sessions"},
		},
		{
			name:  "empty parts preserved",
			input: ";;list-sessions;;",
			want:  []string{"", "", "list-sessions", ""},
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitTmuxCommands(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitTmuxCommands(%q) returned %d parts, want %d\ngot:  %q\nwant: %q",
					tt.input, len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("part[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTokenizeTmuxCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple command",
			input: "list-sessions",
			want:  []string{"list-sessions"},
		},
		{
			name:  "command with flags",
			input: "has-session -t test",
			want:  []string{"has-session", "-t", "test"},
		},
		{
			name:  "double quoted argument",
			input: `send-keys "echo hello world"`,
			want:  []string{"send-keys", "echo hello world"},
		},
		{
			name:  "single quoted argument",
			input: "send-keys 'echo hello world'",
			want:  []string{"send-keys", "echo hello world"},
		},
		{
			name:  "quoted semicolon preserved",
			input: `send-keys "echo a;b"`,
			want:  []string{"send-keys", "echo a;b"},
		},
		{
			name:  "multiple flags and args",
			input: "split-window -h -t %1 -c /tmp",
			want:  []string{"split-window", "-h", "-t", "%1", "-c", "/tmp"},
		},
		{
			name:  "extra whitespace",
			input: "  list-sessions   -F   fmt  ",
			want:  []string{"list-sessions", "-F", "fmt"},
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenizeTmuxCommand(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("tokenizeTmuxCommand(%q) returned %d tokens, want %d\ngot:  %q\nwant: %q",
					tt.input, len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("token[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseTmuxCommandLine(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantCommand string
		wantFlags   map[string]any
		wantArgs    []string
	}{
		{
			name:        "simple command no flags",
			input:       "list-sessions",
			wantCommand: "list-sessions",
			wantFlags:   map[string]any{},
			wantArgs:    []string{},
		},
		{
			name:        "command with string flag",
			input:       "has-session -t test",
			wantCommand: "has-session",
			wantFlags:   map[string]any{"-t": "test"},
			wantArgs:    []string{},
		},
		{
			name:        "command with bool flag",
			input:       "split-window -h -t %1",
			wantCommand: "split-window",
			wantFlags:   map[string]any{"-h": true, "-t": "%1"},
			wantArgs:    []string{},
		},
		{
			name:        "command with flags and positional args",
			input:       "send-keys -t %1 hello world",
			wantCommand: "send-keys",
			wantFlags:   map[string]any{"-t": "%1"},
			wantArgs:    []string{"hello", "world"},
		},
		{
			name:        "unknown command passes all as args",
			input:       "unknown-cmd -x foo bar",
			wantCommand: "unknown-cmd",
			wantFlags:   map[string]any{},
			wantArgs:    []string{"-x", "foo", "bar"},
		},
		{
			name:        "empty input",
			input:       "",
			wantCommand: "",
			wantFlags:   map[string]any{},
			wantArgs:    nil,
		},
		{
			name:        "paste-buffer with separator flag",
			input:       `paste-buffer -s ", " -t %1`,
			wantCommand: "paste-buffer",
			wantFlags:   map[string]any{"-s": ", ", "-t": "%1"},
			wantArgs:    []string{},
		},
		{
			name:        "list-panes with bool and string flags",
			input:       "list-panes -a -f #{session_name}",
			wantCommand: "list-panes",
			wantFlags:   map[string]any{"-a": true, "-f": "#{session_name}"},
			wantArgs:    []string{},
		},
		{
			name:        "string flag at end with no value is silently ignored",
			input:       "has-session -t",
			wantCommand: "has-session",
			wantFlags:   map[string]any{},
			wantArgs:    []string{},
		},
		{
			name:        "select-pane accepts pane style flag",
			input:       "select-pane -P bg=default,fg=colour33",
			wantCommand: "select-pane",
			wantFlags:   map[string]any{"-P": "bg=default,fg=colour33"},
			wantArgs:    []string{},
		},
		{
			name:        "new-session with -e env flag",
			input:       "new-session -e FOO=bar -s test",
			wantCommand: "new-session",
			wantFlags:   map[string]any{"-e": "FOO=bar", "-s": "test"},
			wantArgs:    []string{},
		},
		{
			name:        "select-layout parses target and preset",
			input:       "select-layout -t demo:0 main-vertical",
			wantCommand: "select-layout",
			wantFlags:   map[string]any{"-t": "demo:0"},
			wantArgs:    []string{"main-vertical"},
		},
		{
			name:        "set-option parses scope flags",
			input:       "set-option -p -t %1 pane-active-border-style bg=default,fg=colour33",
			wantCommand: "set-option",
			wantFlags:   map[string]any{"-p": true, "-t": "%1"},
			wantArgs:    []string{"pane-active-border-style", "bg=default,fg=colour33"},
		},
		{
			name:        "set-option parses format flag as bool",
			input:       "set-option -F -g status-left '#{session_name}'",
			wantCommand: "set-option",
			wantFlags:   map[string]any{"-F": true, "-g": true},
			wantArgs:    []string{"status-left", "#{session_name}"},
		},
		{
			name:        "save-buffer with append and named buffer",
			input:       "save-buffer -a -b clip out.txt",
			wantCommand: "save-buffer",
			wantFlags:   map[string]any{"-a": true, "-b": "clip"},
			wantArgs:    []string{"out.txt"},
		},
		{
			name:        "empty quoted string is ignored",
			input:       `send-keys -t %1 ""`,
			wantCommand: "send-keys",
			wantFlags:   map[string]any{"-t": "%1"},
			wantArgs:    []string{},
		},
		{
			name:        "send-keys with -W typewriter flag",
			input:       `send-keys -W -t %3 hello Enter`,
			wantCommand: "send-keys",
			wantFlags:   map[string]any{"-W": true, "-t": "%3"},
			wantArgs:    []string{"hello", "Enter"},
		},
		{
			name:        "send-keys with -N CRLF flag",
			input:       `send-keys -N -t %5 test Enter`,
			wantCommand: "send-keys",
			wantFlags:   map[string]any{"-N": true, "-t": "%5"},
			wantArgs:    []string{"test", "Enter"},
		},
		{
			name:        "send-keys with -N and -W flags combined",
			input:       `send-keys -N -W -t %5 hello`,
			wantCommand: "send-keys",
			wantFlags:   map[string]any{"-N": true, "-W": true, "-t": "%5"},
			wantArgs:    []string{"hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTmuxCommandLine(tt.input)

			if got.Command != tt.wantCommand {
				t.Errorf("Command = %q, want %q", got.Command, tt.wantCommand)
			}

			if len(got.Flags) != len(tt.wantFlags) {
				t.Errorf("Flags count = %d, want %d\ngot:  %v\nwant: %v",
					len(got.Flags), len(tt.wantFlags), got.Flags, tt.wantFlags)
			} else {
				for k, wantV := range tt.wantFlags {
					gotV, ok := got.Flags[k]
					if !ok {
						t.Errorf("missing flag %q", k)
					} else if gotV != wantV {
						t.Errorf("Flags[%q] = %v (%T), want %v (%T)", k, gotV, gotV, wantV, wantV)
					}
				}
			}

			if tt.wantArgs == nil {
				if got.Args != nil {
					t.Errorf("Args = %v, want nil", got.Args)
				}
			} else {
				if len(got.Args) != len(tt.wantArgs) {
					t.Fatalf("Args count = %d, want %d\ngot:  %q\nwant: %q",
						len(got.Args), len(tt.wantArgs), got.Args, tt.wantArgs)
				}
				for i := range got.Args {
					if got.Args[i] != tt.wantArgs[i] {
						t.Errorf("Args[%d] = %q, want %q", i, got.Args[i], tt.wantArgs[i])
					}
				}
			}
		})
	}
}

func TestParseTmuxCommandLineMultipleEnvFlags(t *testing.T) {
	// Multiple -e flags: map semantics means last value wins.
	// This is a known limitation of the current parser.
	got := parseTmuxCommandLine("new-session -e FOO=bar -e BAZ=qux -s test")
	if got.Command != "new-session" {
		t.Fatalf("Command = %q, want %q", got.Command, "new-session")
	}
	if got.Flags["-s"] != "test" {
		t.Fatalf("Flags[-s] = %v, want %q", got.Flags["-s"], "test")
	}
	// Last -e value wins due to map[string]any semantics.
	eVal, ok := got.Flags["-e"]
	if !ok {
		t.Fatal("missing -e flag")
	}
	if eVal != "BAZ=qux" {
		t.Fatalf("Flags[-e] = %v, want %q (last -e wins)", eVal, "BAZ=qux")
	}
}
