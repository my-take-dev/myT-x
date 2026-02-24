# 右サイドバーショートカット設定機能 ─ 実装計画

## Context

右サイドバー（ViewerSystem）の5つのビューのキーボードショートカットがハードコードされている。ユーザーが設定画面からカスタマイズし、`config.yaml` に永続化できるようにする。既存のキーバインド設定タブ内にセクションを追加し、UIパターンを統一する。

### 対象ビュー（5つ）

| View ID | Label | デフォルト | 登録ファイル |
|---|---|---|---|
| `file-tree` | File Tree | `Ctrl+Shift+E` | `views/file-tree/index.ts` |
| `git-graph` | Git Graph | `Ctrl+Shift+G` | `views/git-graph/index.ts` |
| `error-log` | Error Log | `Ctrl+Shift+L` | `views/error-log/index.ts` |
| `diff` | Diff | `Ctrl+Shift+D` | `views/diff-view/index.ts` |
| `input-history` | Input History | `Ctrl+Shift+H` | `views/input-history/index.ts` |

### 設計方針

- **レジストリは不変**: `viewerRegistry` のショートカットはデフォルト値として保持。変更しない
- **ランタイム上書き（方式A）**: `ViewerSystem` が `shortcutMap` 構築時に設定値で上書き
- **空文字 = デフォルト使用**: 設定値が空ならレジストリのデフォルトにフォールバック
- **tmuxキーとの相互バリデーション不要**: プレフィックス+キー vs グローバルCtrl+Shift+Xで領域が異なる

---

## 実装ステップ

### Step 1: Go バックエンド — Config構造体拡張

**ファイル**: `myT-x/internal/config/config.go`

1. `Config` 構造体に `ViewerShortcuts map[string]string` フィールド追加（`yaml:"viewer_shortcuts,omitempty" json:"viewer_shortcuts,omitempty"`）
2. `Clone` 関数に `ViewerShortcuts` のディープコピー追加（`PaneEnv` と同じパターン）
3. `DefaultConfig()` には追加しない（nil = フロントエンドのレジストリデフォルトを使用）
4. `applyDefaultsAndValidate` の変更不要（opaque string mapとして扱う）

### Step 2: フロントエンド型定義

**ファイル**: `myT-x/frontend/src/components/settings/types.ts`

1. `FormState` に `viewerShortcuts: Record<string, string>` 追加
2. `FormAction` に `{ type: "UPDATE_VIEWER_SHORTCUT"; viewId: string; value: string }` 追加

**ファイル**: `myT-x/frontend/src/types/tmux.ts`

3. `AppConfig` に `viewer_shortcuts?: Record<string, string>` 追加

### Step 3: Reducer拡張

**ファイル**: `myT-x/frontend/src/components/settings/settingsReducer.ts`

1. `INITIAL_FORM` に `viewerShortcuts: {}` 追加
2. `LOAD_CONFIG` に `viewerShortcuts: cfg.viewer_shortcuts ?? {}` 追加
3. `UPDATE_VIEWER_SHORTCUT` ケース追加（スプレッドで該当viewIdのみ更新）

### Step 4: 設定UIコンポーネント作成

**新規ファイル**: `myT-x/frontend/src/components/settings/ViewerShortcutSettings.tsx`

- 5つのビューをリスト表示、既存の `ShortcutInput` コンポーネントを再利用
- `placeholder` にデフォルトショートカットを表示
- バリデーションエラー表示対応

### Step 5: KeybindSettings.tsxに統合

**ファイル**: `myT-x/frontend/src/components/settings/KeybindSettings.tsx`

- 既存tmuxキーバインドセクションの下に `<ViewerShortcutSettings>` を埋め込み

### Step 6: バリデーション追加

**ファイル**: `myT-x/frontend/src/components/settings/settingsValidation.ts`

- `validateViewerShortcuts()` 関数追加
  - 重複ショートカット検出（大文字小文字無視）
  - `Ctrl+Shift+X` 形式チェック
  - 空文字はバリデーションスキップ（デフォルト使用を意味）

### Step 7: 保存ロジック拡張

**ファイル**: `myT-x/frontend/src/components/SettingsModal.tsx`

1. `handleSave` のバリデーションに `validateViewerShortcuts` 追加
2. バリデーションエラー時にキーバインドタブに自動遷移
3. payload に `viewer_shortcuts` 追加（空エントリはフィルター、全空なら `undefined`）

### Step 8: ViewerSystem — 設定駆動のショートカットマップ

**ファイル**: `myT-x/frontend/src/components/viewer/ViewerSystem.tsx`

1. Zustandストアから `config?.viewer_shortcuts` を購読
2. `shortcutMap` の `useMemo` を拡張: 設定値があればレジストリデフォルトを上書き

```
effectiveShortcut = configShortcut || view.shortcut
map.set(effectiveShortcut.toLowerCase(), view.id)
```

### Step 9: ActivityStrip — ツールチップ更新

**ファイル**: `myT-x/frontend/src/components/viewer/ActivityStrip.tsx`

1. Zustandストアから `config?.viewer_shortcuts` を購読
2. ツールチップの `title` にカスタムショートカットを優先表示

### Step 10: config.yaml更新

**ファイル**: `myT-x/config.yaml`

- コメントブロックで `viewer_shortcuts` の使用例を末尾に追記（`omitempty` のため実値は書かない）

### Step 11: Wails自動生成ファイル更新

**ファイル**: `myT-x/frontend/wailsjs/go/models.ts`, `myT-x/frontend/wailsjs/go/main/App.d.ts`

- `wails generate` 実行、または手動で `viewer_shortcuts` フィールド追加

---

## テスト計画

### Go テスト（`myT-x/internal/config/config_test.go`）

| テスト | 内容 |
|---|---|
| `TestLoadViewerShortcuts` | YAML読み込み: 未定義→nil、値あり→正しいmap、空map→nil |
| `TestCloneViewerShortcuts` | nil→nil、値あり→ディープコピー独立性 |
| `TestSaveRoundTripViewerShortcuts` | Save→Load往復で値が保持されること |
| フィールド数ガード | `Config` のフィールド数が12であること（既存11+1） |

### フロントエンドテスト

| テスト | 内容 |
|---|---|
| `validateViewerShortcuts` | 空map→エラーなし、重複→エラー、大文字小文字無視、不正形式→エラー |
| Reducer `UPDATE_VIEWER_SHORTCUT` | 該当viewIdのみ更新、他は不変 |
| Reducer `LOAD_CONFIG` | `viewer_shortcuts` あり→viewerShortcutsに反映、なし→空オブジェクト |

### 手動確認

1. 設定画面キーバインドタブにビューアーショートカットセクション表示
2. ショートカット変更→保存→`config.yaml` に `viewer_shortcuts` 反映
3. 変更したショートカットでビューがトグルされる
4. ActivityStripツールチップに変更後のショートカット表示
5. 重複ショートカットでバリデーションエラー
6. アプリ再起動後もカスタムショートカット保持

---

## 変更ファイル一覧

| ファイル | 種別 | 内容 |
|---|---|---|
| `myT-x/internal/config/config.go` | 変更 | Config構造体 + Clone |
| `myT-x/internal/config/config_test.go` | 変更 | テスト追加 |
| `myT-x/frontend/src/components/settings/types.ts` | 変更 | FormState + FormAction |
| `myT-x/frontend/src/types/tmux.ts` | 変更 | AppConfig型 |
| `myT-x/frontend/src/components/settings/settingsReducer.ts` | 変更 | Reducer拡張 |
| `myT-x/frontend/src/components/settings/ViewerShortcutSettings.tsx` | **新規** | 設定UIコンポーネント |
| `myT-x/frontend/src/components/settings/KeybindSettings.tsx` | 変更 | 統合 |
| `myT-x/frontend/src/components/settings/settingsValidation.ts` | 変更 | バリデーション追加 |
| `myT-x/frontend/src/components/SettingsModal.tsx` | 変更 | 保存payload + バリデーション |
| `myT-x/frontend/src/components/viewer/ViewerSystem.tsx` | 変更 | shortcutMap上書き |
| `myT-x/frontend/src/components/viewer/ActivityStrip.tsx` | 変更 | ツールチップ |
| `myT-x/config.yaml` | 変更 | コメント追記 |
| `myT-x/frontend/wailsjs/go/models.ts` | 変更 | 自動生成/手動 |

## 開発フロー

```
confidence-check → Step 1-11 実装 → go fix ./... → テスト作成 → self-review
```
