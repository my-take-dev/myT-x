# 右サイドバーショートカット設定機能 ─ 実装計画

## 1. 概要と目的

現在、右サイドバー（ViewerSystem）の各ビュー（File Tree, Git Graph, Error Log, Diff）のショートカットキーは、各ビューの登録ファイル（`views/*/index.ts`）にハードコードされている。ユーザーがこれらのショートカットをカスタマイズできるよう、設定画面に「右サイドバーショートカット」カテゴリを追加し、YAML設定ファイル（`config.yaml`）に永続化する機能を実装する。

### 現状のハードコードされたショートカット

| ビューID | ラベル | 現在のショートカット | 登録ファイル |
|---|---|---|---|
| `file-tree` | File Tree | `Ctrl+Shift+E` | `views/file-tree/index.ts` |
| `git-graph` | Git Graph | `Ctrl+Shift+G` | `views/git-graph/index.ts` |
| `error-log` | Error Log | `Ctrl+Shift+L` | `views/error-log/index.ts` |
| `diff` | Diff | `Ctrl+Shift+D` | `views/diff-view/index.ts` |

### 実現する価値
- ユーザーが自分の好みのキー組み合わせで右サイドバーを操作できる
- 他のアプリケーションやショートカットとの競合を回避できる
- 既存のキーバインド設定画面（`KeybindSettings`）のUIパターンを踏襲し、統一されたUXを提供

---

## 2. アーキテクチャ概要

### 設計方針
既存の`KeybindSettings`（tmuxプレフィックスキーバインド設定）と**同一のUIパターン**を踏襲する。ただし以下の違いがある：

| 要素 | 既存キーバインド（tmux） | 右サイドバーショートカット（新規） |
|---|---|---|
| **設定対象** | プレフィックスキー後の1キー | グローバルキーコンビネーション（Ctrl+Shift+X等） |
| **入力UI** | `ShortcutInput`（修飾キー+キー） | `ShortcutInput`（同一コンポーネント再利用） |
| **保存先（YAML）** | `keys:` セクション | `viewer_shortcuts:` セクション（新規） |
| **保存先（Go構造体）** | `Config.Keys map[string]string` | `Config.ViewerShortcuts map[string]string`（新規） |
| **消費者** | tmuxのキーバインド処理 | `ViewerSystem.tsx` のキーボードハンドラ |
| **FormState** | `keys: Record<string, string>` | `viewerShortcuts: Record<string, string>`（新規） |

### データフロー
```
config.yaml (viewer_shortcuts)
    ↓ 読み込み
Go Config struct (ViewerShortcuts)
    ↓ Wails API (GetConfig)
フロントエンド FormState (viewerShortcuts)
    ↓ 設定画面で編集
フロントエンド FormState 更新
    ↓ SaveConfig
Go Config struct → config.yaml に永続化
    ↓ app:config-updated イベント
ViewerSystem.tsx が再読み込み → viewerRegistry のショートカットを動的に上書き
```

---

## 3. 具体的な実装ステップ

### Step 1: Go バックエンド — `Config` 構造体の拡張

#### 1-1. `internal/config/config.go` の `Config` 構造体にフィールド追加

```go
type Config struct {
    // ... 既存フィールド ...

    // ViewerShortcuts は右サイドバーの各ビューに割り当てるキーボードショートカット。
    // キーはビューID（"file-tree", "git-graph", "error-log", "diff" 等）、
    // 値はショートカットキー（"Ctrl+Shift+E" 等）。
    ViewerShortcuts map[string]string `yaml:"viewer_shortcuts,omitempty" json:"viewer_shortcuts,omitempty"`
}
```

#### 1-2. デフォルト値の定義

設定ファイルに `viewer_shortcuts` が未定義の場合に使用するデフォルト値を定義する。
`config.go` の `DefaultConfig()` またはロード後のマージロジックに追加：

```go
var DefaultViewerShortcuts = map[string]string{
    "file-tree": "Ctrl+Shift+E",
    "git-graph": "Ctrl+Shift+G",
    "error-log": "Ctrl+Shift+L",
    "diff":      "Ctrl+Shift+D",
}
```

**設計ポイント:**
- `omitempty` を使用し、ユーザーが未設定の場合はYAMLに出力されない
- デフォルトを使っている場合（全てデフォルト値と一致する場合）もYAMLへの保存は許容する（明示的な設定として扱う）

#### 1-3. config.yaml 更新例

```yaml
viewer_shortcuts:
  file-tree: "Ctrl+Shift+E"
  git-graph: "Ctrl+Shift+G"
  error-log: "Ctrl+Shift+L"
  diff: "Ctrl+Shift+D"
```

---

### Step 2: フロントエンド型定義の拡張

#### 2-1. `types/tmux.ts` — `AppConfig` 型にフィールド追加

```typescript
export type AppConfig = AppConfigBase & {
    worktree: AppConfigWorktree;
    agent_model?: AppConfigAgentModel;
    pane_env?: Record<string, string>;
    pane_env_default_enabled?: boolean;
    claude_env?: AppConfigClaudeEnv;
    viewer_shortcuts?: Record<string, string>;  // ← 追加
};
```

※ `AppConfigBase` の `Pick<>` に `viewer_shortcuts` を含めるか、別途追加するかはWailsの自動生成との整合性で判断。

#### 2-2. `settings/types.ts` — `FormState` にフィールド追加

```typescript
export interface FormState {
    // ... 既存フィールド ...
    viewerShortcuts: Record<string, string>;  // ← 追加
}
```

#### 2-3. `settings/types.ts` — `FormAction` の `UPDATE_KEY` との棲み分け

既存の `UPDATE_KEY` アクションは tmuxキーバインド用の `keys` を更新するもの。
右サイドバーショートカット用には新しいアクション `UPDATE_VIEWER_SHORTCUT` を追加する：

```typescript
export type FormAction =
    | /* ... 既存 ... */
    | { type: "UPDATE_VIEWER_SHORTCUT"; key: string; value: string };
```

---

### Step 3: Reducer の拡張 (`settingsReducer.ts`)

#### 3-1. `INITIAL_FORM` にデフォルト値を追加

```typescript
export const INITIAL_FORM: FormState = {
    // ... 既存 ...
    viewerShortcuts: {},
};
```

#### 3-2. `LOAD_CONFIG` アクションでの読み込み

```typescript
case "LOAD_CONFIG": {
    // ... 既存の処理 ...
    return {
        ...state,
        // ... 既存 ...
        viewerShortcuts: cfg.viewer_shortcuts || {},
    };
}
```

#### 3-3. `UPDATE_VIEWER_SHORTCUT` アクションの追加

```typescript
case "UPDATE_VIEWER_SHORTCUT":
    return {
        ...state,
        viewerShortcuts: { ...state.viewerShortcuts, [action.key]: action.value },
    };
```

---

### Step 4: 設定画面UIコンポーネントの作成

#### 4-1. 新規コンポーネント `ViewerShortcutSettings.tsx`

`KeybindSettings.tsx` をテンプレートとして、右サイドバーショートカット用の設定UIを作成する。

```typescript
// ViewerShortcutSettings.tsx
import type { FormDispatch, FormState } from "./types";
import { ShortcutInput } from "./ShortcutInput";

// 右サイドバーの全ビュー定義（IDとラベルとデフォルトのショートカット）
const VIEWER_SHORTCUTS: { key: string; label: string; defaultVal: string }[] = [
    { key: "file-tree",  label: "File Tree",  defaultVal: "Ctrl+Shift+E" },
    { key: "git-graph",  label: "Git Graph",  defaultVal: "Ctrl+Shift+G" },
    { key: "error-log",  label: "Error Log",  defaultVal: "Ctrl+Shift+L" },
    { key: "diff",       label: "Diff",        defaultVal: "Ctrl+Shift+D" },
];

interface ViewerShortcutSettingsProps {
    s: FormState;
    dispatch: FormDispatch;
}

export function ViewerShortcutSettings({ s, dispatch }: ViewerShortcutSettingsProps) {
    return (
        <div className="settings-section">
            <div className="settings-section-title">右サイドバーショートカット</div>
            <span className="settings-desc" style={{ marginBottom: 8, display: "block" }}>
                右サイドバーの各パネルを開閉するショートカットキー
            </span>

            {VIEWER_SHORTCUTS.map((vs) => (
                <div className="form-group" key={vs.key}>
                    <label className="shortcut-label">{vs.label}</label>
                    <ShortcutInput
                        value={s.viewerShortcuts[vs.key] || ""}
                        onChange={(v) =>
                            dispatch({ type: "UPDATE_VIEWER_SHORTCUT", key: vs.key, value: v })
                        }
                        placeholder={vs.defaultVal}
                        ariaLabel={`${vs.key} sidebar shortcut`}
                    />
                </div>
            ))}
        </div>
    );
}
```

**設計ポイント:**
- 既存の `ShortcutInput` コンポーネントをそのまま再利用（修飾キー対応済み）
- デフォルト値を `placeholder` で表示し、未設定時はデフォルトが使われることを示唆
- `VIEWER_SHORTCUTS` 配列を将来のビュー追加時に拡張可能なデータ構造にする

#### 4-2. 設定画面のカテゴリ統合方針（2つの選択肢）

**案A: 既存の「キーバインド」タブ内に統合（推奨）**
- `KeybindSettings.tsx` 内に `ViewerShortcutSettings` を埋め込む
- tmuxキーバインドと右サイドバーショートカットを同一画面内にセクション分けして表示
- ユーザーにとってキー設定が一箇所にまとまり直感的

**案B: 新規カテゴリタブを追加**
- `SettingsCategory` に `"viewer-shortcuts"` を追加
- `SETTINGS_CATEGORIES` 配列に追加
- 独立したタブとして表示

→ **案A を推奨**。理由: 既存のキーバインドタブは項目が5つだけで、右サイドバーの4項目を追加しても多すぎない。ユーザーがショートカット設定を探す際に一箇所で完結する。

#### 4-3. `KeybindSettings.tsx` の修正（案A の場合）

```typescript
export function KeybindSettings({ s, dispatch }: KeybindSettingsProps) {
    return (
        <>
            {/* 既存のtmuxキーバインドセクション */}
            <div className="settings-section">
                <div className="settings-section-title">キーバインド</div>
                {/* ... 既存のコード ... */}
            </div>

            {/* 右サイドバーショートカットセクション（新規追加） */}
            <ViewerShortcutSettings s={s} dispatch={dispatch} />
        </>
    );
}
```

---

### Step 5: 保存ロジックの拡張 (`SettingsModal.tsx`)

#### 5-1. `handleSave` 内の `payload` に `viewer_shortcuts` を追加

```typescript
const payload: WailsConfigInput = {
    // ... 既存 ...
    viewer_shortcuts: Object.keys(s.viewerShortcuts).length > 0
        ? s.viewerShortcuts
        : undefined,
};
```

---

### Step 6: ViewerSystem のショートカット動的解決

#### 6-1. 設定値の取得と反映

現在、`ViewerSystem.tsx` は `viewerRegistry` からショートカットマップを構築している。
設定画面で変更されたショートカットを反映するため、以下の変更を行う：

**方式A: registerView 時にデフォルトを登録し、設定値で上書き**
- 各 `views/*/index.ts` は従来どおりデフォルトショートカットで `registerView` する
- `ViewerSystem.tsx` でショートカットマップ構築時に、設定から読み込んだカスタムショートカットで上書きする

```typescript
// ViewerSystem.tsx
const shortcutMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const view of views) {
        // 設定値があればそちらを優先、なければregistryのデフォルトを使用
        const customShortcut = configViewerShortcuts?.[view.id];
        const shortcut = customShortcut || view.shortcut;
        if (shortcut) {
            map.set(shortcut.toLowerCase(), view.id);
        }
    }
    return map;
}, [views, configViewerShortcuts]);
```

**方式B: 設定変更時にレジストリのショートカットを直接更新**
- `app:config-updated` イベント受信時に、各ビューのショートカットを再登録

→ **方式A を推奨**。理由: レジストリのプラグイン定義自体は不変に保ち、表示・動作レイヤーで上書きする方が副作用が少ない。

#### 6-2. 設定値の購読

`ViewerSystem` が設定の `viewer_shortcuts` にアクセスするため、以下のいずれかの方式で設定値を購読する：

1. **既存の `useBackendSync` を経由**: `app:config-updated` イベントで設定が更新された際に、`viewer_shortcuts` を専用のZustandストア（またはconfig ストア）に保存
2. **起動時にGetConfig()で取得**: 初回ロード時に設定を読み込み、`configStore` 等に保持

→ 既に `useBackendSync` で `app:parsed-config-updated` を購読する仕組みがあるならばそれを拡張する。なければ `ViewerSystem` 内で初回ロード＋イベント購読の軽量な仕組みを追加。

#### 6-3. ActivityStrip のツールチップ更新

`ActivityStrip.tsx` のボタンツールチップ `title={view.shortcut ? ...}` も、カスタムショートカットを反映して正しい値を表示する必要がある。

```typescript
// カスタムショートカットの参照を追加
const customShortcut = configViewerShortcuts?.[view.id];
const displayShortcut = customShortcut || view.shortcut;
title={displayShortcut ? `${view.label} (${displayShortcut})` : view.label}
```

---

### Step 7: バリデーション

#### 7-1. ショートカット重複チェック

同じショートカットキーが複数のビューに割り当てられていないかをバリデーションする。

```typescript
// settingsValidation.ts に追加
export function validateViewerShortcuts(
    viewerShortcuts: Record<string, string>,
): Record<string, string> {
    const errors: Record<string, string> = {};
    const seen = new Map<string, string>(); // shortcut → viewId
    for (const [viewId, shortcut] of Object.entries(viewerShortcuts)) {
        if (!shortcut.trim()) continue;
        const normalized = shortcut.toLowerCase();
        const existing = seen.get(normalized);
        if (existing) {
            errors[`viewer_shortcut_${viewId}`] =
                `「${shortcut}」は「${existing}」と重複しています`;
        } else {
            seen.set(normalized, viewId);
        }
    }
    return errors;
}
```

#### 7-2. `handleSave` へのバリデーション追加

```typescript
const errors = {
    // ... 既存 ...
    ...validateViewerShortcuts(s.viewerShortcuts),
};
```

#### 7-3. バリデーションエラー時のカテゴリ切替

```typescript
if (Object.keys(errors).some((k) => k.startsWith("viewer_shortcut"))) {
    dispatch({ type: "SET_FIELD", field: "activeCategory", value: "keybinds" });
}
```

---

## 4. ファイル構成一覧（新規作成・変更対象）

### 新規作成ファイル

| ファイルパス | 役割 |
|---|---|
| `myT-x/frontend/src/components/settings/ViewerShortcutSettings.tsx` | 右サイドバーショートカット設定UIコンポーネント |

### 変更対象ファイル

| ファイルパス | 変更内容 |
|---|---|
| `myT-x/internal/config/config.go` | `Config` 構造体に `ViewerShortcuts` フィールド追加 |
| `myT-x/config.yaml` | `viewer_shortcuts` セクションの追記 |
| `myT-x/frontend/src/types/tmux.ts` | `AppConfig` 型に `viewer_shortcuts` 追加 |
| `myT-x/frontend/src/components/settings/types.ts` | `FormState` に `viewerShortcuts` 追加、`FormAction` に `UPDATE_VIEWER_SHORTCUT` 追加 |
| `myT-x/frontend/src/components/settings/settingsReducer.ts` | `INITIAL_FORM` / `LOAD_CONFIG` / `UPDATE_VIEWER_SHORTCUT` 対応 |
| `myT-x/frontend/src/components/settings/KeybindSettings.tsx` | `ViewerShortcutSettings` の埋め込み（案A採用時） |
| `myT-x/frontend/src/components/settings/settingsValidation.ts` | ショートカット重複バリデーション追加 |
| `myT-x/frontend/src/components/SettingsModal.tsx` | `handleSave` の payload に `viewer_shortcuts` 追加、バリデーション統合 |
| `myT-x/frontend/src/components/viewer/ViewerSystem.tsx` | ショートカットマップ構築時にカスタム設定を優先する処理 |
| `myT-x/frontend/src/components/viewer/ActivityStrip.tsx` | ツールチップのショートカット表示をカスタム値に対応 |
| `README.md` | `viewer_shortcuts` 設定項目の説明を追記 |

### Wails自動生成（ビルド時）

| ファイルパス | 変更内容 |
|---|---|
| `myT-x/frontend/wailsjs/go/models.ts` | `Config` に `viewer_shortcuts` フィールドが自動追加される |

---

## 5. テスト計画

### Go ユニットテスト
- `Config` の YAML シリアライズ/デシリアライズで `viewer_shortcuts` が正しく読み書きされること
- `viewer_shortcuts` が未設定の場合にパースエラーが発生しないこと
- `viewer_shortcuts` が空マップの場合にYAML出力で `omitempty` により省略されること

### フロントエンド ユニットテスト (`settingsValidation.test.ts` 追加)
- `validateViewerShortcuts` が重複ショートカットを検出すること
- 空のショートカットがバリデーションエラーにならないこと（未設定＝デフォルト使用）
- 大文字小文字を区別せずに重複判定すること

### フロントエンド ユニットテスト (`settingsReducer` 関連)
- `LOAD_CONFIG` で `viewer_shortcuts` が正しく `FormState` に反映されること
- `UPDATE_VIEWER_SHORTCUT` が該当キーのみを更新すること

### 統合テスト（手動確認）
1. 設定画面を開き、キーバインドタブに右サイドバーショートカットセクションが表示されること
2. ショートカット入力欄をクリックし、キーを押して新しいショートカットが設定されること
3. 保存後、`config.yaml` に `viewer_shortcuts` が正しく書き込まれること
4. 変更したショートカットで右サイドバーの該当パネルがトグルされること
5. ActivityStrip のツールチップに変更後のショートカットが表示されること
6. ショートカットが重複している場合にバリデーションエラーが表示されること
7. 設定ファイルに `viewer_shortcuts` が無い場合、デフォルト値で動作すること
8. アプリ再起動後もカスタムショートカットが保持されていること

---

## 6. 設計上の注意点

### ShortcutInput の再利用に関する考慮
- 既存の `ShortcutInput.tsx` は修飾キー（Ctrl, Shift, Alt）＋ キーの組み合わせをキャプチャ可能
- 右サイドバーショートカットは `Ctrl+Shift+X` 形式が前提のため、そのまま利用可能
- 修飾キーなしの単体キー入力は許容しない設計（tmuxキーバインドとの区別）を検討

### グローバルホットキーとの競合回避
- `globalHotkey`（`Ctrl+Shift+F12`）やプレフィックスキー（`Ctrl+b`）との重複チェックも行う必要がある
- バリデーション関数を拡張し、globalHotkey やブラウザ標準ショートカットとの競合を警告として表示する（保存は許容するが、競合警告を出すのが親切）

### 将来のビュー追加への拡張性
- `ViewerShortcutSettings` 内の `VIEWER_SHORTCUTS` 配列は手動管理
- 将来のビュー追加（Input History等）時には、この配列とGoのデフォルトマップの両方にエントリを追加する
- 長期的には `viewerRegistry` から動的に取得する方式も検討可

---

## 7. 今後の拡張案（スコープ外）

- **ショートカットのリセットボタン**: 個別ビューのショートカットをデフォルトに戻すボタン
- **全リセットボタン**: 全ショートカットを一括でデフォルトに戻す
- **ショートカット競合チェッカー**: トースト通知で競合を即座にフィードバック
- **ショートカットのカスタムプレフィックス**: `Ctrl+Shift` 以外のプレフィックスも許容する拡張
