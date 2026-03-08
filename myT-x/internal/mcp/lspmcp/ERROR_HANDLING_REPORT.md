# generic-lsp-mcp エラーハンドリング分析レポート

分析日: 2025-02-28  
対象: lsp_pkg 以外の全 Go ファイル

**更新 (2026-02-28)**: 本レポートに記載された問題の多くは既に修正済みです。Critical 3件、Important 6件、Suggestion の一部は対応完了。現状のコードと整合するよう、各項目の修正状況を確認してください。

---

## Critical（重大）

### 1. server.go:97 — Parse エラー時の writeError 失敗を無視

**ファイル**: `internal/mcp/server.go` 行 96-98

**問題**: JSON パース失敗時に `writeError` でクライアントへエラーを返そうとするが、`writeError` が失敗した場合の戻り値を `_ =` で捨てている。書き込み失敗（接続断など）時にクライアントは応答を受け取れず、Serve ループはそのまま続行する。

```go
if err := json.Unmarshal(payload, &msg); err != nil {
    _ = s.writeError(nil, -32700, "Parse error", err.Error())
    continue
}
```

**推奨修正**:
```go
if err := json.Unmarshal(payload, &msg); err != nil {
    if writeErr := s.writeError(nil, -32700, "Parse error", err.Error()); writeErr != nil {
        return writeErr  // 書き込み不能なら Serve を終了
    }
    continue
}
```

---

### 2. client.go:404-409 — stderrLoop で scanner.Err() を無視

**ファイル**: `internal/lsp/client.go` 行 404-409

**問題**: `bufio.Scanner` は `Scan()` が false になった後、`Err()` で `ErrTooLong` などのエラーを返す。現状はループ終了後に `scanner.Err()` を確認しておらず、stderr の読み取りエラーがログされない。

```go
func (c *Client) stderrLoop() {
    scanner := bufio.NewScanner(c.stderr)
    for scanner.Scan() {
        c.logf("[lsp stderr] %s", scanner.Text())
    }
}
```

**推奨修正**:
```go
func (c *Client) stderrLoop() {
    scanner := bufio.NewScanner(c.stderr)
    for scanner.Scan() {
        c.logf("[lsp stderr] %s", scanner.Text())
    }
    if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
        c.logf("lsp stderr read error: %v", err)
    }
}
```

---

### 3. client.go:467-481 — failPending の default 分岐で送信失敗を無視

**ファイル**: `internal/lsp/client.go` 行 467-481

**問題**: `failPending` で `ch <- responseResult{...}` がブロックする場合、`default` により送信をスキップして `delete` する。その結果、待機中の Request はタイムアウトまで応答を受け取れない。タイムアウトは発生するが、エラー内容（接続終了など）が伝わらない。

**影響**: 接続終了時に「timeout」としか分からず、実際の原因（接続クローズ）が分かりにくい。

**推奨**: 現状の挙動（タイムアウトで抜ける）は許容できるが、`default` に入った場合にログを出すとデバッグしやすい。

```go
default:
    c.logf("failPending: could not notify pending request %s (channel blocked)", key)
```

---

## Important（重要）

### 4. app.go:112-113 — Run の defer で Close エラーを無視

**ファイル**: `app.go` 行 111-114

**問題**: `defer` 内で `runtime.Close()` のエラーを捨てている。正常終了時は問題になりにくいが、LSP プロセスの終了失敗などが隠れる。

```go
defer func() {
    _ = runtime.Close(context.Background())
}()
```

**推奨修正**:
```go
defer func() {
    if err := runtime.Close(context.Background()); err != nil {
        r.cfg.Logger.Printf("Close failed: %v", err)
    }
}()
```
※ `Run` 内では `runtime` が `*Runtime` 型である必要がある。`Config` に Logger を渡すか、`Runtime` から Logger を取得する設計にする。

---

### 5. app.go:136-138 — Start 競合時の client.Close エラーを無視

**ファイル**: `app.go` 行 134-139

**問題**: `client.Start` 成功後に `r.closed` が true になった場合、`client.Close` を呼ぶがそのエラーを捨てている。

```go
if r.closed {
    r.mu.Unlock()
    _ = r.client.Close(context.Background())
    return errors.New("runtime is closed")
}
```

**推奨修正**: 少なくともログ出力する。
```go
if r.closed {
    r.mu.Unlock()
    if closeErr := r.client.Close(context.Background()); closeErr != nil {
        r.cfg.Logger.Printf("Close after Start race: %v", closeErr)
    }
    return errors.New("runtime is closed")
}
```

---

### 6. client.go:156-168 — Close 内の shutdown/exit/Kill エラーを無視

**ファイル**: `internal/lsp/client.go` 行 156-168

**問題**: 以下のエラーが無視されている。
- `Request(shutdownCtx, "shutdown", nil)` の戻り値
- `Notify("exit", nil)` の戻り値
- `Process.Kill()` の戻り値

```go
_, _ = c.Request(shutdownCtx, "shutdown", nil)
_ = c.Notify("exit", nil)
// ...
_ = c.cmd.Process.Kill()
```

**推奨修正**: デバッグ用にログを残す。
```go
if _, err := c.Request(shutdownCtx, "shutdown", nil); err != nil {
    c.logf("shutdown request failed: %v", err)
}
if err := c.Notify("exit", nil); err != nil {
    c.logf("exit notify failed: %v", err)
}
// ...
if c.cmd.Process != nil {
    if err := c.cmd.Process.Kill(); err != nil {
        c.logf("process kill failed: %v", err)
    }
}
```

---

### 7. client.go:141 — initialize 失敗時の Close エラーを無視

**ファイル**: `internal/lsp/client.go` 行 139-142

**問題**: `initialize` 失敗時に `c.Close` でクリーンアップするが、そのエラーを捨てている。

```go
if err := c.initialize(initCtx); err != nil {
    _ = c.Close(context.Background())
    return err
}
```

**推奨**: クリーンアップ時のエラーはログに残す。
```go
if err := c.initialize(initCtx); err != nil {
    if closeErr := c.Close(context.Background()); closeErr != nil {
        c.logf("cleanup after initialize failure: %v", closeErr)
    }
    return err
}
```

---

### 8. edits.go:83-85 — DocumentChanges の Unmarshal 失敗を無視

**ファイル**: `internal/lsp/edits.go` 行 83-85

**問題**: `collectWorkspaceEdits` で `DocumentChanges` の各要素を Unmarshal する際、失敗すると `continue` でスキップするだけ。不正な JSON や想定外フォーマットのエントリが黙って無視され、適用される編集が減る。

```go
if err := json.Unmarshal(raw, &candidate); err != nil {
    continue
}
```

**推奨修正**: エラーを返すか、最低限ログを出す。
```go
if err := json.Unmarshal(raw, &candidate); err != nil {
    return nil, fmt.Errorf("documentChanges entry: %w", err)
}
```
または部分適用を許容する場合は、呼び出し元に「一部スキップした」旨を伝える設計を検討。

---

### 9. registry.go:1113 — handleRename 後の EnsureDocument エラーを無視

**ファイル**: `internal/tools/registry.go` 行 1112-1114

**問題**: リネームで複数ファイルを編集した後、各ファイルで `EnsureDocument` を呼ぶがエラーを捨てている。LSP のドキュメント状態と実際のファイル内容がずれる可能性がある。

```go
for _, file := range summary.Files {
    _, _ = s.client.EnsureDocument(ctx, file.Path)
}
```

**推奨修正**: エラーを集約して返すか、ログに残す。
```go
var ensureErrs []error
for _, file := range summary.Files {
    if _, err := s.client.EnsureDocument(ctx, file.Path); err != nil {
        ensureErrs = append(ensureErrs, fmt.Errorf("%s: %w", file.Path, err))
    }
}
if len(ensureErrs) > 0 {
    // ログまたは summary に含めて呼び出し元に伝える
}
```

---

## Suggestion（提案）

### 10. main.go:104 — ログファイル Close エラーを無視

**ファイル**: `cmd/generic-lsp-mcp/main.go` 行 104

**問題**: `cleanupLog` で `file.Close()` のエラーを捨てている。終了時のクリーンアップなので影響は小さいが、ログ出力しておくとトラブル時に役立つ。

```go
return logger, func() { _ = file.Close() }, nil
```

**推奨**: 可能ならログに出力する（標準エラーなど）。

---

### 11. client.go:450,455 — defaultServerRequestResponse のエラー無視

**ファイル**: `internal/lsp/client.go` 行 450, 455

**問題**:
- `workspace/configuration`: `json.Unmarshal(params, &req)` のエラーを無視。不正な params で空の設定を返す。
- `workspace/workspaceFolders`: `PathToURI(c.cfg.RootDir)` のエラーを無視。空の URI を返す可能性。

**推奨**: エラー時は適切なエラー応答を返すか、ログを出す。

---

### 12. registry.go:325,399,469 — URIToPath 失敗時の continue

**ファイル**: `internal/tools/registry.go` 行 325-327, 399-401, 469-471

**問題**: definitions/references 等で `lsp.URIToPath(loc.URI)` が失敗すると `continue` でその location をスキップする。不正な URI のロケーションが結果から消えるが、理由が分からない。

**推奨**: ログを出すか、結果に「スキップした location 数」などを含める。

---

### 13. registry.go:329,403,473 — previewAround エラーを無視

**ファイル**: `internal/tools/registry.go` 行 329, 403, 473

**問題**: `previewAround(path, ...)` のエラーを捨て、`preview` に空文字が入る。ファイル読み取り失敗時もプレビューが空になるだけで、エラーは伝わらない。

```go
preview, _ := previewAround(path, loc.Range.Start.Line, before, after)
```

**推奨**: エラー時は `"preview": "(unable to read file)"` のようなプレースホルダを入れるか、ログを出す。

---

### 14. paths.go:64-68 — RelativePath の filepath.Rel エラー時のフォールバック

**ファイル**: `internal/lsp/paths.go` 行 64-68

**問題**: `filepath.Rel` 失敗時に元の `path` をそのまま返す。パスが不正確になる可能性があるが、フォールバックとしては妥当。ログを出すと原因追跡しやすい。

---

### 15. tools/call のエラーを isError で返す設計

**ファイル**: `internal/mcp/server.go` 行 197-205

**現状**: `tool.Handler` がエラーを返した場合、`isError: true` の content として返している。クライアントにはエラー内容が伝わる。

**評価**: 適切な設計。特に対応不要。

---

## panic / recover

**結果**: プロジェクト内に `recover` や panic 用の `defer` は見当たらない。

**考察**:
- `readLoop` や `handleMessage` で panic が起きると、`defer c.closeDone()` により `closeDone` は呼ばれる。
- `Serve` ループ内で panic が起きると、MCP サーバー全体が停止する。
- 外部入力（JSON、LSP メッセージ）を扱うため、`json.Unmarshal` などで panic する可能性は低いが、ゼロではない。
- 必要に応じて、`readLoop` や `Serve` の最上位に `defer recover()` を入れてログ＋`failPending` する設計を検討できる。

---

## リソースリーク・並行性

### リソース

- **client.go**: `stdin`/`stdout`/`stderr` は `cmd` に紐づき、`cmd.Wait()` でプロセス終了とともに解放される。明示的な `Close` はないが、プロセス終了で十分。
- **app.go Run**: `defer runtime.Close()` でクリーンアップしている。
- **main.go**: `defer cleanupLog()` でログファイルを閉じている。

### 並行性

- **app.go**: `mu` で `started`/`closed` を保護。`Start` と `Close` の競合は適切に処理されている。
- **client.go**: `pmu`/`dmu`/`xmu`/`cmu`/`writerMu` で共有状態を保護。`failPending` は `pmu` を保持したままチャネルへ送信するため、デッドロックの可能性は低い（チャネルはバッファ 1）。
- **stderrLoop**: `readLoop` とは独立して goroutine で動作。プロセス終了で stderr が閉じればループ終了。リークの心配は小さい。

---

## まとめ

| 重要度   | 件数 | 主な内容                                   |
|----------|------|--------------------------------------------|
| Critical | 3    | writeError 失敗無視、scanner.Err 無視、failPending の default |
| Important| 6    | Close エラー無視、edits の Unmarshal 無視、EnsureDocument 無視 |
| Suggestion | 6  | ログ追加、プレビュー/URI エラー時の扱い改善 |

**優先して対応したい箇所**:
1. `server.go:97` — Parse エラー時の `writeError` 失敗を `return` する
2. `client.go` — `stderrLoop` で `scanner.Err()` をログする
3. `edits.go:83-85` — `DocumentChanges` の Unmarshal 失敗をエラーとして扱うか、ログを出す
4. `registry.go:1113` — リネーム後の `EnsureDocument` エラーをログまたは返す
