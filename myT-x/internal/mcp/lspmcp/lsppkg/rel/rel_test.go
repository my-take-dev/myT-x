package rel

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct rel-ls command",
			command: "rel-ls",
			want:    true,
		},
		{
			name:    "rel-ls executable path",
			command: `C:\tools\rel-ls\rel-ls.exe`,
			want:    true,
		},
		{
			name:    "java jar launch",
			command: "java",
			args:    []string{"-jar", "/opt/rel-ls.jar", "--stdio"},
			want:    true,
		},
		{
			name:    "java class launch",
			command: "java",
			args:    []string{"-cp", "/opt/rel-ls.jar", "org.rel.ls.Main"},
			want:    true,
		},
		{
			name:    "node launch with rel ls script",
			command: "node",
			args:    []string{"./node_modules/rel-ls/dist/server.js", "--stdio"},
			want:    true,
		},
		{
			name:    "java without rel reference",
			command: "java",
			args:    []string{"-jar", "/opt/other-lsp.jar"},
			want:    false,
		},
		{
			name:    "unrelated command",
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
