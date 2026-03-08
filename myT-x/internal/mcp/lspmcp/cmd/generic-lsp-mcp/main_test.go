package main

import (
	"reflect"
	"testing"
)

// TestIsGoplsInvocation は isGoplsInvocation の gopls 起動判定を検証する。
func TestIsGoplsInvocation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		args    []string
		want    bool
	}{
		{
			name:    "direct gopls command",
			command: "gopls",
			want:    true,
		},
		{
			name:    "direct gopls exe command",
			command: `C:\tools\gopls.exe`,
			want:    true,
		},
		{
			name:    "go tool gopls",
			command: "go",
			args:    []string{"tool", "gopls"},
			want:    true,
		},
		{
			name:    "go tool gopls exe",
			command: "go.exe",
			args:    []string{"tool", `C:\go\bin\gopls.exe`},
			want:    true,
		},
		{
			name:    "gopls in args",
			command: "some-wrapper",
			args:    []string{"gopls"},
			want:    true,
		},
		{
			name:    "non gopls command",
			command: "rust-analyzer",
			want:    false,
		},
		{
			name:    "go tool gopls with single arg",
			command: "go",
			args:    []string{"tool"},
			want:    false,
		},
		{
			name:    "go tool xxx not gopls",
			command: "go",
			args:    []string{"tool", "vet"},
			want:    false,
		},
		{
			name:    "empty command",
			command: "",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isGoplsInvocation(tt.command, tt.args); got != tt.want {
				t.Fatalf("isGoplsInvocation(%q, %v) = %v, want %v", tt.command, tt.args, got, tt.want)
			}
		})
	}
}

// TestWithGoplsPullDiagnostics は withGoplsPullDiagnostics のフラグ・オプション適用を検証する。
func TestWithGoplsPullDiagnostics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		initOptions any
		command     string
		args        []string
		enabled     bool
		want        any
		wantErr     bool
	}{
		{
			name:        "disabled flag keeps nil",
			initOptions: nil,
			command:     "gopls",
			enabled:     false,
			want:        nil,
		},
		{
			name:        "enabled flag ignored for non gopls",
			initOptions: map[string]any{"settingA": "value"},
			command:     "rust-analyzer",
			enabled:     true,
			want:        map[string]any{"settingA": "value"},
		},
		{
			name:        "enabled flag creates init options map for gopls",
			initOptions: nil,
			command:     "gopls",
			enabled:     true,
			want:        map[string]any{"pullDiagnostics": true},
		},
		{
			name:        "enabled flag merges into existing map",
			initOptions: map[string]any{"ui": map[string]any{"codelenses": map[string]any{"gc_details": true}}},
			command:     "go",
			args:        []string{"tool", "gopls"},
			enabled:     true,
			want: map[string]any{
				"ui":              map[string]any{"codelenses": map[string]any{"gc_details": true}},
				"pullDiagnostics": true,
			},
		},
		{
			name:        "enabled flag overwrites existing pullDiagnostics false",
			initOptions: map[string]any{"pullDiagnostics": false},
			command:     "gopls",
			enabled:     true,
			want:        map[string]any{"pullDiagnostics": true},
		},
		{
			name:        "enabled flag requires object init options",
			initOptions: []any{"not-object"},
			command:     "gopls",
			enabled:     true,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := withGoplsPullDiagnostics(tt.initOptions, tt.command, tt.args, tt.enabled)
			if (err != nil) != tt.wantErr {
				t.Fatalf("withGoplsPullDiagnostics() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("withGoplsPullDiagnostics() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

// TestHasBaseName は hasBaseName のパス末尾判定を検証する。
func TestHasBaseName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		expected string
		want     bool
	}{
		{"match gopls", "gopls", "gopls", true},
		{"match gopls exe", "gopls.exe", "gopls", true},
		{"match windows path", `C:\tools\gopls.exe`, "gopls", true},
		{"case insensitive", "gopls", "GOPLS", true},
		{"mismatch", "rust-analyzer", "gopls", false},
		{"empty value", "", "gopls", false},
		{"whitespace only", "   ", "gopls", false},
		{"unix path", "/usr/bin/gopls", "gopls", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := hasBaseName(tt.value, tt.expected); got != tt.want {
				t.Fatalf("hasBaseName(%q, %q) = %v, want %v", tt.value, tt.expected, got, tt.want)
			}
		})
	}
}
