# Session Error Log Viewer — 実装プラン

## Context

現在、myT-xアプリケーションにはエラー情報を視覚的に確認する手段がない。バックエンドで発生するエラー（セッション作成失敗、IPC障害、WebSocket起動失敗など）はslogでコンソールに出力されるだけで、ユーザーには見えない。

本機能では、右サイドバーのビューワープラグインとして **セッションエラーログビューア** を追加し、アプリケーション実行中に発生したWarn/Errorレベルのログをリアルタイムで表示する。エラー発生時にはActivity Stripのアイコンにバッジを表示し、視覚的に通知する。

ログはJSONL形式でconfig dirの `session-logs/` フォルダに永続化され、アプリ起動ごとに新しいファイルが作成される。最大100ファイルを保持し、起動時に古いファイルを削除する。

---

## Phase 1: Backend Foundation

### 1-1. セッションログ型定義 (`myT-x/app_session_log_types.go` 新規)

```go
type SessionLogEntry struct {
    Timestamp string `json:"ts"`     // "20060102150405" format
    Level     string `json:"level"`  // "error", "warn"
    Message   string `json:"msg"`
    Source    string `json:"source"` // slog group or component name
}
```

### 1-2. TeeHandler (`myT-x/internal/sessionlog/handler.go` 新規)

既存のslogハンドラをラップし、Warn/Error以上のレコードをコールバックに転送するカスタム `slog.Handler`。

- `NewTeeHandler(base slog.Handler, minLevel slog.Level, callback EntryCallback)`
- `Enabled()`: base に委譲
- `Handle()`: base.Handle()後、level >= minLevel ならcallback呼び出し
- `WithAttrs()`, `WithGroup()`: 新しいTeeHandler返却（base をラップ）
- group名をsource フィールドとして活用

テスト: `myT-x/internal/sessionlog/handler_test.go`
- Error/Warnでcallback呼び出し確認
- Infoレベルでcallback非呼び出し確認
- baseハンドラへの全レベル転送確認
- WithGroup のsource伝播確認

### 1-3. セッションログサービス (`myT-x/app_session_log.go` 新規)

App struct への追加フィールド (`myT-x/app.go`):
```go
sessionLogMu      sync.Mutex
sessionLogFile    *os.File
sessionLogPath    string
sessionLogEntries []SessionLogEntry
```

メソッド:
- `initSessionLog()` — ログディレクトリ作成、ファイルオープン、古いファイルクリーンアップ
  - パス: `filepath.Join(filepath.Dir(a.configPath), "session-logs")`
  - ファイル名: `session-YYYYMMDD-HHmmss.jsonl`
  - ディレクトリ: `0o700`, ファイル: `0o600`
- `cleanupOldSessionLogs()` — `session-*.jsonl` パターンでReadDir、ソート、100超過分を削除
- `writeSessionLogEntry(entry)` — JSON marshal + ファイル書込 + メモリ追加(上限10000) + イベント発行
- `closeSessionLog()` — ファイルハンドルclose
- `GetSessionErrorLog() []SessionLogEntry` — Wails公開API、メモリ内エントリのコピー返却
- `GetSessionLogFilePath() string` — Wails公開API、現在のログファイルパス返却

テスト: `myT-x/app_session_log_test.go`
- ディレクトリ作成、ファイル作成確認
- JSONL書込フォーマット確認
- イベント発行確認
- 100ファイル超過時の古いファイル削除
- メモリ上限キャップ
- ファイルclose確認

### 1-4. ライフサイクル統合 (`myT-x/app_lifecycle.go` 既存)

`startup()`:
- `a.configPath` 設定後、他のサブシステム初期化前に `a.initSessionLog()` 呼び出し
- TeeHandler をデフォルトslogロガーとして設定

`shutdown()`:
- `a.closeSessionLog()` 呼び出し追加

### 1-5. イベント定義

新規イベント: `"app:error-logged"` — ペイロード: `SessionLogEntry`
- `writeSessionLogEntry()` 内で `a.emitRuntimeEvent("app:error-logged", entry)` 呼び出し

---

## Phase 2: Wails Binding更新

### 2-1. バインディング再生成

`GetSessionErrorLog` と `GetSessionLogFilePath` が `App.d.ts`/`App.js` に自動生成される。

### 2-2. API ラッパー更新 (`myT-x/frontend/src/api.ts` 既存)

新規メソッドをインポートしてapiオブジェクトに追加:
- `GetSessionErrorLog`
- `GetSessionLogFilePath`

---

## Phase 3: Frontend Foundation

### 3-1. Error Log Store (`myT-x/frontend/src/stores/errorLogStore.ts` 新規)

Zustand store:
```typescript
interface ErrorLogState {
    entries: ErrorLogEntry[];
    unreadCount: number;
    addEntry: (entry: ErrorLogEntry) => void;
    setEntries: (entries: ErrorLogEntry[]) => void;
    markAllRead: () => void;
}
```

### 3-2. イベント統合 (`myT-x/frontend/src/hooks/useBackendSync.ts` 既存)

- `BackendEventMap` に `"app:error-logged"` 追加
- `onEvent("app:error-logged", ...)` ハンドラ追加（asObjectガード付き）
- `useErrorLogStore.getState().addEntry(entry)` 呼び出し

---

## Phase 4: Frontend UI

### 4-1. アイコン (`myT-x/frontend/src/components/viewer/icons/ErrorLogIcon.tsx` 新規)

警告三角形 + 感嘆符のSVGアイコン (既存アイコンパターン踏襲)

### 4-2. Error Log Hook (`myT-x/frontend/src/components/viewer/views/error-log/useErrorLog.ts` 新規)

- `useErrorLogStore` からentries/unreadCount取得
- マウント時に `api.GetSessionErrorLog()` で初期データ取得
- `markAllRead()`, `copyAll()`, `copyEntry()` 関数提供
- auto-scroll to bottom管理

### 4-3. Error Log View (`myT-x/frontend/src/components/viewer/views/error-log/ErrorLogView.tsx` 新規)

構造:
```
.error-log-view
├── .viewer-header (タイトル "Error Log" + コピーボタン + closeボタン)
└── .error-log-body (スクロール可能、user-select: text)
    └── .error-log-entry × N (新しいものが下)
        ├── .error-log-ts (タイムスタンプ表示)
        ├── .error-log-level (error=赤, warn=オレンジ バッジ)
        ├── .error-log-msg (メッセージ本文)
        └── .error-log-source (コンポーネント名、dimmed)
    └── (エントリ0件時) "No errors logged" メッセージ
```

コピー機能 (FileContentViewer準拠):
- Ctrl+C: 選択テキストコピー
- Copy-on-select: 100msデバウンス
- ヘッダーのコピーボタン: 全エントリをテキストコピー

ビューオープン時に `markAllRead()` を呼び出してバッジクリア。

### 4-4. プラグイン登録 (`myT-x/frontend/src/components/viewer/views/error-log/index.ts` 新規)

```typescript
registerView({
    id: "error-log",
    icon: ErrorLogIcon,
    label: "Error Log",
    component: ErrorLogView,
    shortcut: "Ctrl+Shift+L",
});
```

### 4-5. ViewerSystem統合 (`myT-x/frontend/src/components/viewer/ViewerSystem.tsx` 既存)

- `import "./views/error-log";` 追加 (side-effect import)
- `Ctrl+Shift+L` ショートカットハンドラ追加

### 4-6. Activity Stripバッジ (`myT-x/frontend/src/components/viewer/ActivityStrip.tsx` 既存)

- `useErrorLogStore` の `unreadCount` 購読
- `error-log` ビューのボタンに赤いドットバッジ表示 (`unreadCount > 0` の場合)
- `.viewer-strip-badge` CSS class (absolute positioned, 8px赤丸)

### 4-7. CSS (`myT-x/frontend/src/styles/viewer.css` 既存に追記)

- `.viewer-strip-badge`: 赤ドットバッジ
- `.error-log-view`, `.error-log-body`: レイアウト
- `.error-log-entry`: エントリ行（hover, flex）
- `.error-log-ts`: タイムスタンプ
- `.error-log-level.error` / `.error-log-level.warn`: レベルバッジ色分け
- `.error-log-msg`, `.error-log-source`: テキスト
- `.error-log-empty`: 空状態メッセージ

---

## 対象ファイル一覧

### 新規作成
| ファイル | 概要 |
|---------|------|
| `myT-x/app_session_log_types.go` | ログエントリ型定義 |
| `myT-x/internal/sessionlog/handler.go` | slog TeeHandler |
| `myT-x/internal/sessionlog/handler_test.go` | TeeHandlerテスト |
| `myT-x/app_session_log.go` | セッションログサービス |
| `myT-x/app_session_log_test.go` | サービステスト |
| `frontend/src/stores/errorLogStore.ts` | Zustand store |
| `frontend/src/components/viewer/icons/ErrorLogIcon.tsx` | アイコン |
| `frontend/src/components/viewer/views/error-log/index.ts` | プラグイン登録 |
| `frontend/src/components/viewer/views/error-log/ErrorLogView.tsx` | メインビュー |
| `frontend/src/components/viewer/views/error-log/useErrorLog.ts` | カスタムフック |

### 既存編集
| ファイル | 変更内容 |
|---------|---------|
| `myT-x/app.go` | App structにセッションログフィールド追加 |
| `myT-x/app_lifecycle.go` | startup/shutdownにログ初期化/終了追加 |
| `frontend/src/api.ts` | 新規APIメソッド追加 |
| `frontend/src/hooks/useBackendSync.ts` | イベント型・ハンドラ追加 |
| `frontend/src/components/viewer/ViewerSystem.tsx` | import + ショートカット追加 |
| `frontend/src/components/viewer/ActivityStrip.tsx` | バッジ表示追加 |
| `frontend/src/styles/viewer.css` | エラーログ用CSS追加 |

---

## 開発フロー

```
1. confidence-check (Phase 1 backend)
2. golang-expert → backend実装 (types + handler + service + lifecycle)
3. go-test-patterns → テスト作成
4. self-review → backend完了
5. wails generate → バインディング再生成
6. frontend実装 (store → event → hook → view → registry → badge → CSS)
7. self-review → frontend完了
8. 統合テスト (ビルド + 手動検証)
```

---

## 検証手順

1. **ビルド確認**: `build_project` でコンパイルエラーなし
2. **Goテスト**: `go test ./myT-x/... -v` 全テストパス
3. **手動検証**:
   - アプリ起動後、`Ctrl+Shift+L` でError Logビュー表示
   - Activity Stripに警告アイコン表示確認
   - 無効なパスでセッション作成 → エラーログにエントリ表示
   - バッジ（赤ドット）がアイコンに表示
   - ビューを開くとバッジがクリア
   - テキスト選択でコピー動作確認
   - ヘッダーのコピーボタンで全ログコピー
4. **JSONL永続化**: `$LOCALAPPDATA/myT-x/session-logs/session-*.jsonl` ファイル存在確認
5. **ファイルローテーション**: 起動時に100ファイル以下であること確認
