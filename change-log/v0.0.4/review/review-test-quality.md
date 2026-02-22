# テスト品質レビュー: myT-x 全テストファイル

**日時:** 2025-02-18  
**対象:** myT-x/ 配下の全テストファイル（17ファイル + 追加テスト）  
**レビュー方式:** シニアテストエンジニアによるテスト品質・カバレッジ・正確性レビュー  
**方針:** `go test -race` は環境制約のため指摘対象外。後方互換は開発中のため不要。テーブル駆動テストを推奨。

---

## 目次

1. [Critical Issues（修正必須）](#critical-issues)
2. [Important Issues（修正推奨）](#important-issues)
3. [Missing Test Coverage（カバレッジ不足）](#missing-test-coverage)
4. [Suggestions（改善提案）](#suggestions)
5. [Positive Observations（良い点）](#positive-observations)

---

## Critical Issues

### CT-1: `applyNewProcessTransform` のテストが欠如

| 項目 | 内容 |
|------|------|
| **重要度** | Critical |
| **ファイル** | `cmd/tmux-shim/command_transform.go` L34 |
| **テストファイル** | `cmd/tmux-shim/command_transform_test.go` |

**説明:**  
`applyShellTransform` は十分にテストされているが、同ファイルの `applyNewProcessTransform` は一切テストされていない。この関数は `new-session`/`split-window` の実行パス変換を行う重要な機能であり、Windows/非Windowsでの挙動分岐がある。バグ混入時に検知できない。

**推奨:**  
テーブル駆動テストで `applyNewProcessTransform` のコマンド種別・フラグ組み合わせ・プラットフォーム分岐をカバーする。

---

### CT-2: `applySendKeysTransform` のテストが欠如

| 項目 | 内容 |
|------|------|
| **重要度** | Critical |
| **ファイル** | `cmd/tmux-shim/command_transform.go` L67 |
| **テストファイル** | `cmd/tmux-shim/command_transform_test.go` |

**説明:**  
`applySendKeysTransform` はsend-keysコマンドに対するエンター付加処理を行うが、専用のユニットテストがない。`applyShellTransform` テスト内で間接的にも呼ばれていない。send-keysはクライアントエージェントとの主要なインタラクションポイントであり、カバレッジの欠如は大きなリスク。

---

## Important Issues

### I-1: パッケージレベル関数変数のモッキングが `t.Parallel()` を排除

| 項目 | 内容 |
|------|------|
| **重要度** | Important（設計上の制約、即時対応不要） |
| **影響ファイル** | `app_*.go` 全テストファイル、`cmd/tmux-shim/main_test.go`、`internal/config/config_test.go` |

**説明:**  
`runtimeEventsEmitFn`、`executeRouterRequestFn`、`ensureShimInstalledFn`、`renameFileFn`、`removeFileFn`、`yamlUnmarshalConfigMetadataFn`、`defaultConfigDirFn` 等のパッケージレベル関数変数をテストごとに差し替えるパターンが全テストファイルで使用されている。`t.Cleanup()` でリストアされるが、グローバル共有状態のため `t.Parallel()` は一切使用不可。テスト数の増加に伴い、テスト実行時間が線形に増大する。

**現状の容認理由:**  
開発中フェーズであり、テスト実行時間が問題になるまで許容可能。将来的にインターフェースベースの依存注入（DI）に移行すれば並列化が可能になる。

---

### I-2: `config_test.go` の `TestSaveConcurrentWritesSerialized` はタイミング依存

| 項目 | 内容 |
|------|------|
| **重要度** | Important |
| **ファイル** | `internal/config/config_test.go` |

**説明:**  
並行書き込みテストでgoroutineを10本起動してファイル保存を並行実行するが、結果の検証は「最終ファイルが有効なYAMLであること」「パースエラーがないこと」のみ。実際の書き込み順序や排他制御の正当性は検証されていない。`atomicWrite` のリネーム方式が正しく排他しているかは、このテストでは判定不能。

**推奨:**  
各goroutineが一意の `DefaultShell` 値を書き込み、最終ファイルがいずれかの値で一貫していることを検証する（最後の勝ちパターン）。

---

### I-3: `cmd/tmux-shim/main_test.go` の `TestRunTransformSafePanicRecovery` がリカバリ後のリクエスト復元を部分的にしか検証していない

| 項目 | 内容 |
|------|------|
| **重要度** | Important |
| **ファイル** | `cmd/tmux-shim/main_test.go` |

**説明:**  
パニック後にリクエストが元に戻ることは `Command` フィールドで確認しているが、`Flags`、`Env`、`Args` が完全にロールバックされたかの深い検証がない。`cloneTransformRequest` テストは別途あるが、panic recovery 内でのロールバック整合性を直接確認すべき。

**推奨:**  
パニック前に `Flags` / `Env` / `Args` を変更するtransform関数を用意し、リカバリ後に全フィールドが元に戻ることを検証する。

---

### I-4: `app_events_test.go` のスナップショットデバウンスタイマーテストがタイミング依存

| 項目 | 内容 |
|------|------|
| **重要度** | Important |
| **ファイル** | `app_events_test.go` |

**説明:**  
`requestSnapshot` のデバウンス・コアレシングテストで `time.Sleep` を使った待機・検証パターンが見られる（例: `time.Sleep(20 * time.Millisecond)` 後にカウント確認）。CI環境での負荷によりタイミング条件が崩れ、flaky testになるリスクがある。

**推奨:**  
ポーリング + タイムアウトパターン（`waitForCondition` 等）に統一する。プロジェクト内に `waitForCondition` ヘルパが既に存在するなら、それを活用する。

---

### I-5: `app_worktree_api_test.go` の `copyConfigFilesToWorktree` セキュリティテストで実際のシンボリックリンク作成をスキップ

| 項目 | 内容 |
|------|------|
| **重要度** | Important |
| **ファイル** | `app_worktree_api_test.go` |

**説明:**  
パストラバーサル・シンボリックリンク脱出テストは網羅的で良いが、いくつかのテストケースでは「パスパターンの検証」のみで実際にファイルシステム上でのシンボリック脱出を試行していない。`handleSymlinkInWalk` のテストでは `os.Symlink` を使った実ファイルシステムテストも存在するが、`copyConfigFilesToWorktree` のシンボリックリンク脱出テストとの連携が不完全。

**現状の評価:**  
`handleSymlinkInWalk` 自体はしっかりテストされており、実害リスクは低い。ただし、統合テストとして実際のシンボリックリンクを `copyConfigFilesToWorktree` に渡すケースがあると理想的。

---

## Missing Test Coverage

### MC-1: `app_window_api.go` — `stableWindowTarget` / `routerCommandError` ユニットテスト不在

| 項目 | 内容 |
|------|------|
| **ファイル** | `app_window_api.go` L105, L109 |

**説明:**  
`stableWindowTarget` (フォーマット文字列生成) と `routerCommandError` (レスポンスからエラー変換) は小さなヘルパだが、フォーマット文字列の仕様変更が波及する可能性がある。`AddWindow`/`RemoveWindow`/`RenameWindow` の統合テストで間接的にカバーされているが、ユニットテストが望ましい。

---

### MC-2: `app_helpers.go` — `toString` / `toBytes` テスト不在

| 項目 | 内容 |
|------|------|
| **ファイル** | `app_helpers.go` L7, L19 |

**説明:**  
`toString` と `toBytes` は型アサーション（`string`, `[]byte`, `fmt.Sprintf` フォールバック）を行うヘルパ。`handlePaneOutputEvent` テストで間接的にカバーされているが、`nil` 入力、非string/非byteの型（`int`, `struct`等）のフォールバック動作は直接テストされていない。

---

### MC-3: `app_pane_feed.go` — `startPaneFeedWorker` のパニックリカバリパステスト不在

| 項目 | 内容 |
|------|------|
| **ファイル** | `app_pane_feed.go` L53 |

**説明:**  
`startPaneFeedWorker` 内の `recoverBackgroundPanic` によるリスタートループのテストが存在しない。`TestStartPaneFeedWorkerLifecycle` はライフサイクル（起動→停止）を確認するが、ワーカー内でパニックが発生した場合の自動復旧はテストされていない。

---

### MC-4: `app_lifecycle.go` — `startIdleMonitor` / `configureGlobalHotkey` テスト不在

| 項目 | 内容 |
|------|------|
| **ファイル** | `app_lifecycle.go` L262, L367 |

**説明:**  
`startIdleMonitor` はタイマーベースのアイドル監視ループ、`configureGlobalHotkey` はOS依存のホットキー登録。いずれも外部依存が強くモッキングが困難だが、少なくとも起動即キャンセルのライフサイクルテストは可能。

---

### MC-5: `internal/tmux/command_router_handlers_session.go` — ハンドラ個別テスト不在

| 項目 | 内容 |
|------|------|
| **ファイル** | `internal/tmux/command_router_handlers_session.go` |
| **関連テスト** | `command_router_handlers_session_test.go`（存在は確認したがレビュー対象17ファイル外） |

**補足:** `command_router_handlers_session_test.go` が存在するが、当初のレビュー対象17ファイルには含まれていない。追加の `command_router_handlers_window_test.go`、`command_router_terminal_test.go`、`command_router_pane_env_test.go`、`command_router_options_test.go` も同様。これらの品質確認が必要。

---

### MC-6: `cmd/tmux-shim/parse.go` — `validateTargetFlag` / `validateRequired` / `validateNonNegativeSizeFlags` の直接テスト不在

| 項目 | 内容 |
|------|------|
| **ファイル** | `cmd/tmux-shim/parse.go` L98, L105, L139 |

**説明:**  
これらのバリデーション関数は `parseCommand` テスト経由で間接的にカバーされているが、境界値テスト（例: ちょうど0のサイズフラグ、ターゲットフラグの不正フォーマット）が `parseCommand` のテストケースとして体系的に整理されていない。

---

### MC-7: `app_snapshot_delta.go` — `copySnapshotCache` テスト不在

| 項目 | 内容 |
|------|------|
| **ファイル** | `app_snapshot_delta.go` L254 |

**説明:**  
`copySnapshotCache` はスナップショットキャッシュのディープコピーを行う関数だが、直接テストがない。`snapshotDelta` テスト内で間接的に使用されてはいるが、コピーの独立性（元のmapを変更してもコピーに影響しないこと）は直接検証されていない。プロジェクト全体でディープコピーの正確性を重視している思想と整合性をとるべき。

---

### MC-8: `app_config_api.go` — `DetachSession` / `PickSessionDirectory` のテストが部分的

| 項目 | 内容 |
|------|------|
| **ファイル** | `app_config_api.go` L170, L129 |

**説明:**  
`DetachSession` は `sessions` nil ガードテストのみ。正常パス（セッション存在時の `RemoveSession` 呼び出し + スナップショット発行）のテストがない。`PickSessionDirectory` は `requireRuntimeContext` ガードテストのみ。Wails依存のため完全テストは困難だが、コンテキスト存在時のエラーパスは検証可能。

---

## Suggestions

### S-1: テストヘルパの統一と共有

**対象:** 全テストファイル

**説明:**  
テストヘルパ関数（`newTestApp`, `waitForCondition`, `captureEmitter`等）が各テストファイルに分散している。特に `waitForCondition` はタイミング依存テストの改善に不可欠だが、テストパッケージ間で共有されていない。

**推奨:**  
`internal/testutil/` に共通テストヘルパを集約する（既に存在する場合は活用を拡大する）。

---

### S-2: structフィールドカウントガードテストのパターン化

**対象:** `TestTmuxRequestStructFieldCountForCloneTransformRequest`、`TestConfigStructFieldCounts`、`TestAgentModelStructFieldCounts`、`TestSnapshotFieldCounts`、`TestConfigEventFieldCounts`、`TestPaneFeedItemFieldCount`

**説明:**  
struct フィールド数ガードテストはプロジェクト全体の優れたパターン。ただし現在は `reflect.NumField()` の値を直接ハードコードしている。フィールド追加時にテストが失敗することで検知できるが、**なぜそのフィールド数が期待値なのか**をコメントで明記すると保守性が向上する。

**推奨例:**
```go
// TmuxRequest has 6 fields: Command, Target, Flags, Env, Args, SessionID
// If this changes, update cloneTransformRequest to handle the new field.
if got := reflect.TypeOf(ipc.TmuxRequest{}).NumField(); got != 6 {
```

---

### S-3: エラーメッセージ文字列の定数化

**対象:** 複数テストファイル

**説明:**  
テスト内で `strings.Contains(err.Error(), "sessions not initialized")` のようなエラーメッセージ文字列の部分一致検証が多数ある。実装側のメッセージが変わるとテストが静かに通り抜ける可能性がある。

**推奨:**  
エラー変数 (`var ErrSessionsNotInitialized = errors.New(...)`) または `errors.Is` パターンの導入を検討。すぐに対応不要だが、エラー文字列の重複箇所を把握しておくことは有益。

---

### S-4: `model_transform_test.go` の concurrent access テストを強化

**対象:** `cmd/tmux-shim/model_transform_test.go`

**説明:**  
`TestLoadAgentModelConfigConcurrentAccess` で10本のgoroutineが `loadAgentModelConfig` を並行呼び出しするが、検証は「全結果が同一であること」のみ。キャッシュの排他制御が正しいことを検証するには、キャッシュミス→ロード→キャッシュヒットの遷移を観察するテストがあるとより堅牢。

---

### S-5: `app_snapshot_delta_test.go` の `TestSnapshotDeltaConcurrentCallsRemainStable` は誤検知リスク

**対象:** `app_snapshot_delta_test.go`

**説明:**  
並行スナップショットデルタ呼び出しテストは、10本のgoroutineが同一スナップショットで `snapshotDelta` を呼ぶが、キャッシュへの書き込み競合が発生しても `snapshotCache` はmutexで保護されているため問題ない。テストの意図が「競合でパニックしないこと」であれば、コメントで明記すべき。

---

## Positive Observations

### P-1: テーブル駆動テストの徹底

全17ファイルにおいて、テーブル駆動テストパターンが一貫して使用されている。特に以下が優れている：

- `cmd/tmux-shim/main_test.go`: `parseCommand` テストが21コマンド × 複数フラグ組み合わせで網羅的
- `internal/tmux/key_table_test.go`: 全制御キー + 全特殊キーの網羅テスト
- `internal/tmux/format_test.go`: 全フォーマット変数の網羅テスト `lookupFormatVariable`
- `app_worktree_api_test.go`: セキュリティテストケースのパストラバーサルパターン

---

### P-2: ディープコピー独立性テストの徹底

プロジェクト全体で、返却値の変更が内部状態に波及しないことを検証するテストが網羅的に実装されている：

- `TestListSessionsReturnsIndependentCopies` (session_manager_test.go)
- `TestGetSessionEnvReturnsCopy` (session_manager_test.go)
- `TestGetPaneEnvReturnsCopy` (session_manager_test.go)
- `TestListPanesByWindowTargetAllInSessionDeepCopiesEnv` (session_manager_pane_io_test.go)
- `TestGetWorktreeInfoReturnsCopy` (session_manager_env_test.go)
- `TestGetConfigSnapshotReturnsIndependentCopy` (app_config_state_test.go)
- `TestClonePreservesDeepCopySemanticsForAllSliceFields` (config_test.go)
- `TestGetPaneEnvReturnsDeepcopy` (app_pane_api_test.go)
- `TestResolveSessionTargetReturnsCopy` (session_manager_test.go)
- `TestCloneTransformRequestCreatesIndependentCopy` (main_test.go)

これはデータ競合やエイリアシングバグの予防に非常に効果的。

---

### P-3: structフィールドカウントガードテストの導入

以下のテストでstructフィールド数をハードコードし、フィールド追加時にclone/serialize/比較関数の更新漏れを自動検知する仕組みが優秀：

- `TestTmuxRequestStructFieldCountForCloneTransformRequest`
- `TestConfigStructFieldCounts` (Config, AgentModel, AgentModelOverride, Keys)
- `TestAgentModelStructFieldCounts`
- `TestSnapshotFieldCounts` (SessionSnapshot, WindowSnapshot, PaneSnapshot)
- `TestConfigEventFieldCounts`
- `TestPaneFeedItemFieldCount`

このパターンはフィールド追加→コピー漏れバグの検知に極めて有効で、プロジェクトの品質基準の高さを示している。

---

### P-4: セキュリティテストの充実

`app_worktree_api_test.go` のファイルコピー機能テストが以下の攻撃パターンをカバーしている：

- パストラバーサル（`../`、`..\\`、`C:\`、深くネストしたトラバーサル）
- シンボリックリンク脱出（external symlink escape、chain symlink）
- 絶対パス注入（`/etc/passwd`、`C:\Windows\System32`）
- バジェット超過によるリソース枯渇（ファイル数制限、ディレクトリ深度制限）
- ヌルバイト注入（config_test.go: `null byte in shell name`）

---

### P-5: イベント発行順序の検証

`command_router_handlers_pane_test.go` の `TestHandleSelectPaneTitleSetsTitle` テストで、`captureEmitter` を使いイベントの発行順序（rename → focus）を厳密に検証している。UIの状態更新がイベント順序に依存する場合に重要なテスト。

---

### P-6: transform safety wrapperの網羅的テスト

`cmd/tmux-shim/main_test.go` の `applyShellTransformSafeWith` / `applyModelTransformSafeWith` テストが以下のシナリオをカバーしている：

- 正常変換
- 変換関数内でのパニック → リクエスト完全ロールバック
- 変換関数がエラーを返却 → リクエストロールバック
- nilリクエスト → 安全にスキップ
- 内部の `runTransformSafe` への委譲

shimが変換失敗時に原文をそのまま転送する設計仕様に適合したテスト。

---

### P-7: Terminal参照保持テスト（ConPTYリーク防止）

`session_manager_window_test.go` の `TestRemoveWindowByIDPreservesTerminalReferencesInReturnedPanes` (T-5) は、返却されるペインスライスが `Terminal` フィールド参照を保持していることを検証しており、ConPTYハンドルのリーク防止に直結する重要なテスト。

---

### P-8: デバッグログローテーション・プルーニングの堅牢なテスト

`cmd/tmux-shim/main_test.go` のログ関連テストが以下をカバー：

- ログローテーションの閾値超過/未超過
- 同一秒内の衝突リトライ（`nextRotatedShimDebugLogPath` の1000回ループ）
- ログプルーニングの古い順削除
- プルーニング中のエラー継続
- ローテーション判定とマーキングの一貫性

---

## レビュー対象外テストファイル一覧

以下のテストファイルは `internal/tmux/` に存在するが、当初の17ファイルに含まれていなかった。別途レビューが必要：

| ファイル | 概要 |
|---------|------|
| `command_router_handlers_session_test.go` | session系ハンドラテスト |
| `command_router_handlers_window_test.go` | window系ハンドラテスト |
| `command_router_terminal_test.go` | ターミナル接続テスト |
| `command_router_pane_env_test.go` | pane環境変数テスト |
| `command_router_options_test.go` | ルーターオプションテスト |
| `command_router_test_helpers_test.go` | テストヘルパ定義 |
| `session_manager_env_test.go` | env/worktreeテスト |
| `session_manager_idle_test.go` | アイドル状態テスト |
| `session_manager_snapshot_test.go` | スナップショットテスト |
| `layout_preset_test.go` | レイアウトプリセットテスト |
| `panic_recovery_test.go` | パニックリカバリテスト |
| `integration_test.go` | 統合テスト |

---

## タスク一覧・優先度・並列作業可否表

| ID | 種別 | 内容 | 優先度 | 見積り | 並列可否 | 依存 |
|----|------|------|--------|--------|----------|------|
| CT-1 | Critical | `applyNewProcessTransform` テスト追加 | 高 | 小 | ✅ 独立 | — |
| CT-2 | Critical | `applySendKeysTransform` テスト追加 | 高 | 小 | ✅ 独立 | — |
| I-2 | Important | concurrent write テスト改善 | 中 | 小 | ✅ 独立 | — |
| I-3 | Important | panic recovery ロールバック検証強化 | 中 | 小 | ✅ 独立 | — |
| I-4 | Important | スナップショットデバウンステストのflaky対策 | 中 | 中 | ✅ 独立 | — |
| I-5 | Important | symlink統合テスト追加 | 低 | 中 | ✅ 独立 | — |
| MC-1 | Coverage | `stableWindowTarget`/`routerCommandError` テスト | 低 | 小 | ✅ 独立 | — |
| MC-2 | Coverage | `toString`/`toBytes` テスト | 低 | 小 | ✅ 独立 | — |
| MC-3 | Coverage | paneFeedWorkerパニックリカバリテスト | 中 | 中 | ✅ 独立 | — |
| MC-6 | Coverage | parse.go バリデーション境界値テスト | 低 | 中 | ✅ 独立 | — |
| MC-7 | Coverage | `copySnapshotCache` 独立性テスト | 低 | 小 | ✅ 独立 | — |
| MC-8 | Coverage | `DetachSession` 正常パステスト | 低 | 小 | ✅ 独立 | — |
| S-2 | Suggestion | フィールドカウントテストにコメント追加 | 低 | 小 | ✅ 独立 | — |
| S-3 | Suggestion | エラーメッセージ定数化検討 | 低 | 大 | ⚠️ 広範囲 | — |

**備考:** 全タスクは相互に独立しており、並列作業が可能。S-3のみ広範囲のリファクタリングとなるため、他タスク完了後に着手推奨。

---

## 総合評価

テスト品質は**全体的に非常に高水準**。テーブル駆動テスト、ディープコピー独立性検証、structフィールドカウントガード、セキュリティテスト、イベント順序検証といった、一般的なGoプロジェクトでは見落とされがちなテストパターンが網羅的に適用されている。

主な改善点は `cmd/tmux-shim/command_transform.go` の2つの未テスト関数（CT-1, CT-2）と、タイミング依存テストのflaky対策（I-4）。カバレッジ不足の指摘は主にヘルパ関数の直接テスト不在であり、間接カバレッジは存在する。
