# Plan: review-20260218183000.md 未対応指摘の対応

## Context

`review-20260218183000.md` にまとめられた未対応レビュー指摘（Critical 1件・Important 15件）を解消する。
バグ修正・デッドコード削除・コメント追記・テスト補完が中心。機能追加は含まない。

---

## 対象ファイルと修正内容

### グループ A: バグ修正（Critical/Important）

#### A-1. `R1-C6` — `TerminalPane.tsx` : `pendingFontSize` vs `term.options.fontSize` 競合
**ファイル**: `myT-x/frontend/src/components/TerminalPane.tsx`
**問題の根本**:
- 端末セットアップの `useEffect` は `[paneEvent, props.paneId]` のみを依存配列とし、`fontSize` の更新をクロージャに取り込まない
- `handleWheel` のフォールバック `term.options.fontSize` は React の再レンダリング前（第二 `useEffect` 未適用）では古い値
- 高速ホイール操作でタイマーが発火し `setFontSize(15)` → store 更新 → しかし次のホイール前に `term.options.fontSize` がまだ 13 のまま → 14 として計算されてしまう

**修正**:
1. コンポーネントレベルで `fontSizeRef = useRef(fontSize)` を追加
2. 第二 `useEffect([fontSize])` で `fontSizeRef.current = fontSize` を同期更新
3. タイマー発火時に `fontSizeRef.current = newSize` を `setFontSize` の前に同期更新（次ホイールに即時反映）
4. `handleWheel` 内フォールバックを `term.options.fontSize` → `fontSizeRef.current` に変更

```typescript
// コンポーネントレベル (useEffect の外)
const fontSizeRef = useRef(fontSize);

// 第二 useEffect を修正
useEffect(() => {
  const term = terminalRef.current;
  if (!term) return;
  fontSizeRef.current = fontSize;  // ref を先に同期更新
  term.options.fontSize = fontSize;
  fitAddonRef.current?.fit();
}, [fontSize]);

// handleWheel 内のタイマー
fontSizeTimer = window.setTimeout(() => {
  fontSizeTimer = null;
  if (pendingFontSize === null) return;
  const newSize = pendingFontSize;
  pendingFontSize = null;
  fontSizeRef.current = newSize;  // 次のホイールイベントに即時反映
  setFontSize(newSize);
}, 50);

// handleWheel のフォールバック
const current = pendingFontSize ?? fontSizeRef.current;
```

#### A-2. `R1-I16` — `SearchBar.tsx` : 再オープン時に前回クエリが残存
**ファイル**: `myT-x/frontend/src/components/SearchBar.tsx`
**問題**: `open=true` になっても `setQuery("")` が呼ばれない。
**修正**: `open` を監視する useEffect に `setQuery("")` と `searchAddon?.clearDecorations()` を追加。

#### A-3. `R1-I21` — `command_router_handlers_pane.go` : `append(args, "Enter")` スライス汚染
**ファイル**: `myT-x/internal/tmux/command_router_handlers_pane.go` (L117付近)
**問題**: `append(args, "Enter")` が backing array を破壊する可能性。
**修正**: `append` 前に `argsWithEnter := make([]string, len(args), len(args)+1)` でコピーを作成してから append する（または `append(append([]string{}, args...), "Enter")`）。

#### A-4. `R1-I25` — `format.go` : `session_created_human` nil フォールバックフォーマット不一致
**ファイル**: `myT-x/internal/tmux/format.go`
**問題**: nil 時は `time.RFC3339`、正常時は `"Mon Jan _2 15:04:05 2006"` で不統一。
**修正**: nil フォールバックを `time.Unix(0, 0).Format("Mon Jan _2 15:04:05 2006")` に統一。

#### A-5. `R2-I3-3` — `models.ts` : 配列フィールドに `?? []` フォールバック欠落
**ファイル**: `myT-x/frontend/wailsjs/go/models.ts`
**問題**: `setup_scripts`, `copy_files`, `copy_dirs` が `undefined` になりうる。
**修正**: constructor 内で `?? []` を付与。

---

### グループ B: 定数・デッドコード整理

#### B-1. `R1-I2` — `app_events.go` : マジックナンバー `16ms`, `8*1024`
**ファイル**: `myT-x/app_events.go` (L120-121)
**修正**: パッケージ内定数として切り出す。
```go
const (
    outputFlushIntervalMs = 16 * time.Millisecond
    outputFlushBufSize    = 8 * 1024
)
```

#### B-2. `R2-I5-2` — `session_manager_windows.go` : `removeWindowAtIndexLocked` の `activeWindow == nil` ガードの説明
**ファイル**: `myT-x/internal/tmux/session_manager_windows.go`
**現状確認**: 現在のコードに `session.ActiveWindowID == removedWindowID` という明示条件は存在しない。`findWindowByID(session.Windows, session.ActiveWindowID)` が nil を返すのは、削除されたウィンドウが ActiveWindowID だった場合のみ。コードは正しいが、コメントがない。
**修正**: `activeWindow == nil` チェック前後に、「なぜ activeWindow が nil になりうるか（ActiveWindowID が削除されたウィンドウを指していた場合）」を説明するコメントを追記し、意図を明確化する。

---

### グループ C: コメント追記

#### C-1. `R1-I1` — `app_events.go` : debounce 戦略コメント
**ファイル**: `myT-x/app_events.go` (L247-251付近)
**修正**: `requestSnapshot` 関数に leading-edge fixed-window debounce の動作説明コメントを追記。

#### C-2. `R1-I3` — `app_snapshot_delta.go` : 防御ガードコメント
**ファイル**: `myT-x/app_snapshot_delta.go` (L166付近)
**修正**: `!a.snapshotPrimed` ガードの到達条件（初回スナップショット前）を説明するコメントを追記。

#### C-3. `R2-I3-2` — `SessionView.tsx` : `onSelectWindow` 暗黙依存コメント
**ファイル**: `myT-x/frontend/src/components/SessionView.tsx` (L79-92)
**修正**: `FocusPane` によるウィンドウ切替が `SetActivePane` → `ActiveWindowID` 更新に依存する旨をコメントで明記。

#### C-4. `R2-I4-2` — `format.go` : `window_index` と tmux のセマンティクス差異
**ファイル**: `myT-x/internal/tmux/format.go` (window_index case)
**修正**: `window_index` が tmux の連番インデックスではなく `window.ID` を返す旨のコメントを追記。

#### C-5. `R2-I5-4` — `session_manager_pane_lifecycle.go` : `removedWindowIdx` セマンティクス
**ファイル**: `myT-x/internal/tmux/session_manager_pane_lifecycle.go`
**修正**: `removedWindowIdx` が「空間的なヒントとして隣接 window へのフォールバック選択に使用」する旨のコメントを追記。

---

### グループ D: テスト追加・修正

#### D-1. `R1-I5` — `app_status_test.go` : `resolvePaneTitle` エッジケース
**ファイル**: `myT-x/app_status_test.go`
**追加テスト**:
- `activePN` が pane.Index と一致しない（フォールバックで `Active=true` を使用）
- 全 pane で `Active=false`, `activePN` 不一致時（Title のある最初の pane を返す）

#### D-2. `R1-I6` — `app_events_test.go` : `syncPaneStates` 統合テスト
**ファイル**: `myT-x/app_events_test.go`
**追加テスト**: `syncPaneStates` が `EnsurePane` / `SetActivePanes` / `RetainPanes` を正しく呼ぶことを検証するテーブル駆動テスト。

#### D-3. `R1-I9` — `app_window_api_test.go` : 複数ウィンドウ `RemoveWindow` テスト
**ファイル**: `myT-x/app_window_api_test.go`
**追加テスト**: 2ウィンドウのセッションで1ウィンドウを削除してもセッションが存続することを検証。

#### D-4. `R1-I24` — `session_manager_env_test.go` : ロック無し直接変更
**ファイル**: `myT-x/internal/tmux/session_manager_env_test.go` (`TestGetPaneContextSnapshot` 関数)
**問題**: `CreateSession` が返した `*TmuxPane` ポインタに対し `pane.Env["FOO"] = "bar"` / `pane.Title = "editor"` をロック無しで直接変更している。
**修正方針**:
- ペインレベルの env/title を設定する公開 API（例: `SetPaneTitle(paneID, title)`）が存在する場合は API 経由に変更
- 存在しない場合は、テスト内で `manager.mu.Lock()` を保持して変更するヘルパーを作成するか、sequential test であることをコメントで明記し lint ignore を追加する（この場合は `//nolint:govet` ではなく test-only コメントを付与）
- `TestGetPaneContextSnapshot` の `snapshot.Env["FOO"]` と `snapshot.Title` の検証が壊れないよう、変更後も同一の assert を維持する

---

## 開発フロー

```
confidence-check → golang-expert / フロントエンド修正 → go-test-patterns → self-review
```

1. **confidence-check** スキル適用（新規テスト追加・バグ修正なので必須）
2. **グループ A・B・C** を実装（各ファイル編集 → `reformat_file` → `build_project` → `get_file_problems`）
3. **グループ D** テスト追加
4. **self-review** → 全クリアまで修正

---

## 検証

```bash
# Go テスト
cd myT-x && go test ./... -count=1

# フロントエンドビルド確認
cd myT-x/frontend && bun run build
```

- `go test ./...` が全パス
- フロントエンドビルドエラーなし
- `build_project` でコンパイルエラーなし
