# Plan: `new-window` をセッション作成に変更

## Context

現在、tmux の `new-window` コマンドは既存セッション内にウィンドウ（タブ）を追加する動作になっている。
myT-x では、各セッションが独立した作業単位であり、`new-window` が来た場合は左サイドバーに新しいセッションとして表示する方が自然なモデルである。

**目的**: `new-window` → 新セッション作成、タブUI完全削除、1セッション=1ウィンドウモデルへの移行

**ユーザー確認済み事項**:
- `-t`: 親セッション参照として使用（`is_agent_team`, `UseClaudeEnv`, `UsePaneEnv` を継承）
- ウィンドウモデル: 1セッション1ウィンドウに固定
- セッション名衝突: エラーを返す（呼び出し元が別名で再試行）

---

## 実装ステップ

### Step 1: Backend — `handleNewWindow` 書き換え

**ファイル**: `myT-x/internal/tmux/command_router_handlers_window.go`

`handleNewWindow` を以下のように書き換える:

1. `-t` から**親セッション**を解決（フラグ継承用）
2. `-n` を**新セッション名**として使用（必須化、空ならエラー）
3. `r.sessions.HasSession(sessionName)` で重複チェック → エラー
4. 親セッションの active pane からターミナルサイズを取得
5. `r.sessions.CreateSession(sessionName, "0", width, height)` でセッション作成（`AddWindow` の代わり）
6. 親セッションから以下を継承:
   - `IsAgentTeam` → `r.sessions.SetAgentTeam()`
   - `UseClaudeEnv` → `r.sessions.SetUseClaudeEnv()`
   - `UsePaneEnv` → `r.sessions.SetUsePaneEnv()`
7. 環境変数: `resolveEnvForPaneCreation()` を使用（既存関数再利用）
   - 新セッションのスナップショットを再取得してから呼び出す（フラグ継承後の状態で解決するため）
8. `attachPaneTerminal(pane, workDir, env, nil)` でターミナル接続
   - `workDir` は `-c` フラグ、空なら OS デフォルト（アプリ起動元ディレクトリ）
9. `bestEffortSendKeys()` でブートストラップコマンド実行
10. `-d` でなければ `SetActivePane()` で active 設定
11. イベント: `tmux:session-created` を emit（`tmux:window-created` ではない）
12. `-P`/`-F`: `expandFormatSafe()` で出力

**ロールバック**: セッション作成成功後にターミナル接続失敗 → `r.sessions.RemoveSession()` で巻き戻し（`handleNewSession` と同パターン）

**既存関数の再利用**:
- `r.sessions.CreateSession()` — `session_manager.go`
- `r.resolveEnvForPaneCreation()` — `command_router.go:204`
- `r.attachPaneTerminal()` — 既存
- `r.bestEffortSendKeys()` — 既存
- `expandFormatSafe()` — 既存
- `parseSessionName()` — 既存

### Step 2: Backend — `AddWindow` メソッド削除

**ファイル**: `myT-x/internal/tmux/session_manager_windows.go`

- `AddWindow()` メソッドを削除
- 以下は維持: `RemoveWindow`, `RemoveWindowByID`, `RenameWindow`, `RenameWindowByID`, `WindowIndexInSession`
  （kill-window は1ウィンドウでもセッション破棄として機能するため維持）

### Step 3: Backend — `App.AddWindow` API 削除

**ファイル**: `myT-x/app_window_api.go`

- `AddWindow()` メソッドを削除
- `RemoveWindow()`, `RenameWindow()` は維持（内部的に使用される可能性あり）
- `stableWindowTarget()`, `routerCommandError()` は維持（RemoveWindow/RenameWindow で使用）

### Step 4: Frontend — SessionView タブUI削除

**ファイル**: `myT-x/frontend/src/components/SessionView.tsx`

削除する要素:
- `useWindowRename` インポートと hook 呼び出し全体
- `ConfirmDialog` インポート（window 削除用のみ使用のため）
- `pendingRemoveWindowID` state と関連ロジック
- `onSelectWindow`, `onAddWindow`, `onRemoveWindow`, `confirmRemoveWindow` コールバック
- `windowLabel()` ヘルパー関数
- `RenameInput` サブコンポーネント
- `window-tab-bar` div 全体（`role="tablist"` セクション）
- `ConfirmDialog` コンポーネント render
- `tabpanel` の `role`/`aria-*` 属性

簡素化後の `renderSessionContent`:
```tsx
<>
  <div className="session-view-header">
    <LayoutPresetSelector ... />
    {paneList.length >= 2 && <button ... sync toggle ... />}
  </div>
  <div className="session-view-body">
    <LayoutRenderer ... />
  </div>
</>
```

### Step 5: Frontend — useWindowRename.ts 削除

**ファイル**: `myT-x/frontend/src/hooks/useWindowRename.ts` → 削除

SessionView でのみ使用されており、タブリネーム機能の削除に伴い不要。

### Step 6: Frontend — api.ts から Window API 削除

**ファイル**: `myT-x/frontend/src/api.ts`

削除:
- `AddWindow` のインポートと api オブジェクトのエントリ
- `RemoveWindow` のインポートと api オブジェクトのエントリ
- `RenameWindow` のインポートと api オブジェクトのエントリ

### Step 7: Frontend — CSS タブスタイル削除

**ファイル**: `myT-x/frontend/src/styles/layout.css`

`/* --- Window Tab Bar --- */` セクション全体を削除:
- `.window-tab-bar` 〜 `.window-tab-add:hover` まで（約100行）

### Step 8: テスト更新

**ファイル**: `myT-x/internal/tmux/command_router_handlers_window_test.go`

- `TestHandleNewWindow` — 全面書き換え:
  - 新セッション作成の正常系テスト
  - `tmux:session-created` イベント emit の検証
  - `-t` 必須・`-n` 必須のバリデーションテスト
  - セッション名重複エラーテスト
  - 親セッションから `IsAgentTeam` 継承テスト
  - 親セッションから `UseClaudeEnv`/`UsePaneEnv` 継承テスト
  - ターミナル接続失敗時のロールバックテスト
  - `-d` フラグテスト
  - `-P -F` フォーマット出力テスト
- マルチウィンドウ前提のテストケース削除/更新

**ファイル**: `myT-x/internal/tmux/session_manager_window_test.go`
- `AddWindow` 関連テスト削除
- マルチウィンドウ前提テスト削除/更新

**ファイル**: `myT-x/app_window_api_test.go`
- `TestAddWindowValidation`, `TestAddWindowDelegatesViaRouter` 削除
- マルチウィンドウ前提テスト削除

### Step 9: Wails バインディング再生成

`App.AddWindow` 削除後、Wails バインディングを再生成:
```bash
cd myT-x && wails generate
```

---

## 開発フロー

```
1. confidence-check（本プラン承認をもって完了とする）
2. 実装（Step 1 → Step 2 → Step 3 を順次実行）
   - defensive-coding-checklist 走査
   - golang-expert エージェントで実装
3. フロントエンド実装（Step 4〜7 を並列実行可）
4. Step 9: Wails バインディング再生成
5. テスト作成（Step 8）
   - go-test-patterns でテーブル駆動テスト
6. self-review → 課題修正 → 再レビュー（全クリアまで）
```

---

## 検証

1. **ビルド**: `cd myT-x && go fix ./... && go build ./...`
2. **バックエンドテスト**: `cd myT-x && go test ./internal/tmux/... -v`
3. **アプリテスト**: `cd myT-x && go test ./... -v`
4. **Shim テスト**: `cd myT-x && go test ./cmd/tmux-shim/... -v`
5. **フロントエンドビルド**: `cd myT-x/frontend && npm run build`
6. **手動確認**:
   - SessionView にタブが表示されないこと
   - Agent Teams で `new-window -t parent -n child -c /dir` → 新セッションがサイドバーに出現
   - セッション名重複時にエラーが返ること
   - `-d` フラグでアクティブセッションが切り替わらないこと
   - 親セッションの `is_agent_team` フラグが子セッションに継承されること
