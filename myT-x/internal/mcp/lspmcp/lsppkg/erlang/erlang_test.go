package erlang

import (
	"strings"
	"testing"
)

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct erlang_ls command",
			command: "erlang_ls",
			want:    true,
		},
		{
			name:    "direct sourcer command",
			command: "sourcer",
			want:    true,
		},
		{
			name:    "direct elp command",
			command: "elp",
			want:    true,
		},
		{
			name:    "escript launch with erlang_ls",
			command: "escript",
			args:    []string{`C:\tools\erlang_ls\erlang_ls`},
			want:    true,
		},
		{
			name:    "rebar3 launch with sourcer path",
			command: "rebar3",
			args:    []string{"as", "dev", `C:\src\sourcer`},
			want:    true,
		},
		{
			name:    "cargo launch with elp package",
			command: "cargo",
			args:    []string{"run", "--package", "elp", "--", "server"},
			want:    true,
		},
		{
			name:    "arg contains erlang language platform path",
			command: "wrapper",
			args:    []string{`C:\src\erlang-language-platform\target\release\elp.exe`},
			want:    true,
		},
		{
			name:    "non erlang command",
			command: "gopls",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Matches(tt.command, tt.args)
			if got != tt.want {
				t.Fatalf("Matches(%q, %v) = %v, want %v", tt.command, tt.args, got, tt.want)
			}
		})
	}
}

func TestDescribeCapabilityCommand(t *testing.T) {
	got := describeCapabilityCommand("erlang.applyCodeAction", "Erlang")
	if !strings.Contains(got, "Erlang") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
