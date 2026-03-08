package lsp

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPathToURIURIToPathRoundtrip(t *testing.T) {
	// カレントディレクトリの絶対パスでラウンドトリップ
	wd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("Abs failed: %v", err)
	}

	uri, err := PathToURI(wd)
	if err != nil {
		t.Fatalf("PathToURI failed: %v", err)
	}

	path, err := URIToPath(uri)
	if err != nil {
		t.Fatalf("URIToPath failed: %v", err)
	}

	// 正規化後のパスが一致することを確認
	wdClean := filepath.Clean(wd)
	pathClean := filepath.Clean(path)
	if wdClean != pathClean {
		t.Errorf("roundtrip mismatch: PathToURI(%q)=%q, URIToPath(%q)=%q", wd, uri, uri, path)
	}
}

func TestURIToPathNonFileScheme(t *testing.T) {
	_, err := URIToPath("https://example.com/path")
	if err == nil {
		t.Fatalf("URIToPath with https scheme should return error")
	}
	if err != nil && !strings.Contains(err.Error(), "unsupported URI scheme") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestURIToPathInvalidURI(t *testing.T) {
	_, err := URIToPath("file://%zz")
	if err == nil {
		t.Fatalf("URIToPath with invalid percent-encoding should return error")
	}
}

func TestPathToURIBasic(t *testing.T) {
	uri, err := PathToURI(".")
	if err != nil {
		t.Fatalf("PathToURI failed: %v", err)
	}
	if uri == "" {
		t.Fatalf("PathToURI returned empty URI")
	}
	if uri[:7] != "file://" {
		t.Errorf("PathToURI should return file:// scheme, got %q", uri)
	}
}

func TestURIToPathWindowsStyle(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}
	// file:///C:/path 形式
	path, err := URIToPath("file:///C:/foo/bar")
	if err != nil {
		t.Fatalf("URIToPath failed: %v", err)
	}
	if path == "" {
		t.Fatalf("URIToPath returned empty path")
	}
	// C:\foo\bar または類似の形式になる
	if len(path) < 5 {
		t.Errorf("unexpected short path: %q", path)
	}
}

func TestDetectLanguageID(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"foo.go", "go"},
		{"bar.py", "python"},
		{"baz.unknown", ""},
		{"file.rs", "rust"},
		{"doc.md", "markdown"},
	}
	for _, tt := range tests {
		got := DetectLanguageID(tt.path)
		if got != tt.want {
			t.Errorf("DetectLanguageID(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
