# 速度改善 — 4グループ並列実装計画

## Context

次世代高速モデル（1,000トークン/秒）×10ペイン並列出力に対応するため、バックエンド→フロントエンド間のデータパイプライン全体を最適化する。`task/速度改善.md` の Phase 1〜2 項目（H-01〜H-05, M-01〜M-06, L-01〜L-05）を4グループ並列で実施。H-02（IPCバイナリ転送）は後続タスクとして除外。

---

## 並列グループ構成

| Group | レイヤー | 変更ファイル | リスク |
|-------|---------|-------------|--------|
| **A** | ターミナル出力バッファ/プール | `output_flush_manager.go`, `terminal_io.go`, `app_pane_feed.go`, `app_events.go`(L19のみ) | LOW |
| **B** | paneState/セッション管理ロック | `panestate/manager.go`, `session_manager_pane_io.go` | MEDIUM |
| **C** | スナップショット/イベント/ルーター | `session_manager_snapshot.go`, `app_snapshot_delta.go`, `app_events.go`(L13のみ), `command_router.go` | LOW-MEDIUM |
| **D** | フロントエンド描画最適化 | `useTerminalEvents.ts`, `useTerminalSetup.ts`, `useBackendSync.ts`, `vite.config.ts` | LOW |

**ファイル競合:** `app_events.go` はGroup A(L19定数)とGroup C(L13定数)が非重複箇所を変更。

---

## Group A: ターミナル出力バッファ/プール層

### H-01: OutputFlushManager バッファサイズ 8KB→32KB
- **`myT-x/app_events.go` L19**: `outputFlushBufSize = 8*1024` → `32*1024`
- **`myT-x/internal/terminal/output_flush_manager.go` L51**: デフォルト fallback も `32*1024` に統一
- 根拠コメント必須: 「1000tok/s ≈ 6KB/s/pane、32KBで wakeCh 頻度4x減」
- `outputFlushInterval`(16ms)、`maxBufferedAge`(64ms)、`nextInterval()` は変更不要

### M-01: readSource バッファ 32KB→64KB
- **`myT-x/internal/terminal/terminal_io.go` L131**: `make([]byte, 32*1024)` → `make([]byte, 64*1024)`
- 根拠コメント: 「syscall頻度削減、1ペイン1goroutineのスタックローカル割当」

### L-01: feedBytePool 初期容量 4KB→8KB
- **`myT-x/app_pane_feed.go` L10**: `make([]byte, 0, 4096)` → `make([]byte, 0, 8192)`
- **`myT-x/app_pane_feed.go` L23**: `maxPoolBufSize = 64*1024` → `128*1024`
- 根拠コメント: 「高速モデルのチャンクサイズに合わせ grow-copy 削減」

### テスト
- `output_flush_manager_test.go`: maxBytes=32768 に関するアサーション更新 + バッファ閾値テスト追加
- `app_pane_feed_test.go`: maxPoolBufSize 境界テスト（>128KB廃棄、<=128KBリサイクル確認）

---

## Group B: paneState / セッション管理ロック改善

### M-03: SessionManager.mu ロック早期解放 (**最重要変更**)
- **`myT-x/internal/tmux/session_manager_pane_io.go`**

**WriteToPane (L152-178):**
```go
// 現状: m.mu.RLock() → pane lookup → pane.Terminal.Write() → m.mu.RUnlock()
// 問題: ConPTY syscall中にRLock保持 → 10ペイン並列Write時にボトルネック

// 修正: lookup後にUnlock、Terminal pointerローカルコピーでlock-free Write
m.mu.RLock()
pane := m.panes[id]
if pane == nil || pane.Terminal == nil {
    m.mu.RUnlock()
    return fmt.Errorf(...)
}
term := pane.Terminal
m.mu.RUnlock()
// NOTE: Lock-free I/O — Terminal.Write is internally synchronized via Terminal.mu.
// Terminal pointer is set once at creation and never replaced. See checklist #83.
_, err = term.Write([]byte(data))
```

**WriteToPanesInWindow (L180-213):** 同パターン — ロック内でターミナルポインタをスライスに収集、Unlock後にループWrite。

- ResizePane (L222-246): 変更不要（Resize は軽量、且つpane.Width/Height変更にLock必須）

### H-03: paneState Feed パス ドキュメント追加
- **`myT-x/internal/panestate/manager.go` L179**: Feed()の`TrimSpace`に防御的理由コメント追加
- **paneFeedCh (app.go)**: バッファサイズ4096の根拠コメント追加（変更なし）

### L-02: replayRing 容量ドキュメント追加
- **`myT-x/internal/panestate/manager.go` L120**: デフォルト512KBの根拠コメント追加（変更なし）

### テスト
- `TestWriteToPane_ConcurrentAccess`: 並行WriteToPane + CreateSession でデッドロックしないこと
- `TestWriteToPanesInWindow_PaneKilledDuringWrite`: Write中にペイン削除 → エラー返却（panic なし）
- `go test ./internal/tmux/... -count=100` で競合確認

---

## Group C: スナップショット/イベント/ルーター層

### M-04: デバッグログのレベルガード
- **`myT-x/internal/tmux/command_router.go` L493-499**:
```go
// 現状: fmt.Sprintf("%v", req.Flags) が全Execute呼び出しで割当発生
// 修正: slog.LevelDebug有効時のみ実行
if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
    slog.Debug("[DEBUG-SHIM] Execute",
        "command", req.Command,
        "flags", fmt.Sprintf("%v", req.Flags),
        "args", req.Args, "env", req.Env, "callerPane", req.CallerPane)
}
```
- `"context"` import 追加
- 他の slog.Debug (L220, L335等) は Sprintf なし→ガード不要

### M-02: snapshotCoalesceWindow ドキュメント
- **`myT-x/app_events.go` L13**: 50ms の根拠コメント + TODO(config) 追加

### H-05: スナップショット deep clone 分析 + ベンチマーク
- **`myT-x/internal/tmux/session_manager_snapshot.go`**: `Snapshot()` の3箇所の `cloneSessionSnapshots` はスレッド安全性のため必要
- **`myT-x/app_snapshot_delta.go`**: `copySnapshotCache` は shallow copy で既に最適
- **実施内容:**
  1. ベンチマークテスト追加: `BenchmarkSnapshotClone` (10 sessions×5 windows×4 panes)
  2. 結果が >1ms なら `SnapshotReadOnly()` (ポインタ返却、read-only契約) を導入
  3. 結果が <1ms ならコメント記録のみ（過剰最適化回避）

### テスト
- `BenchmarkSnapshotClone`, `BenchmarkSnapshotDelta_NoChange`, `BenchmarkSnapshotDelta_OneSessionChanged`
- command_router 既存テスト pass 確認

---

## Group D: フロントエンド描画最適化

### H-04: RAF バッチ書き込み最適化
- **`myT-x/frontend/src/hooks/useTerminalEvents.ts` L239-260**:
```typescript
// 現状: pendingWrites.join("") → 中間文字列割当
// 修正: term.write() を複数回呼び出し（xterm.js内部でバッファリング）
try {
    for (let i = 0; i < pendingWrites.length; i++) {
        term.write(pendingWrites[i]);
    }
} catch (err) { ... }
pendingWrites.length = 0;
```
- composingOutput パスは低頻度のため join("") 維持

### M-06: 非表示タブのイベントスロットリング
- **同ファイル**: `document.visibilitychange` リスナー追加
```typescript
let pageHidden = document.hidden;
const onVisibilityChange = () => { pageHidden = document.hidden; };
document.addEventListener("visibilitychange", onVisibilityChange);
```
- `enqueuePendingWrite` に `if (disposed || pageHidden) return;` ガード追加
- cleanup で `document.removeEventListener("visibilitychange", onVisibilityChange)` 必須

### M-05: scrollback 10000→5000
- **`myT-x/frontend/src/hooks/useTerminalSetup.ts` L63**: `scrollback: 5000` + 根拠コメント

### L-03: WebGL リトライ
- **同ファイル L13-28**: `webglUnavailableSince` タイムスタンプ追加、30秒後に新ペインでリトライ許可
```typescript
function shouldAttemptWebgl(): boolean {
    if (!webglUnavailable) return true;
    if (webglUnavailableSince !== null &&
        Date.now() - webglUnavailableSince >= 30_000) {
        webglUnavailable = false;
        webglUnavailableSince = null;
        return true;
    }
    return false;
}
```

### L-04: 初回ロード3者並列化
- **`myT-x/frontend/src/hooks/useBackendSync.ts` L123, L140-142**:
  `GetConfigAndFlushWarnings` を `Promise.allSettled` に統合（3-way parallel）
- `configEventVersionRef.current > 0` ガード維持必須

### L-05: Vite ビルド最適化
- **`myT-x/frontend/vite.config.ts`**: terser minify追加、treeshake設定追加
- 前提: `npm install -D terser`

---

## 全グループ共通ワークフロー（必須）

```
1. confidence-check（90%以上で実行可）
2. defensive-coding-checklist 該当項目走査
3. 実装（GoLand MCP tools 使用: replace_text_in_file → reformat_file → build_project）
4. テスト作成（self-review の前に必須）
5. self-review → 全クリアまで繰り返し
6. 最終検証: go test ./... / npm run build
```

## 防御的コーディング重点項目

| # | チェック項目 | 対象Group |
|---|------------|-----------|
| #54 | mutex保持中に外部呼び出ししていないか | B (M-03で修正) |
| #58 | イベント発行がmutex外で行われているか | B |
| #74 | **全ての定数・マジックナンバーに根拠コメント** | ALL |
| #83 | ロック外I/Oパターンに NOTE コメント | B |
| #96 | RAF/setTimeout/addEventListener に cleanup | D |
| #109 | 同じロジックの重複なし | ALL |
| #145 | ホットパスで重い関数呼び出しをキャッシュ | C (M-04) |

## レビュアー対策: ゼロ指摘基準

- マジックナンバーは全て根拠コメント付き
- ロック解放箇所には安全性の説明コメント
- timer/listener は必ず cleanup
- エラーのサイレント無視禁止
- 既存テストの pass 確認 + 新規テスト追加

## 検証方法

1. `cd myT-x && go test ./...` — 全バックエンドテスト pass
2. `cd myT-x/frontend && npm run build` — フロントエンドビルド成功
3. `build_project` (GoLand MCP) — コンパイルエラーゼロ
4. `get_file_problems` — 各変更ファイルの警告ゼロ
5. 手動: 10ペイン × `yes` コマンド並列実行 → 応答性確認
