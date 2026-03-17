package main

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestValidateDevPanelGitFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid relative path",
			path:    "src/main.go",
			wantErr: false,
		},
		{
			name:    "valid simple filename",
			path:    "file.txt",
			wantErr: false,
		},
		{
			name:    "valid nested path",
			path:    "internal/git/command.go",
			wantErr: false,
		},
		{
			name:    "empty path rejected",
			path:    "",
			wantErr: true,
			errMsg:  "file path is required",
		},
		{
			name:    "whitespace only rejected",
			path:    "   ",
			wantErr: true,
			errMsg:  "file path is required",
		},
		{
			name:    "absolute path rejected",
			path:    `C:\Windows\System32\cmd.exe`,
			wantErr: true,
			errMsg:  "file path must be relative",
		},
		{
			name:    "path traversal rejected",
			path:    "../../../etc/passwd",
			wantErr: true,
			errMsg:  "file path is not local",
		},
		{
			name:    "dot-dot in middle rejected",
			path:    "src/../../secret.txt",
			wantErr: true,
			errMsg:  "file path is not local",
		},
		{
			name:    "path with leading spaces trimmed and validated",
			path:    "  src/main.go  ",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDevPanelGitFilePath(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestFirstLine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "single line", input: "hello world", want: "hello world"},
		{name: "multi line", input: "first\nsecond\nthird", want: "first"},
		{name: "empty string", input: "", want: ""},
		{name: "newline only", input: "\n", want: ""},
		{name: "trailing newline", input: "msg\n", want: "msg"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstLine(tt.input)
			if got != tt.want {
				t.Errorf("firstLine(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveAndValidateGitSession(t *testing.T) {
	t.Run("empty session name returns error", func(t *testing.T) {
		app := &App{}
		_, err := app.resolveAndValidateGitSession("")
		if err == nil {
			t.Fatal("expected error for empty session name")
		}
		if !strings.Contains(err.Error(), "session name is required") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("whitespace session name returns error", func(t *testing.T) {
		app := &App{}
		_, err := app.resolveAndValidateGitSession("   ")
		if err == nil {
			t.Fatal("expected error for whitespace session name")
		}
		if !strings.Contains(err.Error(), "session name is required") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("non-existent session returns error", func(t *testing.T) {
		app := newTestAppWithSession(t, "test-session", t.TempDir())
		_, err := app.resolveAndValidateGitSession("nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existent session")
		}
	})

	t.Run("non-git directory returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		app := newTestAppWithSession(t, "test-session", tmpDir)
		_, err := app.resolveAndValidateGitSession("test-session")
		if err == nil {
			t.Fatal("expected error for non-git directory")
		}
		if !strings.Contains(err.Error(), "not a git repository") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// TestDevPanelGitOpsTypeFieldCounts guards against silent frontend breakage
// when struct fields are added or removed.
// TestDecodeGitPathLiteralForStatusPaths verifies that decodeGitPathLiteral
// correctly handles the path formats produced by git status --porcelain.
func TestDecodeGitPathLiteralForStatusPaths(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantOK  bool
	}{
		{
			name:   "plain ASCII path",
			raw:    "src/main.go",
			want:   "src/main.go",
			wantOK: true,
		},
		{
			name:   "quoted path with spaces",
			raw:    strconv.Quote("plans/plans - copy/test.go"),
			want:   "plans/plans - copy/test.go",
			wantOK: true,
		},
		{
			name:   "quoted path with Japanese (quotepath=false, real UTF-8)",
			raw:    strconv.Quote("plans/plans - コピー/test.go"),
			want:   "plans/plans - コピー/test.go",
			wantOK: true,
		},
		{
			name:   "quoted deep nested path with spaces and Japanese",
			raw:    strconv.Quote("plans/plans - コピー/plans - コピー/新規 テキスト ドキュメント.txt"),
			want:   "plans/plans - コピー/plans - コピー/新規 テキスト ドキュメント.txt",
			wantOK: true,
		},
		{
			name:   "empty string",
			raw:    "",
			want:   "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodeGitPathLiteral(tt.raw)
			if ok != tt.wantOK {
				t.Errorf("decodeGitPathLiteral(%q) ok = %v, want %v", tt.raw, ok, tt.wantOK)
				return
			}
			if got != tt.want {
				t.Errorf("decodeGitPathLiteral(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDevPanelGitOpsTypeFieldCounts(t *testing.T) {
	tests := []struct {
		name      string
		typ       reflect.Type
		wantCount int
	}{
		{
			name:      "DevPanelCommitResult",
			typ:       reflect.TypeFor[DevPanelCommitResult](),
			wantCount: 2, // Hash, Message
		},
		{
			name:      "DevPanelPushResult",
			typ:       reflect.TypeFor[DevPanelPushResult](),
			wantCount: 3, // RemoteName, BranchName, UpstreamSet
		},
		{
			name:      "DevPanelPullResult",
			typ:       reflect.TypeFor[DevPanelPullResult](),
			wantCount: 2, // Updated, Summary
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.typ.NumField()
			if got != tt.wantCount {
				t.Errorf("%s has %d fields, want %d — update frontend model and this test", tt.name, got, tt.wantCount)
			}
		})
	}
}
