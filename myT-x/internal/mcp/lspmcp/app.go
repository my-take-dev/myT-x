package lspmcp

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"myT-x/internal/mcp/lspmcp/internal/lsp"
	"myT-x/internal/mcp/lspmcp/internal/mcp"
	"myT-x/internal/mcp/lspmcp/internal/tools"
)

const (
	// DefaultServerName は Config.ServerName が指定されていない場合に使用する。
	DefaultServerName = "generic-lspmcp"
	// DefaultServerVersion は Config.ServerVersion が指定されていない場合に使用する。
	DefaultServerVersion = "0.1.0"
)

// Config は再利用可能な OSS エントリーポイントの設定を表す。
type Config struct {
	// LSPCommand は起動する LSP サーバー実行ファイル名。
	LSPCommand string
	// LSPArgs は LSP サーバー起動時に渡す追加引数。
	LSPArgs []string
	// RootDir は LSP のワークスペースルートとして扱うディレクトリ。
	RootDir string

	// LanguageID は拡張子から判定できない場合に使う languageId の既定値。
	LanguageID string
	// InitializationOptions は initialize リクエストに渡す任意オプション。
	InitializationOptions any
	// RequestTimeout は LSP リクエスト単位のタイムアウト時間。
	RequestTimeout time.Duration
	// OpenDelay は didOpen/didChange 後に待機する時間。
	OpenDelay time.Duration

	// In は MCP サーバーが受信に使う入力ストリーム。
	In io.Reader
	// Out は MCP サーバーが送信に使う出力ストリーム。
	Out io.Writer
	// Logger はアプリ内部ログの出力先。
	Logger *log.Logger

	// ServerName は MCP initialize 応答に含めるサーバー名。
	ServerName string
	// ServerVersion は MCP initialize 応答に含めるサーバーバージョン。
	ServerVersion string
}

// App は外部呼び出し用の埋め込み可能なランタイムインターフェース。
type App interface {
	Start(ctx context.Context) error
	Serve(ctx context.Context) error
	Close(ctx context.Context) error
}

// Runtime は LSP クライアントと MCP サーバーを接続して App を実装する。
type Runtime struct {
	cfg    Config
	client *lsp.Client
	server *mcp.Server

	mu      sync.Mutex
	started bool
	closed  bool
}

// NewApp はプロセスを起動せずに埋め込み可能なランタイムを構築する。
func NewApp(cfg Config) (App, error) {
	return NewRuntime(cfg)
}

// NewRuntime はプロセスを起動せずに埋め込み可能なランタイムを構築する。
func NewRuntime(cfg Config) (*Runtime, error) {
	normalized, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}

	client := lsp.NewClient(lsp.Config{
		Command:               normalized.LSPCommand,
		Args:                  append([]string(nil), normalized.LSPArgs...),
		RootDir:               normalized.RootDir,
		LanguageID:            normalized.LanguageID,
		InitializationOptions: normalized.InitializationOptions,
		RequestTimeout:        normalized.RequestTimeout,
		OpenDelay:             normalized.OpenDelay,
		Logger:                normalized.Logger,
	})

	registry := tools.BuildRegistry(client, normalized.RootDir, normalized.LSPCommand, normalized.LSPArgs)
	server := mcp.NewServer(mcp.Config{
		Name:     normalized.ServerName,
		Version:  normalized.ServerVersion,
		In:       normalized.In,
		Out:      normalized.Out,
		Logger:   normalized.Logger,
		Registry: registry,
	})

	return &Runtime{
		cfg:    normalized,
		client: client,
		server: server,
	}, nil
}

// Run はワンショット実行用の簡易エントリーポイント。
func Run(ctx context.Context, cfg Config) error {
	runtime, err := NewRuntime(cfg)
	if err != nil {
		return err
	}
	if err := runtime.Start(ctx); err != nil {
		return err
	}
	defer func() {
		if closeErr := runtime.Close(context.Background()); closeErr != nil && runtime.cfg.Logger != nil {
			runtime.cfg.Logger.Printf("ランタイム終了処理でエラー: %v", closeErr)
		}
	}()
	return runtime.Serve(ctx)
}

// Start は LSP プロセスを起動する。
func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return errors.New("runtime is closed")
	}
	if r.started {
		r.mu.Unlock()
		return nil
	}
	r.mu.Unlock()

	if err := r.client.Start(ctx); err != nil {
		return err
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		if closeErr := r.client.Close(context.Background()); closeErr != nil && r.cfg.Logger != nil {
			r.cfg.Logger.Printf("Start 競合時のクリーンアップでエラー: %v", closeErr)
		}
		return errors.New("runtime is closed")
	}
	r.started = true
	r.mu.Unlock()
	return nil
}

// Serve は MCP リクエストを処理するためにブロックする。
func (r *Runtime) Serve(ctx context.Context) error {
	if err := r.Start(ctx); err != nil {
		return err
	}
	return r.server.Serve(ctx)
}

// Close は LSP プロセスを終了する。
func (r *Runtime) Close(ctx context.Context) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	started := r.started
	r.started = false
	r.mu.Unlock()

	if !started {
		return nil
	}
	return r.client.Close(ctx)
}

// normalizeConfig は入力設定を検証し、実行に必要な既定値を補完する。
func normalizeConfig(cfg Config) (Config, error) {
	cfg.LSPCommand = strings.TrimSpace(cfg.LSPCommand)
	if cfg.LSPCommand == "" {
		return Config{}, errors.New("LSPCommand is required")
	}

	if strings.TrimSpace(cfg.RootDir) == "" {
		cfg.RootDir = "."
	}
	rootAbs, err := filepath.Abs(cfg.RootDir)
	if err != nil {
		return Config{}, err
	}
	cfg.RootDir = filepath.Clean(rootAbs)

	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 30 * time.Second
	}
	if cfg.OpenDelay < 0 {
		cfg.OpenDelay = 0
	}

	if cfg.In == nil {
		cfg.In = os.Stdin
	}
	if cfg.Out == nil {
		cfg.Out = os.Stdout
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(io.Discard, "", 0)
	}

	if strings.TrimSpace(cfg.ServerName) == "" {
		cfg.ServerName = DefaultServerName
	}
	if strings.TrimSpace(cfg.ServerVersion) == "" {
		cfg.ServerVersion = DefaultServerVersion
	}

	cfg.LSPArgs = append([]string(nil), cfg.LSPArgs...)
	return cfg, nil
}
