# エラー表示画面の対象情報および出力調整実装

元計画: `plans/error-display-plan.md`

## 実装内容

### Step 1: バックエンドエンドポイント追加 (Go)

**`myT-x/app_session_log.go`**

- `frontendLogMaxMsgLen = 2000` / `frontendLogMaxSourceLen = 200` 定数追加（rune単位）
- `normalizeLogLevel(level string) string` ヘルパー追加: 任意の文字列を `"error"` / `"warn"` / `"info"` に正規化
- `(a *App) LogFrontendEvent(level, msg, source string)` Wailsバインドメソッド追加:
  - level: normalizeLogLevel で正規化
  - msg/source: TrimSpace + rune単位キャップ（マルチバイト安全）
  - msg が空の場合はサイレント廃棄
  - `writeSessionLogEntry` に委譲 → JSONL永続化 + `app:session-log-updated` ping 自動発火

**`myT-x/app_session_log_test.go`**

テーブル駆動テスト4関数追加:
- `TestNormalizeLogLevel`: 10ケース（正規化・大文字小文字・エイリアス・空文字）
- `TestLogFrontendEvent_WritesToSessionLog`: 5ケース（level別・空msg廃棄）
- `TestLogFrontendEvent_TruncatesLongInputs`: 4ケース（上限以内・ちょうど・超過）
- `TestLogFrontendEvent_MultibyteSafeRune`: マルチバイト文字（日本語）でのrune安全トランケーション検証

### Step 2: Wailsバインディング更新

**`myT-x/frontend/wailsjs/go/main/App.js`**
```js
export function LogFrontendEvent(arg1, arg2, arg3) {
  return window['go']['main']['App']['LogFrontendEvent'](arg1, arg2, arg3);
}
```

**`myT-x/frontend/wailsjs/go/main/App.d.ts`**
```ts
export function LogFrontendEvent(arg1:string,arg2:string,arg3:string):Promise<void>;
```

**`myT-x/frontend/src/api.ts`**
- `LogFrontendEvent` をインポート・`api` オブジェクトに追加

### Step 3: グローバル未捕捉例外キャッチ

**`myT-x/frontend/src/main.tsx`**

- `Symbol.for("mytx.global.error.handlers")` による HMR 重複登録防止
- `window.addEventListener("error", ...)` → `api.LogFrontendEvent("error", ..., "frontend/unhandled")`
- `window.addEventListener("unhandledrejection", ...)` → `api.LogFrontendEvent("error", ..., "frontend/promise")`
- 全 `.catch(() => {})` でエラー回復中の再帰ログ防止
- `container!` 非nullアサーション → 明示的ガード＋`throw new Error(...)` に修正 (#99)
- `<ErrorBoundary>` で `<App>` をラップ

### Step 4: React Error Boundary

**`myT-x/frontend/src/components/ErrorBoundary.tsx`** (新規作成)

- `getDerivedStateFromError`: エラーメッセージを state に保持
- `componentDidCatch`: `api.LogFrontendEvent("error", ..., "frontend/react")` 呼び出し
  - component stack を 300文字にクリップしてから source に含める
  - `.catch(() => {})` でサイレント廃棄
- フォールバック UI: エラーメッセージ表示 + 再試行ボタン

### Step 5: useBackendSync.ts の失敗ハンドラ修正

**`myT-x/frontend/src/hooks/useBackendSync.ts`**

API失敗時に `api.LogFrontendEvent` を追加:

| 箇所 | level | source |
|------|-------|--------|
| `GetConfigAndFlushWarnings` 失敗 | error | frontend/api |
| `ListSessions` 失敗 | error | frontend/api |
| `GetActiveSession` 失敗 | warn | frontend/api |
| `tmux:worker-panic` イベント | warn | frontend/worker |
| `tmux:worker-fatal` イベント | error | frontend/worker |

全て `.catch(() => {})` でエラー回復中の再帰ログ防止。

## 防御的コーディングチェックリスト準拠

| # | チェック | 対応 |
|---|---------|------|
| #29 | エラーラップ統一 | Go側: writeSessionLogEntry に委譲、ラップ不要 |
| #44 | 入力バリデーション統一 | TrimSpace + empty check 実施 |
| #77 | ログレベル/プレフィックス整合 | normalizeLogLevel で統一 |
| #84 | catch内でユーザー通知 | notifyWarn + LogFrontendEvent 両立 |
| #88 | HMR安全 | Symbol.for() で重複防止 |
| #95 | fire-and-forget に .catch() | 全箇所で `.catch(() => {})` |
| #99 | 非nullアサーション禁止 | container! を明示ガードに修正 |
| #133 | Wailsバインディング再生成 | App.js / App.d.ts 手動更新 |
