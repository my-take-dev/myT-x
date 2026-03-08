package stylable

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct stylable language server command",
			command: "stylable-language-server",
			want:    true,
		},
		{
			name:    "stylable language service executable path",
			command: `C:\tools\stylable\stylable-language-service.exe`,
			want:    true,
		},
		{
			name:    "node launch with stylable language service arg",
			command: "node",
			args:    []string{`C:\tools\@stylable\language-service\dist\server.js`},
			want:    true,
		},
		{
			name:    "arg contains stylable ls path",
			command: "wrapper",
			args:    []string{`C:\tools\stylable-ls\bin\stylable-ls`},
			want:    true,
		},
		{
			name:    "non stylable command",
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
