package visualforce

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct visualforce language server command",
			command: "visualforce-language-server",
			want:    true,
		},
		{
			name:    "visualforce ls executable path",
			command: `C:\tools\salesforce\visualforce-ls.exe`,
			want:    true,
		},
		{
			name:    "java launch with visualforce server jar",
			command: "java",
			args:    []string{"-jar", `C:\tools\visualforce-language-server.jar`},
			want:    true,
		},
		{
			name:    "wrapper arg includes vf language server",
			command: "wrapper",
			args:    []string{`/opt/vf-language-server/bin/vf-language-server`},
			want:    true,
		},
		{
			name:    "salesforce cli should not match",
			command: "sfdx",
			want:    false,
		},
		{
			name:    "apex server should not match",
			command: "apex-jorje-lsp",
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
