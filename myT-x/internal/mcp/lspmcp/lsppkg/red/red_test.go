package red

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct redlangserver command",
			command: "redlangserver",
			want:    true,
		},
		{
			name:    "red language server executable path",
			command: `C:\tools\redlangserver\red-language-server.exe`,
			want:    true,
		},
		{
			name:    "node launch with redlangserver script",
			command: "node",
			args:    []string{"./node_modules/redlangserver/dist/index.js"},
			want:    true,
		},
		{
			name:    "npx launch",
			command: "npx",
			args:    []string{"redlangserver", "--stdio"},
			want:    true,
		},
		{
			name:    "java launch",
			command: "java",
			args:    []string{"-jar", "/opt/redlangserver.jar"},
			want:    true,
		},
		{
			name:    "java without red reference",
			command: "java",
			args:    []string{"-jar", "/opt/other-server.jar"},
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
