# Claude Code 環境変数対応 実装計画

## Context

Claude Codeの環境変数を設定画面から管理し、セッション作成時にチェックボックスで適用を制御する機能。
現在、追加ペイン用環境変数(`pane_env`)は常に追加ペインに適用されるが、本機能により：
- 新たにClaude Code環境変数(`claude_env`)を全ペイン(初期+追加)に適用可能にする
- 両環境変数の適用をセッション単位でユーザーが制御できるようにする
- 各設定画面にデフォルトON/OFFチェックを追加し、NewSessionModalの初期値に反映する

## 変更ファイル一覧

### Go Backend
| File | Change |
|------|--------|
| `myT-x/internal/config/config.go` | `ClaudeEnvConfig` struct追加、Config に `ClaudeEnv` / `PaneEnvDefaultEnabled` フィールド追加 |
| `myT-x/internal/config/config_test.go` | 新フィールドのLoad/Save/Clone/sanitizeテスト |
| `myT-x/internal/tmux/session_manager.go` | `TmuxSession` に `UseClaudeEnv *bool` / `UsePaneEnv *bool` 追加、setter追加 |
| `myT-x/internal/tmux/command_router.go` | `RouterOptions.ClaudeEnv` 追加、`UpdateClaudeEnv`/`claudeEnvView` 追加、`buildPaneEnvForSession` 追加 |
| `myT-x/internal/tmux/command_router_handlers_session.go` | 変更不要 (セッションフラグはapp層から直接設定) |
| `myT-x/internal/tmux/command_router_handlers_pane.go` | `splitWindowResolved` でセッションフラグ参照、`buildPaneEnvForSession` 呼び出し |
| `myT-x/internal/tmux/command_router_handlers_window.go` | `handleNewWindow` でセッションフラグ参照、`buildPaneEnvForSession` 呼び出し |
| `myT-x/internal/tmux/command_router_terminal.go` | `sanitizeClaudeEnv` (pane_envと同等のサニタイズ) |
| `myT-x/app_session_api.go` | `CreateSession` にパラメータ追加、claude_envマージロジック |
| `myT-x/app_worktree_api.go` | `WorktreeSessionOptions` にフラグ追加、`createSessionForDirectory` パラメータ追加 |
| `myT-x/app_config_api.go` | `SaveConfig` で `applyRuntimeClaudeEnvUpdate` 追加、`GetClaudeEnvVarDescriptions` API追加 |
| `myT-x/app_lifecycle.go` | startup時に `RouterOptions.ClaudeEnv` 設定 |
| `myT-x/app_claude_env_descriptions.go` | **新規**: `claudeEnvVarDescriptions` map定義 |

### Frontend
| File | Change |
|------|--------|
| `myT-x/frontend/src/components/settings/types.ts` | `SettingsCategory` に `"claude-env"` 追加、`ClaudeEnvEntry` type追加、`FormState` にclaudeEnv関連フィールド追加 |
| `myT-x/frontend/src/components/settings/settingsReducer.ts` | claude_env フォーム状態のロード/保存ロジック |
| `myT-x/frontend/src/components/settings/settingsValidation.ts` | claude_env バリデーション |
| `myT-x/frontend/src/components/settings/ClaudeEnvSettings.tsx` | **新規**: CLAUDE CODE環境変数設定パネル |
| `myT-x/frontend/src/components/settings/PaneEnvSettings.tsx` | 「セッション作成時デフォルトON」チェック追加 |
| `myT-x/frontend/src/components/SettingsModal.tsx` | カテゴリ追加、handleSaveでclaude_env構築 |
| `myT-x/frontend/src/components/NewSessionModal.tsx` | 2つの新チェックボックス追加 |
| `myT-x/frontend/src/types/tmux.ts` | `AppConfig` に `claude_env` / `pane_env_default_enabled` 追加 |
| `myT-x/frontend/src/api.ts` | `GetClaudeEnvVarDescriptions` API追加 |

### Config / Generated
| File | Change |
|------|--------|
| `myT-x/config.yaml` | `claude_env` セクション追加、`pane_env_default_enabled` 追加 |
| `myT-x/frontend/wailsjs/go/models.ts` | `wails generate module` で再生成 |
| `myT-x/frontend/wailsjs/go/main/App.d.ts` | 再生成 |
| `myT-x/frontend/wailsjs/go/main/App.js` | 再生成 |

---

## 実装詳細

### Step 1: Go Config 拡張

**`config.go`** — 新型定義とフィールド追加:

```go
// ClaudeEnvConfig holds Claude Code environment variable settings.
// Vars contains key-value pairs applied to terminal panes.
// DefaultEnabled controls the checkbox default in the new session modal.
type ClaudeEnvConfig struct {
    DefaultEnabled bool              `yaml:"default_enabled" json:"default_enabled"`
    Vars           map[string]string `yaml:"vars,omitempty" json:"vars,omitempty"`
}

type Config struct {
    // ... 既存フィールド ...
    PaneEnv               map[string]string `yaml:"pane_env,omitempty" json:"pane_env,omitempty"`
    PaneEnvDefaultEnabled bool              `yaml:"pane_env_default_enabled" json:"pane_env_default_enabled"`
    ClaudeEnv             *ClaudeEnvConfig  `yaml:"claude_env,omitempty" json:"claude_env,omitempty"`
}
```

- `Clone()`: `ClaudeEnvConfig` のディープコピー追加
- `applyDefaultsAndValidate()`: `sanitizeClaudeEnv(cfg)` 呼び出し追加
- `sanitizeClaudeEnv()`: 既存 `sanitizePaneEnv()` と同等ロジック (blocked keys warning、null byte strip、case-insensitive duplicate detection)
- 既存 `PaneEnv map[string]string` はそのまま維持 (後方互換)

### Step 2: claudeEnvVarDescriptions 定義

**`app_claude_env_descriptions.go`** (新規):

仕様書の `claudeEnvVarDescriptions` map をそのまま定義。
Wails公開メソッド `GetClaudeEnvVarDescriptions() map[string]string` を `app_config_api.go` に追加。

### Step 3: TmuxSession フラグ追加

**`session_manager.go`**:

```go
type TmuxSession struct {
    // ... 既存フィールド ...
    // UseClaudeEnv controls whether claude_env config is applied to panes in this session.
    // nil = legacy session (IPC-created), pointer semantics distinguish unset from false.
    UseClaudeEnv *bool `json:"use_claude_env,omitempty"`
    // UsePaneEnv controls whether pane_env config is applied to additional panes.
    // nil = legacy session (pane_env always fills, backward compatible).
    UsePaneEnv   *bool `json:"use_pane_env,omitempty"`
}
```

新メソッド:
- `SetUseClaudeEnv(sessionName string, enabled bool) error`
- `SetUsePaneEnv(sessionName string, enabled bool) error`
- `Snapshot()` 内の `SessionSnapshot` マッピングに反映 (内部用、フロントエンドSnaphotには不要)

### Step 4: CommandRouter 拡張

**`command_router.go`**:

```go
type RouterOptions struct {
    // ... 既存フィールド ...
    ClaudeEnv map[string]string // Claude Code env vars; protected by claudeEnvMu
}

type CommandRouter struct {
    // ... 既存フィールド ...
    claudeEnvMu sync.RWMutex
}
```

新メソッド:
- `UpdateClaudeEnv(claudeEnv map[string]string)` — `UpdatePaneEnv` と同パターン (deep copy + swap)
- `claudeEnvView() map[string]string` — `paneEnvView` と同パターン (copy-on-write contract)
- `ClaudeEnvSnapshot() map[string]string` — 外部テスト用

**`buildPaneEnvForSession`** — 追加ペイン用の新マージメソッド:

```go
// buildPaneEnvForSession builds environment for additional panes, respecting
// session-level UseClaudeEnv and UsePaneEnv flags.
//
// Merge priority (lowest → highest):
//   1. claude_env from config (fills base, when useClaudeEnv)
//   2. inheritedEnv (source pane env, includes claude_env if previously set)
//   3. pane_env from config (when usePaneEnv; overwrite if useClaudeEnv also true, fill-only otherwise)
//   4. shimEnv (shim's -e flag, highest custom priority)
//   5. tmux internal vars (always final)
func (r *CommandRouter) buildPaneEnvForSession(
    inheritedEnv, shimEnv map[string]string,
    sessionID, paneID int,
    useClaudeEnv, usePaneEnv bool,
) map[string]string
```

マージ順序の設計意図:
- `claude_env` は最低優先度 (ベース層)
- 継承元ペインの env が claude_env を上書き (config更新後の新キー補填を許容)
- `pane_env` は `useClaudeEnv` も true の場合に上書きモード (仕様: 「追加ペインが優先」)
- `pane_env` は `useClaudeEnv` が false の場合は fill-only (後方互換)
- shim の `-e` フラグが最高優先度 (Claude Codeの明示的指定)
- tmux内部変数 (GO_TMUX等) は常に最終上書き

### Step 5: Router Handler 変更

**`handleNewSession`** (`command_router_handlers_session.go`):
- 変更不要。セッションフラグはapp層から直接設定する (Step 6参照)

**`splitWindowResolved`** (`command_router_handlers_pane.go`):
```go
// セッションフラグ取得
session, ok := r.sessions.GetSession(targetCtx.SessionName)
if ok && (session.UseClaudeEnv != nil || session.UsePaneEnv != nil) {
    useClaudeEnv := session.UseClaudeEnv != nil && *session.UseClaudeEnv
    usePaneEnv := session.UsePaneEnv != nil && *session.UsePaneEnv
    env = r.buildPaneEnvForSession(targetCtx.Env, extraEnv, targetCtx.SessionID, newPane.ID, useClaudeEnv, usePaneEnv)
} else {
    // Legacy path: 既存 buildPaneEnv (pane_env 常時fill)
    mergedReqEnv := copyEnvMap(targetCtx.Env)
    for k, v := range extraEnv { mergedReqEnv[k] = v }
    env = r.buildPaneEnv(mergedReqEnv, targetCtx.SessionID, newPane.ID)
}
```

**`handleNewWindow`** (`command_router_handlers_window.go`):
- 同様にセッションフラグを参照して `buildPaneEnvForSession` / `buildPaneEnv` を分岐

### Step 6: App Layer API 変更

**`app_session_api.go`**:

```go
func (a *App) CreateSession(
    rootPath string, sessionName string, enableAgentTeam bool,
    useClaudeEnv bool, usePaneEnv bool,
) (snapshot tmux.SessionSnapshot, retErr error)
```

- `createSessionForDirectory` に `useClaudeEnv`, `usePaneEnv` パラメータ追加
- `useClaudeEnv` が true の場合: `cfg.ClaudeEnv.Vars` を `req.Env` にマージ (agent team env が優先)
- セッション作成後 (handleNewSession完了後)、app層から `sessions.SetUseClaudeEnv` / `sessions.SetUsePaneEnv` を直接呼び出し
- handleNewSessionはセッション作成のみ担当。フラグはapp層が所有する (追加ペイン作成前に必ず設定完了)

**`app_worktree_api.go`**:

```go
type WorktreeSessionOptions struct {
    // ... 既存フィールド ...
    UseClaudeEnv bool `json:"use_claude_env"`
    UsePaneEnv   bool `json:"use_pane_env"`
}
```

`CreateSessionWithWorktree` / `CreateSessionWithExistingWorktree` を対応。

**`app_config_api.go`**:
- `SaveConfig` 内で `applyRuntimeClaudeEnvUpdate(event)` を追加
- `GetClaudeEnvVarDescriptions() map[string]string` API追加

**`app_lifecycle.go`**:
- startup で `RouterOptions.ClaudeEnv = cfg.ClaudeEnv.Vars` 設定

### Step 7: Frontend Settings — ClaudeEnvSettings (新規)

**`ClaudeEnvSettings.tsx`**:

```
┌─────────────────────────────────────────────┐
│ CLAUDE CODE 環境変数                         │
│ Claude Codeに渡す環境変数を設定します。      │
│                                             │
│ [x] セッション作成時にデフォルトON            │
│                                             │
│ ┌─ 環境変数一覧 ──────────────────────────┐  │
│ │ [ANTHROPIC_API_KEY    ▾] [sk-ant-...  ] │  │
│ │ 説明: X-Api-Key ヘッダー...              │  │
│ │                                    [×]  │  │
│ │ [CLAUDE_CODE_EFFORT.. ▾] [high        ] │  │
│ │ 説明: サポートされている...              │  │
│ │                                    [×]  │  │
│ └─────────────────────────────────────────┘  │
│ [+ 環境変数追加]                             │
└─────────────────────────────────────────────┘
```

- キー入力: `<input>` + `<datalist>` で `claudeEnvVarDescriptions` のキーを候補表示
- 既知キー選択時: 日本語説明を `settings-desc` として表示
- 値入力: `<input>` テキスト
- 「セッション作成時にデフォルトON」チェックボックス → `claude_env.default_enabled`

### Step 8: PaneEnvSettings 更新

**`PaneEnvSettings.tsx`**:
- 既存UIの最上部に「セッション作成時にデフォルトON」チェックボックス追加
- `dispatch({ type: "SET_FIELD", field: "paneEnvDefaultEnabled", value: e.target.checked })`

### Step 9: SettingsModal カテゴリ追加

**`SettingsModal.tsx`**:
```ts
const SETTINGS_CATEGORIES = [
  { id: "general", label: "基本設定" },
  { id: "keybinds", label: "キーバインド" },
  { id: "worktree", label: "Worktree" },
  { id: "agent-model", label: "Agent Model" },
  { id: "claude-env", label: "CLAUDE CODE環境変数" },  // NEW
  { id: "pane-env", label: "追加ペイン環境変数" },
];
```

`handleSave` に claude_env payload 構築ロジック追加。

### Step 10: NewSessionModal チェックボックス

**`NewSessionModal.tsx`**:

```tsx
{directory && (
  <>
    {/* Agent Team option - 既存 */}
    <div className="form-checkbox-row">...</div>

    {/* Claude Code env option - NEW */}
    <div className="form-checkbox-row">
      <input
        type="checkbox"
        id="use-claude-env"
        checked={useClaudeEnv}
        onChange={(e) => setUseClaudeEnv(e.target.checked)}
      />
      <label htmlFor="use-claude-env">
        Claude Code 環境変数を利用する
      </label>
    </div>

    {/* Pane env option - NEW */}
    <div className="form-checkbox-row">
      <input
        type="checkbox"
        id="use-pane-env"
        checked={usePaneEnv}
        onChange={(e) => setUsePaneEnv(e.target.checked)}
      />
      <label htmlFor="use-pane-env">
        追加ペイン専用環境変数を利用する
      </label>
    </div>
  </>
)}
```

- デフォルト値: `config.claude_env?.default_enabled ?? false` / `config.pane_env_default_enabled ?? false`
- モーダルopen時に `api.GetConfig()` から取得して初期値設定
- `handleSubmit` で `useClaudeEnv`, `usePaneEnv` をAPI呼び出しに含める

### Step 11: TypeScript 型更新

**`types/tmux.ts`**:
```ts
export type AppConfigClaudeEnv = {
  default_enabled: boolean;
  vars?: Record<string, string>;
};

export type AppConfig = AppConfigBase & {
  worktree: AppConfigWorktree;
  agent_model?: AppConfigAgentModel;
  pane_env?: Record<string, string>;
  pane_env_default_enabled?: boolean;  // NEW
  claude_env?: AppConfigClaudeEnv;     // NEW
};
```

### Step 12: Wails bindings 再生成

`wails generate module` で `models.ts`, `App.d.ts`, `App.js` を再生成。

---

## App層直接設定方式

セッションフラグ (`UseClaudeEnv`, `UsePaneEnv`) はapp層が所有・設定する。

```go
// app_session_api.go — createSessionForDirectory 内:

// 1. claude_env をreq.Envにマージ (初期ペイン用)
if useClaudeEnv {
    claudeEnvVars := cfg.ClaudeEnv.Vars  // cfg はConfig snapshot
    for k, v := range claudeEnvVars {
        if _, exists := req.Env[k]; !exists {
            req.Env[k] = v  // fill-only (agent team envが優先)
        }
    }
}

// 2. handleNewSession でセッション作成 (router.HandleRequest)
snapshot, err := ...

// 3. セッション作成成功後、フラグを直接設定
//    追加ペイン作成はUIからのみ発生するため、この時点で必ず間に合う
if err := a.sessions.SetUseClaudeEnv(sessionName, useClaudeEnv); err != nil {
    slog.Warn("[ENV] failed to set UseClaudeEnv flag", "session", sessionName, "err", err)
}
if err := a.sessions.SetUsePaneEnv(sessionName, usePaneEnv); err != nil {
    slog.Warn("[ENV] failed to set UsePaneEnv flag", "session", sessionName, "err", err)
}
```

**設計根拠:**
- sentinel key方式を不採用: プロセスenvへの漏れリスク、handleNewSession内のparse責務増加
- app層直接設定: セッション作成→フラグ設定は同期的に完了。追加ペイン(split/new-window)はUI操作起点のため、フラグ設定前に到達することは物理的に不可能
- `handleNewSession` は変更不要: IPC経由のセッション作成との互換性も維持

---

## 防御的コーディング チェックリスト適用項目

| # | 項目 | 対応内容 |
|---|------|---------|
| 4 | エラー処理 | SetUseClaudeEnv/SetUsePaneEnv のエラーをロールバック分岐で処理 |
| 10 | ログ | SetUseClaudeEnv/SetUsePaneEnv の設定結果をslog.Debugでログ |
| 29 | エラーラップ | `fmt.Errorf("...: %w", err)` で統一 |
| 35 | JSON tag | 既存命名規約 (snake_case) に準拠 |
| 36 | INVARIANT | `claudeEnvMu` のcopy-on-write contract コメント (paneEnvMuと同形式) |
| 42 | omitempty+bool | `DefaultEnabled` は `omitempty` **不使用** (`false` もJSON出力必須) |
| 54 | mutex内emit禁止 | `claudeEnvMu`/`paneEnvMu` 保持中にemit/外部呼び出ししない |
| 55 | 共有フィールド | `ClaudeEnv` アクセスは `claudeEnvView()` / `claudeEnvMu` 経由 |
| 57 | ctx nilガード | イベント発行前のctx nilチェック |
| 67 | os.Getenv安全性 | `mergeEnvironment` は既存パターン維持 |
| 86 | イベント購読 | `config:updated` イベントでclaude_env変更もフロントエンドに反映 |
| 113 | テーブル駆動テスト | `buildPaneEnvForSession` / `sanitizeClaudeEnv` / setter の全パス |
| 130 | 変更伝播 | Config.Clone、settingsReducer LOAD_CONFIG、handleSave payload構築 |
| 133 | Wails再生成 | `wails generate module` 必須 |

---

## 開発フロー

```
1. confidence-check (機能理解の確認)
2. golang-expert: Config拡張 + テスト (Step 1-2)
3. golang-expert: SessionManager + CommandRouter 拡張 (Step 3-5) + テスト
4. golang-expert: App Layer API変更 (Step 6) + テスト
5. frontend: ClaudeEnvSettings + PaneEnvSettings更新 + SettingsModal (Step 7-9)
6. frontend: NewSessionModal チェックボックス (Step 10)
7. frontend: 型更新 + Wails再生成 (Step 11-12)
8. build_project + get_file_problems で全体ビルド検証
9. go test ./... で全テスト通過確認
10. self-review (defensive-coding-checklist 全項目走査)
```

並列実行可能:
- Step 1-2 (Config) と Step 7-9 (Frontend settings UI) は独立
- Step 3-5 は Step 1-2 完了後
- Step 10 は Step 6 完了後

---

## テスト計画

### Go テスト

| テスト対象 | テスト内容 |
|-----------|-----------|
| `config.Load/Save` with `ClaudeEnv` | YAML round-trip、デフォルト値、empty vars |
| `config.Clone` with `ClaudeEnv` | deep copy独立性 (pointer field) |
| `sanitizeClaudeEnv` | blocked keys、null bytes、case-insensitive duplicate、empty key |
| `SessionManager.SetUseClaudeEnv` | 正常設定、存在しないセッション、nil→true/false遷移 |
| `SessionManager.SetUsePaneEnv` | 同上 |
| `CommandRouter.UpdateClaudeEnv` | deep copy、concurrent access |
| `buildPaneEnvForSession` | 5層マージ優先度テスト (各層の上書き/fill確認) |
| `buildPaneEnvForSession` | useClaudeEnv=true/false × usePaneEnv=true/false の4パターン |
| `buildPaneEnvForSession` | blocked key除外確認 |
| App層フラグ設定 | createSessionForDirectory後のSetUseClaudeEnv/SetUsePaneEnv呼び出し |
| Legacy path | UseClaudeEnv=nil/UsePaneEnv=nil → 既存buildPaneEnv使用 |

### Frontend テスト (手動)

1. 設定画面: CLAUDE CODE環境変数タブ表示、key autocomplete、値入力、保存
2. 設定画面: 追加ペイン環境変数タブに「デフォルトON」チェック表示
3. NewSessionModal: デフォルト値が設定画面のチェックに連動
4. セッション作成 → 初期ターミナルにclaude_envが反映されていること
5. 追加ペイン作成 → pane_envが反映、claude_envキーがpane_envで上書きされること

---

## config.yaml 更新例

```yaml
shell: powershell.exe
prefix: Ctrl+b
# ... 既存設定 ...
pane_env:
  CLAUDE_CODE_EFFORT_LEVEL: "high"
pane_env_default_enabled: true
claude_env:
  default_enabled: true
  vars:
    ANTHROPIC_API_KEY: "sk-ant-api03-..."
    CLAUDE_CODE_EFFORT_LEVEL: "high"
    CLAUDE_CODE_DISABLE_AUTO_MEMORY: "1"
```
