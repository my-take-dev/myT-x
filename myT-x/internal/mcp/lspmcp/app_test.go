package lspmcp

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewRuntimeRequiresLSPCommand(t *testing.T) {
	_, err := NewRuntime(Config{
		LSPCommand: "   ",
	})
	if err == nil {
		t.Fatalf("expected error when LSPCommand is empty")
	}
}

func TestNewRuntimeAppliesDefaults(t *testing.T) {
	r, err := NewRuntime(Config{
		LSPCommand: "gopls",
		RootDir:    ".",
		In:         strings.NewReader(""),
		Out:        io.Discard,
	})
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}

	if r.cfg.RequestTimeout != 30*time.Second {
		t.Fatalf("unexpected RequestTimeout: %v", r.cfg.RequestTimeout)
	}
	if r.cfg.OpenDelay != 0 {
		t.Fatalf("unexpected OpenDelay: %v", r.cfg.OpenDelay)
	}
	if r.cfg.ServerName != DefaultServerName {
		t.Fatalf("unexpected ServerName: %q", r.cfg.ServerName)
	}
	if r.cfg.ServerVersion != DefaultServerVersion {
		t.Fatalf("unexpected ServerVersion: %q", r.cfg.ServerVersion)
	}
}

func TestCloseBeforeStartIsNoop(t *testing.T) {
	r, err := NewRuntime(Config{
		LSPCommand: "gopls",
		RootDir:    ".",
		In:         strings.NewReader(""),
		Out:        io.Discard,
	})
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	if err := r.Close(context.Background()); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestStartAfterCloseReturnsError(t *testing.T) {
	r, err := NewRuntime(Config{
		LSPCommand: "gopls",
		RootDir:    ".",
		In:         strings.NewReader(""),
		Out:        io.Discard,
	})
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	if err := r.Close(context.Background()); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	if err := r.Start(context.Background()); err == nil {
		t.Fatalf("Start after Close should return error, got nil")
	}
}

func TestRunReturnsErrorWhenLSPCommandInvalid(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := Run(ctx, Config{
		LSPCommand: "",
		RootDir:    ".",
		In:         strings.NewReader(""),
		Out:        io.Discard,
	})
	if err == nil {
		t.Fatalf("Run with empty LSPCommand should return error")
	}
}

func TestRunReturnsErrorWhenStartFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 存在しないコマンドで Start が失敗する
	err := Run(ctx, Config{
		LSPCommand: "nonexistent-lsp-binary-xyz",
		RootDir:    ".",
		In:         strings.NewReader(""),
		Out:        io.Discard,
	})
	if err == nil {
		t.Fatalf("Run with nonexistent LSP command should return error")
	}
}

func TestNewRuntimeNormalizesRootDir(t *testing.T) {
	r, err := NewRuntime(Config{
		LSPCommand: "gopls",
		RootDir:    ".",
		In:         strings.NewReader(""),
		Out:        io.Discard,
	})
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	// RootDir が絶対パスに正規化されていること
	if !filepath.IsAbs(r.cfg.RootDir) {
		t.Errorf("RootDir should be absolute, got %q", r.cfg.RootDir)
	}
}

func TestNewRuntimeWithEmptyRootDirUsesCurrentDir(t *testing.T) {
	r, err := NewRuntime(Config{
		LSPCommand: "gopls",
		RootDir:    "",
		In:         strings.NewReader(""),
		Out:        io.Discard,
	})
	if err != nil {
		t.Fatalf("NewRuntime failed: %v", err)
	}
	if r.cfg.RootDir == "" {
		t.Error("RootDir should be normalized to . and then to absolute path")
	}
}
