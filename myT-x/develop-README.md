# myT-x

## 概要

Windows 向け tmux 再実装アプリケーション (Wails v2 + Go)。

プロジェクト設定は `wails.json` を編集することで変更可能。
詳細は https://wails.io/docs/reference/project-config を参照。

## 開発

ライブ開発モードで起動するには、プロジェクトディレクトリで `wails dev` を実行する。
Vite 開発サーバーが起動し、フロントエンドの変更がホットリロードされる。
ブラウザから Go メソッドにアクセスしたい場合は、http://localhost:34115 の開発サーバーに接続する。

## ビルド

配布用のプロダクションビルドを作成するには以下を実行する:

```
make build
```

これにより tmux-shim.exe が自動ビルドされ、myT-x.exe に埋め込まれた単一バイナリが生成される。
ビルド成果物は `build/bin/myT-x.exe` に出力される。

ビルド後にバイナリを圧縮する場合（任意）:

`upx --best build/bin/myT-x.exe`

### Makefile ターゲット

| ターゲット              | 説明                                       |
|--------------------|------------------------------------------|
| `make dev`         | 開発モード。shim を別ファイルとしてビルドし `wails dev` を起動 |
| `make build`       | 本番ビルド。shim を myT-x.exe に埋め込んだ単一バイナリを生成   |
| `make build-shim`  | tmux-shim.exe のみをビルド                     |
| `make clean-embed` | 埋め込み用の中間ファイルを削除                          |

## tmux-shim.exe (CLI 互換レイヤー)

tmux-shim.exe は、シェルやスクリプトから `tmux` コマンドを実行可能にするための CLI プロキシ。
tmux 互換のコマンドライン引数を受け取り、Windows Named Pipe (IPC) 経由で myT-x 本体に転送する。

tmux-shim.exe が無くても myT-x 本体は正常に起動・使用できる。
shim が必要になるのは、ペイン内から `tmux` コマンドを呼び出す機能（Agent Teams 等）を利用する場合のみ。

```
シェル / スクリプト
    ↓ tmux new-session -d -s dev
tmux.exe (tmux-shim.exe のリネームコピー)
    ↓ JSON リクエスト
Named Pipe IPC (\\.\pipe\myT-x-{USERNAME})
    ↓
myT-x 本体 (PipeServer → CommandRouter)
    ↓ JSON レスポンス
シェル / スクリプト (stdout/stderr + 終了コード)
```

### 対応コマンド

| コマンド              | 動作              |
|-------------------|-----------------|
| `new-session`     | セッションを新規作成      |
| `has-session`     | セッションの存在確認      |
| `kill-session`    | セッションを終了        |
| `list-sessions`   | 全セッションを一覧表示     |
| `split-window`    | ペインを分割          |
| `select-pane`     | ペインを選択・移動       |
| `list-panes`      | セッション内のペインを一覧表示 |
| `send-keys`       | ペインにキー入力を送信     |
| `display-message` | メッセージを表示        |

### 配置と配布

`make build` で生成された myT-x.exe には **tmux-shim.exe が埋め込まれている**。
myT-x 起動時に埋め込みバイナリを `%LOCALAPPDATA%\myT-x\bin\tmux.exe` として展開する。
同時に `%LOCALAPPDATA%\myT-x\bin` をユーザー PATH (レジストリ `HKCU\Environment\Path`) に追加するため、
ターミナルから `tmux` コマンドとして直接呼び出せるようになる。

| 項目     | パス                                    |
|--------|---------------------------------------|
| 配布バイナリ | `myT-x.exe` (shim 埋め込み済み)             |
| 自動展開先  | `%LOCALAPPDATA%\myT-x\bin\tmux.exe`   |
| デバッグログ | `%LOCALAPPDATA%\myT-x\shim-debug.log` |

開発時 (`make dev`) は shim を埋め込まず、`myT-x.exe` と同階層の `tmux-shim.exe` をファイルベースで検出する。
見つからない場合はワークスペースから `go build` で自動ビルドされる。

### ビルド

```
go build -o tmux-shim.exe ./cmd/tmux-shim
```

## 設定

設定ファイルは `%LOCALAPPDATA%/myT-x/config.yaml` に配置されます。

**設定画面:** メニューバーの「設定 > config.yaml」から GUI で設定を編集できます。一部の設定（Shell、Global Hotkey
等）はアプリ再起動後に反映されます。

**注意:** 配布フォルダ内に `config.yaml` を同梱した場合、初回起動時に `%LOCALAPPDATA%` へコピーされます。一度コピーされた後は
`%LOCALAPPDATA%` 内のファイルが優先して読み込まれるため、配布フォルダ内のファイルを変更しても動作には反映されません。

ファイルが存在しない場合は初回起動時にデフォルト値で自動生成されます。

```yaml
shell: powershell.exe
prefix: Ctrl+b
quake_mode: true
global_hotkey: Ctrl+Shift+F12
keys:
  split-vertical: "%"
  split-horizontal: "\""
  toggle-zoom: z
  kill-pane: x
  detach-session: d
worktree:
  enabled: true
  force_cleanup: false
  setup_scripts: []
  copy_files: []
  copy_dirs: []
agent_model:
  from: "claude-opus-4-6"
  to: "claude-sonnet-4-5-20250929"
  overrides:
    - name: "security"
      model: "claude-opus-4-6"
    - name: "reviewer"
      model: "claude-sonnet-4-5-20250929"
    - name: "coder"
      model: "claude-opus-4-6"
pane_env:
  CLAUDE_CODE_EFFORT_LEVEL: "high"
```

### 基本設定

#### shell

ターミナルペインおよびセットアップスクリプト実行時に使用するシェル。

| 値                        | スクリプト実行フラグ |
|--------------------------|------------|
| `powershell.exe` (デフォルト) | `-Command` |
| `pwsh.exe`               | `-Command` |
| `cmd.exe`                | `/c`       |
| `bash.exe`               | `-c`       |
| `wsl.exe`                | `-c`       |

上記以外の値はセキュリティ上拒否される。絶対パス指定時はファイルの存在確認が行われる。

#### quake_mode

`true` の場合、`global_hotkey` で指定したキーでウィンドウの表示/非表示を切り替える Quake スタイルのトグル機能が有効になる。
`false` の場合、グローバルホットキーは登録されない。

#### global_hotkey

Quake モードのトグルに使用するグローバルホットキー。`quake_mode: true` の場合のみ有効。
形式: `修飾キー+キー名` (例: `Ctrl+Shift+F12`, `Alt+Tilde`)
使用可能な修飾キー: `Ctrl`, `Shift`, `Alt`, `Win`

#### prefix

tmux 互換のプレフィックスキー定義。
現在のフロントエンド実装では `Ctrl+b` がハードコードされており、この設定値による変更は反映されない。

#### keys

プレフィックスキーに続けて入力するアクションキーの定義。

| キー名                | デフォルト | 動作          |
|--------------------|-------|-------------|
| `split-vertical`   | `%`   | ペインを垂直分割    |
| `split-horizontal` | `"`   | ペインを水平分割    |
| `toggle-zoom`      | `z`   | ペインのズーム切り替え |
| `kill-pane`        | `x`   | ペインを閉じる     |
| `detach-session`   | `d`   | セッションからデタッチ |

現在のフロントエンド実装ではハードコードされており、この設定値による変更は反映されない。

### Worktree 設定

#### worktree.enabled

`true` の場合、git worktree を利用したセッション作成機能が有効になる。
`false` の場合、worktree 関連の API 呼び出しはエラーを返す。

#### worktree.force_cleanup

worktree 削除時の安全チェックを制御する。

| 値               | 未コミット変更がある場合 | 削除失敗時          |
|-----------------|--------------|----------------|
| `false` (デフォルト) | 削除を中止しデータを保護 | 削除を中止          |
| `true`          | 警告のみで強制削除    | `--force` で再試行 |

#### worktree.setup_scripts

worktree 作成後に非同期で順番に実行されるスクリプトのリスト。
セッション作成はブロックされない。最初のスクリプト失敗で以降の実行は中止される。
各スクリプトのタイムアウトは 5 分。作業ディレクトリは worktree パス。

```yaml
worktree:
  setup_scripts:
    - "npm install"
    - "npm run setup-dev"
```

#### worktree.copy_files

worktree 作成時にリポジトリルートから worktree へコピーするファイルのリスト。
`.env` など git 管理外のファイルを worktree に複製する用途。
相対パスのみ指定可能。絶対パスや `..` を含むパスはセキュリティ上拒否される。
存在しないファイルは無視される。

```yaml
worktree:
  copy_files:
    - ".env"
    - ".env.local"
```

#### worktree.copy_dirs

worktree 作成時にリポジトリルートから worktree へ再帰的にコピーするディレクトリのリスト。
`.vscode` や `vendor` など git 管理外のディレクトリを worktree に複製する用途。
相対パスのみ指定可能。絶対パスや `..` を含むパスはセキュリティ上拒否される。
存在しないディレクトリは無視される。ディレクトリ内のシンボリックリンクがリポジトリ外を指している場合はスキップされる。

```yaml
worktree:
  copy_dirs:
    - ".vscode"
    - "vendor"
```

### Agent Model 設定

Claude Code Agent Teams が子エージェントを起動する際の `--model` フラグを自動置換する機能。
親ペインからの子ペイン生成時にのみ適用され、操作者が直接入力する初回セッションには影響しない。

```yaml
agent_model:
  from: "claude-opus-4-6"
  to: "claude-sonnet-4-5-20250929"
  overrides:
    - name: "security"
      model: "claude-opus-4-6"
    - name: "reviewer"
      model: "claude-sonnet-4-5-20250929"
    - name: "coder"
      model: "claude-opus-4-6"
```

#### agent_model.from / agent_model.to

デフォルトのモデル置換ルール。子エージェントの `--model` が `from` と一致する場合、`to` に置換する。
`from` に `ALL`（大文字小文字不問）を指定すると、すべてのモデルを `to` に一括置換する。
両方が設定されている場合のみ有効。

#### agent_model.overrides

`--agent-name` の値に基づくモデルオーバーライド。`from`/`to` より優先される。
上から順に評価され、最初にマッチしたルールが適用される（大文字小文字を区別しない部分一致）。

| フィールド   | 必須  | 説明                             |
|---------|-----|--------------------------------|
| `name`  | Yes | `--agent-name` に含まれる文字列（5文字以上） |
| `model` | Yes | 適用するモデル名                       |

**処理優先度:**

1. `overrides` を上から順に走査 → `--agent-name` が `name` を含む → そのルールの `model` を適用
2. `overrides` に一致なし → `from`/`to` で判定 → `from` が `ALL` なら全モデルを `to` に置換、それ以外は `--model` が
   `from` と一致すれば `to` に置換
3. いずれも一致なし → 変更なし

**例:** `--agent-name security-reviewer` の場合、上記設定では `security` が先にマッチし `claude-opus-4-6` が適用される。

#### 指定可能なモデル一覧

2025年2月時点で Claude Code の `--model` フラグに指定可能な主要モデル:

| モデル ID                       | 説明                         |
|------------------------------|----------------------------|
| `claude-opus-4-6`            | Claude Opus 4.6 - 最高性能モデル  |
| `claude-sonnet-4-5-20250929` | Claude Sonnet 4.5 - バランス型  |
| `claude-haiku-4-5-20251001`  | Claude Haiku 4.5 - 高速・低コスト |

モデル ID は Anthropic
のリリースに応じて更新される。最新の利用可能なモデルは [Anthropic ドキュメント](https://docs.anthropic.com/en/docs/about-claude/models)
を参照。

### ペイン環境変数 (pane_env)

ペイン生成時に自動で埋め込む環境変数を設定する。
設定画面の「環境変数」タブから GUI で編集できる。

```yaml
pane_env:
  CLAUDE_CODE_EFFORT_LEVEL: "high"
  MY_CUSTOM_VAR: "value"
```

| フィールド      | 型                   | 説明             |
|------------|---------------------|----------------|
| `pane_env` | `map[string]string` | キー=環境変数名、値=設定値 |

**動作仕様:**

- ペイン生成（`new-session`、`split-window`）時に自動でプロセス環境変数に注入される
- コマンドの `-e` フラグで同じキーを指定した場合、`-e` の値が優先される（`pane_env` は最低優先のデフォルト）
- `PATH`, `COMSPEC`, `SYSTEMROOT` 等のシステム変数は上書きできない（ブロック対象）
- `CLAUDE_CODE_EFFORT_LEVEL` は Claude Code の思考レベルを制御する環境変数

## Known Concurrency Constraints (既知の並行処理制約)

### C-02: resolveTarget の RLock→ポインタ→unlock→使用パターン

#### 概要

`SessionManager.ResolveTarget()` (および内部の `resolveTargetRLocked()`) は、RLock を保持した状態でターゲットペインを検索し、
`*TmuxPane` のライブポインタを返す。呼び出し元はロック解放後にこのポインタのフィールドを読み取る。Go
のメモリモデル上、このパターンは厳密にはデータレースを構成する。

```
RLock → m.panes[id] → *TmuxPane を取得 → RUnlock → pane.ID, pane.Width 等を読み取り
                                                    ↑ この時点でロックは保持していない
```

#### 影響を受けるハンドラー

以下のハンドラーが `resolveTargetFromRequest()` 経由でこのパターンを使用している:

| ハンドラー                        | ファイル                                   | 用途                                   |
|------------------------------|----------------------------------------|--------------------------------------|
| `handleDisplayMessage`       | `command_router_handlers_display.go`   | `-p` フラグ付き display-message でフォーマット展開 |
| `handleSendKeys`             | `command_router_handlers_pane.go:137`  | send-keys のターゲットペイン解決                |
| `handleListPanes`            | `command_router_handlers_pane.go:177`  | list-panes のフォーマット出力                 |
| `handleSplitWindow`          | `command_router_handlers_pane.go:257`  | split-window のターゲット解決                |
| `handleSelectPane`           | `command_router_handlers_pane.go:355`  | select-pane のターゲット解決                 |
| `handleKillPane`             | `command_router_handlers_pane.go:410`  | kill-pane のターゲット解決                   |
| `resolveWindowIDFromRequest` | `command_router_handlers_window.go:78` | ウィンドウ操作のペイン経由解決                      |

**注**: `handleDisplayMessage` については、`expandFormatSafe()` (`format.go`) が TOCTOU-safe
なクローンベースのフォーマット展開を提供している。他のハンドラーでは、ライブポインタから `pane.ID` のようなスカラフィールドのみを即座に読み取り、
`GetPaneContextSnapshot()` を通じて安全なスナップショットに切り替えるパターンが採用されている。

#### 許容理由

1. **プロジェクトポリシー**: `go test -race`
   は本プロジェクトの環境ではセットアップが困難であるため、使用しない方針としている (CLAUDE.md 参照)
2. **陳腐データ許容契約**: ハンドラーは `ResolveTarget()` が返すポインタからスカラフィールド (ID, Width, Height 等)
   のみを即座に読み取る設計となっている。読み取った値が直後に陳腐化する可能性はあるが、tmux プロトコルの応答としては許容される
3. **実践的リスクの低さ**: ペインの削除は `SessionManager.mu` の排他ロック下で行われ、削除直後にポインタが参照されるタイミングウィンドウは極めて狭い。また、ペインの
   ID やサイズ等のスカラフィールドは単一ワード幅で、実質的にアトミックに読み取られる

#### 既存の緩和策

- `GetPaneContextSnapshot()`: RLock 下でペインのコンテキスト情報 (セッション名、ウィンドウ ID 等)
  をコピーして返す。ライブポインタのチェーン (`pane.Window.Session`) を辿る代わりにこれを使う
- `expandFormatSafe()`: フォーマット展開時に完全なディープクローンを使用して TOCTOU レースを排除
- `ResolveDirectionalPane()`: 方向ペインナビゲーション全体を単一ロックスコープで実行

#### 将来の改善パス (Full Snapshot Approach)

理想的な解決策は、`ResolveTarget()` がライブポインタではなく、読み取り専用のスナップショット構造体を返すことである:

```go
// 将来の設計案
type PaneSnapshot struct {
    ID     int
    Index  int
    Width  int
    Height int
    // ... 必要なフィールドのみ
}

func (m *SessionManager) ResolveTarget(target string, callerPaneID int) (PaneSnapshot, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    pane, _, err := m.resolveTargetRLocked(target, callerPaneID)
    if err != nil {
        return PaneSnapshot{}, err
    }
    return PaneSnapshot{ID: pane.ID, Index: pane.Index, Width: pane.Width, Height: pane.Height}, nil
}
```

この変更により、ロック解放後のライブポインタ参照が完全に排除される。ただし、全ハンドラーの戻り値型変更が必要となるため、大規模なリファクタリングとなる。現時点では、スカラフィールドのみの即時読み取りと
`GetPaneContextSnapshot()` の併用で十分に安全である。

---

## 利用されている環境変数一覧

| 環境変数名                    | 主な利用場所                                                        | 意味・役割                                                            | 重要度         |
|:-------------------------|:--------------------------------------------------------------|:-----------------------------------------------------------------|:------------|
| `LOCALAPPDATA`           | `config.go`, `shim_installer_windows.go`, `tmux-shim/main.go` | 設定ファイル(`config.yaml`)およびshimバイナリ(`tmux.exe`)の保存場所を特定するために使用。     | **極めて高い**   |
| `APPDATA`                | `config.go`                                                   | `LOCALAPPDATA`が未設定の場合のフォールバックとして使用。                              | 中           |
| `PATH`                   | `shim_installer_windows.go`, `shim_path_windows.go`           | shimバイナリの存在確認、および子プロセス（ペイン）がshimを認識できるように現在のプロセスのPATHを更新するために使用。 | **高い**      |
| `USERNAME`               | `protocol.go`                                                 | IPC通信用の名前付きパイプ名の生成に使用（`\.\pipe\myT-x-{username}`）。ユーザーごとの分離を確保。  | **高い**      |
| `TMUX_PANE`              | `tmux-shim/main.go`                                           | shim経由でコマンドが実行された際、どのペインから呼び出されたかをサーバー側が特定するために使用。               | **極めて高い**   |
| `GO_TMUX_PIPE`           | `protocol.go`                                                 | 名前付きパイプ名を明示的に指定するためのオーバーライド変数。開発や特殊な環境で使用。                       | 低（任意）       |
| `GO_TMUX_DISABLE_CONPTY` | `terminal_windows.go`                                         | WindowsのConPTY機能を強制的に無効化し、旧来のパイプモードを使用させるためのデバッグ/互換性用フラグ。        | 低（任意）       |
| `GO_TMUX_ENABLE_CONPTY`  | `terminal_windows.go`                                         | ConPTY機能を明示的に有効化するためのフラグ（デフォルトで有効）。                              | 低（任意）       |
| `SHELL`                  | `terminal_unix.go`                                            | Unix環境におけるデフォルトシェルを特定するために使用。                                    | 中（Unix環境のみ） |

---

## Review Finding References

Past review findings referenced in code comments:

| ID | Category | Summary |
|----|----------|---------|
| C-01 | Critical | resolveTargetCore extraction (DRY) |
| C-02 | Critical | Known Concurrency Constraints doc |
| C-03 | Critical | debugLog infinite recursion fix |
| I-01~I-04 | Important | Pane handler TOCTOU fixes |
| I-07~I-08 | Important | removeWindowResult/findWindowIndexByID |
| I-09 | Important | app_pane_feed TOCTOU fix |
| I-10/I-14 | Important | app_lifecycle TOCTOU/shadowing |
| I-19~I-25 | Important | Frontend store fixes |
| I-27~I-28 | Important | model_transform fixes |
| I-29~I-33 | Important | Install/IPC package fixes |
| S-07 | Suggestion | model_transform docs |
| S-23 | Suggestion | shim embed test fixes |
| S-26 | Suggestion | removeWindowResult tests |
| T-01~T-23 | Review-202602190903 | Current review batch |