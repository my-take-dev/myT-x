# 右サイドバー Single Task Runner 調査報告

- 作成日: 2026-04-20
- 対象: `myT-x/` 配下
- 調査対象:
  - 右サイドバーの `single-task-runner` ビュー
  - `single-task-runner` MCP の組み込み起動経路
  - 参照実装: `orchestrator` MCP

## 結論

コード上の接続は概ね成立しています。  
少なくとも以下は揃っています。

- 右サイドバーの専用ビュー登録
- built-in MCP 定義の登録
- `ResolveMCPStdio` 経由のブリッジ解決
- embedded runtime 起動経路
- セッション単位の ServiceManager 管理

そのため、**実装状態としては「正常稼働可能な状態にかなり近い」**と判断します。  
一方で、**Wails 実アプリ上での手動 E2E は未確認**です。  
加えて、**フロントのコンポーネントテスト 2 件が現状 fail** しており、ここは品質上の未完了点です。

## 根拠

### 1. バックエンド初期化と MCP 登録は実装済み

- `singleTaskRunnerManager` は `NewApp()` で初期化済み  
  `myT-x/app.go:201`
- startup 時に built-in `single-task-runner` MCP 定義を registry に登録  
  `myT-x/app_lifecycle.go:399-403`
- `mcp.NewManager(...)` に `SingleTaskRunnerManager` を注入済み  
  `myT-x/app_lifecycle.go:404-409`

これは orchestrator MCP の登録経路と同系統です。

### 2. embedded runtime 起動経路が orchestrator と同じレイヤーにある

- `buildPipeConfig()` は `DefinitionKindSingleTaskRunner` を分岐処理し、外部コマンドではなく `RuntimeFactory` を返す  
  `myT-x/internal/mcp/orchestrator_factory.go:75-94`
- `singleTaskRunnerRuntimeFactory()` は `ServiceManager` から session 固有 `Service` を取得し、`internal/mcp/single-task-runner` runtime を生成する  
  `myT-x/internal/mcp/single_task_runner_factory.go:29-46` は nil manager reject / runtime create をテスト

つまり、MCP としての起動モデルは orchestrator と同じ embedded runtime パターンです。

### 3. CLI bridge 推奨情報も生成される

- `applyBridgeRecommendation()` は `single-task-runner` kind に対して `mcp stdio --mcp single-task-runner` を構築する  
  `myT-x/internal/mcpapi/service.go:176-196`
- そのテストあり  
  `myT-x/internal/mcpapi/service_test.go:369-385`

右サイドバーの MCP Manager 側で bridge コマンドを表示できる前提も満たしています。

### 4. 右サイドバー表示自体は登録済み

- 専用ビュー `single-task-runner` は registry 登録済み  
  `myT-x/frontend/src/components/viewer/views/single-task-runner/index.ts:6-12`
- ViewerSystem でも import 済みで右サイドバーに載る構成  
  `myT-x/frontend/src/components/viewer/ViewerSystem.tsx`
- MCP Manager 側でも orchestrator 不在時は `single-task-runner` を優先表示する実装  
  `myT-x/frontend/src/components/viewer/views/mcp-manager/McpManagerView.tsx:19-24`

### 5. フロントのセッションガードとイベント購読はある

- `useSingleTaskRunner()` は `single-task-runner:updated` / `single-task-runner:stopped` を購読
- generation と active session を比較して stale event を無視
  `myT-x/frontend/src/components/viewer/views/single-task-runner/useSingleTaskRunner.ts:239-274`

この点は、右サイドバーで壊れやすい session 切替後の誤反映を避ける実装になっています。

### 6. queue 本体の依存注入も適切

- `buildSingleTaskRunnerDepsFactory()` で session 内 pane 制約、router 経由送信、runtime context、panic recovery worker が注入されている  
  `myT-x/app_wiring.go:483-534`

orchestrator 参照観点でも、不足している DI は見当たりません。

## テスト結果

### 成功

- GoLand build: 成功
- 対象 Go テスト: 成功

実行コマンド:

```text
cd myT-x
go test . ./internal/mcp ./internal/mcpapi ./internal/singletaskrunner -run "SingleTaskRunner|ResolveMCPStdio"
```

結果:

```text
ok  	myT-x
ok  	myT-x/internal/mcp
ok  	myT-x/internal/mcpapi
ok  	myT-x/internal/singletaskrunner
```

補足:

- `app_single_task_runner_api_test.go` では Wails API 経由の add/get/set/remove などを確認
  `myT-x/app_single_task_runner_api_test.go:327-383`
- `useSingleTaskRunner.test.tsx`: 25 tests pass
- `McpManagerView.test.tsx`: 7 tests pass

### 失敗

以下の frontend test 2 件は fail:

- `tests/singleTaskRunnerComponents.test.tsx`
  - `SingleTaskRunnerList hides edit controls for malformed item payloads`
  - `SingleTaskRunnerList restores the saved delay when updating the delay throws`

直接原因:

- `SingleTaskRunnerList` mount 時に `api.GetValidationRules()` を呼ぶ
  `myT-x/frontend/src/components/viewer/views/single-task-runner/SingleTaskRunnerList.tsx:66-78`
- しかし `singleTaskRunnerComponents.test.tsx` 側でこの API を mock していない
  `myT-x/frontend/tests/singleTaskRunnerComponents.test.tsx:1-160`
- そのため test runtime では `window.go.main.App.GetValidationRules` が未定義で落ちる

重要:

- これは **現時点では production 実装不良の証拠ではなく、テストハーネス不足** です。
- ただし、**SingleTaskRunnerList 周辺の回帰検知が赤のまま** なので、保守上は放置しない方がよいです。

## 現時点の判定

### 稼働可否

- バックエンド起動配線: 問題なし
- MCP 定義登録: 問題なし
- embedded runtime 起動: 問題なし
- bridge 推奨情報生成: 問題なし
- 右サイドバー登録: 問題なし
- フロント session/event guard: 問題なし
- コンポーネントテスト健全性: 問題あり
- 手動 E2E: 未確認

### 総合判定

**実装上は稼働可能。致命的な欠落は見つからず。**  
ただし、**「正常に稼働する」と断定する最終確認は未了**です。

理由:

1. コード上の wiring は orchestrator MCP と同じ層で成立している
2. 対象 Go テストは通る
3. 右サイドバー hook / MCP Manager 表示も通る
4. ただし Wails 実アプリ上の end-to-end 実行確認はまだない
5. frontend component test 2 件が赤で、周辺回帰の監視が弱い

## 次アクション案

### 優先度高

1. `singleTaskRunnerComponents.test.tsx` に `api.GetValidationRules()` mock を追加して赤を解消
2. 実アプリで以下の E2E を確認
   - 右サイドバー `Single Task Runner` を開く
   - タスク追加
   - `Start Queue`
   - MCP bridge から `complete_task` / `fail_task` / `cancel_task`
   - セッション切替時の誤反映有無

### 実装修正の必要性

現時点の調査だけでは、**即修正必須の本番不具合は特定できていません**。  
修正着手の第一候補は、実装本体よりも **frontend test 整備と実機 E2E 確認** です。
