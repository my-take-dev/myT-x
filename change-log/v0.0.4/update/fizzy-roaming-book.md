# フロントエンド マルチウィンドウ対応: tmux-shim連携ギャップ修正

## Context

tmux-shim に `new-window`, `kill-window`, `rename-window`, `select-window`, `list-windows` を実装済み。
バックエンド（SessionManager, CommandRouter, スナップショット）は全て正常動作する。
しかしフロントエンドが **`windows[0]` をハードコード** しているため、
tmux-shim 経由で作成・切替したウィンドウが GUI に一切反映されない。

### 発見したギャップ

| # | 箇所 | 問題 |
|---|------|------|
| G1 | `SessionView.tsx:92` | `const window = props.session.windows[0]` — 常に最初のウィンドウのみ表示 |
| G2 | `App.tsx:21` | `activePaneIdFromSession` が `windows[0]` のみ参照 |
| G3 | `SessionView.tsx:18-27` | `paneList` は全ウィンドウを集約するが、レンダリングは `windows[0]` のみ |
| G4 | `tmuxStore.ts` | `activeWindowId` 等のウィンドウ管理ステートが無い |
| G5 | UI | ウィンドウタブ/セレクター UI が無く、ウィンドウ切替不可 |
| G6 | `api.ts` | `AddWindow`, `RemoveWindow` 等の Wails バインディング未定義 |
| G7 | Go App | ウィンドウ操作の Wails エクスポートメソッドが無い |

### バックエンドの現状（正常動作）

- `SessionManager`: `AddWindow`, `RemoveWindow`, `RenameWindow`, `WindowIndexInSession` 実装済み
- `Snapshot()`: 全ウィンドウを `WindowSnapshot[]` として正しく含む
- `WindowSnapshot` に `active_pane` フィールドあり → アクティブペインを含むウィンドウ = アクティブウィンドウ
- イベント: `tmux:window-created`, `tmux:window-destroyed`, `tmux:window-renamed` → スナップショット再配信済み

---

## 実装計画

### Phase 1: フロントエンド最小修正（tmux-shim連携の即座修正）

tmux-shim経由のウィンドウ操作がGUIに正しく反映されるようにする。

#### Step 1: アクティブウィンドウ解決ロジック追加

**変更ファイル:**
- `myT-x/frontend/src/components/SessionView.tsx`
- `myT-x/frontend/src/App.tsx`

**実装:**

1. **`SessionView.tsx`**: `windows[0]` → アクティブウィンドウ（`pane.active === true` を含むウィンドウ）にフォールバック

```tsx
// Before:
const window = props.session.windows[0];

// After:
const window = props.session.windows.find(w =>
  w.panes.some(p => p.active)
) ?? props.session.windows[0];
```

2. **`SessionView.tsx` paneList**: アクティブウィンドウのペインのみに変更（sync input, zoom はアクティブウィンドウ内で動作すべき）

```tsx
const paneList = useMemo(() => {
  if (!props.session) return [] as PaneSnapshot[];
  const activeWindow = props.session.windows.find(w =>
    w.panes.some(p => p.active)
  ) ?? props.session.windows[0];
  return activeWindow ? activeWindow.panes : [];
}, [props.session]);
```

3. **`App.tsx` `activePaneIdFromSession`**: 同様にアクティブウィンドウから解決

```tsx
function activePaneIdFromSession(session: ...): string | null {
  if (!session || session.windows.length === 0) return null;
  const window = session.windows.find(w =>
    w.panes.some(p => p.active)
  ) ?? session.windows[0];
  const active = window.panes.find(p => p.active);
  return active?.id ?? window.panes[0]?.id ?? null;
}
```

#### Step 2: ウィンドウタブ UI 追加

**変更ファイル:**
- `myT-x/frontend/src/components/SessionView.tsx` — タブバー追加
- `myT-x/frontend/src/styles/session-view.css`（または該当CSSファイル）— タブスタイル

**実装:**

`SessionView` のヘッダー内に、ウィンドウが2つ以上ある場合のみタブバーを表示:

```tsx
{props.session.windows.length > 1 && (
  <div className="window-tabs">
    {props.session.windows.map((w, i) => (
      <button
        key={w.id}
        className={`window-tab ${w === activeWindow ? "active" : ""}`}
        onClick={() => onSelectWindow(w)}
      >
        {w.name || `${i}`}
      </button>
    ))}
  </div>
)}
```

ウィンドウ切替: `api.FocusPane(window.panes[0].id)` で間接的にアクティブウィンドウを変更。
既存の `FocusPane` → `SetActivePane` → スナップショット更新 フローで動作する。

### Phase 2: バックエンド Wails バインディング追加（GUI操作対応）

#### Step 3: App メソッド追加

**変更ファイル:**
- `myT-x/app_sessions.go`（または新規 `app_windows.go`）

**実装:**

```go
// AddWindow creates a new window in the specified session.
func (a *App) AddWindow(sessionName string) error { ... }

// RemoveWindow removes the active window from the specified session.
func (a *App) RemoveWindow(sessionName string, windowID int) error { ... }

// RenameWindow renames a window in the specified session.
func (a *App) RenameWindow(sessionName string, windowID int, newName string) error { ... }
```

既存パターン参考: `app_sessions.go` の `KillSession`, `RenameSession` 等。

#### Step 4: フロントエンド API 追加

**変更ファイル:**
- `myT-x/frontend/src/api.ts` — `AddWindow`, `RemoveWindow`, `RenameWindow` 追加

**実装:**

```ts
import { AddWindow, RemoveWindow, RenameWindow } from "../wailsjs/go/main/App";
// ... api オブジェクトに追加
```

#### Step 5: ウィンドウタブ操作 UI 強化

**変更ファイル:**
- `myT-x/frontend/src/components/SessionView.tsx`

**実装:**

- タブの `+` ボタン → `api.AddWindow(sessionName)`
- タブの `×` ボタン → `api.RemoveWindow(sessionName, windowID)`
- タブのダブルクリック → リネームインライン編集 → `api.RenameWindow(...)`

### Phase 3: テスト + レビュー

#### Step 6: テスト作成

| 対象 | テストファイル | 内容 |
|------|------------|------|
| アクティブウィンドウ解決 | 手動テスト | tmux-shim で `new-window` → `select-window` → GUI 表示確認 |
| App Wails バインディング | `myT-x/app_sessions_test.go` 等 | AddWindow/RemoveWindow/RenameWindow の正常/異常系 |
| ウィンドウタブ UI | 手動テスト | タブ表示、切替、追加、削除、リネーム |

#### Step 7: self-review

全実装完了後、`self-review` エージェントで検証。

---

## CSS スタイル方針

ウィンドウタブのスタイルは既存 `session-view-header` 内に配置。
Sidebar のセッションリストのデザイン言語（ボタンサイズ、ホバー効果）に合わせる。

---

## 検証方法

```bash
# 1. Go テスト（Phase 2 実装後）
go test ./myT-x/...

# 2. tmux-shim 手動検証（Phase 1 完了後）
# ウィンドウ作成
tmux new-window -t "session:0" -n "second"
# → GUI にウィンドウタブが出現し、second ウィンドウが表示される

# ウィンドウ切替
tmux select-window -t "session:0"
# → GUI が最初のウィンドウに切り替わる

# ウィンドウ削除
tmux kill-window -t "session:1"
# → GUI からウィンドウタブが消え、残りのウィンドウが表示される

# 3. GUI 操作検証（Phase 2 完了後）
# タブクリック → ウィンドウ切替
# + ボタン → 新規ウィンドウ追加
# × ボタン → ウィンドウ削除
```

---

## スコープ判断

| Phase | 内容 | 優先度 | 工数見積 |
|-------|------|--------|---------|
| Phase 1 (Step 1-2) | アクティブウィンドウ表示 + タブ UI | **必須** | 低 |
| Phase 2 (Step 3-5) | GUI からのウィンドウ操作 | 任意 | 中 |
| Phase 3 (Step 6-7) | テスト + レビュー | **必須** | 低 |

Phase 1 のみで tmux-shim 連携のギャップは解消される。
Phase 2 は GUI ユーザビリティ向上だが、tmux-shim 連携には不要。
