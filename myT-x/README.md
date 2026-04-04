# myT-x

Windows専用のデスクトップ ターミナルマルチプレクサ。Claude Code (Anthropic AI) を含む複数AIエージェントの並列セッション管理に最適化。tmux互換のセッション/ウィンドウ/ペイン操作を全てインプロセスで実現し、実際のtmuxバイナリは不要。

## 技術スタック

| レイヤー | 技術 | バージョン |
|---------|------|-----------|
| **バックエンド** | Go | 1.26 |
| **デスクトップフレームワーク** | Wails v2 (WebView2) | 2.11.0 |
| **フロントエンド** | React + TypeScript | React 18.2 / TS 5.9 |
| **ターミナルエミュレーション** | xterm.js (WebGL) | @xterm/xterm 6.0 |
| **状態管理** | Zustand | 5.0 |
| **ビルド** | Vite | 7.3 |
| **テスト (Go)** | go test | 標準 |
| **テスト (Frontend)** | Vitest + jsdom | 4.0 |
| **DB** | SQLite (modernc.org/sqlite) | 入力履歴永続化用 |

### 主要Go依存パッケージ

| パッケージ | 用途 |
|-----------|------|
| `github.com/Microsoft/go-winio` | Windows Named Pipe IPC |
| `github.com/gorilla/websocket` | バイナリターミナル出力ストリーミング |
| `github.com/creack/pty` | Unix PTY (フォールバック) |
| `golang.org/x/sys` | Windows ConPTY syscall |
| `go.yaml.in/yaml/v3` | YAML設定ファイルパース |
| `modernc.org/sqlite` | 入力履歴SQLiteストレージ |

---

## アーキテクチャ概要

```
┌─────────────────────────────────────────────────┐
│  Frontend (React 18 + xterm.js / WebView2)       │
│  stores/ hooks/ components/ services/            │
└──────────┬──────────────┬───────────────────────┘
           │              │
      Wails IPC      WebSocket (binary)
      (JSON RPC)     (127.0.0.1:動的ポート)
           │              │
┌──────────┴──────────────┴───────────────────────┐
│  App Layer (main package)                         │
│  app.go: App struct — 全サービスの集約ルート       │
│  app_*.go: Wailsバインド済みAPI群 (35ファイル)     │
└──────────┬──────────────────────────────────────┘
           │
┌──────────┴──────────────────────────────────────┐
│  Internal Services Layer (internal/)              │
│  tmux/ session/ snapshot/ config/ mcp/            │
│  orchestrator/ worktree/ terminal/ ipc/ ...       │
└──────────┬──────────────────────────────────────┘
           │
      Named Pipe IPC
      (\\.\pipe\myT-x-<username>)
           │
┌──────────┴──────────────────────────────────────┐
│  tmux-shim (cmd/tmux-shim/)                       │
│  tmux CLI互換バイナリ — parse → transform → IPC   │
└─────────────────────────────────────────────────┘
```

### 通信チャネル (3系統)

| チャネル | 方向 | 用途 | プロトコル |
|---------|------|------|-----------|
| **Wails IPC** | 双方向 | API呼び出し + イベント通知 | JSON over WebView bridge |
| **WebSocket** | Backend→Frontend | 高スループット ペイン出力 | バイナリフレーム `[1byte:IDLen][paneID][data]` |
| **Named Pipe** | tmux-shim→Backend | tmuxコマンドルーティング | JSON行区切り (`\n`) |

WebSocket接続不可時はWails IPC (`pane:data:<paneId>` イベント) にフォールバック。

---

## ディレクトリ構造

```
myT-x/
├── main.go                    # アプリケーションエントリポイント
├── main_mcp_cli.go            # MCPブリッジCLIモード (os.Args[1] == "mcp")
├── app.go                     # App struct定義、NewApp()、ロック順序ドキュメント
├── app_lifecycle.go           # startup() / shutdown() — 全サブシステム初期化順序
├── app_events.go              # emitBackendEvent — イベントルーティングハブ
├── app_session_wiring.go      # buildXxxServiceDeps() — DI配線ヘルパー
├── app_session_api.go         # セッションCRUD API
├── app_pane_api.go            # ペイン操作API
├── app_config_api.go          # 設定読み書きAPI
├── app_mcp_api.go             # MCP管理API
├── app_mcp_orchestrator.go    # 組み込みオーケストレーターMCP登録
├── app_orchestrator_team_*.go # Agent Teams CRUD + 起動
├── app_devpanel_*.go          # 右パネル (ファイルツリー/Git)
├── app_chat_api.go            # チャットオーバーレイ入力
├── app_worktree_api.go        # Worktreeライフサイクル
├── app_sendkeys.go            # send-keys操作 (sendKeysIO DI)
├── app_guards.go              # requireSessions/requireRouter — 起動前ガード
├── Makefile                   # build-shim → prepare-embed → wails build
├── wails.json                 # Wailsプロジェクト設定
│
├── cmd/
│   ├── tmux-shim/             # tmux CLI互換shimバイナリ
│   │   ├── main.go            # エントリ: parse → transform → Named Pipe送信
│   │   ├── parse.go           # tmuxコマンドライン引数パース
│   │   ├── spec.go            # tmuxコマンド仕様定義
│   │   ├── usage.go           # ヘルプ/使用法メッセージ
│   │   ├── command_transform.go # シェルコマンド変換 (Unix→Windows)
│   │   └── model_transform.go # AIモデル名置換 (ModelFrom→ModelTo)
│   ├── mcp-pipe-bridge/       # MCP stdio↔Named Pipeブリッジ
│   └── go-tmux/               # tmux直接コマンドユーティリティ
│
├── internal/
│   ├── tmux/                  # コア: セッション/ウィンドウ/ペイン状態機械 + コマンドルーティング
│   │   ├── doc.go             # パッケージ全体のファイルマップ (必読)
│   │   ├── types.go           # 全ドメイン型: TmuxSession, TmuxWindow, TmuxPane, Snapshot, Delta
│   │   ├── session_manager.go # SessionManager: RWMutex保護の状態管理
│   │   ├── command_router.go  # CommandRouter: コマンドディスパッチ + 環境変数解決
│   │   ├── command_router_handlers_*.go  # コマンドハンドラ群
│   │   ├── format.go          # tmux #{var} フォーマット展開
│   │   ├── key_table.go       # send-keys / copy-mode キー変換
│   │   ├── layout.go          # ペインレイアウトツリー (LayoutNode)
│   │   └── tmux_command_parser.go  # CLI引数パース
│   │
│   ├── session/               # セッションライフサイクル (create/rename/kill)
│   │   ├── service.go         # session.Service + Deps struct
│   │   ├── types.go           # セッション関連型定義
│   │   └── helpers.go         # ヘルパー関数
│   │
│   ├── snapshot/              # ペイン出力パイプライン (8ファイル構成)
│   │   ├── service.go         # Service struct, Deps, NewService, Shutdown
│   │   ├── output.go          # ペイン出力バッファリング、フラッシュ管理
│   │   ├── cache.go           # スナップショットキャッシュ、デバウンス発行
│   │   ├── delta.go           # スナップショット差分計算
│   │   ├── feed.go            # ゼロアロケーションPTYチャンクパス
│   │   ├── convert.go         # ペイロード型変換
│   │   ├── metrics.go         # ペイロードサイズ推定/メトリクス
│   │   └── policy.go          # イベント→スナップショット発行ポリシーマップ
│   │
│   ├── config/                # YAML設定ロード/保存/状態サービス
│   │   ├── config.go          # Config struct定義 + DefaultConfig()
│   │   ├── types.go           # ClaudeEnvConfig, MCPServerConfig, AgentModel, WorktreeConfig
│   │   ├── state.go           # StateService: 設定スナップショット + バージョニング
│   │   ├── io.go              # ファイルI/O (Load/Save)
│   │   ├── path.go            # 設定ファイルパス解決
│   │   ├── probe.go           # メタデータパース
│   │   ├── clone.go           # 設定クローン
│   │   └── validate.go        # 設定値バリデーション
│   │
│   ├── terminal/              # ConPTY/PTYプロセスラッパー
│   │   ├── terminal.go        # Terminal struct: プロセスライフサイクル
│   │   └── output_buffer.go   # OutputBuffer: リングバッファ (512KB)
│   │
│   ├── ipc/                   # Windows Named Pipeサーバー/クライアント
│   │   ├── pipe_server.go     # PipeServer: accept loop, DACL, max64接続
│   │   ├── pipe_client.go     # Send(): shimからの同期送信
│   │   └── protocol.go        # TmuxRequest / TmuxResponse ワイヤプロトコル
│   │
│   ├── wsserver/              # WebSocketハブ (バイナリペイン出力)
│   │   └── hub.go             # Hub: 単一接続設計、ping/pong、サブスクリプション
│   │
│   ├── mcp/                   # MCPサーバー管理
│   │   ├── registry.go        # Registry: 静的MCP定義テンプレート
│   │   ├── manager.go         # Manager: セッション別インスタンス追跡 (generation counter)
│   │   ├── types.go           # Definition, InstanceState
│   │   ├── mcppipe.go         # MCPパイプ通信
│   │   ├── mcpruntime.go      # MCPランタイム管理
│   │   ├── orchestrator_factory.go  # オーケストレーターMCP生成
│   │   ├── agent-orchestrator/      # エージェントオーケストレーターMCPサーバー実装
│   │   ├── pipebridge/              # Named Pipeブリッジ
│   │   └── lspmcp/                  # 組み込みLSP MCPサーバー群
│   │
│   ├── orchestrator/          # Agent Teams CRUD + 起動
│   │   ├── service.go         # Service: チーム定義管理
│   │   ├── types.go           # TeamDefinition, TeamMember
│   │   └── usecase/           # エージェントオーケストレーターMCPユースケース
│   │
│   ├── worktree/              # Gitワークツリーライフサイクル
│   │   ├── service.go         # Service + Deps struct
│   │   ├── types.go           # ワークツリー関連型定義
│   │   ├── create.go          # ワークツリー作成
│   │   ├── cleanup.go         # ワークツリー削除
│   │   ├── copy.go            # ファイル/ディレクトリコピー操作
│   │   ├── commit.go          # コミット/プッシュ操作
│   │   ├── query.go           # クエリ操作 (一覧、ステータス)
│   │   └── helpers.go         # ヘルパー関数
│   │
│   ├── panestate/             # VT100ターミナル状態管理 (content query用)
│   ├── devpanel/              # 右パネル: ファイルブラウズ + Git操作
│   ├── scheduler/             # 時間ベースのペインメッセージ配信
│   ├── inputhistory/          # SQLiteベースの入力コマンド履歴
│   ├── sessionlog/            # Warn/Errorログキャプチャ (slog.Handler tee)
│   ├── hotkeys/               # Windowsグローバルホットキー (Quakeモード)
│   ├── install/               # tmux-shimバイナリ埋め込み/インストール
│   ├── singleinstance/        # Windows Mutexによる単一インスタンス保証
│   ├── git/                   # Git CLIラッパー
│   ├── shell/                 # Unixコマンドパース/Windows変換
│   ├── pane/                  # ペインサービス抽出
│   ├── mcpapi/                # MCP API操作 (App層向け)
│   ├── apptypes/              # 共有インターフェース (RuntimeEventEmitter)
│   ├── workerutil/            # バックグラウンドワーカーpanicリカバリー
│   ├── procutil/              # サブプロセスコンソール非表示
│   ├── userutil/              # ユーザー名解決
│   └── testutil/              # テストユーティリティ
│
└── frontend/
    ├── package.json
    ├── vite.config.ts         # マニュアルチャンク分割、Terser 2パス圧縮
    └── src/
        ├── main.tsx           # createRoot → <App/>
        ├── App.tsx            # ルートコンポーネント: レイアウト + グローバルキーボード
        ├── App.css            # グローバルレイアウト
        ├── api.ts             # Wailsバインディング一覧 (全バックエンドAPI)
        ├── i18n.ts            # 日英2言語 translate() / useI18n()
        │
        ├── stores/
        │   ├── tmuxStore.ts       # 中央ストア: セッション/ペイン状態、delta適用
        │   ├── canvasStore.ts     # キャンバスモード: ノード位置/タスクエッジ
        │   ├── mcpStore.ts        # MCPサーバースナップショット
        │   ├── errorLogStore.ts   # セッションエラーログ
        │   ├── inputHistoryStore.ts # 入力履歴
        │   └── notificationStore.ts # トースト通知
        │
        ├── hooks/
        │   ├── useTerminalSetup.ts    # Terminal生成、WebGL、リプレイバッファ
        │   ├── useTerminalEvents.ts   # I/O: WebSocket+IPC二重リスナー、IME、コピペ
        │   ├── useTerminalResize.ts   # ResizeObserver + fit
        │   ├── useTerminalFontSize.ts # Ctrl+Wheel フォントサイズ
        │   ├── useBackendSync.ts      # 全同期フック統合
        │   ├── usePrefixKeyMode.ts    # Ctrl+B プレフィックスモード
        │   └── sync/
        │       ├── useSnapshotSync.ts   # セッションライフサイクルイベント購読
        │       ├── useConfigSync.ts     # 設定リアルタイム同期
        │       ├── useMCPSync.ts        # MCP状態変更同期
        │       ├── useSessionLogSync.ts # エラーログ同期
        │       └── useInputHistorySync.ts
        │
        ├── services/
        │   └── paneDataStream.ts  # WebSocketシングルトン: バイナリフレーム受信/再接続
        │
        ├── components/
        │   ├── TerminalPane.tsx       # ターミナルペイン (memo最適化)
        │   ├── SessionView.tsx        # アクティブセッション表示切替
        │   ├── LayoutRenderer.tsx     # tmux分割ツリー再帰レンダリング
        │   ├── LayoutNodeView.tsx     # レイアウトノード → TerminalPane
        │   ├── Sidebar.tsx            # セッションリスト (react-window仮想化)
        │   ├── MenuBar.tsx            # トップメニューバー
        │   ├── ChatInputBar.tsx       # チャット入力オーバーレイ
        │   ├── StatusBar.tsx          # ステータスバー
        │   ├── QuickSearch.tsx        # Ctrl+P クイック検索
        │   ├── SettingsModal.tsx       # 設定モーダル
        │   ├── NewSessionModal.tsx     # セッション作成ダイアログ
        │   ├── viewer/
        │   │   ├── viewerRegistry.ts  # プラグインレジストリ (registerView/getRegisteredViews)
        │   │   ├── ViewerSystem.tsx   # 右パネルシステム
        │   │   ├── ActivityStrip.tsx  # アイコンサイドバー
        │   │   └── views/             # FileTree, GitGraph, Diff, MCP, Scheduler等
        │   └── canvas/
        │       └── CanvasView.tsx     # ReactFlowグラフモード
        │
        └── types/
            ├── tmux.ts            # SessionSnapshot, PaneSnapshot等
            └── mcp.ts             # MCPSnapshot型 + 正規化
```

---

## エントリポイントと起動シーケンス

### アプリケーション起動 (`main.go`)

```
main() → run()
  ├── runMCPCLIMode()     # os.Args[1]=="mcp" → MCPブリッジCLIとして動作
  ├── singleinstance.TryLock()  # Windows Mutex: 二重起動防止
  │   └── 既存インスタンスがあれば activate-window を Named Pipe 送信して終了
  └── wails.Run()
      ├── app.startup()   # 全サブシステム初期化
      └── app.shutdown()  # グレースフル停止
```

### startup() 初期化順序 (`app_lifecycle.go`)

```
1. UTF-8 codepage設定、workspace/launchDir取得
2. sessionlog初期化 (slog.Default に TeeHandler 設定)
3. inputhistory初期化 (SQLiteストレージ)
4. config.EnsureFile → YAML設定ロード (エラー時はDefaultConfig使用)
5. tmux.SessionManager + tmux.CommandRouter 生成 (RouterOptions: PaneEnv, ClaudeEnv, コールバック)
6. mcp.Registry 生成 (設定MCPサーバー + 組み込みLSP + orchestrator)
7. mcp.Manager 生成
8. ipc.PipeServer 起動 (Named Pipe listen開始)
9. tmux-shimバイナリ インストール/更新 (go:embed)
10. wsserver.Hub 起動 (WebSocketバイナリストリーミング)
11. グローバルホットキー登録 (Quakeモード)
12. snapshot パイプラインワーカー + アイドルモニター起動
```

---

## データフロー

### ターミナル出力パス (最重要パフォーマンスパス)

```
ConPTY → terminal.Terminal (出力読み取りgoroutine)
    → OutputBuffer (リングバッファ 512KB)
    → snapshot.Service.PaneFeedWorker (~60Hz ポーリング)
    → OutputFlushManager (バッチ化)
    → wsserver.Hub.BroadcastPaneData()
    → WebSocket バイナリフレーム [IDLen][paneID][data]
    → paneDataStream.ts (フレームパース、ハンドラディスパッチ)
    → xterm.Terminal.write(data) → WebGLレンダラー
```

### tmuxコマンドフロー (例: new-session)

```
Claude Code: tmux new-session -s myname -d
    → tmux-shim.exe (CLI解析)
    → applyShellTransform (Unix→Windows変換)
    → applyModelTransform (モデル名置換、非オペレーターのみ)
    → Named Pipe JSON送信
    → ipc.PipeServer.handleConnection()
    → tmux.CommandRouter.Execute()
    → handleNewSession()
    → SessionManager.CreateSession()
    → terminal.Terminal起動 (ConPTY)
    → EventsEmit("tmux:snapshot" or delta)
    → Frontend: useTmuxStore.applySessionDelta()
    → React再レンダリング
```

### イベント発行フロー

```
バックエンド状態変更
    → app.emitBackendEvent(eventName, payload)
    ├── "tmux:pane-output" → snapshotService (WebSocket/IPCで配信)
    ├── "app:activate-window" → ウィンドウフォーカス
    └── その他 → runtimeEventsEmitFn + スナップショットポリシー評価
                ├── バイパス系: session-created/destroyed/renamed, pane-focused
                └── デバウンス系: pane-created, layout-changed, mcp:state-changed
```

---

## 主要ドメイン型

### バックエンド (`internal/tmux/types.go`)

| 型 | 説明 |
|----|------|
| `TmuxSession` | セッション: Windows配列、env、worktreeメタ、IsAgentTeamフラグ |
| `TmuxWindow` | ウィンドウ: ペインレイアウトツリー (LayoutNode) |
| `TmuxPane` | ペイン: `*terminal.Terminal` (ConPTY)、envマップ |
| `SessionSnapshot` | フロントエンド安全な不変コピー (JSON化用) |
| `SessionSnapshotDelta` | 差分更新: `Upserts []SessionSnapshot` + `Removed []string` |
| `LayoutNode` | 再帰的分割レイアウトツリー (horizontal/vertical split) |

### IPC (`internal/ipc/protocol.go`)

| 型 | 説明 |
|----|------|
| `TmuxRequest` | `{Command, Flags, Args, Env, CallerPane}` — shimからのリクエスト |
| `TmuxResponse` | `{ExitCode, Stdout, Stderr}` — shimへのレスポンス |

### 設定 (`internal/config/`)

| 型 | 説明 |
|----|------|
| `Config` | ルート設定: Shell, Prefix, QuakeMode, Worktree, AgentModel, PaneEnv, ClaudeEnv, MCPServers等 |
| `AgentModel` | モデル名From→To置換 + エージェント名ベースオーバーライド |
| `WorktreeConfig` | Worktree有効化、セットアップスクリプト、コピー対象 |
| `ClaudeEnvConfig` | Claude Code専用環境変数 + デフォルト有効フラグ |
| `MCPServerConfig` | MCP定義: コマンド、引数、env、config_params |
| `TaskSchedulerConfig` | タスクスケジューラ設定: pre-exec待ち時間、対象ペイン、メッセージテンプレート |
| `MessageTemplate` | 再利用可能なメッセージテンプレート: 名前 + メッセージ本文 |

### フロントエンド (`frontend/src/types/`)

| 型 | 説明 |
|----|------|
| `SessionSnapshot` | バックエンドのSessionSnapshotに対応するTS型 |
| `PaneSnapshot` | ペインID、タイトル、アクティブ状態、寸法 |
| `MCPSnapshot` | MCPインスタンスの実行時状態 |

---

## 設計パターン

### 1. インプロセス tmux エミュレーション
`SessionManager`が全セッション/ウィンドウ/ペインツリーをメモリ内に保持。`SessionManager.mu` (RWMutex) で保護。`CommandRouter`は独立した`paneEnvMu`、`claudeEnvMu`、`shimMu`を持つ（同時取得禁止）。`Locked`/`RLocked`サフィックス規約でロック要件を明示。

### 2. Snapshot + Delta パターン
全体スナップショットではなく`SessionSnapshotDelta`（Upserts + Removed）を発行し、フロントエンドの`applySessionDelta()`で差分適用。フル置換による再レンダリングを回避。

### 3. Copy-on-Write環境変数マップ
`CommandRouter.UpdatePaneEnv`/`UpdateClaudeEnv`はマップ全体をアトミックに置換（ミューテーション禁止）。`paneEnvView()`は参照を直接返却可能。

### 4. Generation Counterパターン
`mcp.Manager`の`instance.generation`と`snapshot.Service`の`snapshotRequestGeneration`で、非同期goroutineの遅延書き込み（stale write）を防止。追加ロック不要。

### 5. 二重出力チャネル + 自動フォールバック
WebSocket（高スループット）とWails IPC（低スループット）の二重リスナー。WebSocket接続不可時にIPC自動切替。`useTerminalEvents.ts`で実行時判定。

### 6. 依存性注入: Deps structパターン
各サービスは`Deps` structでクロージャ経由の依存を受け取る。`buildXxxServiceDeps(app *App)`ヘルパーで配線。`requireSessions()`/`requireRouter()`で`startup()`前呼び出しを防止。

### 7. shim: 薄いIPCプロキシ
`tmux-shim.exe`はparse→transform→send→receive→exitのみ。全ロジックはメインプロセスに集約。panicリカバリー付き。

---

## tmux互換コマンド一覧

`CommandRouter.handlers`に登録されたコマンド:

| カテゴリ | コマンド |
|---------|---------|
| **セッション** | `new-session`, `has-session`, `kill-session`, `rename-session`, `list-sessions`, `attach-session` |
| **ウィンドウ** | `new-window`, `kill-window`, `rename-window`, `list-windows`, `select-window`, `activate-window` |
| **ペイン** | `split-window`, `select-pane`, `kill-pane`, `resize-pane`, `capture-pane`, `copy-mode` |
| **入力** | `send-keys` |
| **表示** | `display-message` |
| **バッファ** | `list-buffers`, `set-buffer`, `paste-buffer`, `load-buffer`, `save-buffer` |
| **環境変数** | `show-environment`, `set-environment` |
| **シェル** | `run-shell`, `if-shell` |
| **拡張** | `mcp-resolve-stdio`, `resolve-session-by-cwd` |

---

## 設定システム

**設定ファイル:** `%LOCALAPPDATA%\myT-x\config.yaml`

**パース方針:**
- パースエラーは非致命的 → `DefaultConfig()`で起動、警告を`GetConfigAndFlushWarnings` APIで通知
- 不明フィールドは無視
- 設定追加時はconfig.yamlとREADME.mdを同時更新

**デフォルト値:** `config.go:DefaultConfig()` 参照
- Shell: `powershell.exe`
- Prefix: `Ctrl+b`
- QuakeMode: `true`
- GlobalHotkey: `Ctrl+Shift+F12`
- ViewerSidebarMode: `overlay`

---

## ビルドシステム

```bash
# 開発 (shim別プロセス + ホットリロード)
make dev    # go build tmux-shim.exe && wails dev

# プロダクションビルド (shimを埋め込み)
make build  # build-shim → prepare-embed → wails build -tags embed_shim
            # 出力: build/bin/myT-x.exe (shim内蔵シングルバイナリ)

# shimのみビルド
make build-shim

# 埋め込みリソースクリーンアップ
make clean-embed
```

フロントエンド: `npm install && npm run build` (Vite + TypeScript + Terser 2パス)

---

## フロントエンドアーキテクチャ詳細

### Zustandストア一覧

| ストア | ファイル | 責務 |
|--------|---------|------|
| `useTmuxStore` | `stores/tmuxStore.ts` | **中央ストア**: セッション/ウィンドウ/ペイン状態、delta適用、アクティブセッション管理、セッション順序 |
| `useCanvasStore` | `stores/canvasStore.ts` | キャンバスモード: ノード位置/サイズ、タスクエッジマップ |
| `useMCPStore` | `stores/mcpStore.ts` | セッション別MCPスナップショット |
| `useErrorLogStore` | `stores/errorLogStore.ts` | セッションエラーログエントリ |
| `useInputHistoryStore` | `stores/inputHistoryStore.ts` | ペイン別入力履歴 |
| `useNotificationStore` | `stores/notificationStore.ts` | トースト通知 (8秒自動消去) |

### ターミナルフック呼び出し順序 (不変条件)

`TerminalPane.tsx`内での呼び出し順序は不変条件:

```
1. useTerminalSetup     → terminalRef設定、WebGL/DOM選択、リプレイ復元
2. useTerminalEvents    → I/Oハンドラ登録 (isComposingRef設定)
3. useTerminalResize    → ResizeObserver + fit (isComposingRef依存)
4. useTerminalFontSize  → Ctrl+Wheel
```

### Viewerパネル プラグインアーキテクチャ

`viewerRegistry.ts`にサイドエフェクトimportで自己登録:

```
registerView({id, label, icon, shortcut, component})
→ ActivityStrip (アイコンバー) + ViewOverlay (パネル描画)
```

登録済みビュー: FileTree, GitGraph, Diff, InputHistory, McpManager, PaneScheduler, OrchestratorTeams, ErrorLog

---

## internal パッケージ依存関係

```
apptypes ← (全パッケージ: RuntimeEventEmitter共有インターフェース)

tmux ← terminal, ipc, apptypes
session ← tmux, config, mcp, snapshot, apptypes
snapshot ← tmux, terminal, apptypes, workerutil, panestate
config ← (標準ライブラリのみ + yaml)
ipc ← (go-winio)
wsserver ← (gorilla/websocket)
mcp ← ipc, config, apptypes
orchestrator ← ipc, config
worktree ← git, config
terminal ← (golang.org/x/sys: ConPTY)
panestate ← (標準ライブラリのみ)
devpanel ← git
scheduler ← ipc
inputhistory ← (modernc.org/sqlite)
```

---

## 重要ファイル クイックリファレンス

AIエージェントが最初に読むべきファイル (優先順):

| 順位 | ファイル | 理由 |
|------|---------|------|
| 1 | `app.go` | App struct全体定義、ロック順序、NewApp() |
| 2 | `app_lifecycle.go` | startup/shutdown: 全サブシステム初期化順序 |
| 3 | `internal/tmux/doc.go` | tmuxパッケージのファイルマップ (ナビゲーション) |
| 4 | `internal/tmux/types.go` | 全ドメイン型定義 |
| 5 | `internal/tmux/command_router.go` | コマンドディスパッチ + RouterOptions |
| 6 | `internal/ipc/pipe_server.go` | Named Pipe IPC: プロトコル、セキュリティ |
| 7 | `cmd/tmux-shim/main.go` | shimライフサイクル: parse→transform→send |
| 8 | `frontend/src/stores/tmuxStore.ts` | フロントエンド中央ストア |
| 9 | `frontend/src/services/paneDataStream.ts` | WebSocketバイナリストリーミング |
| 10 | `frontend/src/api.ts` | 全バックエンドAPI一覧 |

---

## 主要機能一覧

| 機能 | バックエンド | フロントエンド |
|------|------------|--------------|
| マルチセッション管理 | `session.Service`, `tmux.SessionManager` | `Sidebar`, `tmuxStore` |
| ペイン分割/レイアウト | `tmux.CommandRouter` (split-window) | `LayoutRenderer`, `LayoutNodeView` |
| キャンバスモード | - | `CanvasView`, `canvasStore` (ReactFlow) |
| Agent Teams | `orchestrator.Service` | `OrchestratorTeamsView` |
| MCP管理 | `mcp.Manager`, `mcp.Registry` | `McpManagerView`, `mcpStore` |
| Gitワークツリー | `worktree.Service` | `NewSessionModal`, `PromoteBranchModal` |
| チャットオーバーレイ | `app_chat_api.go` | `ChatInputBar` |
| ペインスケジューラー | `scheduler.Service` | `PaneSchedulerView` |
| ファイルブラウザ | `devpanel.Service` | `FileTreeView` |
| Gitグラフ/Diff | `devpanel.Service` | `GitGraphView`, `DiffView` |
| 入力履歴 | `inputhistory.Service` (SQLite) | `InputHistoryView` |
| Quakeモード | `hotkeys.Manager` | - |
| 単一インスタンス | `singleinstance.TryLock` | - |
| i18n (日英) | - | `i18n.ts` |

---

## 開発時の注意事項

CLAUDE.mdに開発規約・コーディング規約・テスト要件が詳述されているため、そちらを必ず参照すること。本READMEはプロジェクトの全体像と構造に特化している。
