# Changelog

## v1.0.3

### MCPオーケストレーター

- **ツール大幅拡張**（10 → 18ツール）
  - `get_task_detail` 追加（タスク詳細・応答内容・進捗・依存関係を1件ずつ取得）
  - `acknowledge_task` 追加（タスク受領確認の記録、冪等実装）
  - `update_status` / `get_agent_status` 追加（エージェントアイドル状態管理）
  - `cancel_task` 追加（送信者によるキャンセル、`blocked` 状態も対応）
  - `update_task_progress` 追加（進捗率・進捗メモの更新）
  - `send_tasks` 追加（複数エージェントへの一括タスク送信、`group_id` 管理）
  - 依存関係タスク機能追加（`send_task` の `depends_on` 拡張 + `activate_ready_tasks` ツール）
  - タスク有効期限機能追加（`send_task` の `expires_after_minutes` 拡張、遅延評価方式）

- **ツール名リネーム**（破壊的変更）
  - `get_my_task` → `get_task_message`（`get_my_tasks` との1文字差の混同を解消）
  - `check_tasks` → `list_all_tasks`（全体監視の意図を名前で明示）
  - `check_ready_tasks` → `activate_ready_tasks`（状態変更の副作用を名前で明示）

- **ツール説明文の英語化**
  - 全18ツールの `Description` と InputSchema パラメータ説明を英語に変換（AIのトークン効率向上）
  - 日本語説明はGoコメントとして保持（開発者向け）
  - `help.go` の日本語ヘルプシステムは維持

- **メッセージ配信信頼性向上**（3層防御）
  - ブートストラップメッセージ強化（ポーリング・status報告の行動指針を追加）
  - `get_my_tasks` インライン配信（`max_inline` パラメータで未確認タスクのメッセージを確実に届ける）
  - `update_status(idle)` 時の自動再配信（未配信タスクを自動SendKeys再注入、`redelivered_count` 返却）
  - `activate_ready_tasks` でのSendKeys配信（blocked→pending遷移後にメッセージを担当ペインへ配信）

- **説明文品質改善**
  - SQLite実装詳細の除去（目的・動作に焦点を当てた記述に変更）
  - `check_ready_tasks` → `activate_ready_tasks` の呼び出しタイミングガイダンス追加
  - ツール使い分けの明確化（`get_my_tasks` vs `list_all_tasks`、`get_task_message` vs `get_task_detail`）

### エディタービュー

- 右サイドバーにMonaco Editorベースのエディタビューを追加
  - ファイルツリー + コードエディタの2ペインレイアウト
  - ファイルCRUD操作対応（新規作成・リネーム・削除）
  - コンテキストメニュー（New File / New Folder / Rename / Delete / Copy Path）
  - 未保存変更インジケータ（`*`）と Ctrl+S 保存
  - 1MB超ファイルのtruncated警告表示
  - 高速ファイル切替時のAbortControllerによる競合防止
  - devpanelサービスへの書き込み系API追加（WriteFile / CreateFile / CreateDirectory / RenameFile / DeleteFile / GetFileInfo）
  - Windows固有のファイルロックリトライ機能（指数バックオフ: 10〜160ms, 最大5回）

- エディタービューにファイル検索機能を追加
  - 既存 `useFileSearch` + `FileSearchPanel` を再利用（変更なし）
  - ヘッダーの🔍ボタンで検索モードに切り替え
  - セッション変更時に検索モード自動リセット

### ファイルツリー改善

- ファイルツリーの展開矢印をSVGシェブロンに置き換え（`ChevronIcon.tsx` 新規作成）
  - Unicodeの▶文字からSVGへの変更でサブピクセルジッターを解消
  - CSS `transition: transform 0.15s ease` による滑らかな90度回転アニメーション
  - FileTree / EditorFileTree / DiffFileSidebar / StagingFileRow / DiffViewerで統一

- ファイルツリー状態管理をZustandに移行（安定性向上）
  - `createFileTreeStore()` ファクトリ関数でFileTree/Editorが独立インスタンスを使用
  - `useFileTreeActions.ts` 共通フックで重複コードを統合（useFileTree.ts 340行 + useEditor.ts 277行 → 共通150行）
  - refミラーパターン全廃（`expandedPathsRef`, `childrenCacheRef` 等を削除）

- ファイルツリー階層ツリー構造への移行（flat cache廃止）
  - `FileNode` インターフェース + `mergeChildrenIntoTree` でキャッシュ参照不整合を解消
  - `hasChildren` フラグ追加（空ディレクトリは展開矢印を非表示）

- バックエンドディレクトリキャッシュ追加（TTL 5秒）
  - `sync.RWMutex` + TTLキャッシュで連続展開/折りたたみのファイルシステムI/Oを削減
  - CRUD操作後に影響パスを自動無効化

- ファイルウォッチャー統合（fsnotify）
  - セッション作業ディレクトリの再帰監視
  - 外部からのファイル変更を自動検知してツリーを更新
  - 100msデバウンスで高頻度イベントをバッチ処理
  - `.git`, `node_modules` 除外

### タスクスケジューラー

- 設定画面の追加（ツールバー左端の設定ボタンから遷移）
  - 待ち時間設定（preExecResetDelay, preExecIdleTimeout, targetMode）を `config.yaml` に永続化
  - メッセージテンプレートのCRUD管理（名前 + 本文）
  - タスク作成フォームにテンプレート選択ドロップダウン追加（メッセージ末尾に追記）
  - `blockNonNumericKeys` を共通化して両コンポーネントで再利用

- 秒数入力欄をテキスト入力に変更（`type="number"` → `type="text"`）
  - ブラウザのスピナー（上下ボタン）を除去
  - `onKeyDown` で数字・制御キー以外をブロック

### tmux互換性改善

- Agent Teams window IDバグを修正（2セッション目以降でlist-panesが失敗する問題）
  - `#{window_index}` の出力（window ID）をsliceインデックスとして参照していた不一致を解消
  - `findWindowByID` でID参照に統一

- tmux shimパースエラーの修正（Agent Teams起動時の4カテゴリエラーを解消）
  - `resize-pane -x/-y` がパーセント値（`30%`）を受け付けるよう対応（`flagInt` → `flagString`）
  - `select-pane -P` フラグ追加（ペインスタイル設定、no-op実装）
  - `set-option` コマンド追加（ペインボーダー色設定、no-op実装）
  - `select-layout` コマンド追加（レイアウト整列、no-op実装）


---

## v1.0.2

### タスクスケジューラー
- タスク耐障害性の向上（失敗・完了・スキップ済みタスクの編集・削除を許可、編集時に自動でpendingリセット）
- タスク追加前のオーケストレーター準備状況チェック（orchestrator.db存在確認＋エージェント登録数検証）
- 未準備時のアラート画面追加（DB未存在・エージェント未登録に応じたメッセージと登録導線）
- アラート画面からペインへのメンバー登録画面へのワンクリック遷移
- Pre-Executionフェーズの実装（キュー開始前の全ペイン/newリセット＋役割リマインド送信）
  - `QueuePreparing` ステータス追加（前準備中の進捗表示: resetting / waiting_reset / sending_reminders / waiting_idle）
  - QueueConfig拡張（PreExecEnabled, PreExecTargetMode, PreExecResetDelay, PreExecIdleTimeoutフィールド）
  - ターゲットモード対応（task_panes: タスク対象ペインのみ / all_panes: セッション全ペイン）
  - 全ペインのアイドル待機機能（OutputFlushManager再利用、タイムアウト後もキュー実行継続）
- Pre-Executionフェーズの/newコマンド未到達バグ修正（ConPTYコンテンションによる5ペイン環境での問題）
  - ペイン間ディレイ追加（resetコマンド: 2秒、役割リマインド: 500ms）

### チャット入力
- マルチセッション時の送信先ペイン誤送信バグ修正（`initializedRef`による初回セッション固定の解消）
  - セッション切替時にselectedPaneIdをペインリストで有効性チェック、無効なら自動リセット

### オーケストレーター
- 無所属チーム（`__unaffiliated__`）のメンバー編集・一括保存機能の追加
  - `SaveUnaffiliatedTeamMembers`メソッド新設（全置換方式、空スライスで全削除対応）
  - TeamEditorでのシステムチーム表示調整（チーム名・保存先を読取専用、説明・待機時間を非表示）
  - 保存ルーティング追加（システムチーム判定で新APIへ自動振り分け）

---

## v1.0.1

### リファクタリング
- App構造体のリファクタリング（God Object解消）
  - Phase 1: Facade+Serviceパターンで7つの独立サービスに分離
  - Phase 2: Session/Worktree/Paneサービス抽出、EventEmitter統一、tmuxパッケージ再編
  - Phase 3: ファイル肥大化解消（最大200行制限）
- MCP APIサービスの独立パッケージ化（app_mcp_api.go → internal/mcpapi/）
- NSISインストーラーの廃止（ポータブル配布に移行）

### タスクスケジューラー
- タスクスケジューラー機能の基盤実装（キュー管理、完了ポーリング、コンテキストリセット）
- セッションスコープ化と安全なペイン再起動対応
- シングルモード簡素化（チームモード複雑性の除去、ClearBefore/ClearCommandフラグ追加）

### MCP・オーケストレーター
- `get_my_task` MCPツール追加（タスクメッセージ本文＋メタデータ取得）
- `add_member` MCPツール追加（AIによる動的チームメンバー追加）
- `--session`フラグのMCPマネージャー設定パネル表示
- LSP-MCPステータス表示の削除（冗長な同期状態インジケーター除去）

### UI/UX改善
- キャンバスレイアウトアルゴリズム改善（水平レイアウト、双方向エッジ、接続重み付け）
- ファイルツリー検索機能追加（名前＋内容検索、VS Code風UI）
- ペインサイズ管理の改善（均等分割、セッションスコープでのサイズ永続化）
- ペイン分割時のタイトルクリア（オーケストレーター重複名防止）
- メンバー追加画面のレイアウト修正（ヘッダー/フッター固定、スクロール対応）
- Auto Enter機能の改善（分→秒単位変更、ビジー検知スキップ、テキスト入力UI化）

### 日本語入力（IME）対応
- 日本語IME二重確定バグの修正（3層防御＋診断ログ）
- WebView2プロセス分離によるIMEコンテキスト復旧機能
- IMEリセットボタンの追加（ツールバー）

### 安定性・品質改善
- ワークツリーセッション管理の改善（孤立クリーンアップ、自動プルーン、ヘルスチェック）
- Git Diffサイドバーの操作修正（楽観的更新ロールバック、Push/Pull改善）
- Diffステージング速度改善（gitコマンド削減: 10-12回→4-6回）
- エラーログ・通知システムの拡充（未通知操作23件対応、クリップボード10件対応）
- send_task vs チームブートストラップの安定性修正（TrimRight、遅延増加、select-pane追加）

---

## v1.0.0
- 異なるセッション間のペインを見えないように再修正
- オーケストレーターMCPの安定稼働対策
- ワークツリーデータの遅延読み込みによる高速化対応
- 入力欄のペイン番号が適当だった件の対応
- github連携対応
- 入力拡張枠の自動閉じるチェックボックス対応
- 入力拡張枠の左右上下対応
- View機能のハイライト制限値の緩和


---

## v0.0.19
- 軽微なレビュー指摘の修正
- キャンバスツリー表示の最適化
- キャンバスターミナルの拡大対応
- ペインへのメッセージ入力欄の送信後閉じる対応
- 異なる左サイドバーセッション間のペインを見えないように修正
- チーム機能のサンプルを初期値として設定
- オーケストレーターMCPの引数の見直し
 - これにより、worktree時にもコピーしてる流用可能

---

## v0.0.18
- チーム機能の永続化UIを提供
- チーム開始及び共有しやすい環境の構築
- 各ターミナルへの直接文字列送信機能追加
  - 誤送信防止
- キャンバスモードを追加
  - 視覚的に連携先と線が繋がることで連携状況を視覚化
- その他軽微な修正？

## v0.0.17
- 英語対応

---

## v0.0.16
- タスクスケジューラー機能の追加

---

## v0.0.14~15
- オーケストレーターMCPの追加
  - claude,gemini,codex対応
- 二重入力不具合の修正

---

## v0.0.11~13
- LSP-MCP対応(簡単な引数で接続可能)
- 右サイドバー機能をオーバーライドからドッキングに変更
- その他軽微な修正と数百回のレビューに対する修正

---

## v0.0.10
- ファイルツリーをおしゃれに修正
- mdファイル参照時に、プレビュー機能の追加

---

## v0.0.9
- 右サイドバーのファイルツリーにPathコピー機能を追加
- Edge起動時にカーソルがとられて日本語変換できない件での対応を追加
- 右サイドバーに組み込みMCP枠を追加

---

## v0.0.8
- 入力履歴機能の追加（右サイドバーに Input History ビューを新設。JSONL永続化・リアルタイム表示・バッジ通知・Ctrl+Shift+H ショートカット対応）
- 入力履歴のラインバッファリング方式への変更（100msバッチングを廃止し、Enter区切りのコマンド単位記録に改善。CSI/OSCシーケンス除去・Backspace編集再現・Ctrl+C/D記録対応）
- エラー表示画面の対象情報拡充（フロントエンド未捕捉例外・React Error Boundary・API通信失敗をバックエンドログへ一元化）
- 右サイドバーショートカット設定機能の追加（Viewer各ビューのキーボードショートカットを設定画面からカスタマイズ・`config.yaml` に永続化）
- クイックスタートセッション機能の追加（セッション未選択画面からワンクリックでデフォルトディレクトリのセッションを即座に作成）
- サイドバーアイコン並び順の修正（Input HistoryをDiff直下に配置、Error Logを最下部固定。import順序ルールをコメントで明記）
- Diff アイコン位置修正（誤って最下部に配置されていた `position: "bottom"` を削除）
- Input History の `position: "bottom"` 誤設定修正（上部グループに正しく配置）

---

## v0.0.7
- 右サイドバー（Activity Strip）のアイコン配置変更（エラーログを最下部固定）
- DIFFサイドバー: サブディレクトリ・日本語ファイル名の非表示バグ修正（`core.quotepath`・NUL区切り対応）
- Diff View プラグインの追加（`git diff HEAD` をファイルツリー＋行単位diffで表示）
- セッションエラーログビューアの追加（Warn/ErrorをJSONL永続化・リアルタイム表示・バッジ通知）
- Diff View ディレクトリ階層修正 & 「Expand hidden lines」折りたたみ機能追加
- `new-window` をセッション作成に変更（1セッション=1ウィンドウモデルへ移行・タブUI削除）
- コードレビュー指摘対応（DevPanel API・セッションログ・フロントエンド品質改善）
- Diff View サブディレクトリ表示バグ修正（`addedDirs`管理・state更新順序）

---

## v0.0.6
- Viewer System（プラグイン型ビューア基盤）の追加
  - File Tree ビュー（仮想化ツリー＋ファイルプレビュー）
  - Git Graph ビュー（SVGグラフ＋diff表示）
  - File Content Viewer: パスコピー・Ctrl+Cコピー・マウス選択コピー機能追加
- 速度改善（バッファサイズ拡大・ロック早期解放・フロントエンド描画最適化）
- WebSocket化による高スループットペインデータ転送（pane:dataをWebSocketバイナリストリームに移行）
  - `workerutil` パッケージ新規作成（パニックリカバリ共通ヘルパー）
  - `wsserver` パッケージ新規作成（WebSocketサーバー）

---

## v0.0.5
- Claude Code環境変数（`claude_env`）設定機能の追加
  - 設定画面からClaude Code環境変数を管理
  - セッション作成時のチェックボックスで適用制御（`use_claude_env` / `use_pane_env`）
  - `pane_env` にもデフォルトON/OFFチェック追加

---

## v0.0.4
- レガシーShimパス問題の修正（旧`%LOCALAPPDATA%\github.com\my-take-dev\myT-x\bin`の自動クリーンアップ）
- モデル置換 `from: "ALL"` ワイルドカード対応（全モデルを一括置換先に変更可能）
- フロントエンド マルチウィンドウ対応（tmux-shim連携ギャップ修正・ウィンドウタブUI追加）
- Claude-Code-Communication互換対応（`select-pane -T`フラグ・`attach-session`コマンド追加）
- コードレビュー指摘対応（フォントサイズ競合・SearchBar再オープン時クエリ残存・バッファ定数化など）

---

## v0.0.3
- ドラッグ＆ドロップ時のパス表示を最適化（`\\` → `\`）
- パフォーマンス改善調査レポート & 実装計画
- ペイン分割時の作業ディレクトリ修正
- IME変換障害 調査結果 & 修正計画

---

## v0.0.2
- プログラム一式アップロード
- gitworktreeの不要な情報のクリーンアップを追加
  - 実態が削除済みなのに、UIから選択可能な状態の解消
- アプリケーション起動時にコマンドプロンプト画面が2連続で画面表示される状態を修正

---

## v0.0.1
- プレビュー版
