package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"myT-x/internal/mcp/lspmcp/internal/jsonrpc"
)

// Config は LSP プロセスとリクエストの動作を制御する。
type Config struct {
	// Command は起動する LSP サーバーの実行コマンド。
	Command string
	// Args は LSP サーバー起動時に付与する引数。
	Args []string
	// RootDir は LSP ワークスペースとして扱うルートディレクトリ。
	RootDir string
	// LanguageID は拡張子から判定できない場合の languageId 既定値。
	LanguageID string
	// InitializationOptions は initialize リクエストに渡す追加設定。
	InitializationOptions any
	// RequestTimeout は LSP リクエストごとの待機上限時間。
	RequestTimeout time.Duration
	// OpenDelay は didOpen/didChange 後に待つ時間。
	OpenDelay time.Duration
	// Logger はクライアント内部ログの出力先。
	Logger *log.Logger
}

type responseResult struct {
	result json.RawMessage
	rpcErr *jsonrpc.Error
}

type openDocument struct {
	version    int
	text       string
	languageID string
}

var (
	lspClientLookPath        = exec.LookPath
	lspClientGOOS            = runtime.GOOS
	lspClientShutdownTimeout = 5 * time.Second
	errLSPClientClosed       = errors.New("lsp client is closed")
)

// Client は stdio 経由の最小限の LSP クライアント。
type Client struct {
	cfg Config

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	writerMu sync.Mutex

	nextID  atomic.Int64
	pending map[string]chan responseResult
	pmu     sync.Mutex

	docs  map[string]*openDocument
	dmu   sync.Mutex
	diags map[string][]Diagnostic
	xmu   sync.RWMutex

	capabilities map[string]json.RawMessage
	cmu          sync.RWMutex

	logger *log.Logger

	readLoopDone     chan struct{}
	readLoopDoneOnce sync.Once
	closeMu          sync.Mutex
	closeOnce        sync.Once
	closeDone        chan struct{}
	closeErr         error
}

// NewClient は LSP クライアントを構築する。
func NewClient(cfg Config) *Client {
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 30 * time.Second
	}
	if cfg.OpenDelay < 0 {
		cfg.OpenDelay = 0
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(io.Discard, "", 0)
	}

	return &Client{
		cfg:          cfg,
		pending:      make(map[string]chan responseResult),
		docs:         make(map[string]*openDocument),
		diags:        make(map[string][]Diagnostic),
		capabilities: make(map[string]json.RawMessage),
		logger:       cfg.Logger,
		readLoopDone: make(chan struct{}),
		closeDone:    make(chan struct{}),
	}
}

// Start は言語サーバープロセスを起動して初期化する。
func (c *Client) Start(ctx context.Context) error {
	if strings.TrimSpace(c.cfg.Command) == "" {
		return errors.New("lsp command is required")
	}

	rootAbs, err := filepath.Abs(c.cfg.RootDir)
	if err != nil {
		return fmt.Errorf("resolve root dir: %w", err)
	}
	c.cfg.RootDir = filepath.Clean(rootAbs)

	cmd := buildLSPExecCommand(ctx, c.cfg.Command, c.cfg.Args)
	cmd.Dir = c.cfg.RootDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("open lsp stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open lsp stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("open lsp stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start lsp process: %w", err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout
	c.stderr = stderr

	go c.readLoop()
	go c.stderrLoop()

	initCtx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()
	if err := c.initialize(initCtx); err != nil {
		if closeErr := c.Close(context.Background()); closeErr != nil {
			c.logf("initialize 失敗後のクリーンアップでエラー: %v", closeErr)
		}
		return err
	}

	return nil
}

// Close は LSP プロセスを正常終了する。
func (c *Client) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	c.closeMu.Lock()
	cmdAvailable := c.cmd != nil
	closeDone := c.closeDone
	c.closeMu.Unlock()

	if !cmdAvailable {
		select {
		case <-closeDone:
			c.closeMu.Lock()
			err := c.closeErr
			c.closeMu.Unlock()
			return err
		default:
			return nil
		}
	}

	started := false
	c.closeOnce.Do(func() {
		started = true
		err := c.closeInternal(ctx)
		c.closeMu.Lock()
		c.closeErr = err
		close(c.closeDone)
		c.closeMu.Unlock()
	})
	if started {
		c.closeMu.Lock()
		err := c.closeErr
		c.closeMu.Unlock()
		return err
	}

	select {
	case <-closeDone:
		c.closeMu.Lock()
		err := c.closeErr
		c.closeMu.Unlock()
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) closeInternal(ctx context.Context) error {
	waitCtx := ctx
	if waitCtx == nil {
		waitCtx = context.Background()
	}

	c.closeMu.Lock()
	cmd := c.cmd
	if cmd == nil {
		c.closeMu.Unlock()
		return nil
	}
	stdout := c.stdout
	stderr := c.stderr
	c.closeMu.Unlock()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), lspClientShutdownTimeout)
	var stopCancel func() bool
	if ctx != nil {
		stopCancel = context.AfterFunc(ctx, cancel)
	}
	defer func() {
		if stopCancel != nil {
			stopCancel()
		}
		cancel()
	}()

	if _, err := c.Request(shutdownCtx, "shutdown", nil); err != nil {
		c.logf("shutdown request failed: %v", err)
	}
	if err := c.Notify("exit", nil); err != nil {
		c.logf("exit notification failed: %v", err)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case <-shutdownCtx.Done():
		killed := false
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
				c.logf("failed to kill lsp process: %v", err)
			} else {
				killed = true
			}
		}
		if killed {
			if waitErr := <-waitCh; waitErr != nil && !errors.Is(waitErr, os.ErrProcessDone) {
				c.logf("lsp process exited after forced kill: %v", waitErr)
			}
		} else {
			select {
			case waitErr := <-waitCh:
				if waitErr != nil && !errors.Is(waitErr, os.ErrProcessDone) {
					c.logf("lsp process exited after shutdown timeout: %v", waitErr)
				}
			default:
			}
		}
	case waitErr := <-waitCh:
		if waitErr != nil {
			c.logf("lsp process exited with error: %v", waitErr)
		}
	}

	c.writerMu.Lock()
	stdin := c.stdin
	c.stdin = nil
	c.writerMu.Unlock()
	if stdin != nil {
		if err := stdin.Close(); err != nil {
			c.logf("close lsp stdin failed: %v", err)
		}
	}
	if stdout != nil {
		if err := stdout.Close(); err != nil {
			c.logf("close lsp stdout failed: %v", err)
		}
	}
	if stderr != nil {
		if err := stderr.Close(); err != nil {
			c.logf("close lsp stderr failed: %v", err)
		}
	}
	c.closeMu.Lock()
	c.cmd = nil
	c.stdout = nil
	c.stderr = nil
	c.closeMu.Unlock()

	select {
	case <-c.readLoopDone:
	case <-waitCtx.Done():
		return fmt.Errorf("lsp client close timed out waiting for read loop: %w", waitCtx.Err())
	}
	return nil
}

// Request は LSP リクエストを送信してレスポンスを待つ。
func (c *Client) Request(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	key := strconv.FormatInt(id, 10)
	responseCh := make(chan responseResult, 1)

	c.pmu.Lock()
	c.pending[key] = responseCh
	c.pmu.Unlock()

	request := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		request["params"] = params
	}

	if err := c.writeMessage(request); err != nil {
		c.pmu.Lock()
		delete(c.pending, key)
		c.pmu.Unlock()
		return nil, err
	}

	timeout := c.cfg.RequestTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			remaining = 1 * time.Millisecond
		}
		if remaining < timeout {
			timeout = remaining
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case resp := <-responseCh:
		c.pmu.Lock()
		delete(c.pending, key)
		c.pmu.Unlock()

		if resp.rpcErr != nil {
			return nil, fmt.Errorf("%s (%d)", resp.rpcErr.Message, resp.rpcErr.Code)
		}
		return resp.result, nil
	case <-ctx.Done():
		c.pmu.Lock()
		delete(c.pending, key)
		c.pmu.Unlock()
		return nil, ctx.Err()
	case <-timer.C:
		c.pmu.Lock()
		delete(c.pending, key)
		c.pmu.Unlock()
		return nil, fmt.Errorf("lsp request timeout: %s", method)
	}
}

// ExecuteCommand は workspace/executeCommand を実行し、デコードした結果を返す。
func (c *Client) ExecuteCommand(ctx context.Context, command string, arguments []any) (any, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("command is required")
	}
	if arguments == nil {
		arguments = []any{}
	}

	raw, err := c.Request(ctx, "workspace/executeCommand", map[string]any{
		"command":   command,
		"arguments": arguments,
	})
	if err != nil {
		return nil, err
	}

	return decodeRawJSONAny(raw)
}

// Notify は LSP 通知を送信する。
func (c *Client) Notify(method string, params any) error {
	notification := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		notification["params"] = params
	}
	return c.writeMessage(notification)
}

// EnsureDocument は LSP サーバー内のドキュメントを開くか更新する。
func (c *Client) EnsureDocument(ctx context.Context, path string) (DocumentSnapshot, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return DocumentSnapshot{}, err
	}
	absPath = filepath.Clean(absPath)

	contentBytes, err := os.ReadFile(absPath)
	if err != nil {
		return DocumentSnapshot{}, fmt.Errorf("read file %s: %w", absPath, err)
	}
	content := string(contentBytes)

	uri, err := PathToURI(absPath)
	if err != nil {
		return DocumentSnapshot{}, fmt.Errorf("build URI for %s: %w", absPath, err)
	}

	languageID := DetectLanguageID(absPath)
	if languageID == "" {
		if c.cfg.LanguageID != "" {
			languageID = c.cfg.LanguageID
		} else {
			languageID = "plaintext"
		}
	}

	var version int
	sendOpen := false
	sendChange := false

	c.dmu.Lock()
	doc, exists := c.docs[uri]
	if !exists {
		version = 1
		c.docs[uri] = &openDocument{
			version:    version,
			text:       content,
			languageID: languageID,
		}
		sendOpen = true
	} else {
		version = doc.version
		if doc.text != content {
			doc.version++
			doc.text = content
			version = doc.version
			sendChange = true
		}
		languageID = doc.languageID
	}
	c.dmu.Unlock()

	if sendOpen {
		err = c.Notify("textDocument/didOpen", map[string]any{
			"textDocument": map[string]any{
				"uri":        uri,
				"languageId": languageID,
				"version":    version,
				"text":       content,
			},
		})
		if err != nil {
			return DocumentSnapshot{}, err
		}
		if err := sleepWithContext(ctx, c.cfg.OpenDelay); err != nil {
			return DocumentSnapshot{}, err
		}
	}

	if sendChange {
		err = c.Notify("textDocument/didChange", map[string]any{
			"textDocument": map[string]any{
				"uri":     uri,
				"version": version,
			},
			"contentChanges": []map[string]any{
				{"text": content},
			},
		})
		if err != nil {
			return DocumentSnapshot{}, err
		}
		if err := sleepWithContext(ctx, c.cfg.OpenDelay); err != nil {
			return DocumentSnapshot{}, err
		}
	}

	return DocumentSnapshot{
		URI:        uri,
		Path:       absPath,
		LanguageID: languageID,
		Version:    version,
		Content:    content,
	}, nil
}

// CloseDocument は LSP サーバーにドキュメントが閉じられたことを通知する。
func (c *Client) CloseDocument(uri string) error {
	c.dmu.Lock()
	delete(c.docs, uri)
	c.dmu.Unlock()
	return c.Notify("textDocument/didClose", map[string]any{
		"textDocument": map[string]any{
			"uri": uri,
		},
	})
}

// Diagnostics は URI に対する現在のキャッシュ済みプッシュ診断を返す。
func (c *Client) Diagnostics(uri string) []Diagnostic {
	c.xmu.RLock()
	defer c.xmu.RUnlock()

	current := c.diags[uri]
	out := make([]Diagnostic, len(current))
	copy(out, current)
	return out
}

// Logger はクライアントのログ出力先を返す。tools パッケージ等でログの一元管理に利用する。
func (c *Client) Logger() *log.Logger {
	if c.logger != nil {
		return c.logger
	}
	return log.New(io.Discard, "", 0)
}

// Capabilities は initialize で得た capabilities のコピーを返す。
func (c *Client) Capabilities() map[string]json.RawMessage {
	c.cmu.RLock()
	defer c.cmu.RUnlock()

	out := make(map[string]json.RawMessage, len(c.capabilities))
	for k, v := range c.capabilities {
		copyRaw := append(json.RawMessage(nil), v...)
		out[k] = copyRaw
	}
	return out
}

// SupportsCapability はトップレベルの capability キーが有効かどうかを確認する。
func (c *Client) SupportsCapability(key string) bool {
	c.cmu.RLock()
	defer c.cmu.RUnlock()
	return capabilityEnabled(c.capabilities[key])
}

func (c *Client) initialize(ctx context.Context) error {
	rootURI, err := PathToURI(c.cfg.RootDir)
	if err != nil {
		return fmt.Errorf("root URI: %w", err)
	}

	rootName := filepath.Base(c.cfg.RootDir)
	if rootName == "" {
		rootName = "workspace"
	}

	params := map[string]any{
		"processId": os.Getpid(),
		"clientInfo": map[string]any{
			"name":    "generic-lspmcp",
			"version": "0.1.0",
		},
		"rootPath": c.cfg.RootDir,
		"rootUri":  rootURI,
		"workspaceFolders": []map[string]any{
			{
				"uri":  rootURI,
				"name": rootName,
			},
		},
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"synchronization": map[string]any{
					"didSave":             true,
					"dynamicRegistration": false,
				},
				"hover": map[string]any{
					"contentFormat": []string{"markdown", "plaintext"},
				},
				"definition": map[string]any{
					"linkSupport": true,
				},
				"completion": map[string]any{
					"completionItem": map[string]any{
						"snippetSupport": true,
					},
				},
				"publishDiagnostics": map[string]any{
					"relatedInformation": true,
				},
			},
			"workspace": map[string]any{
				"workspaceFolders": true,
				"configuration":    true,
			},
		},
	}
	if c.cfg.InitializationOptions != nil {
		params["initializationOptions"] = c.cfg.InitializationOptions
	}

	raw, err := c.Request(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	var result struct {
		Capabilities map[string]json.RawMessage `json:"capabilities"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}

	c.cmu.Lock()
	c.capabilities = result.Capabilities
	c.cmu.Unlock()

	if err := c.Notify("initialized", map[string]any{}); err != nil {
		return fmt.Errorf("initialized notification failed: %w", err)
	}

	return nil
}

func (c *Client) readLoop() {
	reader := bufio.NewReader(c.stdout)
	defer c.signalReadLoopDone()

	for {
		payload, err := jsonrpc.ReadMessage(reader)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				c.logf("lsp read loop stopped: %v", err)
			}
			c.failPending(err)
			return
		}

		var msg jsonrpc.Message
		if err := json.Unmarshal(payload, &msg); err != nil {
			c.logf("failed to decode lsp message: %v", err)
			// 不正 JSON が pending リクエストの応答だった場合、タイムアウトまで応答を受け取れない。
			// id を抽出して対応する pending にエラーを伝播する。
			if idKey := extractIDFromRawJSON(payload); idKey != "" {
				c.pmu.Lock()
				ch := c.pending[idKey]
				delete(c.pending, idKey)
				c.pmu.Unlock()
				if ch != nil {
					select {
					case ch <- responseResult{rpcErr: &jsonrpc.Error{
						Code:    -32700,
						Message: fmt.Sprintf("invalid JSON in LSP response: %v", err),
					}}:
					default:
						c.logf("failPending: リクエスト %s は既に応答済みのためエラー伝播をスキップ", idKey)
					}
				}
			} else {
				c.logf("invalid JSON: id を抽出できず、pending がタイムアウトする可能性あり")
			}
			continue
		}

		c.handleMessage(msg)
	}
}

func (c *Client) stderrLoop() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		c.logf("[lsp stderr] %s", scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		c.logf("lsp stderr scanner error: %v", err)
	}
}

func (c *Client) handleMessage(msg jsonrpc.Message) {
	switch {
	case msg.IsResponse():
		c.handleResponse(msg)
	case msg.IsRequest():
		c.handleServerRequest(msg)
	case msg.IsNotification():
		c.handleServerNotification(msg)
	}
}

func (c *Client) handleResponse(msg jsonrpc.Message) {
	key := jsonrpc.IDKey(msg.ID)
	c.pmu.Lock()
	ch := c.pending[key]
	delete(c.pending, key)
	c.pmu.Unlock()

	if ch == nil {
		return
	}
	ch <- responseResult{result: msg.Result, rpcErr: msg.Error}
}

func (c *Client) handleServerRequest(msg jsonrpc.Message) {
	result, rpcErr := c.defaultServerRequestResponse(msg.Method, msg.Params)
	response := map[string]any{
		"jsonrpc": "2.0",
		"id":      jsonrpc.ParseID(msg.ID),
	}
	if rpcErr != nil {
		response["error"] = rpcErr
	} else {
		response["result"] = result
	}
	if err := c.writeMessage(response); err != nil {
		c.logf("failed to send response to server request %s: %v", msg.Method, err)
	}
}

func (c *Client) handleServerNotification(msg jsonrpc.Message) {
	if msg.Method != "textDocument/publishDiagnostics" {
		return
	}

	var params struct {
		URI         string       `json:"uri"`
		Diagnostics []Diagnostic `json:"diagnostics"`
	}
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		c.logf("failed to parse diagnostics notification: %v", err)
		return
	}

	c.xmu.Lock()
	c.diags[params.URI] = params.Diagnostics
	c.xmu.Unlock()
}

func (c *Client) defaultServerRequestResponse(method string, params json.RawMessage) (any, *jsonrpc.Error) {
	switch method {
	case "workspace/configuration":
		var req struct {
			Items []json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(params, &req); err != nil {
			c.logf("workspace/configuration の params 解析に失敗: %v", err)
		}
		out := make([]map[string]any, len(req.Items))
		for i := range out {
			out[i] = map[string]any{}
		}
		return out, nil
	case "workspace/workspaceFolders":
		rootURI, err := PathToURI(c.cfg.RootDir)
		if err != nil {
			c.logf("workspaceFolders 用の root URI 解決に失敗: %v", err)
			return nil, &jsonrpc.Error{Code: -32603, Message: fmt.Sprintf("failed to resolve root URI: %v", err)}
		}
		return []map[string]any{
			{
				"uri":  rootURI,
				"name": filepath.Base(c.cfg.RootDir),
			},
		}, nil
	case "window/workDoneProgress/create":
		return map[string]any{}, nil
	case "client/registerCapability", "client/unregisterCapability":
		return map[string]any{}, nil
	default:
		// 未知のリクエストには null を返す方が汎用 LSP 互換性の観点で安全。
		return nil, nil
	}
}

func (c *Client) writeMessage(value any) error {
	c.writerMu.Lock()
	defer c.writerMu.Unlock()
	stdin := c.stdin
	if stdin == nil {
		return errLSPClientClosed
	}
	return jsonrpc.WriteJSON(stdin, value)
}

func (c *Client) failPending(err error) {
	c.pmu.Lock()
	defer c.pmu.Unlock()
	for key, ch := range c.pending {
		select {
		case ch <- responseResult{rpcErr: &jsonrpc.Error{
			Code:    -32000,
			Message: fmt.Sprintf("lsp connection closed: %v", err),
		}}:
		default:
			// 非ブロッキング送信にして、接続断時の後始末で詰まることを防ぐ。
			// responseCh はバッファ1のため、チャネルが満杯の場合は default に来る（既に応答が入っているケース）。
			c.logf("failPending: リクエスト %s は既に応答済みのためエラー伝播をスキップ", key)
		}
		delete(c.pending, key)
	}
}

func (c *Client) signalReadLoopDone() {
	c.readLoopDoneOnce.Do(func() {
		close(c.readLoopDone)
	})
}

func (c *Client) logf(format string, args ...any) {
	if c.logger != nil {
		c.logger.Printf(format, args...)
	}
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// extractIDFromRawJSON は不正な JSON ペイロードから id フィールドを抽出し、
// jsonrpc.IDKey と互換のキー文字列を返す。抽出できない場合は空文字を返す。
func extractIDFromRawJSON(payload []byte) string {
	var partial struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.Unmarshal(payload, &partial); err != nil {
		return ""
	}
	return jsonrpc.IDKey(partial.ID)
}

func capabilityEnabled(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "false" && trimmed != "null"
}

func decodeRawJSONAny(raw json.RawMessage) (any, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func buildLSPExecCommand(ctx context.Context, command string, args []string) *exec.Cmd {
	if shouldWrapWindowsBatchCommand(command) {
		wrappedArgs := make([]string, 0, 4+1+len(args))
		wrappedArgs = append(wrappedArgs, "/d", "/s", "/c", command)
		wrappedArgs = append(wrappedArgs, args...)
		cmd := exec.CommandContext(ctx, "cmd.exe", wrappedArgs...)
		applyPlatformExecOptions(cmd)
		return cmd
	}
	cmd := exec.CommandContext(ctx, command, args...)
	applyPlatformExecOptions(cmd)
	return cmd
}

func shouldWrapWindowsBatchCommand(command string) bool {
	if lspClientGOOS != "windows" {
		return false
	}
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return false
	}
	if hasWindowsBatchExtension(trimmed) {
		return true
	}
	if filepath.Ext(trimmed) != "" {
		return false
	}
	resolved, err := lspClientLookPath(trimmed)
	if err != nil {
		return false
	}
	return hasWindowsBatchExtension(resolved)
}

func hasWindowsBatchExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	return ext == ".cmd" || ext == ".bat"
}
