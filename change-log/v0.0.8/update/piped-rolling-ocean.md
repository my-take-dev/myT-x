# クイックスタートセッション機能

## Context

現在、新規セッションの作成には NewSessionModal を開き、ディレクトリピッカーでフォルダを選び、セッション名を入力する複数ステップが必要。
この機能により、ワンクリックでセッションを開始できるようにする。
セッション未選択時の空画面に「クイックスタート」ボタンを配置し、押下時にデフォルトディレクトリ（設定値 or アプリ起動ディレクトリ）でセッションを即座に作成する。

---

## 変更一覧

### 1. Go Config: `DefaultSessionDir` フィールド追加

**`myT-x/internal/config/config.go`**

- `Config` struct に `DefaultSessionDir string` フィールドを追加（`ViewerShortcuts` の後）
  ```go
  DefaultSessionDir string `yaml:"default_session_dir,omitempty" json:"default_session_dir,omitempty"`
  ```
- `applyDefaultsAndValidate` 末尾で `validateDefaultSessionDir(cfg)` を呼ぶ
  - 空文字 → そのまま（未設定扱い）
  - 絶対パスでない → Warn ログ出力 + 空文字にリセット（non-fatal）
- `Clone()` は string なので変更不要

### 2. Go Config テスト更新

**`myT-x/internal/config/config_test.go`**

- `TestConfigStructFieldCounts`: `12` → `13`
- `TestIsZeroConfig`: `DefaultSessionDir` セットケース追加
- 新テスト `TestValidateDefaultSessionDir`: 絶対パス保持、相対パスクリア、空文字パス
- 新テスト `TestSaveRoundTripDefaultSessionDir`: Save→Load往復確認

### 3. Go App: 起動ディレクトリ保存

**`myT-x/app.go`** — App struct に `launchDir string` フィールド追加

**`myT-x/app_lifecycle.go`** — `startup()` 内、`a.workspace = workspace` の直後に:
```go
a.launchDir = workspace
```
- `workspace` は `os.Getwd()` で取得済み（フォールバック付き）
- `launchDir` は write-once / read-many で mutex 不要

### 4. Go API: `QuickStartSession` メソッド

**`myT-x/app_session_api.go`**

```go
func (a *App) QuickStartSession() (tmux.SessionSnapshot, error)
```

ロジック:
1. `cfg.DefaultSessionDir` が非空ならそれを使用、空なら `a.launchDir` にフォールバック
2. 両方空なら error 返却
3. `os.Stat` でディレクトリ存在確認
4. `findSessionByRootPath` で競合チェック → 既存セッションがあればそれを activate して返す
5. `filepath.Base(dir)` でセッション名自動生成
6. `a.CreateSession(dir, sessionName, CreateSessionOptions{})` に委譲

### 5. Go API テスト

**`myT-x/app_session_api_test.go`**

テーブル駆動テスト `TestQuickStartSession`:
| ケース | DefaultSessionDir | launchDir | 期待結果 |
|--------|-------------------|-----------|----------|
| 設定値使用 | 有効なtempdir | 別dir | 設定値dirでセッション作成 |
| launchDirフォールバック | "" | 有効なtempdir | launchDirでセッション作成 |
| 両方空でエラー | "" | "" | error |
| dir存在しない | 存在しないパス | "" | error |

### 6. config.yaml 更新

**`myT-x/config.yaml`**

`websocket_port` の後にコメント例を追加:
```yaml
# default_session_dir: クイックスタートで使用するデフォルト作業ディレクトリ
# 未設定の場合はアプリ起動時のカレントディレクトリが使用されます
# default_session_dir: "C:\\Users\\username\\projects"
```

### 7. Frontend: Wails バインディング再生成

`wails generate` で `QuickStartSession` の TS バインディングを生成。

### 8. Frontend: api.ts 更新

**`myT-x/frontend/src/api.ts`**

- import に `QuickStartSession` 追加
- `api` オブジェクトに `QuickStartSession` 追加

### 9. Frontend: types/tmux.ts 更新

**`myT-x/frontend/src/types/tmux.ts`**

`AppConfig` 型に追加:
```typescript
default_session_dir?: string;
```

`AppConfigBase` の `Pick` に `default_session_dir` を追加（wails.Config に追加される為）。

### 10. Frontend: 設定フォーム状態

**`myT-x/frontend/src/components/settings/types.ts`**

`FormState` に追加:
```typescript
defaultSessionDir: string;
```

**`myT-x/frontend/src/components/settings/settingsReducer.ts`**

- `INITIAL_FORM` に `defaultSessionDir: ""` 追加
- `LOAD_CONFIG` ケースに `defaultSessionDir: cfg.default_session_dir || ""` 追加

### 11. Frontend: 基本設定に UI 追加

**`myT-x/frontend/src/components/settings/GeneralSettings.tsx`**

Global Hotkey の後に追加:
- ラベル: `デフォルトセッションディレクトリ`
- テキスト入力 + 「参照...」ボタン（`api.PickSessionDirectory()` 呼出）+ クリアボタン
- placeholder: `未設定（起動ディレクトリを使用）`
- 説明文: `クイックスタートで使用する作業ディレクトリ。未設定の場合はアプリ起動時のディレクトリが使用されます`

`api` import を追加。`handlePickDir` ハンドラを作成。

### 12. Frontend: 設定保存ペイロード更新

**`myT-x/frontend/src/components/SettingsModal.tsx`**

`handleSave` の `payload` に追加:
```typescript
default_session_dir: s.defaultSessionDir.trim() || undefined,
```

### 13. Frontend: クイックスタートボタン

**`myT-x/frontend/src/components/SessionView.tsx`**

`!props.session` の空状態を変更:

```tsx
// Before:
<div className="session-empty">セッションを作成してください。</div>

// After:
<div className="session-empty">
  <div className="session-empty-content">
    <p className="session-empty-message">セッションを作成してください。</p>
    <button className="session-quick-start-btn" onClick={handleQuickStart} disabled={quickStartLoading}>
      {quickStartLoading ? "開始中..." : "▶ クイックスタート"}
    </button>
    {quickStartError && <p className="session-quick-start-error">{quickStartError}</p>}
  </div>
</div>
```

- `useState` で `quickStartLoading`, `quickStartError` 管理
- `handleQuickStart`: `api.QuickStartSession()` → 成功時 `api.SetActiveSession(snapshot.name)` + store更新

### 14. Frontend: CSS 追加

**`myT-x/frontend/src/styles/base.css`**

`.session-empty` の後に追加:
```css
.session-empty-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}
.session-empty-message { color: var(--fg-dim); margin: 0; }
.session-quick-start-btn {
  padding: 10px 24px;
  border: 1px solid var(--accent);
  border-radius: 8px;
  background: var(--accent-06);
  color: var(--accent);
  font-size: 0.95rem;
  cursor: pointer;
  transition: background-color 0.15s;
}
.session-quick-start-btn:hover:not(:disabled) { background: var(--accent-22); }
.session-quick-start-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.session-quick-start-error { color: var(--danger); font-size: 0.8rem; margin: 0; }
.form-input-with-button { display: flex; gap: 6px; align-items: center; }
.form-input-with-button .form-input { flex: 1; }
```

---

## 実装順序

```
Backend:
  1. config.go          — DefaultSessionDir フィールド + バリデーション
  2. config_test.go     — フィールド数ガード更新 + 新テスト
  3. app.go             — launchDir フィールド追加
  4. app_lifecycle.go    — startup() で launchDir 設定
  5. app_session_api.go  — QuickStartSession メソッド
  6. app_session_api_test.go — テスト
  7. config.yaml         — コメント追加
  8. build_project       — ビルド確認

Frontend:
  9.  wails generate     — バインディング再生成
  10. api.ts             — QuickStartSession 追加
  11. types/tmux.ts      — AppConfig 更新
  12. settings/types.ts  — FormState 更新
  13. settingsReducer.ts — 初期値 + LOAD_CONFIG 更新
  14. GeneralSettings.tsx — ディレクトリ入力UI
  15. SettingsModal.tsx   — 保存ペイロード更新
  16. SessionView.tsx     — クイックスタートボタン
  17. base.css           — スタイル追加
```

## 検証方法

1. **Go テスト**: `go test ./internal/config/... ./...` で全テスト PASS 確認
2. **ビルド**: `build_project` でコンパイルエラーなし確認
3. **動作確認**:
   - アプリ起動 → セッション未選択状態で「▶ クイックスタート」ボタン表示確認
   - ボタン押下 → アプリ起動ディレクトリでセッション作成確認
   - 設定画面 → 基本設定にディレクトリ入力欄表示確認
   - 「参照...」でフォルダ選択 → 保存 → 再度クイックスタート → 設定ディレクトリでセッション作成確認
   - 同じディレクトリの既存セッションがある場合 → 既存セッションが activate されること確認
