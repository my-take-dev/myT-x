# 入力履歴機能 実装計画

## Context

ユーザーがターミナルペインに入力した内容の履歴を、右サイドバー（ViewerSystem）で閲覧可能にする。
既存のエラーログ表示機能（`ErrorLogView`）と**同一のアーキテクチャパターン**（JSONL永続化 + ping-fetchモデル + Zustandストア + Viewerレジストリ）を踏襲し、実装コスト・保守コストを最小化する。

**元計画:** `plans/input-history-plan.md`

---

## 実装ステップ

### Step 1: バックエンド型定義 + リングバッファ

**新規:** `myT-x/app_input_history_types.go`

- `InputHistoryEntry` struct (`seq`, `ts`, `pane_id`, `input`, `source`, `session`)
- `inputHistoryRingBuffer` — `sessionLogRingBuffer`（`app_session_log_types.go`）と同一ロジックをコピー
  - `newInputHistoryRingBuffer(capacity)`, `push()`, `snapshot()`, `len()`
  - capacity <= 0 は 1 にクランプ、`snapshot()` は独立コピーを返却

**定数:**
```go
const (
    inputHistoryDir             = "input-history"
    inputHistoryMaxFiles        = 50
    inputHistoryMaxEntries      = 10000
    inputHistoryEmitMinInterval = 50 * time.Millisecond
    inputBatchWindow            = 100 * time.Millisecond
    inputHistoryMaxInputLen     = 4000 // rune count
)
```

**テスト:** リングバッファ push/snapshot/容量クランプ/スナップショット独立性

---

### Step 2: バックエンド入力履歴ロジック（バッチング含む）

**新規:** `myT-x/app_input_history.go`

#### 2-1. バッチング機構

`SendInput` はキー1つごとに発火するため、100msウィンドウでペインごとにバッチングする。

```go
type inputBatch struct {
    input   strings.Builder
    runes   int
    timer   *time.Timer
    paneID  string
    source  string
    session string
}
```

**2つの独立mutex:**
- `inputBatchMu sync.Mutex` — バッチmap+タイマーライフサイクル保護
- `inputHistoryMu sync.RWMutex` — リングバッファ/ファイル/seq/emit保護

**ロック順序:** `inputBatchMu` を保持中に `inputHistoryMu` を取得しない。フラッシュ時は `inputBatchMu` でバッチ抽出→解放→`inputHistoryMu` で書き込み。

**関数:**
- `recordInput(paneID, input, source, session)` — バッチに追加、4000ルーン超は切り詰め、満杯時は即時フラッシュ+新バッチ開始
- `flushInputBatchByPaneID(paneID)` — タイマーコールバックおよびshutdownから呼び出し
- `flushInputBatch(batch)` — バッチをエントリに変換→`writeInputHistoryEntry`
- `flushAllInputBatches()` — shutdown用: 全タイマー停止→全バッチフラッシュ

#### 2-2. 永続化・イベント

- `initInputHistory()` — `app_session_log.go:initSessionLog()` と同一パターン（ディレクトリ作成、PID付きファイル名、`O_CREATE|O_WRONLY|O_APPEND`）
- `writeInputHistoryEntry(entry)` — `app_session_log.go:writeSessionLogEntry()` と同一パターン
  - Lock → seq++ → json.Marshal+Write → ring push → throttle判定 → Unlock → emit outside lock
  - **重要: inputHistoryMu保持中にslog禁止**（TeeHandler再帰デッドロック防止）→ `fmt.Fprintf(os.Stderr, ...)`
  - ctx nilガード後に `emitRuntimeEvent("app:input-history-updated", nil)`
- `cleanupOldInputHistory()` — 最新50ファイルを保持
- `closeInputHistory()` — Lock → file.Close → file = nil → Unlock
- `GetInputHistory() []InputHistoryEntry` — RLock → snapshot → return（Wailsバインド）
- `GetInputHistoryFilePath() string` — RLock → return path（Wailsバインド）

**テスト:**
- writeInputHistoryEntry: JSONL書き込み + リングバッファ追加 + seq単調増加
- バッチング: 100ms内の連続入力が1エントリに集約 / 異なるペインは別エントリ / 4000ルーン切り詰め / 満杯即時フラッシュ / flushAllInputBatches
- init/cleanup/close
- ファイル未初期化時にwrite してもpanicしない（メモリのみ記録）

---

### Step 3: App構造体 + ライフサイクル統合

**変更:** `myT-x/app.go`
- sessionLogフィールド群の直後に入力履歴フィールドを追加:
```go
// Input history state (captures terminal input from SendInput/SendSyncInput).
// Protected by inputHistoryMu (RWMutex).
inputHistoryMu       sync.RWMutex
inputHistoryFile     *os.File
inputHistoryPath     string
inputHistoryEntries  inputHistoryRingBuffer
inputHistoryLastEmit time.Time
inputHistorySeq      uint64

// Input batching state (separate lock from inputHistoryMu).
inputBatchMu   sync.Mutex
inputBatches   map[string]*inputBatch
```
- ロック順序コメントの `Independent locks` に `inputHistoryMu`, `inputBatchMu` を追記
- `NewApp()` に `inputHistoryEntries: newInputHistoryRingBuffer(inputHistoryMaxEntries)` 追加

**変更:** `myT-x/app_lifecycle.go`
- `startup()`: `a.initSessionLog()` の直後に `a.initInputHistory()` を追加
- `shutdown()`: `a.closeSessionLog()` の直前に以下を追加:
```go
a.flushAllInputBatches()  // 全ペンディングバッチをフラッシュ（ファイルがまだ開いている間に）
a.closeInputHistory()
```

---

### Step 4: 入力キャプチャフック

**変更:** `myT-x/app_pane_api.go`

`SendInput()` — 成功した `WriteToPane` の後:
```go
a.recordInput(paneID, input, "keyboard", "")
```

`SendSyncInput()` — 成功した `WriteToPanesInWindow` の後:
```go
a.recordInput(paneID, input, "sync-input", "")
```

- 記録失敗はSendInputの戻り値に影響させない（fire-and-forget）
- inputはTrimしない（意図的な空白・改行を保持、既存パターンと一致）

---

### Step 5: フロントエンドストア

**新規:** `myT-x/frontend/src/stores/inputHistoryStore.ts`

`errorLogStore.ts` と同一パターンのZustandストア:
- `InputHistoryEntry` interface (`seq`, `ts`, `pane_id`, `input`, `source`, `session`)
- `isValidSeq()`, `isValidEntryShape()` — 全6フィールドの型ガード
- `normalizeEntries()` — フィルタ+seq昇順ソート+MAX_ENTRIES(10000)制限
- 初回ロード: 既存エントリを既読扱い
- seq後退でlastReadSeqリセット（バックエンド再起動検出）
- `setEntries`, `markAllRead`

**テスト:** `myT-x/frontend/tests/inputHistoryStore.test.ts`

---

### Step 6: バックエンド同期（useBackendSync）

**変更:** `myT-x/frontend/src/hooks/useBackendSync.ts`

- `BackendEventMap` に `"app:input-history-updated": null` 追加
- `INPUT_HISTORY_DEBOUNCE_MS = 80`, `INPUT_HISTORY_FETCH_RETRY_DELAY_MS = 250`
- `fetchInputHistory` — `fetchErrorLog` と同一パターン（monotonic fetchSeq、isMountedRefガード、1回リトライ）
- `onEvent("app:input-history-updated", ...)` でデバウンス付きfetch
- 初回 `fetchInputHistory()` 呼び出し
- クリーンアップ: debounce/retryタイマーのclearTimeout

---

### Step 7: フロントエンドAPIバインディング

**変更:** `myT-x/frontend/src/api.ts`
- `GetInputHistory`, `GetInputHistoryFilePath` をimport+export

**自動生成:** `wailsjs/go/main/App.js`, `App.d.ts`
- `wails dev` 実行時に自動反映

---

### Step 8: カスタムフック

**新規:** `myT-x/frontend/src/components/viewer/views/input-history/useInputHistory.ts`

`useErrorLog.ts` と同一パターン:
- ストアセレクタ: entries, unreadCount, markAllRead
- `formatInputForDisplay(input)` — 制御文字変換:
  - `\r`, `\n` → `↵` / `\t` → `⇥` / `\x03` → `^C` / その他制御文字 → `^X`
  - ANSIエスケープシーケンス除去: `/\x1B\[[0-9;]*[A-Za-z]/g`
- `copyEntry`, `copyAll` — フォーマット: `{ts} [{pane_id}] {input} ({source})`
- `registerBodyElement` — callback ref
- `formatTimestamp` — 再利用
- 自動スクロール（末尾60px以内の場合）

---

### Step 9: ビューコンポーネント

**新規:** `myT-x/frontend/src/components/viewer/views/input-history/InputHistoryView.tsx`

`ErrorLogView.tsx` と同一パターン:
- タイトル: "Input History"
- 空状態: "No input history"
- `useLayoutEffect` で開いている間markAllRead
- Ctrl+C コピー、copy-on-select（100msデバウンス）、click-to-copy
- callback ref パターン（useRef+useEffect([])ではなく）
- `key={entry.seq}` でReactキー
- エントリ表示: タイムスタンプ、ペインID、入力内容（formatInputForDisplay）、ソースタグ

---

### Step 10: アイコンコンポーネント

**新規:** `myT-x/frontend/src/components/viewer/icons/InputHistoryIcon.tsx`

- `{ size?: number }` props（ErrorLogIconと同一interface）
- ターミナル+時計をモチーフにしたSVGアイコン
- 20x20 viewBox、`currentColor` stroke

---

### Step 11: Viewerレジストリ登録

**新規:** `myT-x/frontend/src/components/viewer/views/input-history/index.ts`
```typescript
registerView({
    id: "input-history",
    icon: InputHistoryIcon,
    label: "Input History",
    component: InputHistoryView,
    shortcut: "Ctrl+Shift+H",
    position: "bottom",
});
```

**変更:** `myT-x/frontend/src/components/viewer/ViewerSystem.tsx`
- `import "./views/input-history";` を追加

---

### Step 12: ActivityStripバッジ

**変更:** `myT-x/frontend/src/components/viewer/ActivityStrip.tsx`
- `useInputHistoryStore` からunreadCount取得
- `view.id === "input-history"` のバッジ表示（errorLogと同一パターン）

---

## 防御的コーディング対応一覧

| チェック項目 | 対応箇所 |
|------------|---------|
| Nil/Bounds (#1-8) | リングバッファsnapshot独立コピー、batch map nil初期化、rune境界切り詰め |
| Error Handling (#9-32) | I/Oエラーは非致命(log+continue)、`_ = func()`禁止、Close エラー収集、エラーラップ統一 |
| Concurrency (#33-53) | emit/sync outside lock、slog禁止(inputHistoryMu下)、ctx nilガード、timer goroutineライフサイクル、2mutex分離でデッドロック防止 |
| Resource Lifecycle (#54-64) | defer close file、timer.Stop() in cleanup、flushAllInputBatches in shutdown |
| Input/API Safety (#65-88) | rune-count truncation、JSON tag snake_case統一、omitemptyなし |
| Frontend Safety (#89-108) | isMountedRef guards、timer cleanup、key={entry.seq}、empty catch禁止、callback ref pattern |
| Change Propagation (#109-114) | App struct追加→NewApp更新、Wailsバインディング自動生成、ロック順序コメント更新 |
| Code Integrity (#115-136) | 定数根拠コメント、ロック外I/OのNOTEコメント、DRY(errorLogと同一パターン踏襲) |

---

## 開発フロー

各ステップで:
1. `defensive-coding-checklist` の該当項目を走査
2. `golang-expert` (Go) / `xterm-react-frontend` (Frontend) エージェントで実装
3. テスト作成（テーブル駆動テスト）
4. `self-review` → 全クリアまで修正

**並列実行可能なステップ:**
- Step 1-4 (Backend) → 独立してGoテスト実行可能
- Step 5, 8, 9, 10 (Frontend) → 相互独立
- Step 6 は Step 5 に依存
- Step 11-12 は Step 8-10 に依存

---

## 検証方法

### 自動テスト
- `go test ./myT-x/... -run TestInputHistory` — バックエンド全テスト
- フロントエンドストアテスト（vitest）

### 手動確認
1. ターミナルでコマンド入力 → Input Historyパネルに表示
2. `input-history/input-*.jsonl` にJSONL永続化
3. アプリ再起動後に前セッション履歴が表示
4. Copy All / 個別コピーが動作
5. 未読バッジの表示・リセット
6. `Ctrl+Shift+H` でパネルトグル
7. 高速連続入力が100msバッチで集約されること

### ビルド検証
- `build_project` でコンパイルエラーなし
- `get_file_problems` で警告なし
