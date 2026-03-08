package sway

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct sway lsp command",
			command: "sway-lsp",
			want:    true,
		},
		{
			name:    "direct forc lsp command",
			command: "forc-lsp",
			want:    true,
		},
		{
			name:    "forc subcommand lsp invocation",
			command: "forc",
			args:    []string{"lsp", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains sway lsp path",
			command: "wrapper",
			args:    []string{`C:\tools\sway\bin\sway-lsp.exe`},
			want:    true,
		},
		{
			name:    "non sway command",
			command: "rust-analyzer",
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
