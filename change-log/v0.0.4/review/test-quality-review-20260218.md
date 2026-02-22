# Test Quality Review — 2026-02-18

## 対象: 13テストファイルペア

| # | 実装ファイル | テストファイル | パッケージ |
|---|------------|-------------|----------|
| 1 | `app_events.go` | `app_events_test.go` | myT-x (root) |
| 2 | `app_lifecycle.go` | `app_lifecycle_test.go` | myT-x (root) |
| 3 | `app_pane_api.go` | `app_pane_api_test.go` | myT-x (root) |
| 4 | `app_session_api.go` | `app_session_api_test.go` | myT-x (root) |
| 5 | `app_worktree_api.go` | `app_worktree_api_test.go` | myT-x (root) |
| 6 | `command_transform.go` | `command_transform_test.go` | cmd/tmux-shim |
| 7 | `main.go` | `main_test.go` | cmd/tmux-shim |
| 8 | `model_transform.go` | `model_transform_test.go` | cmd/tmux-shim |
| 9 | `command_router_handlers_pane.go` | `command_router_handlers_pane_test.go` | internal/tmux |
| 10 | `format.go` | `format_test.go` | internal/tmux |
| 11 | `key_table.go` | `key_table_test.go` | internal/tmux |
| 12 | `session_manager*.go` | `session_manager_test.go` | internal/tmux |
| 13 | `config.go` | `config_test.go` | internal/config |

---

## Critical (テスト設計上の重大な問題)

### TC-1: `handleKillPane` の Terminal Close 呼び出し回数が未検証
- **ファイル**: `internal/tmux/command_router_handlers_pane_test.go`
- **対象実装**: `handleKillPane` (`command_router_handlers_pane.go:260-282`)
- **説明**: `handleKillPane` は `target.Terminal` をキャプチャ後、`KillPane` を呼び出す。`KillPane` 内部で既にターミナルの Close を実行している可能性があるが、`handleKillPane` がキャプチャした同一ポインタに対して再度 `Close()` を呼ぶパスがテストされていない。Close が冪等でない場合、パニック/リソースリークが発生する。
- **修正案**: Mock Terminal を注入し、`Close()` 呼び出し回数を検証するテストケースを追加。

### TC-2: `app_worktree_api_test.go` の `CreateSessionWithWorktree` エンドツーエンドテスト欠如
- **ファイル**: `app_worktree_api_test.go`
- **対象実装**: `CreateSessionWithWorktree` (`app_worktree_api.go`)
- **説明**: 個別の内部関数 (`copyConfigFilesToWorktree`, `copyConfigDirsToWorktree`, `chooseWorktreeIdentifier`, `shellExecFlag`, `runSetupScripts*`) はそれぞれ十分にテストされているが、`CreateSessionWithWorktree` の統合テスト（git worktree 作成→ファイルコピー→セットアップスクリプト実行→セッション作成の全フロー）が存在しない。ロールバックパス (`rollbackWorktree`) の統合テストも不足。
- **修正案**: テスト用のgitリポジトリfixture を使った統合テストを追加し、成功パスとロールバックパスの両方を検証。

---

## Important (カバレッジ上の重要な欠落)

### TI-1: `SendSyncInput` の専用テスト不在
- **ファイル**: `app_pane_api_test.go`
- **対象実装**: `SendSyncInput` (`app_pane_api.go`)
- **説明**: `SendInput` は十分にテストされているが、`SendSyncInput`（同期版入力送信）の専用テストが存在しない。同期/非同期の動作差異がテストで担保されていない。
- **修正案**: `SendSyncInput` のバリデーション、成功パス、エラーパスのテーブル駆動テストを追加。

### TI-2: `handleSendKeys` の空ペイロード・nil Terminal パステスト不足
- **ファイル**: `command_router_handlers_pane_test.go`
- **対象実装**: `handleSendKeys` (`command_router_handlers_pane.go:143-166`)
- **説明**: `handleSendKeys` には `target.Terminal == nil` ガードと `len(payload) == 0` の早期リターンが存在するが、これらのパスの直接テストがない。`SplitWindowWorkDirFallback` テスト内で間接的にルーターを通じて send-keys が実行される程度。
- **修正案**: `TestHandleSendKeys` テーブル駆動テストを追加（nil Terminal、空引数、正常引数のケース）。

### TI-3: `SplitWindowInternal` の直接テスト不在
- **ファイル**: `command_router_handlers_pane_test.go`
- **対象実装**: `SplitWindowInternal` (`command_router_handlers_pane.go:11-25`)
- **説明**: GUI から呼ばれるFast-path メソッド `SplitWindowInternal` は IPC 経由の `handleSplitWindow` とは異なるバリデーションパス（空文字列チェック、`-d` フラグなし、`-P`/`-F` フォーマットなし）だが、直接テストが存在しない。
- **修正案**: `TestSplitWindowInternal` テストを追加（空ターゲット、無効ターゲット、正常分割）。

### TI-4: `handleListPanes` テスト不在
- **ファイル**: `command_router_handlers_pane_test.go`
- **対象実装**: `handleListPanes` (`command_router_handlers_pane.go:310-325`)
- **説明**: `handleListPanes` はフォーマット文字列展開 + callerPaneID 解決 + session/window スコープの分岐があるが、本テストファイルにカバレッジが無い。`format_test.go` で `formatPaneLine` は個別にテストされているが、IPC リクエスト→レスポンスの統合テストが欠落。
- **修正案**: `-s` フラグ有無、`-F` カスタムフォーマット、`CallerPane` 解決のテーブル駆動テストを追加。

### TI-5: `EnsureFile` のテスト不在
- **ファイル**: `config_test.go`
- **対象実装**: `EnsureFile` (`config.go`)
- **説明**: `EnsureFile` は「ファイル無しなら作成→Load」のフローを持ち、`Save` のバリデーションとの相互作用がある。直接テストがなく、外部から呼ばれるエントリポイントとしてのカバレッジが不足。
- **修正案**: ファイル無し→作成確認、既存ファイル→変更なし確認、不正ファイル→エラーハンドリングのテストを追加。

### TI-6: `Clone` 関数のテスト不在
- **ファイル**: `config_test.go`
- **対象実装**: `Clone` (`config.go`)
- **説明**: `Clone` はディープコピーを保証する重要な関数で、`Keys`, `AgentModel.Overrides`, `PaneEnv`, `Worktree.CopyFiles/CopyDirs/SetupScripts` の独立性を確保する。フィールド追加時にコピー漏れが発生するリスクがあるが、テストが存在しない。
- **修正案**: 全フィールド設定→Clone→元の変更→Clone側の不変性検証テスト + フィールド数ガードテスト。

### TI-7: `configureGlobalHotkey` の単体テスト不在
- **ファイル**: `app_lifecycle_test.go`
- **対象実装**: `configureGlobalHotkey` (`app_lifecycle.go`)
- **説明**: ホットキー設定は OS 依存の挙動を含むが、設定値パース部分やエラーハンドリングロジックはテスト可能。テスト不在によりホットキー設定ロジックの回帰検出ができない。
- **修正案**: OS 依存部をインターフェース注入で分離し、パースロジックのテストを追加。

### TI-8: `startIdleMonitor` のテスト不在
- **ファイル**: `app_lifecycle_test.go`
- **対象実装**: `startIdleMonitor` (`app_lifecycle.go`)
- **説明**: session_manager_test.go で `SessionIdleStateTransitions` / `UpdateActivityByPaneID` は検証されているが、`startIdleMonitor` のポーリングループ（起動→停止→パニックリカバリ）は直接テストされていない。
- **修正案**: mock time + チャネルドリブンで `startIdleMonitor` の起動・検出・停止テストを追加。

### TI-9: `readLimitedFile` の上限超過テスト不在
- **ファイル**: `config_test.go`
- **対象実装**: `readLimitedFile` (`config.go`)
- **説明**: `maxConfigFileBytes` (1MB) を超過するファイルの拒否テストが不在。`readLimitedFile` は `Load` の入口であり、DoS 防御の境界テストとして重要。
- **修正案**: 1MB+1 byte のファイルを作成して Load がエラーを返すことを検証。

---

## Suggestions (テスト改善の提案)

### TS-1: `app_events_test.go` — `ensureOutputFlusher` の並行呼び出しテスト
- **説明**: `ensureOutputFlusher` はグローバル状態を管理し複数回呼ばれても安全であるべきだが、並行呼び出しテストがない。
- **影響**: 低（現時点で単一ゴルーチンからの呼び出しが前提）

### TS-2: `app_events_test.go` — `detachStaleOutputBuffers` の専用テスト
- **説明**: `detachAllOutputBuffers` は直接テストされているが、`detachStaleOutputBuffers`（条件付き切り離し）は専用テストなし。
- **影響**: 低（`detachAllOutputBuffers` テストで間接的にカバー可能）

### TS-3: `app_session_api_test.go` — `isWorktreeCleanForRemoval` のエッジケース
- **説明**: ワークツリー削除前のクリーンチェックは `KillSession` フロー内で間接テストされているが、uncommited changes がある場合の拒否パスの直接テストを推奨。

### TS-4: `main_test.go` — `main()` 関数の間接テスト
- **説明**: `main()` は `os.Exit` を呼ぶため直接テスト困難。現在は `parseCommand`, `runTransformSafe` 等の個別関数テストでカバーされているが、`ipc.Send` のモック版による統合テストを検討。
- **影響**: 中（IPC エラーパスのエンドツーエンド検証に有用）

### TS-5: `model_transform_test.go` — `applyFromToReplacement` の `--model=` 空値ガード
- **説明**: `applyModelOverride` パスでは `--model=` の空値テストが充実しているが、`applyFromToReplacement` パスで同等のガードテストがない。両パスの対称性確保を推奨。

### TS-6: `command_router_handlers_pane_test.go` — `emitLayoutChangedForSession` エッジケース
- **説明**: 存在しないセッション名でのログ出力確認、`paneForLayoutSnapshot` に `nil` セッションが渡される場合のテストを推奨。

### TS-7: `session_manager_test.go` — `AddWindow` の非存在セッションエラーテスト
- **説明**: `AddWindow` の成功パスはテストされているが、存在しないセッション名での `AddWindow` エラーパスのテストが見当たらない。

### TS-8: `session_manager_test.go` — `RemoveWindowByID` の直接テスト
- **説明**: `RemoveWindowByID` は他テスト (`TestAddWindowDefaultNameUsesUniqueWindowID`) のセットアップで使用されているが、専用のテスト（最後のウィンドウ削除拒否、存在しないウィンドウIDなど）が不在。

### TS-9: `config_test.go` — `renameFileWithRetry` の Windows リトライテスト
- **説明**: `atomicWrite` 内の `renameFileWithRetry` はWindows固有のファイルロック競合時リトライを行うが、このリトライロジックのテストがない。
- **影響**: 低（実行環境依存、テスト困難な場合はスキップ可）

### TS-10: `config_test.go` — `sanitizePaneEnv` の直接テスト
- **説明**: `sanitizePaneEnv` は `Load`/`Save` 経由で間接テストされているが、null byte 除去、`=` 含有キー拒否、case-insensitive 重複検出の個別テストが `applyDefaultsAndValidate` の内部として隠蔽されている。直接テストで各サニタイゼーションルールを明示化推奨。

---

## Strengths (テスト品質の良い点)

### 全体的な品質

| 観点 | 評価 | 詳細 |
|------|------|------|
| テーブル駆動テスト | ★★★★★ | 全13ファイルで一貫して活用。Go慣用パターンの模範 |
| 関数変数インジェクション | ★★★★★ | `renameFileFn`, `removeFileFn`, `executeRouterRequestFn` 等で外部依存を安全にモック |
| t.Cleanup パターン | ★★★★★ | グローバル状態リセットが全テストで確実に実行される |
| エッジケース網羅 | ★★★★☆ | nil, 空文字列, whitespace, 境界値が体系的にテスト |
| セキュリティテスト | ★★★★★ | パストラバーサル、symlink脱出、null byte注入、バジェット上限 |

### ファイル別の優秀なテスト設計

#### `app_events_test.go`
- **`TestSnapshotEventPolicyConsistency`**: `bypassDebounce=true && trigger=false` の矛盾設定を自動検出する invariant テスト。構造体フィールド追加時の抜け漏れ防止に有効。
- **`TestEmitSnapshotDebounceCoalescesMultipleRequests`**: デバウンスウィンドウ内の複数要求が1回にまとめられることを検証。タイミング依存テストだが `time.Sleep` の使用量が最小限。

#### `app_worktree_api_test.go`
- **セキュリティテストの充実**: `symlink-escape-outside-repo`, `path-traversal-dot-dot`, `nested-symlink-in-source-dir`, `budget-max-file-count`, `budget-max-total-size` — 攻撃パターンを網羅。
- **`TestCopyConfigDirsToWorktreePartialFailureContinues`**: 部分失敗時に処理を継続し、最終的にエラーを集約する動作を検証。

#### `cmd/tmux-shim/main_test.go`
- **ログローテーションテスト**: `TestRotateShimDebugLogIfNeededScenarios` はサイズ制限、不在ファイル、リネームコリジョン、プルーニングを体系的に検証。
- **`TestDebugLogFallbackMessageEmitsOnlyFirstNMessages`**: メッセージ抑制の閾値テストが上限値±1で正確に検証。

#### `cmd/tmux-shim/model_transform_test.go`
- **`TestApplyModelTransformAllWildcard`**: 10パターンの包括的テーブルで `ALL` ワイルドカードの全バリエーション（inline/tokenized/equals/case-insensitive/override優先/空値スキップ）を網羅。
- **`TestLoadAgentModelConfigConcurrentCalls`**: 16ゴルーチンの並行呼び出しでキャッシュポインタの同一性を検証。

#### `internal/tmux/format_test.go`
- **`TestLookupFormatVariableAllVariables`**: 全フォーマット変数の期待値をテーブルで一覧化。enumeration test として新規変数追加時のテスト更新を促す。
- **nil chain テスト**: nil pane → nil window → nil session の各階層で適切なデフォルト値返却を体系的に検証。

#### `internal/tmux/key_table_test.go`
- **`TestParseControlKeyFullAlphabetLowercase`/`Uppercase`**: a-z/A-Zの全26文字をループで自動検証。1文字追加漏れのリスクゼロ。
- **`TestTranslateSendKeysTableOverridesParseControlKey`**: テーブルルックアップ vs fallback パスの実装詳細に依存しない behavioral テスト。

#### `internal/config/config_test.go`
- **`TestAgentModelStructFieldCounts`**: struct フィールド数をリフレクションで検証し、フィールド追加時のテスト更新を強制。
- **`TestIsZeroConfig`**: 全フィールドの個別変更がzero判定を破ることを網羅。フィールド追加漏れ検知に有効。

---

## Coverage Gaps (カバレッジギャップ一覧)

### 優先度: 高

| # | 対象関数/メソッド | 実装ファイル | テスト状態 | 推奨アクション |
|---|-----------------|-----------|---------|------------|
| G-1 | `handleKillPane` Terminal Close回数 | `command_router_handlers_pane.go` | 二重Close未検証 | Mock Terminal で Close 回数テスト |
| G-2 | `CreateSessionWithWorktree` 統合テスト | `app_worktree_api.go` | 個別関数のみ | E2E統合テスト |
| G-3 | `SendSyncInput` | `app_pane_api.go` | テスト無し | バリデーション+成功パステスト |
| G-4 | `Clone` (Config) | `config.go` | テスト無し | ディープコピー検証 + フィールド数ガード |
| G-5 | `EnsureFile` | `config.go` | テスト無し | ファイル無し/既存/不正の3パターン |

### 優先度: 中

| # | 対象関数/メソッド | 実装ファイル | テスト状態 | 推奨アクション |
|---|-----------------|-----------|---------|------------|
| G-6 | `handleSendKeys` (nil Terminal, 空payload) | `command_router_handlers_pane.go` | 間接テストのみ | 専用テーブル駆動テスト |
| G-7 | `SplitWindowInternal` | `command_router_handlers_pane.go` | テスト無し | GUI パスの直接テスト |
| G-8 | `handleListPanes` | `command_router_handlers_pane.go` | テスト無し | IPC統合テスト |
| G-9 | `configureGlobalHotkey` | `app_lifecycle.go` | テスト無し | パースロジック分離 + テスト |
| G-10 | `startIdleMonitor` | `app_lifecycle.go` | テスト無し | mock time テスト |
| G-11 | `readLimitedFile` 上限超過 | `config.go` | テスト無し | 1MB+1 byte テスト |
| G-12 | `RemoveWindowByID` 専用テスト | `session_manager_windows.go` | セットアップ使用のみ | エラーパス + 境界テスト |
| G-13 | `AddWindow` 非存在セッション | `session_manager_windows.go` | テスト無し | エラーパステスト |

### 優先度: 低

| # | 対象関数/メソッド | 実装ファイル | テスト状態 | 推奨アクション |
|---|-----------------|-----------|---------|------------|
| G-14 | `ensureOutputFlusher` 並行呼び出し | `app_events.go` | テスト無し | 並行安全性テスト |
| G-15 | `detachStaleOutputBuffers` | `app_events.go` | 間接テストのみ | 条件付き切り離しの直接テスト |
| G-16 | `sanitizePaneEnv` 個別ルール | `config.go` | Load/Save内で間接テスト | 直接テスト化 |
| G-17 | `renameFileWithRetry` リトライ | `config.go` | テスト無し | OS 依存のため低優先 |
| G-18 | `applyFromToReplacement` --model=空値 | `model_transform.go` | 未テスト | applyModelOverride との対称テスト |
| G-19 | `emitLayoutChangedForSession` nil session | `command_router_handlers_pane.go` | 間接テストのみ | ログ出力確認テスト |
| G-20 | `toString()`/`toBytes()` | `app_helpers.go` | 間接テストのみ | 全分岐テーブル駆動テスト |

---

## テストの脆さ分析

### 低リスク（良好な設計）

| テストファイル | 脆さ要因 | 対策状況 |
|-------------|---------|---------|
| `app_events_test.go` | グローバル状態(`runtimeEventsEmitFn`等) | `t.Cleanup` で確実にリストア ✓ |
| `main_test.go` | グローバル状態(`renameFileFn`, `removeFileFn`) | `t.Cleanup` で確実にリストア ✓ |
| `model_transform_test.go` | グローバルキャッシュ(`modelConfigCached`) | `resetModelConfigLoadState` + `t.Cleanup` ✓ |
| `config_test.go` | 環境変数(`LOCALAPPDATA`) | `t.Setenv` で自動復元 ✓ |

### 注意が必要

| テストファイル | 脆さ要因 | リスク | 対策案 |
|-------------|---------|-------|--------|
| `app_events_test.go` | `time.Sleep` によるデバウンステスト | タイミング依存でCI環境で不安定になる可能性 | `time.AfterFunc` をモック可能にする検討 |
| `app_lifecycle_test.go` | `waitWithTimeout` のタイムアウトテスト | 短いスリープ値(10ms)がCI負荷で失敗する可能性 | マージン倍率を拡大 |
| `main_test.go` | `captureStderr` がos.Stderrを置換 | `t.Parallel()` 禁止が前提。将来的な並列化で壊れる | ファイル先頭にコメント明記済 ✓ |
| `session_manager_test.go` | `manager.mu.Lock()` でインターナル直接操作 | テストが実装詳細に結合 | 代替手段がないため許容。リファクタ時に注意 |

---

## 総合評価

| 評価項目 | スコア | コメント |
|---------|--------|---------|
| テスト設計品質 | **A** | テーブル駆動テスト、関数変数インジェクション、t.Cleanup が全ファイルで一貫 |
| 正常系カバレッジ | **A-** | 主要な公開関数の成功パスは網羅。一部 GUI Fast-path (SplitWindowInternal) が欠如 |
| 異常系カバレッジ | **A** | バリデーションエラー、nil入力、空文字列、存在しないリソース等を体系的に検証 |
| エッジケース | **A** | 境界値、Unicode、パストラバーサル、並行呼び出し等が充実 |
| セキュリティテスト | **A+** | symlink脱出、null byte、相対パス、バジェット上限、shell allowlist が模範的 |
| テストの安定性 | **B+** | 一部タイミング依存テストあり。グローバル状態管理は優秀 |
| カバレッジ網羅性 | **B+** | 内部関数のテストは充実だが、統合テスト・GUI Fast-path テストに改善余地 |

### 総合: **A-** (優秀)

全体として非常に高品質なテストスイート。テーブル駆動テスト、関数変数インジェクション、構造体フィールド数ガード等の Go 慣用パターンが徹底されている。主な改善点は統合テストの追加（`CreateSessionWithWorktree` E2E, `handleListPanes` IPC統合）と、GUI Fast-path (`SplitWindowInternal`) のカバレッジ拡充。
