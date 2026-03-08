package swift

import "testing"

func TestMatches(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct sourcekit lsp command",
			command: "sourcekit-lsp",
			want:    true,
		},
		{
			name:    "sourcekit lsp executable path",
			command: "/Applications/Xcode.app/Contents/Developer/Toolchains/XcodeDefault.xctoolchain/usr/bin/sourcekit-lsp",
			want:    true,
		},
		{
			name:    "xcrun invocation with sourcekit lsp arg",
			command: "xcrun",
			args:    []string{"sourcekit-lsp", "--stdio"},
			want:    true,
		},
		{
			name:    "arg contains sourcekit lsp path",
			command: "wrapper",
			args:    []string{"/opt/swift/bin/sourcekit-lsp"},
			want:    true,
		},
		{
			name:    "non swift command",
			command: "clangd",
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
