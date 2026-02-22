# フロントエンド マルチウィンドウ対応

**実施日:** 2026-02-17
**ステータス:** 完了（全Phase実装済み）
**関連計画:** `plans/fizzy-roaming-book.md`（tmux-shim ギャップ対応の一部）

---

## 背景

`fizzy-roaming-book.md` の計画に基づき、tmux-shim に以下のウィンドウコマンドを実装済み:
- `new-window`, `kill-window`, `rename-window`, `select-window`, `list-windows`

バックエンド（SessionManager, CommandRouter, スナップショット）は全て正常動作していたが、
**フロントエンドが `windows[0]` をハードコード** していたため、
tmux-shim 経由で作成・切替したウィンドウが GUI に一切反映されないギャップが存在した。

---

## 発見したギャップ

| # | 箇所 | 問題 |
|---|------|------|
| G1 | `SessionView.tsx:92` | `const window = props.session.windows[0]` — 常に最初のウィンドウのみ表示 |
| G2 | `App.tsx:21` | `activePaneIdFromSession` が `windows[0]` のみ参照 |
| G3 | `SessionView.tsx:18-27` | `paneList` は全ウィンドウを集約するが、レンダリングは `windows[0]` のみ |
| G4 | `tmuxStore.ts` | ウィンドウ管理ステートが無い |
| G5 | UI | ウィンドウタブ/セレクター UI が無く、ウィンドウ切替不可 |
| G6 | `api.ts` | `AddWindow`, `RemoveWindow` 等の Wails バインディング未定義 |
| G7 | Go App | ウィンドウ操作の Wails エクスポートメソッドが無い |

---

## 実装内容

### Phase 1: フロントエンド最小修正（G1-G3, G5 対応）

#### Step 1: アクティブウィンドウ解決ロジック

**変更ファイル:**
- `myT-x/frontend/src/components/SessionView.tsx`
- `myT-x/frontend/src/App.tsx`

**実装詳細:**
- `windows[0]` → `windows.find(w => w.panes.some(p => p.active)) ?? windows[0]` に変更
- `paneList` をアクティブウィンドウのペインのみに限定（sync input, zoom のスコープ修正）
- `activePaneIdFromSession` も同様にアクティブウィンドウから解決

#### Step 2: ウィンドウタブ UI

**変更ファイル:**
- `myT-x/frontend/src/components/SessionView.tsx`
- `myT-x/frontend/src/styles/layout.css`

**実装詳細:**
- ウィンドウが2つ以上の場合のみタブバーを表示
- タブクリック → `api.FocusPane(window.panes[0].id)` で間接的にウィンドウ切替
- タブダブルクリック → インラインリネーム編集
- `+` ボタン → 新規ウィンドウ追加
- `×` ボタン → ウィンドウ削除（ホバー時のみ表示）

**設計判断:**
- ウィンドウ切替は `FocusPane` → `SetActivePane` → スナップショット更新の既存フローを再利用
- 専用の `activeWindowId` ストア状態は不要（`pane.active` から暗黙的に解決）

### Phase 2: バックエンド Wails バインディング（G6-G7 対応）

#### Step 3: Go App メソッド

**新規ファイル:** `myT-x/app_window_api.go`

| メソッド | 説明 |
|---------|------|
| `AddWindow(sessionName)` | `executeRouterRequestFn` で `new-window` に委譲 |
| `RemoveWindow(sessionName, windowID)` | `WindowIndexInSession` でインデックス解決後、`kill-window` に委譲 |
| `RenameWindow(sessionName, windowID, newName)` | 同上、`rename-window` に委譲 |

**設計判断:**
- `KillSession` と同じ `executeRouterRequestFn` パターンを採用
- CommandRouter ハンドラにターミナルアタッチ・イベント発行・ロールバック等の複雑なロジックが既にあるため、直接 SessionManager を呼ばずルーター経由で委譲

#### Step 4: フロントエンド API

**変更ファイル:** `myT-x/frontend/src/api.ts`

- `AddWindow`, `RemoveWindow`, `RenameWindow` を import に追加し、`api` オブジェクトにエクスポート

### Phase 3: テスト + レビュー

#### Step 5: テスト

**新規ファイル:** `myT-x/app_window_api_test.go`（18テストケース）

| テスト | ケース数 | 内容 |
|--------|---------|------|
| `TestAddWindowValidation` | 3 | 空名前、ルーター未初期化、正常系 |
| `TestRemoveWindowValidation` | 4 | 空名前、ルーター未初期化、ウィンドウ不在、正常系 |
| `TestRenameWindowValidation` | 5 | 空名前、空新名前、ルーター未初期化、ウィンドウ不在、正常系 |
| `TestAddWindowDelegatesViaRouter` | 1 | IPC リクエスト構築の検証 |
| `TestRemoveWindowDelegatesViaRouter` | 1 | IPC リクエスト構築の検証（`-t session:idx` 形式） |
| `TestRenameWindowDelegatesViaRouter` | 1 | IPC リクエスト構築 + Args の検証 |

#### Step 6: self-review

- CSS変数 `--border` → `--line` の修正（プロジェクトでは `--line` を使用）
- テスト追加の指摘 → `app_window_api_test.go` 作成で対応

---

## 変更ファイル一覧

| ファイル | 変更種別 | 内容 |
|---------|---------|------|
| `myT-x/frontend/src/components/SessionView.tsx` | 修正 | アクティブウィンドウ解決 + タブUI + 操作コールバック |
| `myT-x/frontend/src/App.tsx` | 修正 | `activePaneIdFromSession` のウィンドウ解決修正 |
| `myT-x/frontend/src/api.ts` | 修正 | `AddWindow`, `RemoveWindow`, `RenameWindow` 追加 |
| `myT-x/frontend/src/styles/layout.css` | 修正 | ウィンドウタブバーCSS追加 |
| `myT-x/app_window_api.go` | **新規** | Wails バインディング3メソッド |
| `myT-x/app_window_api_test.go` | **新規** | 18テストケース |

---

## テスト結果

```
ok  myT-x                   12.394s   (app_window_api_test.go 含む)
ok  myT-x/cmd/tmux-shim     (cached)
ok  myT-x/internal/tmux     (cached)
... 全14パッケージ PASS
```

---

## 今後の課題

- **手動テスト未実施**: tmux-shim 経由での `new-window` → GUI タブ表示の実動作確認が必要
- **フロントエンドビルド**: Wails バインディング生成（`wails generate`）後にフロントエンドのビルド確認が必要
- **単一ウィンドウ時の `+` ボタン**: 現在はウィンドウが2つ以上の場合のみタブバーが表示される。単一ウィンドウ時にウィンドウを追加する UI 導線は未実装（tmux-shim 経由でのみ追加可能）
