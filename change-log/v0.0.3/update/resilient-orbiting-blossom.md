# ペイン分割時の作業ディレクトリ修正

## Context

セッション画面でペインを手動追加（分割）した場合、新しいペインの作業ディレクトリがアプリケーションのカレントディレクトリになる不具合がある。セッションで開いている作業フォルダで開くべき。

**根本原因**: `SplitWindowInternal` が `splitWindowResolved` に `workDir=""` を渡しており、ターミナルプロセスがアプリのCWDを継承してしまう。

## 方針: PaneContextSnapshot を拡張

`PaneContextSnapshot` に `SessionWorkDir` フィールドを追加し、`splitWindowResolved` で `workDir` が空の場合にフォールバックとして使用する。

- スレッドセーフ: 既存の `mu.RLock` スコープ内で追加読み取り
- IPC経路は影響なし: `-c` フラグが明示指定される為、フォールバックは発動しない
- 最小変更: 本体2ファイル + テスト3ファイル

## 実装ステップ

### Step 1: `PaneContextSnapshot` にフィールド追加
**File**: `myT-x/internal/tmux/session_manager_env.go`

```go
type PaneContextSnapshot struct {
    SessionID      int
    SessionName    string
    Layout         *LayoutNode
    Env            map[string]string
    Title          string
    SessionWorkDir string  // セッションの実効作業ディレクトリ
}
```

### Step 2: `GetPaneContextSnapshot` でフィールドを設定
**File**: `myT-x/internal/tmux/session_manager_env.go`

セッションの実効作業ディレクトリ解決ロジック:
- Worktreeセッション (`Worktree.Path` が非空): `Worktree.Path` を使用
- 通常セッション: `RootPath` を使用

```go
workDir := pane.Window.Session.RootPath
if wt := pane.Window.Session.Worktree; wt != nil && strings.TrimSpace(wt.Path) != "" {
    workDir = wt.Path
}
```

### Step 3: `splitWindowResolved` でフォールバック追加
**File**: `myT-x/internal/tmux/command_router_handlers_pane.go`

`targetCtx` 取得後、`workDir` が空の場合に `targetCtx.SessionWorkDir` を使用:

```go
if strings.TrimSpace(workDir) == "" {
    workDir = targetCtx.SessionWorkDir
}
```

### Step 4: フィールド数ガードテスト更新
**File**: `myT-x/app_snapshot_delta_test.go` (L135)

`PaneContextSnapshot` のフィールド数を `5` → `6` に更新。

### Step 5: `TestGetPaneContextSnapshot` 拡張
**File**: `myT-x/internal/tmux/session_manager_env_test.go`

以下のケースを追加:
1. **通常セッション**: `SetRootPath` 後、`SessionWorkDir == rootPath` を検証
2. **Worktreeセッション**: `SetWorktreeInfo(Path非空)` 後、`SessionWorkDir == worktreePath` を検証（RootPathより優先）
3. **未設定**: 両方未設定の場合、`SessionWorkDir == ""` を検証

### Step 6: `SplitPane` 統合テスト
**File**: `myT-x/app_pane_api_test.go`

セッションに `RootPath` を設定した状態で `SplitPane` を呼び出し、ペインが正常に作成されることを検証。

## 開発フロー

1. `confidence-check` (実装前チェック)
2. `defensive-coding-checklist` クイックチェック走査
3. `golang-expert` で実装
4. `go-test-patterns` でテスト作成
5. `self-review` (全クリアまで)

## 検証方法

1. `go test ./myT-x/internal/tmux/... -run TestGetPaneContextSnapshot` - スナップショットのSessionWorkDir検証
2. `go test ./myT-x/... -run TestSnapshot` - フィールド数ガードテスト
3. `go build ./myT-x/...` - ビルド成功確認
4. 手動: アプリ起動 → セッション作成 → ペイン分割 → 新ペインの作業ディレクトリがセッションのフォルダであることを確認
