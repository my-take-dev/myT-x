# Diff View: Untracked Files in Subdirectories Fix

## Context

サンプル画像 (`sample/diff_image.png`) では、フォルダ階層を保持したファイル単位のdiff表示がされている。
現在の実装には以下の問題がある:

1. **`git diff HEAD` 失敗時に untracked files 収集に到達しない** — 初回コミット前リポジトリでは `git diff HEAD` がエラーとなり、メソッドが即座にエラーを返すため、untracked files が一切表示されない
2. **`git status --porcelain -uall` の信頼性** — `-uall` は個別ファイルを返すはずだが、gitバージョンや設定による差異の可能性がある。`git ls-files --others --exclude-standard` をより確実な代替手段として検討

## Fix Plan

### Step 1: Backend — `git diff HEAD` 失敗を graceful に処理

**File:** `myT-x/app_devpanel_api.go` (`DevPanelWorkingDiff`)

現在:
```go
output, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"diff", "HEAD"})
if gitErr != nil {
    return WorkingDiffResult{}, fmt.Errorf("git diff HEAD failed: %w", gitErr)
}
```

修正: `git diff HEAD` 失敗時にエラーで返さず、tracked diff を空として続行し untracked files 収集を行う:
```go
output, gitErr := gitpkg.RunGitCLIPublic(workDir, []string{"diff", "HEAD"})
if gitErr != nil {
    // HEAD が存在しない場合（初回コミット前）は tracked diff を空として続行。
    slog.Debug("[DEBUG-DEVPANEL] git diff HEAD failed, continuing with untracked only", "error", gitErr)
    output = nil
}
```

### Step 2: Backend — untracked files 検出を `git ls-files` に変更

**File:** `myT-x/app_devpanel_api.go` (`collectUntrackedFiles`)

`git ls-files --others --exclude-standard` は `-uall` より信頼性が高い:
- 常に個別ファイルパスを返す（ディレクトリエントリを返さない）
- gitバージョン間で挙動が安定している
- `.gitignore` を正しく適用する

```go
func collectUntrackedFiles(workDir string) []string {
    output, err := gitpkg.RunGitCLIPublic(workDir, []string{
        "ls-files", "--others", "--exclude-standard",
    })
    // ... パース (行分割のみ、?? prefix パース不要)
}
```

### Step 3: Backend — テスト更新

**File:** `myT-x/app_devpanel_api_test.go`

- `TestCollectUntrackedFiles` は不要 (git コマンド依存のため integration test 扱い)
- 既存の `TestBuildUntrackedFileDiff` テストはそのまま維持

### Step 4: TypeScript — 変更なし

フロントエンドの `buildDiffTree` ロジックは正しく動作する:
- 全ディレクトリが初回ロード時に auto-expand される
- React 18 のバッチ更新で `diffResult` と `expandedDirs` が同一レンダリングで適用される
- ネストされたパス（例: `app/boards/[boardId]/pages.tsx`）の中間ディレクトリも正しく生成される

## Critical Files

| File | Action |
|------|--------|
| `myT-x/app_devpanel_api.go` | `DevPanelWorkingDiff`: `git diff HEAD` 失敗を graceful 処理、`collectUntrackedFiles`: `git ls-files` に変更 |

## Verification

1. `go test -run "TestBuildUntrackedFileDiff\|TestParseWorkingDiff\|TestWorkingDiff" -v -count=1 .` — 全テスト通過
2. `go build ./...` — コンパイル成功
3. `npx tsc --noEmit` (frontend/) — TypeScript エラーなし
4. アプリ起動 → git リポジトリのあるセッションで:
   - 追跡済み変更ファイルがフォルダ階層付きで表示
   - 新規追加（untracked）ファイルもフォルダ内のものを含めて表示
   - 初回コミット前リポジトリでも untracked files が表示される
