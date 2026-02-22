# Claude-Code-Communication互換対応

## Context

`sample/Claude-Code-Communication-main` は、tmuxを使って複数のClaude Codeインスタンスを協調動作させるマルチエージェントシステム。
現在のmyT-xのtmux shimでは、セットアップスクリプト(`setup.sh`)が `select-pane -T` フラグ（ペインタイトル設定）でパースエラーを起こし、`set -e` により**スクリプト全体が中断**する。

### ギャップ分析結果

| コマンド | setup.sh使用 | myT-x対応 | 影響度 |
|---------|:----------:|:---------:|:-----:|
| `kill-session -t` | line 24-25 | OK | - |
| `new-session -d -s -n` | line 38, 76 | OK | - |
| `split-window -h/-v -t` | line 41, 43, 45 | OK | - |
| `select-pane -t` | line 42, 44 | OK | - |
| **`select-pane -t -T`** | **line 52** | **NG** | **致命的** |
| `send-keys -t ... C-m` | line 55-67, 77-81 | OK | - |
| `has-session -t` | agent-send.sh:85 | OK | - |
| `list-sessions` | line 95 | OK | - |
| `attach-session -t` | help text only | NG | 低(手動コマンド) |

## 実装計画

### Step 1: `select-pane -T` フラグ追加（必須）

**変更ファイル:**
- `myT-x/cmd/tmux-shim/spec.go` — `-T` を flagString として追加
- `myT-x/internal/tmux/command_router_handlers_pane.go` — `handleSelectPane` に `-T` 処理追加

**実装内容:**
1. `spec.go` の `select-pane` に `"-T": flagString` を追加
2. `handleSelectPane` で `-T` フラグがある場合、`r.sessions.RenamePane(target.IDString(), title)` を呼び出し
3. タイトル変更時に `tmux:pane-title-changed` イベントを発行（フロントエンドでタイトル反映するため）

**再利用する既存関数:**
- `sessions.RenamePane(paneID string, title string)` — `session_manager_pane_io.go:178` に既に実装済み
- `handleSelectPane` 内の既存ターゲット解決ロジック

### Step 2: `attach-session` コマンド追加（推奨）

**変更ファイル:**
- `myT-x/cmd/tmux-shim/spec.go` — コマンド定義追加
- `myT-x/internal/tmux/command_router.go` — ハンドラ登録
- `myT-x/internal/tmux/command_router_handlers_session.go` — ハンドラ実装

**実装内容:**
1. `spec.go` に `attach-session` コマンドを追加（flags: `-t` flagString）
2. ハンドラ: セッション存在確認 → `app:activate-window` イベント発行 → 成功応答
3. myT-xはGUIアプリなので、実質的にはウィンドウをアクティブにするだけ

### Step 3: テスト作成

**新規テスト:**
- `select-pane -T` のテスト: タイトル設定、空タイトル、セレクトと同時にタイトル設定
- `attach-session` のテスト: 存在するセッション、存在しないセッション

**既存テスト確認:**
- `TestRenamePane` (`session_manager_test.go:44`) — 既にパス済み

### Step 4: self-review

実装完了後、`self-review` エージェントで検証。

## 検証方法

1. `go test ./myT-x/cmd/tmux-shim/...` — shimパーサーテスト
2. `go test ./myT-x/internal/tmux/...` — ハンドラテスト
3. myT-xアプリ起動後、以下のコマンドが成功することを確認:
   ```bash
   tmux new-session -d -s test -n "main"
   tmux split-window -h -t "test:0"
   tmux select-pane -t "test:0.0" -T "boss1"
   tmux send-keys -t "test:0.0" "echo hello" C-m
   tmux has-session -t test
   tmux attach-session -t test
   tmux kill-session -t test
   ```
