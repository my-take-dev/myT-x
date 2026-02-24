# 入力履歴 ラインバッファリング修正実装計画

## Context

`plans/input-history-fix-plan.md` の通り、現行の100msバッチング方式では `claude` + Enter が `"c"`, `"la"`, `"ude"`, `"\r"` と分割記録される問題がある。
Enter (`\r`) を区切りとしたラインバッファリング方式に変更し、コマンド単位の入力履歴を実現する。

---

## 開発フロー

```
confidence-check → 実装 → テスト作成 → self-review
```

skills:
- `defensive-coding-checklist`（並行処理・ロック順序）
- `go-performance-optimization`（ロック粒度）
- `go-test-patterns`（テーブル駆動テスト）

---

## 変更ファイルと実装内容

### Step 1: `app_input_history_types.go`

**削除:**
- `inputBatch` struct
- `inputBatchWindow` 定数 (100ms)

**追加:**
```go
type inputLineBuffer struct {
    buf     strings.Builder
    runes   int
    timer   *time.Timer
    paneID  string
    source  string
    session string
}

const inputLineFlushTimeout = 5 * time.Second
```

---

### Step 2: `app_input_history.go`

**削除する関数:**
- `flushInputBatchByPaneID()`
- `flushInputBatch()`
- `flushAllInputBatches()`

**追加する関数:**

#### `processInputString(input string) string`
- CSI シーケンス (`\x1b[...`) を除去
- OSC シーケンス (`\x1b]...`) を除去
- 通常文字・`\r`・`\x03`・`\x04`・`\x7f`・`\x08` を保持
- 純粋関数（副作用なし）→ 最初に実装・テスト可能

#### `recordInput(paneID, input, source, session string)`
現行の100msバッチ方式を以下に書き換え:
1. `processInputString(input)` でCSIを除去（空になれば即return）
2. `inputLineBufMu.Lock()` でバッファ取得
3. ルーン走査:
   - `\r` → バッファフラッシュ → `writeInputHistoryEntry` （unlock→write→lock再取得）
   - `\x03` → バッファクリア + `"^C"` エントリ記録
   - `\x04` → `"^D"` エントリ記録（バッファ残があれば `"bufText (^D)"`）
   - `\x7f`/`\x08` → バッファ末尾1文字削除 + タイマーリセット
   - `\n`/`\t`/その他制御文字 → スキップ
   - 通常文字 → バッファ追加（maxLen超で切り捨て）+ タイマーリセット
4. `inputLineBufMu.Unlock()`

#### `resetLineBufferTimer(lb *inputLineBuffer, paneID string)`
- 既存タイマー停止 → `time.AfterFunc(inputLineFlushTimeout, ...)` でリセット
- `inputLineBufMu` 保持中に呼び出す

#### `flushLineBuffer(paneID string)`
- `inputLineBufMu.Lock()` → バッファ抽出 → `Unlock()` → `writeInputHistoryEntry()`
- バッファ空ならスキップ

#### `flushAllLineBuffers()`
- 全ペインのタイマー停止 → 各ペインを `flushLineBuffer()` で処理

---

### Step 3: `app.go`

フィールド名変更:
```go
// 変更前
inputBatchMu  sync.Mutex
inputBatches  map[string]*inputBatch

// 変更後
inputLineBufMu   sync.Mutex
inputLineBuffers map[string]*inputLineBuffer
```

---

### Step 4: `app_lifecycle.go`

```go
// 変更前
a.flushAllInputBatches()

// 変更後
a.flushAllLineBuffers()
```

---

### Step 5: `app_input_history_test.go`

**書き換え対象テスト（バッチ → ラインバッファ）:**

| 旧テスト | 新テスト |
|---------|---------|
| `TestRecordInput_*` (100ms) | `TestRecordInput_EnterFlush`, `TestRecordInput_IgnoreControlSequences` 等 |
| `TestFlushInputBatch*` | `TestFlushLineBuffer*` |

**新規テスト:**
- `TestProcessInputString_CSIRemoval`
- `TestProcessInputString_OSCRemoval`
- `TestProcessInputString_PreservesNormalChars`
- `TestRecordInput_EnterFlush`
- `TestRecordInput_IgnoreControlSequences`
- `TestRecordInput_BackspaceEditing`
- `TestRecordInput_CtrlC`
- `TestRecordInput_CtrlD`
- `TestRecordInput_TimeoutFlush`
- `TestRecordInput_MultilineInput`
- `TestRecordInput_EmptyEnter`
- `TestRecordInput_MaxInputLen`
- `TestFlushAllLineBuffers`

---

### Step 6: `frontend/src/components/viewer/views/input-history/useInputHistory.ts`

`formatInputForDisplay` を簡素化:
```typescript
// バックエンドで CSI 除去済み → 残存制御文字のみ対応
export function formatInputForDisplay(input: string): string {
    return input.replace(/[\x00-\x08\x0b\x0c\x0e-\x1f]/g, (ch) => {
        const code = ch.charCodeAt(0);
        return `^${String.fromCharCode(code + 64)}`;
    });
}
```

---

## ロック順序（defensive-coding-checklist 準拠）

```
inputLineBufMu → (unlock) → inputHistoryMu
```
- フラッシュ時: `inputLineBufMu` でバッファ抽出 → Unlock → `writeInputHistoryEntry`（内部でRLock/Lock）
- `inputLineBufMu` を保持したまま `inputHistoryMu` を取得しない

---

## 実装順序

1. `processInputString` 実装 + `TestProcessInputString_*` テスト（純粋関数なので独立）
2. `app_input_history_types.go` の型/定数変更
3. `app.go` のフィールド名変更
4. `app_input_history.go` の新関数実装（旧関数削除）
5. `app_lifecycle.go` の呼び出し修正
6. `app_input_history_test.go` のテスト書き換え
7. `useInputHistory.ts` の `formatInputForDisplay` 修正
8. `go build ./...` → `go test ./...` → `build_project` で確認

---

## 検証方法

### 自動テスト
```bash
go test ./myT-x/... -v -run "TestProcessInputString|TestRecordInput|TestFlushLineBuffer|TestFlushAllLine"
```

### 手動検証
1. アプリ起動 → ターミナルで `claude` + Enter → Input History に `"claude"` が1エントリ
2. `ls -la` + Enter → `"ls -la"` が1エントリ
3. ↑矢印キー + Enter → バッファ空なので記録なし
4. Ctrl+C → `"^C"` が記録
5. フォーカスイン/アウト → 記録なし
6. `input-history/input-*.jsonl` のJSONLフォーマット確認
