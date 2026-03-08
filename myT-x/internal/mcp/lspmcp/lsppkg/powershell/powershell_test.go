package powershell

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
			name:    "direct powershell editor services command",
			command: "powershell-editor-services",
			want:    true,
		},
		{
			name:    "powershell editor services executable path",
			command: `C:\tools\PowerShellEditorServices\PowerShellEditorServices.exe`,
			want:    true,
		},
		{
			name:    "pwsh launch with start editor services script",
			command: "pwsh",
			args:    []string{"-File", `C:\tools\PowerShellEditorServices\Start-EditorServices.ps1`},
			want:    true,
		},
		{
			name:    "powershell launch with editor services module path",
			command: "powershell.exe",
			args:    []string{"-NoLogo", "-Command", `Import-Module C:\modules\PowerShellEditorServices\PowerShellEditorServices.psd1`},
			want:    true,
		},
		{
			name:    "dotnet launch with editor services hosting dll",
			command: "dotnet",
			args:    []string{`C:\servers\Microsoft.PowerShell.EditorServices.Hosting.dll`},
			want:    true,
		},
		{
			name:    "wrapper arg contains powershell editor services host path",
			command: "wrapper",
			args:    []string{`/opt/PowerShellEditorServices/PowerShellEditorServices.Host.exe`},
			want:    true,
		},
		{
			name:    "pwsh launch with unrelated script",
			command: "pwsh",
			args:    []string{"-File", `C:\scripts\deploy.ps1`},
			want:    false,
		},
		{
			name:    "plain powershell command without editor services args",
			command: "powershell",
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

func TestDescribeCapabilityCommand(t *testing.T) {
	got := describeCapabilityCommand("powershell.expandAlias", "PowerShell")
	if !strings.Contains(got, "PowerShell") {
		t.Fatalf("expected language in description, got %q", got)
	}
	if !strings.Contains(got, "server-specific") {
		t.Fatalf("expected server-specific note in description, got %q", got)
	}
}
