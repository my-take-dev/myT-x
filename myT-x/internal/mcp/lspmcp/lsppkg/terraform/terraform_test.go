package terraform

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
			name:    "direct terraform ls command",
			command: "terraform-ls",
			want:    true,
		},
		{
			name:    "direct terraform lsp executable path",
			command: `C:\tools\terraform\terraform-lsp.exe`,
			want:    true,
		},
		{
			name:    "go launch with terraform lsp module path",
			command: "go",
			args:    []string{"run", "github.com/juliosueiras/terraform-lsp"},
			want:    true,
		},
		{
			name:    "wrapper args contain terraform ls path",
			command: "wrapper",
			args:    []string{`/usr/local/bin/terraform-ls`},
			want:    true,
		},
		{
			name:    "non terraform command",
			command: "terrascan",
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
	got := describeCapabilityCommand("terraform.apply", "Terraform")
	if !strings.Contains(got, "Terraform") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
