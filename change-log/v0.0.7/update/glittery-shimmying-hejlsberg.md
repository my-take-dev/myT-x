# Diff View - Right Sidebar Implementation Plan

## Context

右サイドバーに4つ目のビュープラグイン「Diff」を追加する。セッションのワーキングディレクトリの変更（`git diff HEAD`）をフォルダ構造付きファイルツリー+行単位diffで表示する機能。参考画像: `sample/diff_image.png`

**データソース**: `git diff HEAD` (staged + unstaged changes vs HEAD)

---

## Implementation Steps

### Step 1: Backend Types (`myT-x/app_devpanel_types.go`)

`GitStatusResult` の後に追加:

```go
type WorkingDiffFile struct {
    Path      string `json:"path"`
    OldPath   string `json:"old_path"`
    Status    string `json:"status"`    // "modified" | "added" | "deleted" | "renamed"
    Additions int    `json:"additions"`
    Deletions int    `json:"deletions"`
    Diff      string `json:"diff"`
}

type WorkingDiffResult struct {
    Files        []WorkingDiffFile `json:"files"`
    TotalAdded   int               `json:"total_added"`
    TotalDeleted int               `json:"total_deleted"`
    Truncated    bool              `json:"truncated"`
}
```

### Step 2: Backend API (`myT-x/app_devpanel_api.go`)

`DevPanelListBranches` の後に追加:

- `DevPanelWorkingDiff(sessionName string) (WorkingDiffResult, error)` メソッド
  - `resolveSessionWorkDir` で作業ディレクトリ取得
  - `gitpkg.IsGitRepository` チェック (非gitリポ → 空結果、エラー無し)
  - `gitpkg.RunGitCLIPublic(workDir, ["diff", "HEAD"])` 実行
  - サイズ制限: `devPanelMaxDiffSize` (500KB) 超過時は truncate
  - `parseWorkingDiff(raw)` でファイル単位に分割

- `parseWorkingDiff(raw string) []WorkingDiffFile` 関数 (private)
  - `diff --git` ヘッダーでファイル分割
  - ステータス検出: `new file mode` → added, `deleted file mode` → deleted, `rename` → renamed, else → modified
  - `+`/`-` 行カウントで additions/deletions 計算
  - 各ファイルの raw diff を `Diff` フィールドに格納

### Step 3: Backend Tests (`myT-x/app_devpanel_api_test.go`)

- `TestWorkingDiffFileFieldCountGuard` (6 fields)
- `TestWorkingDiffResultFieldCountGuard` (4 fields)
- `TestParseWorkingDiff` テーブル駆動テスト
  - empty string, single modified file, added/deleted file, multi-file, malformed input

### Step 4: Wails Binding 再生成

`go build` で `frontend/wailsjs/go/main/App.js` + `App.d.ts` 自動生成

### Step 5: Frontend Icon (`myT-x/frontend/src/components/viewer/icons/DiffIcon.tsx`)

新規作成。既存 `GitGraphIcon.tsx` パターン準拠 (SVG, viewBox 20x20, stroke currentColor)

### Step 6: Frontend Types (`myT-x/frontend/src/components/viewer/views/diff-view/diffViewTypes.ts`)

```typescript
export interface WorkingDiffFile {
  path: string; old_path: string;
  status: "modified" | "added" | "deleted" | "renamed";
  additions: number; deletions: number; diff: string;
}
export interface WorkingDiffResult {
  files: WorkingDiffFile[];
  total_added: number; total_deleted: number; truncated: boolean;
}
export interface DiffTreeNode {
  name: string; path: string; isDir: boolean;
  depth: number; isExpanded: boolean; file?: WorkingDiffFile;
}
```

### Step 7: Frontend Hook (`myT-x/frontend/src/components/viewer/views/diff-view/useDiffView.ts`)

- `useDiffView()` — `useFileTree` パターン準拠
  - `tmuxStore.activeSession` 監視、セッション切替時リセット
  - `api.DevPanelWorkingDiff(session)` でデータ取得
  - `buildDiffTree(files, expandedDirs)` でフォルダ階層ツリー構築
  - stale request guard (mountedRef + sessionRef)
  - 初回ロード時は最初のファイルを自動選択

### Step 8: Frontend Components

**`DiffFileSidebar.tsx`** — 左パネル (ファイルツリー)
- `react-window` `FixedSizeList` による仮想化 (ROW_HEIGHT=28)
- `FileTreeSidebar.tsx` パターン準拠 (ResizeObserver + FixedSizeList)
- フォルダ: 展開/折りたたみ、ファイル: クリックでdiff表示
- 各ファイルに `+additions -deletions` 表示 (緑/赤)
- ステータス色分け: added=緑, deleted=赤, modified=白

**`DiffContentViewer.tsx`** — 右パネル (diff表示)
- ファイルパス + `+additions -deletions` ヘッダー
- `parseFileDiff()` で unified diff をパース (既存 `DiffViewer.tsx` の `parseDiff` ロジック流用)
- 行番号 (old/new) + 色分け (追加=緑背景, 削除=赤背景)
- 既存CSS再利用: `.diff-viewer`, `.diff-hunk-header`, `.diff-line`, `.diff-line.added`, `.diff-line.removed`

**`DiffView.tsx`** — メインコンテナ
- `FileTreeView.tsx` レイアウトパターン準拠
- ヘッダー: "Diff" タイトル + `+N -N` 統計 + "Files Changed: N" + Refresh/Close ボタン
- ボディ: `file-tree-body` 内に `DiffFileSidebar` + `DiffContentViewer`

**`index.ts`** — ビュー登録
- `registerView({ id: "diff", icon: DiffIcon, label: "Diff", component: DiffView, shortcut: "Ctrl+Shift+D" })`

### Step 9: Wiring

**`ViewerSystem.tsx`** に追加:
- `import "./views/diff-view";` (side-effect import)
- `Ctrl+Shift+D` キーボードショートカット

**`api.ts`** に追加:
- import `DevPanelWorkingDiff` from wailsjs
- `api` オブジェクトに追加

### Step 10: CSS (`myT-x/frontend/src/styles/viewer.css`)

追加クラス:
- `.diff-view` — flex column, height 100%
- `.diff-header-stats` — 統計表示 (additions/deletions)
- `.diff-header-file-count` — ファイル数表示
- `.diff-tree-stats` — ツリー行内の `+N -N` バッジ
- `.diff-tree-additions` — color: var(--git-staged) (緑)
- `.diff-tree-deletions` — color: var(--danger) (赤)
- `.diff-content-viewer` — 右パネルコンテナ
- `.diff-content-header` — ファイルパスヘッダー
- `.diff-content-body` — スクロール可能な diff 表示領域

既存再利用 (追加不要): `.diff-viewer`, `.diff-hunk-header`, `.diff-line.*`, `.diff-line-number`, `.diff-line-content`, `.file-tree-body`, `.file-tree-sidebar`, `.file-tree-content`, `.tree-node-row`, `.tree-node-arrow`, `.tree-node-icon`, `.tree-node-name`

---

## Critical Files

| File | Action |
|------|--------|
| `myT-x/app_devpanel_types.go` | Edit: add WorkingDiffFile, WorkingDiffResult |
| `myT-x/app_devpanel_api.go` | Edit: add DevPanelWorkingDiff, parseWorkingDiff |
| `myT-x/app_devpanel_api_test.go` | Edit: add tests |
| `myT-x/frontend/src/api.ts` | Edit: add DevPanelWorkingDiff |
| `myT-x/frontend/src/components/viewer/ViewerSystem.tsx` | Edit: add import + shortcut |
| `myT-x/frontend/src/components/viewer/icons/DiffIcon.tsx` | **New** |
| `myT-x/frontend/src/components/viewer/views/diff-view/index.ts` | **New** |
| `myT-x/frontend/src/components/viewer/views/diff-view/diffViewTypes.ts` | **New** |
| `myT-x/frontend/src/components/viewer/views/diff-view/useDiffView.ts` | **New** |
| `myT-x/frontend/src/components/viewer/views/diff-view/DiffFileSidebar.tsx` | **New** |
| `myT-x/frontend/src/components/viewer/views/diff-view/DiffContentViewer.tsx` | **New** |
| `myT-x/frontend/src/components/viewer/views/diff-view/DiffView.tsx` | **New** |
| `myT-x/frontend/src/styles/viewer.css` | Edit: add diff-view CSS |

## Reuse

- `DiffViewer.tsx` の `parseDiff` ロジック → `DiffContentViewer` の `parseFileDiff` に流用
- `FileTreeSidebar.tsx` の ResizeObserver + FixedSizeList パターン → `DiffFileSidebar` に流用
- `FileTreeView.tsx` のレイアウト構造 → `DiffView` に流用
- 既存 diff CSS クラス群 → そのまま再利用
- `useFileTree.ts` の stale request guard パターン → `useDiffView` に流用

## Verification

1. `go test ./...` (myT-x/ ディレクトリ) — バックエンド全テスト通過
2. `wails build` or `go build` — コンパイル成功
3. GoLand `build_project` + `get_file_problems` — 静的解析エラーなし
4. アプリ起動 → 右サイドバーに Diff アイコン表示確認
5. `Ctrl+Shift+D` でビュー開閉
6. Git リポジトリのあるセッションで変更ファイルがツリー表示されること
7. ファイルクリックで右パネルにdiff表示されること
8. 非gitセッションでエラーではなく空表示になること

## Development Flow

```
confidence-check → golang-expert (Step 1-4) → テスト作成 (Step 3)
→ フロントエンド実装 (Step 5-10) → self-review → 全クリアまで修正
```
