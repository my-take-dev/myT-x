package shell

import (
	"reflect"
	"runtime"
	"testing"
)

func TestParseCommandCore(t *testing.T) {
	tests := []struct {
		name          string
		cmd           string
		wantWorkDir   string
		wantExtraEnv  map[string]string
		wantCleanArgs []string
	}{
		{
			name:          "actual Claude Code agent teams format",
			cmd:           `cd 'C:\Users\test\workspace' && CLAUDECODE=1 CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 'C:\Users\test\.local\bin\claude.exe' --agent-id tech-architect@team --agent-name tech-architect`,
			wantWorkDir:   `C:\Users\test\workspace`,
			wantExtraEnv:  map[string]string{"CLAUDECODE": "1", "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"},
			wantCleanArgs: []string{`& 'C:\Users\test\.local\bin\claude.exe' --agent-id tech-architect@team --agent-name tech-architect`},
		},
		{
			name:          "cd with single quotes and simple command",
			cmd:           `cd 'C:\Projects\myapp' && claude`,
			wantWorkDir:   `C:\Projects\myapp`,
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"claude"},
		},
		{
			name:          "cd with double quotes",
			cmd:           `cd "C:\Projects\my app" && claude --flag`,
			wantWorkDir:   `C:\Projects\my app`,
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"claude --flag"},
		},
		{
			name:          "cd with quoted exe path",
			cmd:           `cd '/tmp/workspace' && '/usr/bin/claude' --flag`,
			wantWorkDir:   "/tmp/workspace",
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"& '/usr/bin/claude' --flag"},
		},
		{
			name:          "cd unquoted path",
			cmd:           "cd /tmp && cmd",
			wantWorkDir:   "/tmp",
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"cmd"},
		},
		{
			name:          "env vars only",
			cmd:           "FOO=bar BAZ=qux claude",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{"FOO": "bar", "BAZ": "qux"},
			wantCleanArgs: []string{"claude"},
		},
		{
			name:          "env var with quoted exe",
			cmd:           `MY_VAR=1 'C:\path\to\exe' --arg`,
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{"MY_VAR": "1"},
			wantCleanArgs: []string{`& 'C:\path\to\exe' --arg`},
		},
		{
			name:          "no transformation needed",
			cmd:           "claude",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"claude"},
		},
		{
			name:          "empty string",
			cmd:           "",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: nil,
		},
		{
			name:          "whitespace only",
			cmd:           "   ",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: nil,
		},
		{
			name:          "fallback: && replaced with ; when no cd pattern",
			cmd:           "echo foo && echo bar",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"echo foo ; echo bar"},
		},
		{
			name:          "fallback: multiple && replaced",
			cmd:           "cmd1 && cmd2 && cmd3",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"cmd1 ; cmd2 ; cmd3"},
		},
		{
			name:          "cd extracted removes && from rest so no fallback needed",
			cmd:           `cd 'C:\path' && echo hello`,
			wantWorkDir:   `C:\path`,
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"echo hello"},
		},
		{
			name:          "command with flags not mistaken for env vars",
			cmd:           "claude --agent-id foo --flag bar",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"claude --agent-id foo --flag bar"},
		},
		{
			name:          "env var with empty value",
			cmd:           "FOO= claude",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{"FOO": ""},
			wantCleanArgs: []string{"claude"},
		},
		{
			name:          "cd with unclosed single quote returns no match",
			cmd:           "cd 'unclosed && rest",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"cd 'unclosed ; rest"},
		},
		{
			name:          "cd without && separator",
			cmd:           "cd /tmp ; ls",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"cd /tmp ; ls"},
		},
		{
			name:          "env var value contains equals",
			cmd:           "FOO=bar=baz claude",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{"FOO": "bar=baz"},
			wantCleanArgs: []string{"claude"},
		},
		{
			name:          "cd then remaining && after extraction",
			cmd:           `cd 'C:\path' && cmd1 && cmd2`,
			wantWorkDir:   `C:\path`,
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"cmd1 ; cmd2"},
		},
		{
			name:          "env prefix with KEY=VALUE pairs",
			cmd:           `cd 'C:\workspace' && env CLAUDECODE=1 CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 'C:\bin\claude.exe' --flag`,
			wantWorkDir:   `C:\workspace`,
			wantExtraEnv:  map[string]string{"CLAUDECODE": "1", "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS": "1"},
			wantCleanArgs: []string{`& 'C:\bin\claude.exe' --flag`},
		},
		{
			name:          "env prefix without cd",
			cmd:           `env CLAUDECODE=1 'C:\bin\claude.exe' --flag`,
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{"CLAUDECODE": "1"},
			wantCleanArgs: []string{`& 'C:\bin\claude.exe' --flag`},
		},
		{
			name:          "env as command name not stripped",
			cmd:           "env --version",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"env --version"},
		},
		{
			name:          "env followed by non-assignment token",
			cmd:           "env mycommand --flag",
			wantWorkDir:   "",
			wantExtraEnv:  map[string]string{},
			wantCleanArgs: []string{"env mycommand --flag"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCommandCore(tt.cmd)
			if got.WorkDir != tt.wantWorkDir {
				t.Errorf("WorkDir = %q, want %q", got.WorkDir, tt.wantWorkDir)
			}
			if !reflect.DeepEqual(got.ExtraEnv, tt.wantExtraEnv) {
				t.Errorf("ExtraEnv = %v, want %v", got.ExtraEnv, tt.wantExtraEnv)
			}
			if !equalStringSlice(got.CleanArgs, tt.wantCleanArgs) {
				t.Errorf("CleanArgs = %v, want %v", got.CleanArgs, tt.wantCleanArgs)
			}
		})
	}
}

func TestExtractCDPath(t *testing.T) {
	tests := []struct {
		name     string
		afterCD  string
		wantPath string
		wantRest string
		wantOK   bool
	}{
		{
			name:     "single quoted path",
			afterCD:  `'C:\Users\test' && rest`,
			wantPath: `C:\Users\test`,
			wantRest: "rest",
			wantOK:   true,
		},
		{
			name:     "double quoted path",
			afterCD:  `"C:\Users\test" && rest`,
			wantPath: `C:\Users\test`,
			wantRest: "rest",
			wantOK:   true,
		},
		{
			name:     "unquoted path",
			afterCD:  "/tmp && rest",
			wantPath: "/tmp",
			wantRest: "rest",
			wantOK:   true,
		},
		{
			name:     "path with spaces in quotes",
			afterCD:  `'C:\My Documents\test' && cmd`,
			wantPath: `C:\My Documents\test`,
			wantRest: "cmd",
			wantOK:   true,
		},
		{
			name:     "empty after cd",
			afterCD:  "",
			wantPath: "",
			wantRest: "",
			wantOK:   false,
		},
		{
			name:     "no && separator",
			afterCD:  "/tmp ; rest",
			wantPath: "",
			wantRest: "",
			wantOK:   false,
		},
		{
			name:     "unclosed single quote",
			afterCD:  "'unclosed && rest",
			wantPath: "",
			wantRest: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, rest, ok := extractCDPath(tt.afterCD)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			if ok && rest != tt.wantRest {
				t.Errorf("rest = %q, want %q", rest, tt.wantRest)
			}
		})
	}
}

func TestIsEnvVarName(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"uppercase", "FOO", true},
		{"with underscore", "CLAUDE_CODE_VAR", true},
		{"with digits", "VAR123", true},
		{"starts with underscore", "_VAR", true},
		{"lowercase", "foo", true},
		{"mixed case", "myVar", true},
		{"starts with digit", "1VAR", false},
		{"contains hyphen", "MY-VAR", false},
		{"empty", "", false},
		{"contains dot", "MY.VAR", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEnvVarName(tt.s); got != tt.want {
				t.Errorf("isEnvVarName(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestAddCallOperatorIfNeeded(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{"single quoted path", `'C:\path\to\exe' --flag`, `& 'C:\path\to\exe' --flag`},
		{"double quoted path", `"C:\path\to\exe" --flag`, `& "C:\path\to\exe" --flag`},
		{"unquoted command", "claude --flag", "claude --flag"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := addCallOperatorIfNeeded(tt.cmd); got != tt.want {
				t.Errorf("addCallOperatorIfNeeded(%q) = %q, want %q", tt.cmd, got, tt.want)
			}
		})
	}
}

func TestTranslateBashToPowerShell(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want string
	}{
		{
			name: "actual Claude Code agent teams format",
			cmd:  `cd 'C:\Users\test\workspace' && CLAUDECODE=1 CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 'C:\Users\test\.local\bin\claude.exe' --agent-id tech@team --agent-name tech`,
			want: `cd 'C:\Users\test\workspace'; $env:CLAUDECODE='1'; $env:CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS='1'; & 'C:\Users\test\.local\bin\claude.exe' --agent-id tech@team --agent-name tech`,
		},
		{
			name: "cd and simple command",
			cmd:  `cd 'C:\path' && claude`,
			want: `cd 'C:\path'; claude`,
		},
		{
			name: "no transformation needed",
			cmd:  "claude --flag",
			want: "claude --flag",
		},
		{
			name: "env vars only",
			cmd:  "FOO=bar BAZ=qux cmd",
			want: "$env:BAZ='qux'; $env:FOO='bar'; cmd",
		},
		{
			name: "env prefix full agent teams command",
			cmd:  `cd 'C:\workspace' && env CLAUDECODE=1 CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1 'C:\bin\claude.exe' --agent-id tech@team`,
			want: `cd 'C:\workspace'; $env:CLAUDECODE='1'; $env:CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS='1'; & 'C:\bin\claude.exe' --agent-id tech@team`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := translateBashToPowerShell(tt.cmd)
			if got != tt.want {
				t.Errorf("translateBashToPowerShell()\n  got  = %q\n  want = %q", got, tt.want)
			}
		})
	}
}

func TestTranslateSendKeysArgsIntegration(t *testing.T) {
	args := []string{
		`cd 'C:\path' && CLAUDECODE=1 'C:\bin\claude.exe' --flag`,
		"Enter",
	}

	result := TranslateSendKeysArgs(args)
	if len(result) != 2 {
		t.Fatalf("expected 2 args, got %d", len(result))
	}
	if result[1] != "Enter" {
		t.Errorf("Enter arg should be preserved, got %q", result[1])
	}
	if runtime.GOOS == "windows" {
		want := `cd 'C:\path'; $env:CLAUDECODE='1'; & 'C:\bin\claude.exe' --flag`
		if result[0] != want {
			t.Errorf("translated command = %q, want %q", result[0], want)
		}
		return
	}
	if result[0] != args[0] {
		t.Errorf("non-windows command should be unchanged: got %q, want %q", result[0], args[0])
	}
}

func TestParseUnixCommandPublicAPI(t *testing.T) {
	args := []string{
		`cd 'C:\workspace' && CLAUDECODE=1 'C:\bin\claude.exe' --resume abc`,
	}

	parsed := ParseUnixCommand(args, `C:\fallback`)
	if runtime.GOOS != "windows" {
		if parsed.WorkDir != "" {
			t.Fatalf("WorkDir = %q, want empty on non-windows", parsed.WorkDir)
		}
		if len(parsed.ExtraEnv) != 0 {
			t.Fatalf("ExtraEnv = %v, want empty on non-windows", parsed.ExtraEnv)
		}
		if !reflect.DeepEqual(parsed.CleanArgs, args) {
			t.Fatalf("CleanArgs = %v, want unchanged args %v", parsed.CleanArgs, args)
		}
		return
	}

	if parsed.WorkDir != `C:\workspace` {
		t.Fatalf("WorkDir = %q, want %q", parsed.WorkDir, `C:\workspace`)
	}
	if parsed.ExtraEnv["CLAUDECODE"] != "1" {
		t.Fatalf("CLAUDECODE = %q, want 1", parsed.ExtraEnv["CLAUDECODE"])
	}
	if len(parsed.CleanArgs) != 1 {
		t.Fatalf("CleanArgs length = %d, want 1", len(parsed.CleanArgs))
	}
	want := `& 'C:\bin\claude.exe' --resume abc`
	if parsed.CleanArgs[0] != want {
		t.Fatalf("CleanArgs[0] = %q, want %q", parsed.CleanArgs[0], want)
	}
}

func TestParseUnixCommandUsesCurrentWorkDirFallback(t *testing.T) {
	args := []string{"claude --resume abc"}
	parsed := ParseUnixCommand(args, `C:\existing`)

	if runtime.GOOS != "windows" {
		if parsed.WorkDir != "" {
			t.Fatalf("WorkDir = %q, want empty on non-windows", parsed.WorkDir)
		}
		return
	}
	if parsed.WorkDir != `C:\existing` {
		t.Fatalf("WorkDir = %q, want %q", parsed.WorkDir, `C:\existing`)
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}
