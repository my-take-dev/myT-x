# Viewer System: File Tree + Git Graph (プラグイン型ビューア)

## Context

myT-xターミナルマルチプレクサで、開発者がファイル構造やGit履歴を視覚的に確認する手段がない。
**右端の小さなアイコンボタン**を押すと**専用の全画面ビュー**が表示される仕組みを追加する。
ビュー表示中はターミナル操作不可（OK）。本体とは分離した**プラグイン型アーキテクチャ**で実装する。

**参考実装**: `sample/iori-editor/` の FileExplorer、DiffViewer、ViewSwitcher パターンを参照。
特に `flattenTree` + `react-window` による仮想化ツリーと遅延読み込みパターンを踏襲する。

---

## アーキテクチャ方針

### プラグイン型ビューアシステム
- 全コードを `components/viewer/` に自己完結させる
- **ViewPlugin レジストリパターン**: 各ビューが自己登録、本体は `<ViewerSystem />` を1行追加するのみ
- 独立 Zustand ストア (`viewerStore`) で `tmuxStore` とは完全分離
- ビュー表示中はオーバーレイがターミナル領域を完全に覆い、ターミナルは操作不可

### 画面動作
```
[ボタンクリック]
  → オーバーレイが main-content 領域を覆う (position: fixed)
  → SessionView はマウント維持 (ターミナル状態保持) だがオーバーレイの下
  → もう一度同じボタン or ✕ or Escape → ビュー閉じてターミナルに復帰
```

### レイアウト図
```
閉じた状態:                                     ┌ Strip(36px)
┌─ MenuBar ────────────────────────────────────┤ [File]
├─ Sidebar ──┬─ SessionView ───────────────────┤ [Git]
│ Sessions   │ Header / Tabs / TerminalPanes   │
│            │                                 │
├────────────┴─ StatusBar ─────────────────────┤
└──────────────────────────────────────────────┘

File Tree ビュー表示時:                         ┌ Strip
┌─ MenuBar ────────────────────────────────────┤ [File]*
├─ Sidebar ──┬─ ViewOverlay ───────────────────┤ [Git]
│ Sessions   │ ┌ Header "File Tree" [↻][✕] ──┐│
│            │ │ TreeSidebar │ FileContent     ││
│            │ │ (280px)     │ (flex:1)        ││
│            │ └─────────────┴────────────────┘│
└────────────┴─────────────────────────────────┘

Git Graph ビュー表示時:                         ┌ Strip
┌─ MenuBar ────────────────────────────────────┤ [File]
├─ Sidebar ──┬─ ViewOverlay ───────────────────┤ [Git]*
│ Sessions   │ ┌ Header "Git" [branch▼][↻][✕]┐│
│            │ │ StatusBar (branch/ahead/behind)│
│            │ │ SVG Graph + CommitList (上)   ││
│            │ │ ──── divider ────            ││
│            │ │ CommitDetail + Diff (下)     ││
│            │ └──────────────────────────────┘│
└────────────┴─────────────────────────────────┘
```

---

## Phase 1: Viewer System 基盤

### 1-1. ViewPlugin レジストリ

**ファイル**: `frontend/src/components/viewer/viewerRegistry.ts`

```typescript
export interface ViewPlugin {
  id: string;
  icon: ComponentType<{ size?: number }>;
  label: string;
  component: ComponentType;
  shortcut?: string;
}

const registry: ViewPlugin[] = [];
export function registerView(plugin: ViewPlugin): void { ... }
export function getRegisteredViews(): readonly ViewPlugin[] { ... }
```

各ビューは `index.ts` で自己登録 (side-effect import):
```typescript
// views/file-tree/index.ts
registerView({ id: "file-tree", icon: FileTreeIcon, ... });
```

### 1-2. Zustand Store

**ファイル**: `frontend/src/components/viewer/viewerStore.ts`

```typescript
interface ViewerState {
  activeViewId: string | null;
  toggleView: (viewId: string) => void;
  closeView: () => void;
}
```

### 1-3. コンポーネント

| ファイル | 役割 |
|---------|------|
| `viewer/index.ts` | ViewerSystem のみ export |
| `viewer/ViewerSystem.tsx` | コンテナ: ストリップ + オーバーレイ + キーボードショートカット |
| `viewer/ActivityStrip.tsx` | 右端 36px アイコンバー (position: fixed) |
| `viewer/ViewOverlay.tsx` | 全画面オーバーレイ (position: fixed) |
| `viewer/icons/FileTreeIcon.tsx` | SVG アイコン (sample_button.png 参考) |
| `viewer/icons/GitGraphIcon.tsx` | SVG アイコン (sample_button.png 参考) |

### 1-4. App.tsx 変更 (最小限)

```tsx
import { ViewerSystem } from "./components/viewer";

<div className="app-body">
  <Sidebar ... />
  <main className="main-content">
    <SessionView ... />
    <StatusBar />
  </main>
  <ViewerSystem />   {/* ← この1行のみ追加 */}
</div>
```

### 1-5. CSS

**ファイル**: `frontend/src/styles/viewer.css`

- Activity Strip: `position: fixed; top: 0; right: 0; bottom: 0; width: 36px;`
- Strip ボタン: 28x28px, hover/active でアクセントカラー + 左端2px インジケーター
- View Overlay: `position: fixed; top: 0; left: 280px; right: 36px; bottom: 0; z-index: 40;`
- アニメーション: スライドイン 150ms ease
- `base.css` の `.main-content` に `margin-right: 36px` 追加

---

## Phase 2: File Tree ビュー

### 2-1. バックエンド API

**ファイル**: `myT-x/app_devpanel_api.go` (新規)

```go
// DevPanelListDir - ディレクトリ内容を遅延読み込みで返す
// (iori-editor の GetDirectoryChildren パターンに準拠)
func (a *App) DevPanelListDir(sessionName string, dirPath string) ([]FileEntry, error)

// DevPanelReadFile - ファイル内容を読み取り専用で返す
// (iori-editor の ReadFilePartial パターンに準拠、サイズ制限あり)
func (a *App) DevPanelReadFile(sessionName string, filePath string) (FileContent, error)
```

**型定義** (`myT-x/app_devpanel_types.go`):
```go
type FileEntry struct {
    Name  string `json:"name"`
    Path  string `json:"path"`    // root 相対パス
    IsDir bool   `json:"is_dir"`
    Size  int64  `json:"size"`    // ファイルサイズ (dir は 0)
}

type FileContent struct {
    Path      string `json:"path"`
    Content   string `json:"content"`
    LineCount int    `json:"line_count"`
    Size      int64  `json:"size"`
    Truncated bool   `json:"truncated"` // 1MB超過
    Binary    bool   `json:"binary"`    // バイナリ検出
}
```

**実装詳細**:
- パス解決: セッション名 → `sessions.GetRootPath()` or worktree path → 絶対パス結合
- セキュリティ: `filepath.Clean` + ルートパス外アクセス防止 (`app_worktree_api.go` のパターン再利用)
- シンボリックリンク: 解決後に再検証
- ファイルサイズ上限: 1MB / バイナリ検出: 先頭8KBにNULLバイト
- ディレクトリエントリ上限: 5000件
- 除外パターン: `.git/`, `node_modules/` (ハードコード、将来config化可)
- ソート: ディレクトリ優先 → アルファベット順

### 2-2. フロントエンド コンポーネント

**iori-editor パターンを踏襲**: `flattenTree` + `react-window` FixedSizeList

| ファイル | 役割 |
|---------|------|
| `views/file-tree/index.ts` | registerView() 自己登録 |
| `views/file-tree/FileTreeView.tsx` | ルート: ヘッダー + 左右分割 (ツリー280px / コンテンツflex:1) |
| `views/file-tree/FileTreeSidebar.tsx` | FixedSizeList による仮想化ツリー表示 |
| `views/file-tree/TreeNodeRow.tsx` | 行コンポーネント (インデント + アイコン + 名前) |
| `views/file-tree/FileContentViewer.tsx` | `<pre>` + 行番号の読み取り専用表示 |
| `views/file-tree/useFileTree.ts` | データ取得フック (expandedPaths, cache 管理) |
| `views/file-tree/treeUtils.ts` | `flattenTree()` ユーティリティ (iori-editor参考) |
| `views/file-tree/fileTreeTypes.ts` | 型定義 |

**ツリー仮想化パターン** (iori-editor準拠):
```typescript
// treeUtils.ts
interface FlatNode {
  path: string;
  name: string;
  isDir: boolean;
  depth: number;    // インデントレベル
  isExpanded: boolean;
  isLoading: boolean;
}

function flattenTree(
  entries: FileEntry[],
  expandedPaths: Set<string>,
  childrenCache: Map<string, FileEntry[]>,
  loadingPaths: Set<string>,
  depth: number = 0,
): FlatNode[]
```

- `FixedSizeList` で行高 28px の仮想化リスト
- フォルダクリック → `expandedPaths` toggle + 未ロードなら `DevPanelListDir` 呼出
- ファイルクリック → `DevPanelReadFile` → 右側プレビューに表示

**File Tree ビューレイアウト**:
```
┌─ ヘッダー "File Tree"                              [↻][✕] ┐
├──────────────┬─────────────────────────────────────────────┤
│ ツリー(280px)│ ファイルプレビュー (flex:1)                   │
│              │                                             │
│ ▶ src/       │ File: App.tsx (2.3 KB)                      │
│   ▼ comp/    │ ───────────────────────────                 │
│     App.tsx◀ │  1 │ import { useEffect } from "react";     │
│     Menu.tsx │  2 │ import "@xterm/xterm/css/xterm.css";    │
│   ▶ styles/  │  3 │ ...                                    │
│ go.mod       │                                             │
└──────────────┴─────────────────────────────────────────────┘
```

---

## Phase 3: Git Graph ビュー

### 3-1. バックエンド API

**ファイル**: `myT-x/app_devpanel_api.go` (追加)

```go
// DevPanelGitLog - コミット履歴を親ハッシュ付きで返す (グラフ描画用)
func (a *App) DevPanelGitLog(sessionName string, maxCount int, allBranches bool) ([]GitGraphCommit, error)

// DevPanelGitStatus - ワーキングツリー状態
func (a *App) DevPanelGitStatus(sessionName string) (GitStatusResult, error)

// DevPanelCommitDiff - コミットの unified diff
func (a *App) DevPanelCommitDiff(sessionName string, commitHash string) (string, error)

// DevPanelListBranches - グラフフィルタ用ブランチ一覧
func (a *App) DevPanelListBranches(sessionName string) ([]string, error)
```

**型定義** (`myT-x/app_devpanel_types.go`):
```go
type GitGraphCommit struct {
    Hash        string   `json:"hash"`          // 短縮7文字
    FullHash    string   `json:"full_hash"`
    Parents     []string `json:"parents"`       // 親ハッシュ (グラフ描画用)
    Subject     string   `json:"subject"`
    AuthorName  string   `json:"author_name"`
    AuthorDate  string   `json:"author_date"`   // ISO 8601
    Refs        []string `json:"refs"`          // ブランチ/タグ名
}

type GitStatusResult struct {
    Branch    string   `json:"branch"`
    Modified  []string `json:"modified"`
    Staged    []string `json:"staged"`
    Untracked []string `json:"untracked"`
    Ahead     int      `json:"ahead"`
    Behind    int      `json:"behind"`
}
```

**Git CLI コマンド**:
- `git log --format="%H%x00%h%x00%P%x00%s%x00%an%x00%aI%x00%D" -n M [--all]`
- `git status --porcelain -b` + `git rev-list --left-right --count @{u}...HEAD`
- `git diff-tree -p <hash>` (diff取得、500KB上限)
- `git branch --format="%(refname:short)" -a` (ブランチ一覧)

**制限**: maxCount最大1000, commitHash は `ValidateCommitish()` で検証。
全操作は既存 `runGitCLI` セマフォで同時実行数制限。

### 3-2. レーン計算アルゴリズム

**ファイル**: `views/git-graph/laneComputation.ts` (純粋関数)

```typescript
interface LaneAssignment {
  commitHash: string;
  lane: number;                    // 列番号 (0起点)
  connections: ParentConnection[]; // 親への接続線
  activeLaneCount: number;         // その行のアクティブレーン数
}

interface ParentConnection {
  fromLane: number;
  toLane: number;
  type: "straight" | "merge-left" | "merge-right";
}

function computeLanes(commits: GitGraphCommit[]): LaneAssignment[]
```

**アルゴリズム**:
1. `activeLanes: string[]` で各レーンの待機中コミットハッシュを管理
2. 各コミットを git log 順 (新しい順) に処理
3. コミットの位置: `activeLanes` 内で自分のハッシュを探す → その index がレーン番号
4. 第1親 → 同レーン引き継ぎ / 第2親以降 → 別レーン割当 (マージ線描画)
5. 親なし → レーン解放、末尾null削除でレーン数コンパクト化

### 3-3. フロントエンド コンポーネント

| ファイル | 役割 |
|---------|------|
| `views/git-graph/index.ts` | registerView() 自己登録 |
| `views/git-graph/GitGraphView.tsx` | ルート: ヘッダー + 上下分割 |
| `views/git-graph/BranchStatusBar.tsx` | ブランチ + ahead/behind + 変更ファイル数 |
| `views/git-graph/CommitGraph.tsx` | SVGグラフ + コミットリスト (react-window FixedSizeList) |
| `views/git-graph/CommitRow.tsx` | 行コンポーネント: SVGセグメント + コミット情報 |
| `views/git-graph/DiffViewer.tsx` | `<pre>` ベース unified diff 色分け表示 |
| `views/git-graph/laneComputation.ts` | DAGレーン計算 |
| `views/git-graph/useGitGraph.ts` | データ取得フック |
| `views/git-graph/gitGraphTypes.ts` | 型定義 |

**SVGグラフ描画** (行ごとに小さな SVG):
- 行高: 32px, SVG幅 = `activeLaneCount * 20px`
- コミットドット: 通常=塗り円(r=4), マージ(parents>=2)=二重円
- レーン縦線: 各アクティブレーンに色付き縦線
- マージ線: bezier curve で別レーンからの接続を描画
- レーン色 (6色ローテーション): `["#f6d365","#3de4b7","#5492ff","#ff6b6b","#c084fc","#f97316"]`

**Diff表示** (Monaco不使用、軽量`<pre>`ベース):
- unified diff形式をパース → ファイルセクション分割
- 追加行 (+): `rgba(61, 228, 183, 0.15)` 背景
- 削除行 (-): `var(--danger-08)` 背景
- ハンクヘッダー (@@): `var(--accent-10)` 背景
- 行番号 (旧/新) 表示
- ファイルヘッダー: 折り畳み可能

**Git Graph ビューレイアウト**:
```
┌─ ヘッダー "Git Graph"  [branch: dev-v7 ▼]          [↻][✕] ┐
├─ ステータス: main ↑2↓1 [3変更] [1ステージ済] ────────────────┤
├──────────────────────────────────────────────────────────────┤
│ GRAPH   │ HASH    │ MESSAGE              │ AUTHOR  │ DATE   │
│─────────┼─────────┼──────────────────────┼─────────┼────────│
│ ●─┐     │ 7d03f65 │ Merge PR #11         │ user    │ 2h     │
│ │ ●     │ 0c49289 │ v0.0.6               │ user    │ 3h     │
│ │ ●     │ 1422426 │ 速度改善              │ user    │ 4h     │
│ ●─┘     │ c82d2d1 │ Merge PR #10         │ user    │ 1d     │
│ [さらに読み込む...]                                           │
├──────────────────────────────────────────────────────────────┤
│ Commit: 7d03f65                                              │
│ Author: user <user@email.com>                                │
│ Merge pull request #11 from my-take-dev/dev-v6               │
│──────────────────────────────────────────────────────────────│
│ diff --git a/myT-x/app.go b/myT-x/app.go                    │
│ @@ -10,6 +10,7 @@                                            │
│  import (                                                     │
│ +    "sync/atomic"                                            │
│  )                                                            │
└──────────────────────────────────────────────────────────────┘
```

---

## Phase 4: 仕上げ

- キーボードショートカット: `Ctrl+Shift+E` (FileTree), `Ctrl+Shift+G` (GitGraph), `Escape` (閉じる)
- セッション切替時: ビュー内キャッシュクリア (tmuxStore.activeSession subscribe)
- エッジケース: セッションなし→Strip非表示, root_pathなし→メッセージ, Git非対応→メッセージ
- バイナリ/大容量ファイル→適切なメッセージ表示

---

## ファイル構成

### 新規 Go バックエンド
| ファイル | 内容 |
|---------|------|
| `myT-x/app_devpanel_api.go` | DevPanelListDir, DevPanelReadFile, DevPanelGitLog, DevPanelGitStatus, DevPanelCommitDiff, DevPanelListBranches |
| `myT-x/app_devpanel_types.go` | FileEntry, FileContent, GitGraphCommit, GitStatusResult 型定義 |
| `myT-x/app_devpanel_api_test.go` | テスト |

### 新規フロントエンド (`frontend/src/components/viewer/`)
| ファイル | 内容 |
|---------|------|
| `index.ts` | ViewerSystem のみ export |
| `ViewerSystem.tsx` | コンテナ (ストリップ + オーバーレイ + ショートカット) |
| `ActivityStrip.tsx` | 右端アイコンバー |
| `ViewOverlay.tsx` | 全画面オーバーレイ |
| `viewerStore.ts` | Zustand (activeViewId) |
| `viewerRegistry.ts` | ViewPlugin レジストリ |
| `icons/FileTreeIcon.tsx` | SVGアイコン |
| `icons/GitGraphIcon.tsx` | SVGアイコン |
| **File Tree ビュー** | |
| `views/file-tree/index.ts` | 自己登録 |
| `views/file-tree/FileTreeView.tsx` | ルート (左右分割) |
| `views/file-tree/FileTreeSidebar.tsx` | react-window 仮想化ツリー |
| `views/file-tree/TreeNodeRow.tsx` | ツリー行コンポーネント |
| `views/file-tree/FileContentViewer.tsx` | ファイル内容表示 |
| `views/file-tree/useFileTree.ts` | データ取得フック |
| `views/file-tree/treeUtils.ts` | flattenTree ユーティリティ |
| `views/file-tree/fileTreeTypes.ts` | 型 |
| **Git Graph ビュー** | |
| `views/git-graph/index.ts` | 自己登録 |
| `views/git-graph/GitGraphView.tsx` | ルート (上下分割) |
| `views/git-graph/BranchStatusBar.tsx` | ブランチステータス |
| `views/git-graph/CommitGraph.tsx` | SVG + コミットリスト (react-window) |
| `views/git-graph/CommitRow.tsx` | コミット行コンポーネント |
| `views/git-graph/DiffViewer.tsx` | `<pre>` ベース diff 色分け |
| `views/git-graph/laneComputation.ts` | レーン計算 |
| `views/git-graph/useGitGraph.ts` | データ取得フック |
| `views/git-graph/gitGraphTypes.ts` | 型 |

### 新規 CSS
| ファイル | 内容 |
|---------|------|
| `frontend/src/styles/viewer.css` | Activity Strip + Overlay + FileTree + GitGraph 全CSS |

### 既存ファイル変更 (最小)
| ファイル | 変更内容 |
|---------|---------|
| `myT-x/frontend/src/App.tsx` | `import { ViewerSystem }` + JSX 1行追加 |
| `myT-x/frontend/src/style.css` | `@import "./styles/viewer.css"` 追加 |
| `myT-x/frontend/src/styles/base.css` | `.main-content` に `margin-right: 36px` 追加 |
| `myT-x/frontend/src/api.ts` | DevPanel系API関数の追加 |

---

## 再利用する既存コード

| パターン | ファイル | 用途 |
|---------|---------|------|
| パストラバーサル防止 | `myT-x/app_worktree_api.go` | ListDir/ReadFile のパス検証 |
| `requireSessions()` | `myT-x/app_guards.go` | 全APIの前提条件チェック |
| `runGitCLI` + セマフォ | `myT-x/internal/git/command.go` | Git CLI 同時実行数制限 |
| `ValidateCommitish` | `myT-x/internal/git/validation.go` | commitHash 検証 |
| `react-window` | `package.json` (既存依存) | ツリー/コミットリスト仮想化 |
| `worktree-branch-badge` CSS | `styles/base.css` | ref バッジスタイル |
| Zustand パターン | `stores/tmuxStore.ts` | viewerStore 構造参考 |
| flattenTree パターン | `sample/iori-editor` | ツリー → フラットリスト変換参考 |
| FileExplorer パターン | `sample/iori-editor` | 遅延読み込みツリー設計参考 |

---

## 検証方法

### ビルド
```bash
cd myT-x && go build ./...
cd myT-x/frontend && npm run build
```

### テスト
```bash
cd myT-x && go test ./... -v
cd myT-x/frontend && npm test
```

### 手動確認
1. アプリ起動 → 右端に小さなActivity Strip (File/Gitアイコン) 表示
2. Fileアイコンクリック → 全画面にFile Treeビュー表示 (左: ツリー280px, 右: プレビュー)
3. フォルダ展開 → 子要素が遅延読み込みで表示 (react-window 仮想化)
4. ファイルクリック → 右側にファイル内容が行番号付きで表示
5. もう一度Fileアイコン or ✕ or Escape → ビュー閉じてターミナルに復帰
6. Gitアイコンクリック → 全画面にGit Graphビュー表示
7. ブランチグラフがSVGで色分け描画、コミット一覧が仮想スクロール表示
8. コミットクリック → 下部にコミット詳細 + diff が色分け表示
9. `Ctrl+Shift+E` / `Ctrl+Shift+G` でビュー切替
10. セッション切替 → ビュー内データがリセット・再取得

### 開発フロー
```
confidence-check → golang-expert (Go API) → go-test-generator (テスト) →
xterm-react-frontend (React) → self-review → 全クリアまで修正
```
