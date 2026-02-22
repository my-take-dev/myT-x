# テストコードレビュー: diff_tests.txt

**レビュー日**: 2026-02-18  
**対象カテゴリ**: Tests  
**差分ファイル**: `review/diff_tests.txt` (2821行)

---

## Critical Issues (即座に修正が必要)

なし。重大なバグやセキュリティ上の問題は検出されませんでした。

---

## Important Issues (修正すべき)

### I-1. `session_manager_targets.go` (278行追加) のテストカバレッジ不足

session_manager_targets.go には公開関数が3つ存在するが、diff に追加されたテストでは直接的なカバレッジ増加が見当たらない:

- **`ResolveDirectionalPane()`**: 既存テスト (`session_manager_test.go`) にカバレッジがあるか確認したところ、テスト名 `TestResolveDirectionalPane` は diff に含まれておらず、既存テストのみである。追加された278行に対して、**新しい境界値テスト**（例: `DirPrev` で先頭ペイン、`DirNext` で末尾ペイン、nil window、空 panes リスト）の追加を検討すべき。

- **`ResolveTarget()` の RLock → Lock アップグレードパス**: `resolveTargetRLocked` が `needsRepair=true` を返し、`resolveTargetWriteLocked` にフォールバックするパスのテストは `TestResolveTargetRepairsStaleActiveWindowID` のみ。concurrent access 時のダブルチェックロッキングの検証テストがない。

  - [myT-x/internal/tmux/session_manager_targets.go:24] `ResolveTarget` の RLock/Lock アップグレードパス
  - [myT-x/internal/tmux/session_manager_targets.go:205] `ResolveDirectionalPane` の境界値テスト

### I-2. `RemoveWindowByID` の戻り値変更に対するテスト更新が不十分

  - [myT-x/internal/tmux/session_manager_test.go:117] `RemoveWindowByID` の戻り値が3つから4つに変更 (`_, _, err` → `_, _, _, err`)。session_manager_test.go 内での修正は1箇所のみ。他の全呼び出し箇所 (session_manager_window_test.go など) も確認が必要。

### I-3. `TestApplyModelTransformSafeSwallowsLoaderError` のアサーション変更の安全性

  - [myT-x/cmd/tmux-shim/main_test.go:904-920] shim仕様の変更に伴い `err == nil` → `err != nil` へ反転。`applyModelTransformSafeWith` は load error を swallow して `(false, nil)` を返す仕様だが、テスト名「`SafeOnLoaderError`」のまま。テスト名を `SkipsOnLoaderError` や `SwallowsLoaderError` に変更した方が意図が明確。

### I-4. `TestApplyModelTransformSkipsOnLoaderError` のデバッグログ検証でのOS依存

  - [myT-x/cmd/tmux-shim/model_transform_test.go:621-648] `t.Setenv("LOCALAPPDATA", logDir)` でログファイルパスを設定し、`shim-debug.log` への書き込みを検証している。しかしこの検証は **Windows 環境以外では `LOCALAPPDATA` が使われない** 可能性があり、CI環境によっては失敗する恐れがある。プロジェクトが Windows 専用であれば問題ないが、確認が必要。

### I-5. `TestParseCommandFlagEnvEqualsInValue` で `PATH =/usr/bin` のスペース混入

  - [myT-x/cmd/tmux-shim/main_test.go: "value with spaces around =" テストケース] `"PATH =/usr/bin"` はキー名に末尾スペースを含む文字列。`parseCommand` の `-e` フラグパーサーが `SplitN("=", 2)` → TrimSpace する想定だが、実装が `PATH` と `PATH ` のどちらをキーとして格納するかの検証が曖昧。`wantEnv: map[string]string{"PATH": "/usr/bin"}` はトリムされたキーを期待しているが、実装側のトリムロジックが確実に適用されるか確認すべき。

---

## Suggestions (改善提案)

### S-1. format_test.go: `newTestFixture()` のフィクスチャでペインインデックスが不正

  - [myT-x/internal/tmux/format_test.go: newTestFixture()] フィクスチャのペイン `Index: 1`（ID=3, Index=1）だが、ウィンドウの `Panes` は1要素のみ。実際のシステムでは Index は0ベースでスライス位置と一致するはず。テストの `"all pane variables in one string"` のwant `"#{pane_index}"` → `"1"` は正しいが、シナリオとしてはやや不自然。Index を 0 にするか、2つのペインを追加して整合性を取ることを推奨。

### S-2. key_table_test.go: `parseControlKey` の `C-[` テストケースの文書化

  - [myT-x/internal/tmux/key_table_test.go: "C-[ handled by table not parseControlKey" テストケース] `parseControlKey("C-[")` が `false` を返すことを検証しているが、`sendKeysTable` に `"C-["` が存在するのでテーブル優先で解決される設計仕様。これはテストコメントで正しく説明されているが、テスト名を `"C-[ is resolved by table, not parseControlKey"` のようにより明確にすることを推奨。

### S-3. app_events_test.go: waitForCondition のタイムアウト延長の理由

  - [myT-x/app_events_test.go:478] `snapshotCoalesceWindow+300ms` → `2*time.Second` への変更が3箇所([行478](myT-x/app_events_test.go), [行522](myT-x/app_events_test.go), [行567](myT-x/app_events_test.go))。CI環境でのフラップ対策と思われるが、2秒は相対的に長い。テスト全体の実行時間への影響を考慮し、コメントで理由を明記することを推奨。

### S-4. model_transform_test.go: `resetModelConfigCache()` のエイリアス関数の必要性

  - [myT-x/cmd/tmux-shim/model_transform_test.go:700-704] `resetModelConfigCache()` は `resetModelConfigLoadState()` の単なるエイリアス。テスト内で2つの名前が混在すると可読性が低下する。どちらかに統一することを推奨。

### S-5. app_pane_api_test.go: `TestApplyLayoutPresetErrorPaths` の "session has no windows" テストケース

  - [myT-x/app_pane_api_test.go:620-640] `KillPane` でペインを削除してセッションが消えた後に "session not found" を期待するテスト。コメントで正しく説明されているが、テストケース名が `"session has no windows"` のままで、実際の検証は `"session not found"` エラー。名前を `"session removed after last pane killed"` に変更する方が正確。

### S-6. app_pane_api_test.go: `TestResizePaneBoundaryValues` でゼロ・負のサイズを「パニックしないこと」のみ検証

  - [myT-x/app_pane_api_test.go:682-700] ゼロ・負のcols/rowsがパニックしないことを検証しているが、エラーメッセージの内容は検証していない。ResizePane が入力バリデーションエラーを返すべきか（例: "cols must be positive"）、それとも内部実装がデフォルト値にフォールバックすべきかの仕様を明確にすべき。

### S-7. session_manager_test.go: `TestCloneSessionSnapshotsIndependence` で Worktree の `BaseBranch` と `IsDetached` のバリエーション不足

  - [myT-x/internal/tmux/session_manager_test.go:1059-1145] `BaseBranch` が空文字列、`IsDetached` が true のケースのみテスト。`BaseBranch` に値がある場合のクローン独立性も検証すべき。

### S-8. `list-sessions` のテストケースが parseCommand レベルのみ

  - [myT-x/cmd/tmux-shim/main_test.go: TestParseCommandListSessionsIntegration] `list-sessions` コマンドの parse テストはあるが、実際の `CommandRouter` ハンドラ経由の振る舞いテスト（空セッション時の出力、`-F` フォーマットの展開）がないように見える。format_test.go の `TestFormatSessionLineCustomFormat` でカバーされている部分はあるが、end-to-end のカバレッジ追加を推奨。

---

## Positive Observations (良い点)

### P-1. テーブル駆動テストの一貫した採用
新規テスト全体で `tests := []struct { ... }` パターンが一貫して使用されており、正常系・異常系の両方が体系的にカバーされている。特に:
- `TestEmitSnapshot`: nil context / nil sessions / 初回フルスナップショット / デルタの4パターン + サブテスト2件
- `TestLookupFormatVariableAllVariables`: 全変数の網羅的テスト
- `TestParseControlKeyFullAlphabetLowercase/Uppercase`: a-z/A-Z の全文字テスト
- `TestApplyModelTransformAllWildcard`: ALL ワイルドカードの11パターン

### P-2. 防御的テストの充実
- `TestLookupFormatVariableNilPane/NilWindow/NilSession`: nil チェーンの各レベルでの安全性検証
- `TestRequireSessionsWithPaneIDTrimsAndValidates`: ホワイトスペースのみの入力、トリム後の値確認
- `TestGetPaneReplayErrorPaths` / `TestGetPaneEnvErrorPaths`: nil paneStates / 空ID / ホワイトスペースIDの全異常系

### P-3. フィールド数ガードテストの維持
既存のフィールド数ガードテスト（`TestSnapshotFieldCounts`, `TestRouterOptionsStructFieldCounts` 等）は diff では変更されていないが、今回の変更で struct フィールドの追加はないため、更新不要の判断は正しい。

### P-4. テスト間の独立性に関する明確なドキュメント
パッケージレベルの関数変数（`runtimeEventsEmitFn` 等）を上書きするテストファイル群に、以下のコメントが一貫して追加されている:
```go
// NOTE: This file overrides package-level function variables
// (runtimeEventsEmitFn). Do not use t.Parallel() here.
```
これは `app_pane_api_test.go`, `app_session_api_test.go`, `app_worktree_api_test.go`, `app_lifecycle_test.go` の全ファイルに追加されている。

### P-5. 仕様変更に伴うテストの正確な更新
`applyModelTransform` のエラーハンドリング変更（load error を swallow する仕様）に対して:
- `model_transform_test.go` (`TestApplyModelTransformSkipsOnLoaderError`): テスト名の変更 + アサーション反転
- `main_test.go` (`TestApplyModelTransformSafeSwallowsLoaderError`): 同様のアサーション反転 + 仕様コメント追加
- デバッグログへの書き込み検証の追加 (T-8)

### P-6. cleanupLegacyShimInstallsFn のテスト追加
新しい `cleanupLegacyShimInstallsFn` フック変数に対して:
- `TestEnsureShimReadyCallsLegacyCleanup`: 呼び出し確認の専用テスト
- 既存の全 `ensureShimReady` テスト: `cleanupLegacyShimInstallsFn = func() error { return nil }` のスタブ追加（7箇所）

### P-7. format.go の包括的テスト拡充
`lookupFormatVariable` と `expandFormat` に対して:
- 全変数の網羅テスト (`TestLookupFormatVariableAllVariables`)
- nil pane / nil window / nil session の3レベル安全性テスト
- `expandFormat` の複合プレースホルダ、空文字列、未知変数、nil ペイン、隣接プレースホルダ
- `formatWindowLine` / `formatSessionLine` / `formatPaneLine` のデフォルト/カスタムフォーマットテスト
- `joinLines` ヘルパーのテスト

---

## カバレッジサマリ

| テストファイル | 追加テスト数(概算) | カバレッジ評価 |
|---|---|---|
| app_events_test.go | 4 (TestEmitSnapshot 6パターン + 2サブテスト、TestSnapshotEventPolicyConsistency、TestLegacyMapPaneOutputTypeMismatchLog) | ◎ 良好 |
| app_lifecycle_test.go | 1 (TestEnsureShimReadyCallsLegacyCleanup) + 既存テスト修正7箇所 | ○ 十分 |
| app_pane_api_test.go | 6 (ErrorPaths系3テスト + BoundaryValues + FallbackToFirstWindow + RequireSessionsWithPaneID) | ◎ 良好 |
| app_session_api_test.go | コメント追加のみ | ー 変更なし |
| app_worktree_api_test.go | コメント追加のみ | ー 変更なし |
| config_test.go | 2 (ALLワイルドカード受容テスト) | ○ 十分 |
| command_router_handlers_pane_test.go | 1 (rootPath+worktreePathの両方空テスト) | ○ 十分 |
| format_test.go | 20+ (全変数テスト、nil系列、format関数系、joinLines) | ◎ 非常に良好 |
| key_table_test.go | 7 (全特殊キー、コントロールキー全アルファベット、無効入力、複合シーケンス) | ◎ 非常に良好 |
| session_manager_test.go | 7 (cloneSessionSnapshots独立性、CreateSession重複/空名/ゼロサイズ/負サイズ) | ◎ 良好 |
| command_transform_test.go | 3 (applyNewProcessTransform直接テスト、applySendKeysTransform、applyShellTransform) | ○ 十分 |
| main_test.go | 14+ (resize-pane方向フラグ、list-sessions、show-env、flagEnv=値内等号、"--"、set-env空値) | ◎ 良好 |
| model_transform_test.go | 6+ (resetModelConfigCache、--model=空値、isAllModelFrom、ALLワイルドカード11パターン) | ◎ 非常に良好 |

**総合評価**: テスト品質は高い。テーブル駆動テストの一貫性、正常系/異常系のバランス、nil安全性テストの網羅性が優れている。主な改善機会は `session_manager_targets.go` への追加テストと、一部のテスト名の明確化。
