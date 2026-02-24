# 入力履歴機能 修正計画 — ラインバッファリングへの変更

## 1. 問題の概要

### 現状の動作

現行実装（`delegated-foraging-pillow.md` に基づく実装）では、`SendInput` がキーストローク1つごとに呼ばれるたびに `recordInput` でバッチングしている。100msウィンドウで時間ベースの集約を行っているが、結果として以下の問題が発生している。

**実際のログ出力** (`input-20260223-162604-16240.jsonl` より)：

```jsonl
{"seq":4,"ts":"20260223162630","pane_id":"%0","input":"c","source":"keyboard","session":""}
{"seq":5,"ts":"20260223162630","pane_id":"%0","input":"la","source":"keyboard","session":""}
{"seq":6,"ts":"20260223162630","pane_id":"%0","input":"ude","source":"keyboard","session":""}
{"seq":7,"ts":"20260223162630","pane_id":"%0","input":"\r","source":"keyboard","session":""}
```

**問題点:**
1. `claude` + Enter と入力したのに、`"c"`, `"la"`, `"ude"`, `"\r"` と4つのエントリに分割されている
2. `\u001b[I]`（フォーカスイン）、`\u001b[O]`（フォーカスアウト）、`\u001b[A]`（矢印キー上）、`\u001b[Z]`（Shift+Tab）など、コマンドと無関係な制御シーケンスがそのまま記録されている
3. ユーザーが期待する「コマンド単位の履歴」にはなっていない

### 期待する動作

ターミナル上で文字を入力し **Enterキー（`\r`）を押して送信した時点** でのテキスト全体を1つの入力履歴エントリとして記録する。

**期待されるログ出力:**
```jsonl
{"seq":1,"ts":"20260223162630","pane_id":"%0","input":"claude","source":"keyboard","session":""}
```

- `claude` と入力してEnterを押した場合 → `Input: claude` と**1エントリ**で記録
- 制御シーケンス（フォーカスイン/アウト、矢印キー等）は履歴に**記録しない**

---

## 2. 根本原因の分析

### xterm.js `onData` の特性

xterm.js の `term.onData()` コールバックはキー入力のたびに発火する：
- 通常文字: `"a"`, `"b"` 等（1文字ずつ、またはブラウザのイベントバッファリングで数文字まとめて）
- Enter: `"\r"`
- 矢印キー: `"\u001b[A"` (Up), `"\u001b[B"` (Down) 等
- フォーカスイベント: `"\u001b[I"` (Focus In), `"\u001b[O"` (Focus Out)
- Ctrl+C: `"\x03"`

### 現行アーキテクチャの問題

```
キー入力 → onData → SendInput(paneID, input) → recordInput → 100ms バッチ → writeEntry
```

この設計では、`recordInput` は「100ms以内に来たキーストロークをまとめる」だけで、Enter送信を区切りとしたコマンド単位の入力認識ができない。また、制御シーケンスもフィルタされず全て記録される。

---

## 3. 修正方針 — ラインバッファリング

### 新アーキテクチャ

**バックエンド（Go側）のバッチング機構を廃止**し、代わりに**ラインバッファリング方式**を採用する。

```
キー入力 → onData → SendInput(paneID, input)
                         ↓
                   recordInput(paneID, input, ...)
                         ↓
                   ペインごとのラインバッファに蓄積
                         ↓
                   "\r" を検出 → フラッシュ → writeEntry（制御シーケンス除去済み）
```

### 設計のポイント

1. **Enter (`\r`) を区切りとしてコマンド単位で記録**
   - `"\r"` を検出した時点でバッファ内容をフラッシュし、1エントリとして書き込む
   - `"\r"` 自体はエントリの `input` フィールドに含めない

2. **制御シーケンスのフィルタリング**
   - フォーカスイン/アウト (`\u001b[I`, `\u001b[O`) は完全に無視（バッファに追加しない）
   - 矢印キー (`\u001b[A-D]`) はバッファに追加しない（シェルの履歴操作であり、入力テキストではない）
   - その他のCSIシーケンス (`\u001b[...`) は全般的に無視
   - Backspace (`\x7f` / `\x08`) はバッファから最後の1文字を削除（テキスト編集の再現）
   - Ctrl+C (`\x03`) は特殊処理：バッファをクリアし、`^C` をエントリとして記録（SIGINT送信の記録として有用）
   - Ctrl+D (`\x04`) も同様、`^D` として記録（EOF送信の記録として有用）

3. **タイムアウトフラッシュの維持**
   - 対話型プログラム（`python`, `node` REPL 等）ではEnterなしで入力が完了する場合もある
   - **5秒間** 追加入力がない場合、バッファ内容をフラッシュして記録する（改行なし入力の救済）
   - このタイムアウトは元の100msバッチウィンドウとは異なり、長めに設定して通常のタイピングには干渉しない

4. **ペースト入力の対応**
   - 複数行ペーストの場合、`\r` ごとに分割して個別エントリとして記録
   - ブラケットペースト (`\u001b[200~` ... `\u001b[201~`) のエスケープシーケンスは除去

---

## 4. 変更対象ファイルと具体的な修正内容

### Step 1: バックエンド型定義の変更

**変更ファイル:** `myT-x/app_input_history_types.go`

#### 1-1. `inputBatch` を `inputLineBuffer` に置き換え

```go
// inputLineBuffer は Enter を区切りとしたペイン単位のラインバッファ。
// ユーザーが入力した文字を蓄積し、'\r' を検出した時点でフラッシュする。
//
// Not safe for concurrent use; callers must hold inputLineBufMu.
type inputLineBuffer struct {
    buf     strings.Builder // 蓄積中の入力テキスト（制御シーケンス除去済み）
    runes   int             // 蓄積中のルーン数
    timer   *time.Timer     // タイムアウトフラッシュ用タイマー（5秒無入力でフラッシュ）
    paneID  string
    source  string
    session string
}
```

#### 1-2. 定数の変更

```go
const (
    inputHistoryDir             = "input-history"
    inputHistoryMaxFiles        = 50
    inputHistoryMaxEntries      = 10000
    inputHistoryEmitMinInterval = 50 * time.Millisecond
    inputHistoryMaxInputLen     = 4000 // rune count

    // inputLineFlushTimeout は非改行入力のタイムアウトフラッシュ間隔。
    // 対話型REPL等でEnterなしに入力が完了する場合の救済として、
    // 5秒間追加入力がなければバッファをフラッシュする。
    inputLineFlushTimeout = 5 * time.Second
)
```

**削除する定数:**
- `inputBatchWindow` (100ms) — ラインバッファリングでは不要

---

### Step 2: バックエンド入力履歴ロジックの変更

**変更ファイル:** `myT-x/app_input_history.go`

#### 2-1. `recordInput` をラインバッファリング方式に書き換え

現行の `recordInput` は時間ベース（100ms）でバッチングしているが、以下のロジックに変更する：

```go
// recordInput はペイン単位のラインバッファに入力を蓄積する。
// '\r' を検出した時点でバッファ内容をフラッシュし、1エントリとして記録する。
// 制御シーケンスはフィルタリングして除去する。
//
// Lock ordering: acquires inputLineBufMu only. Never holds inputLineBufMu while
// acquiring inputHistoryMu.
func (a *App) recordInput(paneID, input, source, session string) {
    if input == "" {
        return
    }

    a.inputLineBufMu.Lock()

    if a.inputLineBuffers == nil {
        a.inputLineBuffers = make(map[string]*inputLineBuffer)
    }

    lb, exists := a.inputLineBuffers[paneID]
    if !exists {
        lb = &inputLineBuffer{paneID: paneID, source: source, session: session}
        a.inputLineBuffers[paneID] = lb
    }

    for _, r := range input {
        switch {
        case r == '\r':
            // Enter検出 — バッファをフラッシュ
            if lb.timer != nil {
                lb.timer.Stop()
                lb.timer = nil
            }
            text := lb.buf.String()
            lb.buf.Reset()
            lb.runes = 0
            a.inputLineBufMu.Unlock()

            if text != "" {
                a.writeInputHistoryEntry(InputHistoryEntry{
                    Timestamp: time.Now().Format("20060102150405"),
                    PaneID:    paneID,
                    Input:     text,
                    Source:    source,
                    Session:   session,
                })
            }

            a.inputLineBufMu.Lock()
            // lb は unlock 中に別の goroutine で操作される可能性があるため再取得
            lb, exists = a.inputLineBuffers[paneID]
            if !exists {
                lb = &inputLineBuffer{paneID: paneID, source: source, session: session}
                a.inputLineBuffers[paneID] = lb
            }

        case r == '\x03': // Ctrl+C
            // バッファクリア + ^C をエントリとして記録
            if lb.timer != nil {
                lb.timer.Stop()
                lb.timer = nil
            }
            lb.buf.Reset()
            lb.runes = 0
            a.inputLineBufMu.Unlock()

            a.writeInputHistoryEntry(InputHistoryEntry{
                Timestamp: time.Now().Format("20060102150405"),
                PaneID:    paneID,
                Input:     "^C",
                Source:    source,
                Session:   session,
            })

            a.inputLineBufMu.Lock()
            lb, exists = a.inputLineBuffers[paneID]
            if !exists {
                lb = &inputLineBuffer{paneID: paneID, source: source, session: session}
                a.inputLineBuffers[paneID] = lb
            }

        case r == '\x04': // Ctrl+D
            // ^D をエントリとして記録
            if lb.timer != nil {
                lb.timer.Stop()
                lb.timer = nil
            }
            bufText := lb.buf.String()
            lb.buf.Reset()
            lb.runes = 0
            a.inputLineBufMu.Unlock()

            entryInput := "^D"
            if bufText != "" {
                entryInput = bufText + " (^D)"
            }
            a.writeInputHistoryEntry(InputHistoryEntry{
                Timestamp: time.Now().Format("20060102150405"),
                PaneID:    paneID,
                Input:     entryInput,
                Source:    source,
                Session:   session,
            })

            a.inputLineBufMu.Lock()
            lb, exists = a.inputLineBuffers[paneID]
            if !exists {
                lb = &inputLineBuffer{paneID: paneID, source: source, session: session}
                a.inputLineBuffers[paneID] = lb
            }

        case r == '\x7f' || r == '\x08': // Backspace / Delete
            // バッファから最後の1文字を削除
            if lb.runes > 0 {
                s := lb.buf.String()
                runes := []rune(s)
                runes = runes[:len(runes)-1]
                lb.buf.Reset()
                lb.buf.WriteString(string(runes))
                lb.runes--
            }
            a.resetLineBufferTimer(lb, paneID)

        case r == '\x1b': // ESC — CSIシーケンスの開始
            // ESC自体は無視。CSIシーケンス全体のスキップは
            // isCSISequence ヘルパーで input 文字列レベルで処理する
            // （後述の processInputString で対応）
            // ここでは単独 ESC としてスキップ
            continue

        case r == '\n': // LF は改行の一部として無視（\r\n の場合）
            continue

        case r == '\t': // Tab — シェル補完のためバッファに影響しない
            continue

        case r < 0x20: // その他の制御文字は無視
            continue

        default:
            // 通常の可視文字 — バッファに追加
            if lb.runes < inputHistoryMaxInputLen {
                lb.buf.WriteRune(r)
                lb.runes++
            }
            a.resetLineBufferTimer(lb, paneID)
        }
    }

    a.inputLineBufMu.Unlock()
}
```

**ただし、上記は概念コード**であり、実際の実装では `input` 文字列にCSIシーケンス（`\u001b[...`）が複数文字にまたがって含まれるため、ルーン単位ではなくバイト/文字列レベルでのCSIパーサーが必要。以下の `processInputString` で対応する。

#### 2-2. CSIシーケンスフィルタリング用ヘルパー

```go
// processInputString は入力文字列からCSIシーケンスを除去し、
// 通常文字・特殊文字（\r, \x03 等）のみを返す。
// 返却される文字列にはCSI/OSCエスケープシーケンスが含まれない。
func processInputString(input string) string {
    var out strings.Builder
    runes := []rune(input)
    i := 0
    for i < len(runes) {
        r := runes[i]
        if r == '\x1b' && i+1 < len(runes) {
            next := runes[i+1]
            if next == '[' {
                // CSI sequence: ESC [ ... (param bytes 0x30-0x3F)* (intermediate 0x20-0x2F)* (final 0x40-0x7E)
                j := i + 2
                for j < len(runes) && runes[j] >= 0x20 && runes[j] <= 0x3F {
                    j++
                }
                for j < len(runes) && runes[j] >= 0x20 && runes[j] <= 0x2F {
                    j++
                }
                if j < len(runes) && runes[j] >= 0x40 && runes[j] <= 0x7E {
                    j++ // skip final byte
                }
                i = j
                continue
            } else if next == ']' {
                // OSC sequence: ESC ] ... ST
                j := i + 2
                for j < len(runes) {
                    if runes[j] == '\x07' { // BEL terminator
                        j++
                        break
                    }
                    if runes[j] == '\x1b' && j+1 < len(runes) && runes[j+1] == '\\' { // ST
                        j += 2
                        break
                    }
                    j++
                }
                i = j
                continue
            }
            // 単独ESCまたは未知のシーケンス — スキップ
            i += 2
            continue
        }
        out.WriteRune(r)
        i++
    }
    return out.String()
}
```

#### 2-3. `resetLineBufferTimer` ヘルパー

```go
// resetLineBufferTimer はラインバッファのタイムアウトタイマーをリセットする。
// inputLineBufMu を保持した状態で呼び出すこと。
func (a *App) resetLineBufferTimer(lb *inputLineBuffer, paneID string) {
    if lb.timer != nil {
        lb.timer.Stop()
    }
    lb.timer = time.AfterFunc(inputLineFlushTimeout, func() {
        a.flushLineBuffer(paneID)
    })
}
```

#### 2-4. `flushLineBuffer` — タイムアウトフラッシュ

```go
// flushLineBuffer はペインのラインバッファをフラッシュし、入力履歴エントリとして記録する。
// タイムアウトコールバックおよびシャットダウンから呼び出される。
func (a *App) flushLineBuffer(paneID string) {
    a.inputLineBufMu.Lock()
    lb, exists := a.inputLineBuffers[paneID]
    if !exists || lb.buf.Len() == 0 {
        a.inputLineBufMu.Unlock()
        return
    }

    text := lb.buf.String()
    source := lb.source
    session := lb.session

    if lb.timer != nil {
        lb.timer.Stop()
        lb.timer = nil
    }
    lb.buf.Reset()
    lb.runes = 0

    a.inputLineBufMu.Unlock()

    a.writeInputHistoryEntry(InputHistoryEntry{
        Timestamp: time.Now().Format("20060102150405"),
        PaneID:    paneID,
        Input:     text,
        Source:    source,
        Session:   session,
    })
}
```

#### 2-5. `flushAllLineBuffers` — シャットダウン用

```go
// flushAllLineBuffers は全ペインのラインバッファをフラッシュする。
// shutdown時に呼び出し、未送信の入力を永続化する。
func (a *App) flushAllLineBuffers() {
    a.inputLineBufMu.Lock()
    paneIDs := make([]string, 0, len(a.inputLineBuffers))
    for id, lb := range a.inputLineBuffers {
        if lb.timer != nil {
            lb.timer.Stop()
            lb.timer = nil
        }
        paneIDs = append(paneIDs, id)
    }
    a.inputLineBufMu.Unlock()

    for _, id := range paneIDs {
        a.flushLineBuffer(id)
    }
}
```

#### 2-6. recordInput の最終形（processInputString 統合版）

上記2-1の概念コードを改良し、最終的には以下のフローとする：

```go
func (a *App) recordInput(paneID, input, source, session string) {
    if input == "" {
        return
    }

    // 1. CSI/OSC シーケンスを除去
    cleaned := processInputString(input)
    if cleaned == "" {
        return // 制御シーケンスのみの入力（フォーカスイン/アウト等）は完全に無視
    }

    // 2. クリーン済みの文字列をルーン走査してバッファリング
    a.inputLineBufMu.Lock()
    defer a.inputLineBufMu.Unlock()

    // ... (以下、ルーン走査でバッファリング + \r/\x03/\x04 検出時のフラッシュ)
    // 注意: フラッシュ時は mutex を一旦 unlock して writeInputHistoryEntry を呼び、再度 lock
}
```

**重要な設計判断:**
- `processInputString` でCSIシーケンスを最初に除去することで、`recordInput` 内のルーン走査ロジックを簡潔に保つ
- CSI除去後に残った文字が `\r`, `\x03`, `\x04`, `\x7f`, `\x08`, 通常文字のみとなるため、分岐がシンプルになる

---

### Step 3: App構造体 + ライフサイクルの変更

**変更ファイル:** `myT-x/app.go`

既存のフィールド名を変更：

```go
// 変更前:
inputBatchMu   sync.Mutex
inputBatches   map[string]*inputBatch

// 変更後:
inputLineBufMu     sync.Mutex
inputLineBuffers   map[string]*inputLineBuffer
```

**変更ファイル:** `myT-x/app_lifecycle.go`

```go
// 変更前:
a.flushAllInputBatches()

// 変更後:
a.flushAllLineBuffers()
```

---

### Step 4: フロントエンドの表示調整

**変更ファイル:** `myT-x/frontend/src/components/viewer/views/input-history/useInputHistory.ts`

`formatInputForDisplay` の変更：

```typescript
// 変更前: 制御文字を変換して表示
//   \r, \n → ↵ / \t → ⇥ / \x03 → ^C / ANSI除去

// 変更後: バックエンド側で制御シーケンスは除去済みなのでシンプルに
export function formatInputForDisplay(input: string): string {
    // バックエンドで CSI シーケンスは除去済み。
    // フロントエンドでは残存する可能性のある制御文字のみ対応。
    return input
        .replace(/[\x00-\x08\x0b\x0c\x0e-\x1f]/g, (ch) => {
            const code = ch.charCodeAt(0);
            return `^${String.fromCharCode(code + 64)}`;
        });
}
```

---

## 5. 削除対象のコード

以下のコード/関数は不要になるため削除する：

| 削除対象 | ファイル | 理由 |
|---------|---------|------|
| `inputBatch` struct | `app_input_history_types.go` | `inputLineBuffer` に置き換え |
| `inputBatchWindow` 定数 | `app_input_history_types.go` | ラインバッファリングでは100msウィンドウ不要 |
| `flushInputBatchByPaneID` | `app_input_history.go` | `flushLineBuffer` に置き換え |
| `flushInputBatch` | `app_input_history.go` | `flushLineBuffer` に統合 |
| `flushAllInputBatches` | `app_input_history.go` | `flushAllLineBuffers` に置き換え |

---

## 6. 入力パターン別の期待動作

| ユーザー操作 | `onData` が送信する文字列 | 記録されるエントリ |
|-------------|------------------------|------------------|
| `claude` + Enter | `"c"`, `"l"`, `"a"`, `"u"`, `"d"`, `"e"`, `"\r"` | `Input: "claude"` × 1エントリ |
| `ls -la` + Enter | `"l"`, `"s"`, `" "`, `"-"`, `"l"`, `"a"`, `"\r"` | `Input: "ls -la"` × 1エントリ |
| ↑キー（履歴選択）+ Enter | `"\u001b[A"`, `"\r"` | バッファが空の状態で `\r` → 記録なし（空入力は無視） |
| Ctrl+C | `"\x03"` | `Input: "^C"` × 1エントリ |
| `hello` + Ctrl+C | `"h"`, `"e"`, `"l"`, `"l"`, `"o"`, `"\x03"` | `Input: "^C"` × 1エントリ（バッファクリア） |
| フォーカスイン/アウト | `"\u001b[I"`, `"\u001b[O"` | 記録なし（CSI除去で空になる） |
| 複数行ペースト | `"line1\rline2\rline3\r"` | `Input: "line1"`, `Input: "line2"`, `Input: "line3"` × 3エントリ |
| Tab（補完） | `"\t"` | 記録なし（タブは無視） |
| Backspace + 修正入力 | `"h"`, `"r"`, `"\x7f"`, `"e"`, `"l"`, `"l"`, `"o"`, `"\r"` | `Input: "hello"` × 1エントリ（`"hr"` → BS → `"h"` → `"ello"` → `"hello"`） |
| REPL入力（Enter なし、5秒放置） | `"x"`, (5秒経過) | `Input: "x"` × 1エントリ（タイムアウトフラッシュ） |

---

## 7. エッジケースと注意事項

### 7-1. Backspaceの実装上の制約

Backspace (`\x7f`) によるバッファ上の文字削除は「見た目の入力」を再現するための**ベストエフォート**である。以下の制約を認識すること：

- シェルがラインエディティングモード（readline等）の場合、Backspaceは手前の文字を削除するため、バッファ操作と一致する
- viモードやEmacs風の複雑なカーソル移動がある場合、完全な再現は不可能（矢印キーによるカーソル移動は無視するため）
- **許容範囲:** 完璧なコマンドライン再現ではなく、「ユーザーが何を入力しようとしたか」の概要が分かれば十分

### 7-2. ペインの削除・再作成

既存の実装と同様、ペイン削除時にラインバッファは残存する。次に同じペインIDが使われた場合、古いバッファが残っている可能性がある。
→ 対策: `KillPane` 時に対象ペインのラインバッファをフラッシュ＆削除する処理を追加する。

### 7-3. ロック順序

既存の設計と同一のロック順序を維持する：
- `inputLineBufMu` → `inputHistoryMu` の順序で取得しない
- フラッシュ時は `inputLineBufMu` でバッファ抽出 → 解放 → `inputHistoryMu` で書き込み

### 7-4. CSIパーサーの堅牢性

`processInputString` のCSIパーサーは、壊れたシーケンス（ESCの後にCSIパラメータが欠損等）に対して安全に動作する必要がある。パニックやバッファオーバーランが起きないよう、境界チェックを徹底すること。

---

## 8. テスト計画

### Go ユニットテスト

#### 新規テスト

| テストケース | 検証内容 |
|------------|---------|
| `TestProcessInputString_CSIRemoval` | CSI シーケンス（`\u001b[A`, `\u001b[I`, `\u001b[O` 等）が除去されること |
| `TestProcessInputString_OSCRemoval` | OSC シーケンスが除去されること |
| `TestProcessInputString_PreservesNormalChars` | 通常の可視文字と `\r`, `\x03` 等が保持されること |
| `TestRecordInput_EnterFlush` | `\r` でバッファがフラッシュされ1エントリとして記録されること |
| `TestRecordInput_IgnoreControlSequences` | フォーカスイン/アウト等でエントリが生成されないこと |
| `TestRecordInput_BackspaceEditing` | Backspaceでバッファの最後の文字が削除されること |
| `TestRecordInput_CtrlC` | `\x03` でバッファクリア + `^C` エントリが記録されること |
| `TestRecordInput_CtrlD` | `\x04` で `^D` エントリが記録されること |
| `TestRecordInput_TimeoutFlush` | 5秒タイムアウトでバッファがフラッシュされること |
| `TestRecordInput_MultilineInput` | 複数の `\r` で複数エントリが正しく分割されること |
| `TestRecordInput_EmptyEnter` | 空バッファで `\r` が来てもエントリが生成されないこと |
| `TestRecordInput_MaxInputLen` | `inputHistoryMaxInputLen` (4000 rune) でバッファが切り詰められること |
| `TestFlushAllLineBuffers` | シャットダウン時に全ペインのバッファがフラッシュされること |

#### 既存テストの修正

- `TestRecordInput` 系のテストは、時間ベースのバッチングテストからラインバッファリングテストに書き換え
- `TestFlushInputBatch*` は `TestFlushLineBuffer*` に名前と内容を変更

### フロントエンド テスト

- `formatInputForDisplay` のテストを修正（制御文字変換のテストケースを簡素化）

---

## 9. 変更のリスク評価

| リスク | 影響度 | 対策 |
|-------|-------|------|
| ラインバッファリングにより一部の入力が失われる | 中 | タイムアウトフラッシュ(5秒)で救済。シャットダウン時も全バッファフラッシュ |
| CSIパーサーのバグでクラッシュ | 高 | テーブル駆動テストで多様なシーケンスをカバー。recover不要（パニックしない設計） |
| Backspace処理でバッファ操作が不整合 | 低 | ベストエフォートと割り切り。ユニットテストでカバー |
| 既存の `recordInput` 呼び出し元への影響 | 低 | API シグネチャは変更なし（`recordInput(paneID, input, source, session)`） |
| `inputBatch` → `inputLineBuffer` のリファクタリング漏れ | 中 | 全参照箇所をgrepで確認。コンパイルエラーで検出可能 |

---

## 10. 実装順序

1. **Step 1:** `processInputString` ヘルパーの実装 + テスト  
   （他のコードに依存しない純粋関数なので最初に実装・テスト可能）

2. **Step 2:** `inputLineBuffer` 型定義の変更 + `inputBatch` 関連コードの削除  
   （コンパイルエラーが出るが、次のステップで修正）

3. **Step 3:** `recordInput` / `flushLineBuffer` / `flushAllLineBuffers` / `resetLineBufferTimer` の実装  
   （コンパイル通過→バックエンドテスト実行可能）

4. **Step 4:** `app.go` / `app_lifecycle.go` のフィールド名・呼び出し修正

5. **Step 5:** フロントエンドの `formatInputForDisplay` 修正

6. **Step 6:** 全テスト実行 + 手動確認

---

## 11. 手動検証シナリオ

1. ターミナルで `claude` と入力してEnter → Input Historyパネルに `claude` が1エントリで表示
2. ターミナルで `ls -la` と入力してEnter → `ls -la` が1エントリで表示
3. ↑キーでシェル履歴を選択してEnter → 矢印キー分は記録されず、選択されたコマンドが空でなければ記録（※シェルが送出するエコーは記録対象外なので、バッファに文字がなければ記録されない）
4. Ctrl+C で中断 → `^C` が記録
5. タブやフォーカスイン/アウト → 記録されない
6. 5秒間何も入力しない → タイムアウトでバッファがフラッシュ
7. `input-history/input-*.jsonl` のJSONLが期待通りのフォーマット
8. アプリ再起動後に前セッション履歴が表示
