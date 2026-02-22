# Diff View: ディレクトリ階層修正 + Expand hidden lines 機能追加

## Context

Diff View で**新規作成フォルダ配下のファイルが非表示**になる問題を修正する。
根本原因: `buildUntrackedFileDiff()` が `info.IsDir()` のとき `nil` を返しディレクトリごとスキップしている。
`git ls-files --others --exclude-standard` がディレクトリパスを返すケース（Windows/一部git構成）で全ファイルが欠落する。

追加要件: サンプル画像 (`sample/diff_image.png`) に合わせて以下も実装する。
- サイドバーから絵文字アイコン除去（三角矢印のみ）
- hunk 間の「Expand N hidden lines」折りたたみ機能

---

## 修正対象ファイル

| # | ファイル | 変更内容 |
|---|---------|---------|
| 1 | `myT-x/app_devpanel_api.go` | Backend: ディレクトリ再帰展開 + パス正規化 |
| 2 | `myT-x/app_devpanel_api_test.go` | Backend: テスト追加 |
| 3 | `myT-x/frontend/src/components/viewer/views/diff-view/DiffFileSidebar.tsx` | Frontend: 絵文字アイコン除去 |
| 4 | `myT-x/frontend/src/components/viewer/views/diff-view/DiffContentViewer.tsx` | Frontend: Expand hidden lines 機能 |
| 5 | `myT-x/frontend/src/styles/viewer.css` | CSS: expand-bar スタイル追加 |

---

## Step 1: Backend — ディレクトリ再帰展開 (`app_devpanel_api.go`)

### 1a. `parseWorkingDiff()` に `\r\n` 正規化を追加

関数先頭で Windows 改行コードを正規化する。パス末尾に `\r` が混入する問題を防止。

```go
func parseWorkingDiff(raw string) []WorkingDiffFile {
    if strings.TrimSpace(raw) == "" {
        return nil
    }
    // Normalize Windows line endings for robust path extraction.
    raw = strings.ReplaceAll(raw, "\r\n", "\n")
    raw = strings.ReplaceAll(raw, "\r", "\n")
    // ... 以降既存ロジック
```

### 1b. `collectUntrackedFiles()` に末尾 `/` トリミング追加

`git ls-files` がディレクトリ末尾に `/` を付けるケースに対応:

```go
line = strings.TrimRight(line, "\r")
line = strings.TrimRight(line, "/\\")  // 追加: 末尾パス区切り除去
line = strings.TrimSpace(line)
```

### 1c. `buildUntrackedFileDiff()` をディレクトリ対応にリファクタ

既存の `buildUntrackedFileDiff` を `buildUntrackedFileDiffSingle` にリネームし、
新しい `buildUntrackedFileDiffs` (複数形) でディレクトリ再帰を処理する。

```go
// buildUntrackedFileDiffs は untracked パスに対する synthetic diff エントリを生成する。
// パスがディレクトリの場合は再帰的にウォークし、各ファイルのエントリを返す。
func buildUntrackedFileDiffs(workDir, relPath string) []WorkingDiffFile {
    absPath := filepath.Join(workDir, filepath.FromSlash(relPath))
    info, err := os.Stat(absPath)
    if err != nil {
        slog.Debug("[DEBUG-DEVPANEL] failed to stat untracked path", "path", relPath, "error", err)
        return nil
    }
    if !info.IsDir() {
        entry := buildUntrackedFileDiffSingle(workDir, relPath)
        if entry == nil {
            return nil
        }
        return []WorkingDiffFile{*entry}
    }
    // ディレクトリ: 再帰ウォークで個別ファイルを収集
    var results []WorkingDiffFile
    _ = filepath.WalkDir(absPath, func(path string, d fs.DirEntry, walkErr error) error {
        if walkErr != nil || d.IsDir() {
            return nil
        }
        // .git, node_modules 配下はスキップ
        rel, relErr := filepath.Rel(workDir, path)
        if relErr != nil {
            return nil
        }
        normalized := filepath.ToSlash(rel)
        entry := buildUntrackedFileDiffSingle(workDir, normalized)
        if entry != nil {
            results = append(results, *entry)
        }
        return nil
    })
    return results
}
```

### 1d. `DevPanelWorkingDiff()` の呼び出し箇所を更新

```go
// 旧: entry := buildUntrackedFileDiff(workDir, relPath)
// 新:
entries := buildUntrackedFileDiffs(workDir, relPath)
for _, entry := range entries {
    consumedSize += len(entry.Diff)
    files = append(files, entry)
}
```

既存の `buildUntrackedFileDiff` → `buildUntrackedFileDiffSingle` にリネーム。
`import "io/fs"` を追加（`fs.DirEntry` 使用のため）。

---

## Step 2: Backend — テスト追加 (`app_devpanel_api_test.go`)

### 2a. `TestParseWorkingDiff_WindowsLineEndings`
- `\r\n` 改行の diff 入力でパスに `\r` が混入しないことを検証

### 2b. `TestBuildUntrackedFileDiffs_Directory`
- tmpDir にサブディレクトリを作成し、その中にファイルを2つ配置
- `buildUntrackedFileDiffs(tmpDir, "subdir")` が 2 エントリを返すことを検証
- 各エントリの Path が `subdir/a.txt`, `subdir/b.txt` (forward slash) であることを検証

### 2c. `TestBuildUntrackedFileDiffs_SingleFile`
- 既存の単一ファイルケースが引き続き動作することを検証

### 2d. `TestCollectUntrackedFiles_TrailingSlash`
- 末尾 `/` がトリミングされることを検証（関数レベルテスト）

---

## Step 3: Frontend — 絵文字アイコン除去 (`DiffFileSidebar.tsx`)

`Row` コンポーネントから `tree-node-icon` の `<span>` を削除:

```tsx
// 削除:
// <span className="tree-node-icon">
//   {node.isDir ? "\uD83D\uDCC1" : "\uD83D\uDCC4"}
// </span>
```

サイドバーは三角矢印 (`▶` / 回転時 `▼`) + ファイル名のみの表示に変更。
※ `tree-node-icon` CSS クラスは file-tree ビューでも使用されているため CSS は削除しない。

---

## Step 4: Frontend — Expand hidden lines 機能 (`DiffContentViewer.tsx`)

### 4a. Gap 計算ロジック追加

`parseFileDiff` の戻り値にギャップ情報を含める:

```typescript
interface ParsedDiff {
    hunks: DiffHunk[];
    gaps: DiffGap[];  // hunk 間の非表示行数情報
}

interface DiffGap {
    afterHunkIndex: number;
    hiddenLineCount: number;
}
```

hunk 間のギャップを計算:
- 前の hunk の最終行番号と次の hunk の開始行番号の差分

### 4b. 展開状態管理

`expandedGaps` を `Set<number>` で管理（hunk index をキー）:

```typescript
const [expandedGaps, setExpandedGaps] = useState<Set<number>>(new Set());
```

### 4c. UI レンダリング

hunk 間にクリック可能なバーを挿入:

```tsx
{gap && !expandedGaps.has(hi) && (
    <div className="diff-expand-bar" onClick={() => toggleGap(hi)}>
        Expand {gap.hiddenLineCount} hidden lines
    </div>
)}
```

**NOTE**: 展開時は hunk ヘッダーのみ非表示にし、既存の context 行で代替する簡易実装とする。
（フルファイル取得による本格的な展開は後回し）

---

## Step 5: CSS — expand-bar スタイル (`viewer.css`)

```css
.diff-expand-bar {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 2px 12px;
    background: var(--accent-06);
    border-top: 1px solid var(--line);
    border-bottom: 1px solid var(--line);
    color: var(--fg-dim);
    font-size: 0.72rem;
    cursor: pointer;
    user-select: none;
    transition: background 0.15s;
}

.diff-expand-bar:hover {
    background: var(--accent-10);
    color: var(--fg-main);
}
```

---

## 実行順序

```
Step 1 (Backend修正) → Step 2 (テスト追加) → go test 実行で検証
    ↓ (並列可)
Step 3 (絵文字除去) + Step 4 (Expand機能) + Step 5 (CSS)
    ↓
wails dev で画面確認
```

## 検証方法

1. `go test ./myT-x/... -run TestParseWorkingDiff` — パス正規化テスト
2. `go test ./myT-x/... -run TestBuildUntrackedFileDiffs` — ディレクトリ再帰テスト
3. `wails dev` でアプリ起動 → 新規フォルダ+ファイルを含むセッションで Diff ビューを開く
4. サイドバーにディレクトリ階層が表示され、配下のファイルが選択・diff閲覧可能であること
5. hunk 間に「Expand N hidden lines」バーが表示され、クリックで折りたたみ切替可能であること
6. 絵文字アイコンが表示されないこと
