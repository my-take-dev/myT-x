package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	lspmcp "myT-x/internal/mcp/lspmcp"
)

// stringSliceFlag は繰り返し指定されるフラグ値を保持する flag.Value 実装。
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return fmt.Sprintf("%v", []string(*s))
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	var (
		lspBin          string
		rootDir         string
		defaultLangID   string
		initOptionsJSON string
		goplsPullDiag   bool
		requestTimeout  int
		openDelay       int
		logFilePath     string
		lspArgs         stringSliceFlag
	)

	flag.StringVar(&lspBin, "lsp", "", "LSP server binary (required)")
	flag.Var(&lspArgs, "lsp-arg", "LSP server argument (repeatable)")
	flag.StringVar(&rootDir, "root", ".", "Workspace root directory")
	flag.StringVar(&defaultLangID, "language-id", "", "Fallback languageId for unknown file extensions")
	flag.StringVar(&initOptionsJSON, "init-options", "", "Raw JSON for initialize.initializationOptions")
	flag.BoolVar(&goplsPullDiag, "gopls-pull-diagnostics", false, "Enable gopls pull diagnostics via initialize.initializationOptions.pullDiagnostics=true")
	flag.IntVar(&requestTimeout, "request-timeout-ms", 30000, "LSP request timeout in milliseconds")
	flag.IntVar(&openDelay, "open-delay-ms", 80, "Delay after didOpen/didChange before requesting data")
	flag.StringVar(&logFilePath, "log-file", "", "Optional log file path")
	flag.Parse()

	if lspBin == "" {
		fmt.Fprintln(os.Stderr, "error: -lsp is required")
		flag.Usage()
		os.Exit(2)
	}

	var initOptions any
	if initOptionsJSON != "" {
		if err := json.Unmarshal([]byte(initOptionsJSON), &initOptions); err != nil {
			fmt.Fprintf(os.Stderr, "error: parse -init-options JSON: %v\n", err)
			os.Exit(1)
		}
	}
	if merged, err := withGoplsPullDiagnostics(initOptions, lspBin, []string(lspArgs), goplsPullDiag); err != nil {
		fmt.Fprintf(os.Stderr, "error: gopls pull diagnostics init: %v\n", err)
		os.Exit(1)
	} else {
		initOptions = merged
	}

	logger, cleanupLog, err := makeLogger(logFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: setup logger: %v\n", err)
		os.Exit(1)
	}
	defer cleanupLog()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := lspmcp.Run(ctx, lspmcp.Config{
		LSPCommand:            lspBin,
		LSPArgs:               []string(lspArgs),
		RootDir:               rootDir,
		LanguageID:            defaultLangID,
		InitializationOptions: initOptions,
		RequestTimeout:        time.Duration(requestTimeout) * time.Millisecond,
		OpenDelay:             time.Duration(openDelay) * time.Millisecond,
		In:                    os.Stdin,
		Out:                   os.Stdout,
		Logger:                logger,
		ServerName:            "generic-lspmcp",
		ServerVersion:         "0.1.0",
	}); err != nil && err != context.Canceled {
		fmt.Fprintf(os.Stderr, "error: mcp server failed: %v\n", err)
		os.Exit(1)
	}
}

// makeLogger は path が空の場合は何も出力しないロガーを、
// 空でない場合はファイルにログ出力するロガーを生成する。
func makeLogger(path string) (*log.Logger, func(), error) {
	if path == "" {
		return log.New(io.Discard, "", 0), func() {}, nil
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, nil, err
	}
	logger := log.New(file, "[generic-lspmcp] ", log.LstdFlags|log.Lmicroseconds)
	return logger, func() {
		if err := file.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "[generic-lspmcp] ログファイル Close エラー: %v\n", err)
		}
	}, nil
}

// withGoplsPullDiagnostics は、enabled かつ gopls 起動の場合に initOptions へ pullDiagnostics=true を付与する。
// initOptions が nil のときは新規 map を返し、map[string]any でない場合はエラーを返す。
func withGoplsPullDiagnostics(initOptions any, command string, args []string, enabled bool) (any, error) {
	if !enabled || !isGoplsInvocation(command, args) {
		return initOptions, nil
	}
	if initOptions == nil {
		return map[string]any{"pullDiagnostics": true}, nil
	}

	optionsMap, ok := initOptions.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("-gopls-pull-diagnostics requires -init-options JSON object (or omit -init-options)")
	}

	merged := make(map[string]any, len(optionsMap)+1)
	maps.Copy(merged, optionsMap)
	merged["pullDiagnostics"] = true
	return merged, nil
}

// isGoplsInvocation は command と args から gopls 起動かどうかを判定する。
// 直接 gopls、args 内の gopls、go tool gopls のいずれかであれば true。
func isGoplsInvocation(command string, args []string) bool {
	if hasBaseName(command, "gopls") {
		return true
	}

	for _, arg := range args {
		if hasBaseName(arg, "gopls") {
			return true
		}
	}

	return hasBaseName(command, "go") &&
		len(args) >= 2 &&
		strings.EqualFold(strings.TrimSpace(args[0]), "tool") &&
		hasBaseName(args[1], "gopls")
}

// hasBaseName は value のパス末尾（拡張子 .exe を除く）が expected と一致するか判定する。
// 大文字小文字は区別しない。
func hasBaseName(value string, expected string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	base := strings.ToLower(filepath.Base(trimmed))
	base = strings.TrimSuffix(base, ".exe")
	return base == strings.ToLower(expected)
}
