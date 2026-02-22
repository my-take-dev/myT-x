# レガシーShimパス問題の修正

## Context

myT-xでエージェントチームを起動してもshimログが出ず、モデル変換設定(`from: ALL, to: haiku`)も動作していない。

### 根本原因

Windows PATH上に**2つの`tmux.exe`**が存在し、古い方が先に見つかっている:

| 場所 | バイナリ日付 | サイズ | PATH順 | 状態 |
|------|------------|--------|--------|------|
| `%LOCALAPPDATA%\github.com\my-take-dev\myT-x\bin\` | 2/14 | 5,288,448B | **先** | 旧版・from/to空 |
| `%LOCALAPPDATA%\myT-x\bin\` | 2/18 | 5,297,152B | 後 | 最新版・未使用 |

**証拠:**
- 旧ディレクトリに`shim-debug.log`(579KB, 本日19:28更新)が存在 → 旧shimが呼ばれている
- 旧`config.yaml`の`agent_model.from: ""`/`to: ""` → モデル変換なし
- 新ディレクトリにログなし → 新shimは呼ばれていない

**追加問題:** テスト実行時のPATH汚染(`TestEnsureShimInstalled_*`のtemp dir が多数残存)

## 修正方針

起動時にレガシーディレクトリのPATHエントリとファイルをクリーンアップする。

---

## 実装ステップ

### Step 1: `myT-x/internal/install/shim_cleanup_windows.go` を新規作成

レガシークリーンアップ専用ファイル。

**実装する関数:**

1. **`CleanupLegacyShimInstalls() error`** — メインエントリポイント
   - `%LOCALAPPDATA%`を取得
   - レガシーパス `github.com\my-take-dev\myT-x\bin` を構築
   - `removeLegacyPathEntry(legacyDir)` でレジストリPATHから削除
   - `removeLegacyDirectory()` でファイル・ディレクトリ削除
   - `cleanupStalePathEntries()` でテストtemp dirをPATHから除去
   - エラーは全てログのみ、startup阻害しない(常にnil返却)

2. **`removeLegacyPathEntry(legacyDir string) bool`**
   - `ensurePathMu`ロック取得(既存mutexを共有)
   - `registry.CreateKey`で`HKCU\Environment`を開く
   - `readUserPathFromRegistryKeyWithType(key)`でPATH読取
   - `containsPathEntry`で存在確認
   - 該当エントリをフィルタして`setPathRegistryValue`で書戻し
   - `broadcastEnvironmentSettingChange()`で通知

3. **`removeLegacyDirectory(parentDir string)`**
   - 既知ファイル(`tmux.exe`, `tmux.exe.sha256`, `config.yaml`, `shim-debug.log`, `shim-debug-*.log`)を個別削除
   - `bin`ディレクトリ→親ディレクトリを空なら順に削除(`%LOCALAPPDATA%`直下まで)
   - `os.RemoveAll`は使わない(防御的コーディング)

4. **`cleanupStalePathEntries()`**
   - PATHから`\Temp\TestEnsureShim`を含むエントリを除去
   - 変更あればレジストリ書戻し+broadcast

5. **`removeProcessPathEntry(dir string) bool`**
   - プロセスレベルPATH(`os.Getenv("PATH")`)からレガシーエントリを除去
   - 子プロセスがレガシーshimを見つけないようにする

**再利用する既存関数** (`shim_path_windows.go`):
- `ensurePathMu` — mutex
- `containsPathEntry()` — PATH検索
- `readUserPathFromRegistryKeyWithType()` — レジストリ読取
- `setPathRegistryValue()` — レジストリ書込
- `selectPathRegistryValueType()` — 型判定
- `broadcastEnvironmentSettingChange()` — WM_SETTINGCHANGE通知
- `countPathEntries()` — ログ用

### Step 2: `myT-x/internal/install/shim_cleanup_other.go` を新規作成

```go
//go:build !windows

package install

func CleanupLegacyShimInstalls() error { return nil }
```

### Step 3: `myT-x/app_lifecycle.go` を修正

`ensureShimReady`の**先頭**でクリーンアップを呼び出す:

```go
var cleanupLegacyShimInstallsFn = install.CleanupLegacyShimInstalls

func (a *App) ensureShimReady(workspace string) {
    // レガシーPATHエントリを先に除去し、NeedsShimInstallが正しい状態を見るようにする
    if err := cleanupLegacyShimInstallsFn(); err != nil {
        slog.Warn("[shim] legacy cleanup failed", "error", err)
    }

    needsInstallBefore, err := needsShimInstallFn()
    // ... 以下変更なし
}
```

### Step 4: テスト作成

**`myT-x/internal/install/shim_cleanup_windows_test.go`:**

| テスト | 内容 |
|--------|------|
| `TestCleanupLegacyShimInstalls_RemovesLegacyDirectory` | tempにレガシー構造を作成→クリーンアップ→削除確認 |
| `TestCleanupLegacyShimInstalls_IdempotentWhenNoLegacy` | 存在しない場合→エラーなし |
| `TestRemoveLegacyPathEntry_RemovesMatchingEntry` | PATH文字列操作の単体テスト |
| `TestRemoveLegacyPathEntry_PreservesOtherEntries` | 他エントリ保持確認 |
| `TestCleanupStalePathEntries_RemovesTempTestDirs` | テストtemp dirの除去確認 |
| `TestRemoveLegacyDirectory_SkipsMissing` | 存在しないpathの安全性確認 |

**`myT-x/app_lifecycle_test.go` の更新:**
- `cleanupLegacyShimInstallsFn`を既存テストでno-opにモック
- `TestEnsureShimReadyCallsLegacyCleanup` 追加

### Step 5: self-review

CLAUDE.mdのワークフロー通り、self-reviewを実行。

---

## 重要な設計判断

1. **クリーンアップはinstall前** — `NeedsShimInstall`がレジストリPATHを読むため、先にレガシーエントリを除去
2. **同一mutex** — `ensurePathMu`でPATH操作を直列化(既存パターン踏襲)
3. **選択的ファイル削除** — `os.RemoveAll`ではなく既知ファイルのみ削除(防御的)
4. **関数変数パターン** — テスト容易性のため`cleanupLegacyShimInstallsFn`変数を使用(既存パターン)
5. **broadcast1回** — 全PATH変更後に1度だけWM_SETTINGCHANGE送信

---

## 検証手順

1. `go test ./internal/install/...` — 新規テスト通過確認
2. `go test ./...` — 全テスト(既存含む)通過確認
3. `make build` → `build/bin/myT-x.exe` 起動
4. `where tmux` → `%LOCALAPPDATA%\myT-x\bin\tmux.exe` のみ表示されること
5. `%LOCALAPPDATA%\myT-x\shim-debug.log` にログが出力されること
6. エージェントチーム起動 → モデル変換(haiku起動)を確認
7. 旧ディレクトリ `%LOCALAPPDATA%\github.com\` が削除されていること
