# IME変換障害 調査結果 & 修正計画

## Context

### 報告された症状
- セッション開始後、ターミナルでの日本語入力時にIME変換（スペースキーでの漢字変換）が効かない
- ひらがな入力は正常（`ka` → `か` は出る）が、変換候補が出ず確定されてしまう
- ペインタイトル入力欄ではIME変換は正常に動作する

### 根本原因: アプリの二重起動によるWebView2ブラウザプロセス競合

調査の結果、**アプリが二重起動されている場合にのみ発生する**ことが判明。

Wails v2はWebView2を使用しており、2つのインスタンスが同一のユーザーデータディレクトリ（`%LOCALAPPDATA%`配下）を共有する。これにより:

1. **WebView2ブラウザプロセスの共有/競合**: 2つのインスタンスが同一ブラウザプロセスを共有し、IMEコンテキストが破損。ひらがな入力（レンダラーレベル処理）は動作するが、漢字変換（ブラウザプロセス調整が必要）が失敗する
2. **Named Pipe競合**: 両インスタンスが同一パイプ名 `\\.\pipe\myT-x-<user>` でリッスンを試みる（2つ目は失敗）
3. **グローバルホットキー競合**: デフォルトで `QuakeMode: true` のため、両インスタンスが `Ctrl+Shift+F12` を `RegisterHotKey` で登録しようとする（2つ目は失敗）

両インスタンスを閉じて1つだけ起動すると復旧することは、WebView2の共有ブラウザプロセスの状態破損を裏付ける。

## 修正方針: Windows Named Mutex による単一インスタンス制御

### 設計判断

- **Named Mutex採用理由**: Windows OSカーネルがプロセス終了時（クラッシュ含む）に自動解放する。ファイルロックのようなスタルロック問題がない
- **チェック位置**: `main()` 内、`wails.Run()` の前。WebView2初期化前にブロックする必要がある
- **既存インスタンスへの通知**: 既存の Named Pipe IPC を通じて `activate-window` コマンドを送信し、既存ウィンドウを前面に表示してから2つ目のインスタンスを終了

## 変更対象ファイル

### 新規作成

| ファイル | 内容 |
|---------|------|
| `myT-x/internal/singleinstance/singleinstance_windows.go` | `TryLock`, `Release`, `DefaultMutexName`, `ErrAlreadyRunning` |
| `myT-x/internal/singleinstance/singleinstance_other.go` | 非Windows用 no-op スタブ |
| `myT-x/internal/singleinstance/singleinstance_windows_test.go` | テーブル駆動テスト |
| `myT-x/internal/tmux/command_router_handlers_window.go` | `handleActivateWindow` ハンドラ |

### 既存ファイル修正

| ファイル | 変更内容 |
|---------|---------|
| `myT-x/main.go` | `wails.Run()` 前にmutexチェック追加、`ipc.Send` でactivate-windowコマンド送信 |
| `myT-x/internal/tmux/command_router.go` | handlers mapに `"activate-window"` 登録（L124付近） |
| `myT-x/app_events.go` | `emitBackendEvent` に `"app:activate-window"` ハンドリング追加 |
| `myT-x/app_lifecycle.go` | `bringWindowToFront()` メソッド追加（`toggleQuakeWindow` のShow部分を再利用） |

## 実装手順

### Step 1: `internal/singleinstance` パッケージ作成

**`singleinstance_windows.go`** (`//go:build windows`):
- `TryLock(name string) (*Lock, error)`: `windows.CreateMutex` で名前付きmutex作成。`ERROR_ALREADY_EXISTS` 時は `ErrAlreadyRunning` を返す
- `(*Lock).Release()`: `windows.CloseHandle` でハンドル解放。nil安全・冪等
- `DefaultMutexName() string`: `Global\myT-x-<sanitized-username>` を返す。ユーザー名サニタイズは `ipc/protocol.go:sanitizeUsername` のロジックを複製（循環参照回避）

**`singleinstance_other.go`** (`//go:build !windows`):
- 全関数のno-opスタブ

### Step 2: `activate-window` IPCコマンド追加

**`command_router_handlers_window.go`**:
```go
func (r *CommandRouter) handleActivateWindow(req ipc.TmuxRequest) ipc.TmuxResponse {
    slog.Info("[DEBUG-IPC] activate-window command received")
    r.emitter.Emit("app:activate-window", nil)
    return ipc.TmuxResponse{ExitCode: 0, Stdout: "ok\n"}
}
```

**`command_router.go`** L124のhandlers mapに追加:
```go
"activate-window": router.handleActivateWindow,
```

### Step 3: App側のウィンドウ前面表示

**`app_events.go`** `emitBackendEvent` 内、`tmux:pane-output` チェックの前に追加:
```go
if name == "app:activate-window" {
    a.bringWindowToFront()
    return
}
```

**`app_lifecycle.go`** に `bringWindowToFront` 追加（`toggleQuakeWindow` のShow部分と同一ロジック）:
```go
func (a *App) bringWindowToFront() {
    ctx := a.runtimeContext()
    if ctx == nil {
        return
    }
    runtimeWindowShowFn(ctx)
    runtimeWindowUnminimiseFn(ctx)
    runtimeWindowSetAlwaysOnTopFn(ctx, true)
    runtimeWindowSetAlwaysOnTopFn(ctx, false)
    a.setWindowVisible(true)
}
```

### Step 4: `main.go` にmutexチェック追加

`wails.Run()` の前に:
```go
mutexLock, err := singleinstance.TryLock(singleinstance.DefaultMutexName())
if errors.Is(err, singleinstance.ErrAlreadyRunning) {
    slog.Info("[DEBUG-SINGLE] another instance is already running, signaling activation")
    if _, sendErr := ipc.Send("", ipc.TmuxRequest{Command: "activate-window"}); sendErr != nil {
        slog.Warn("[DEBUG-SINGLE] failed to signal existing instance", "error", sendErr)
    }
    return
}
if err != nil {
    slog.Warn("[DEBUG-SINGLE] mutex creation failed, proceeding without guard", "error", err)
}
if mutexLock != nil {
    defer mutexLock.Release()
}
```

### Step 5: テスト作成

| テストケース | ファイル |
|------------|---------|
| TryLock 成功（初回） | `singleinstance_windows_test.go` |
| TryLock で `ErrAlreadyRunning`（二重取得） | `singleinstance_windows_test.go` |
| Release 後の再取得成功 | `singleinstance_windows_test.go` |
| Release 冪等性 | `singleinstance_windows_test.go` |
| DefaultMutexName のフォーマット検証 | `singleinstance_windows_test.go` |
| handleActivateWindow のレスポンス検証 | `command_router_handlers_window_test.go` |

## 開発フロー

```
confidence-check → 実装 → テスト作成 → self-review
```

1. `confidence-check` で実装前チェック
2. `golang-expert` エージェントで実装
3. `go-test-patterns` でテーブル駆動テスト作成
4. `self-review` で検証（全クリアまで）

## 検証方法

1. **自動テスト**: `go test ./internal/singleinstance/... ./internal/tmux/...`
2. **手動E2Eテスト**:
   - アプリを起動 → 日本語IME変換が正常に動作することを確認
   - 2つ目のインスタンスを起動 → 1つ目のウィンドウが前面に来て、2つ目が自動終了することを確認
   - 1つ目のインスタンスでIME変換が引き続き正常に動作することを確認
3. **ビルド確認**: `mcp goland build_project` でコンパイルエラーがないことを確認
