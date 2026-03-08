package clangd

import (
	"regexp"
	"testing"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
)

var triadDescriptionPattern = regexp.MustCompile(`^when: .+ args: .+ effect: (read|edit|exec|read or edit)\.$`)

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct clangd command",
			command: "clangd",
			want:    true,
		},
		{
			name:    "absolute clangd.exe path",
			command: `C:\llvm\bin\clangd.exe`,
			want:    true,
		},
		{
			name:    "arg contains clangd path",
			command: "wrapper",
			args:    []string{`C:\llvm\bin\clangd.exe`},
			want:    true,
		},
		{
			name:    "non clangd command",
			command: "ccls",
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

func TestBuildToolsDescriptionTriadFormat(t *testing.T) {
	tools := BuildTools(&lsp.Client{}, ".")
	if len(tools) == 0 {
		t.Fatal("BuildTools returned no tools")
	}

	for _, tool := range tools {
		if !triadDescriptionPattern.MatchString(tool.Description) {
			t.Fatalf("tool %q has non-triad description: %q", tool.Name, tool.Description)
		}
	}
}
