package wgsl

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct wgsl analyzer command",
			command: "wgsl_analyzer",
			want:    true,
		},
		{
			name:    "wgsl language server executable path",
			command: `C:\tools\wgsl\wgsl-language-server.exe`,
			want:    true,
		},
		{
			name:    "wrapper args include wgsl analyzer path",
			command: "wrapper",
			args:    []string{`/opt/wgsl-analyzer/bin/wgsl-analyzer`},
			want:    true,
		},
		{
			name:    "similarly named shader tool should not match",
			command: "wgslfmt",
			want:    false,
		},
		{
			name:    "unrelated server should not match",
			command: "glsl-language-server",
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
