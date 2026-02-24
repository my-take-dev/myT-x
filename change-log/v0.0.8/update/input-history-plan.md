# 入力履歴の右サイドバー表示機能 ─ 実装計画

## 1. 概要と目的

ユーザーがターミナルペインに入力した内容（キーストローク列）の履歴を、右サイドバー（ViewerSystem）上の専用パネルで閲覧できるようにする。

エラー表示機能（`ErrorLogView`）と **同一の管理方法**（バックエンドJSONL永続化 ＋ ping-fetchモデル ＋ zustandストア ＋ Viewerレジストリ）を踏襲し、実装コストと保守コストを最小化する。

### 実現する価値
- 過去に各ペインで何を入力したかを一覧で振り返れる
- JSONL ファイルへの永続化によりセッションログと同等の追跡性を確保
- エラーログと同じ UI パターンのため、ユーザーに馴染みのある操作体験を提供

---

## 2. アーキテクチャ概要（エラー表示との対比）

| 要素 | エラー表示（既存） | 入力履歴（新規） |
|---|---|---|
| **バックエンドファイル** | `app_session_log.go` / `app_session_log_types.go` | `app_input_history.go` / `app_input_history_types.go` |
| **永続化ファイル** | `session-logs/session-*.jsonl` | `input-history/input-*.jsonl` |
| **リングバッファ** | `sessionLogRingBuffer` | `inputHistoryRingBuffer` （同一実装をジェネリクスまたはコピーで再利用） |
| **Wailsバインド API** | `GetSessionErrorLog()` | `GetInputHistory()` |
| **ping イベント** | `app:session-log-updated` | `app:input-history-updated` |
| **フロントエンド Store** | `errorLogStore.ts` | `inputHistoryStore.ts` |
| **フロントエンド Hook** | `useErrorLog.ts` | `useInputHistory.ts` |
| **ビューコンポーネント** | `ErrorLogView.tsx` | `InputHistoryView.tsx` |
| **アイコン** | `ErrorLogIcon.tsx` | `InputHistoryIcon.tsx` |
| **Viewer 登録** | `views/error-log/index.ts` | `views/input-history/index.ts` |
| **ショートカット** | `Ctrl+Shift+L` | `Ctrl+Shift+H`（仮、重複確認後に確定） |
| **ActivityStrip バッジ** | 未読エラー数 | 未読入力数 |

---

## 3. 入力履歴エントリの型定義

### Go 側 (`app_input_history_types.go`)

```go
// InputHistoryEntry は入力履歴の1レコードを表す。
type InputHistoryEntry struct {
    Seq       uint64 `json:"seq"`       // 累積自動インクリメントカウンタ
    Timestamp string `json:"ts"`        // "20060102150405" 形式
    PaneID    string `json:"pane_id"`   // 入力先ペインID
    Input     string `json:"input"`     // ユーザーが入力した文字列
    Source    string `json:"source"`    // "keyboard", "paste", "sync-input" 等
    Session   string `json:"session"`   // セッション名（任意）
}
```

### フロントエンド側 (`inputHistoryStore.ts`)

```typescript
export interface InputHistoryEntry {
    seq: number;
    ts: string;
    pane_id: string;
    input: string;
    source: string;
    session: string;
}
```

---

## 4. 具体的な実装ステップ

### Step 1: バックエンドの入力履歴基盤 (Go)

#### 1-1. 型定義ファイル作成 (`app_input_history_types.go`)
- `InputHistoryEntry` 構造体を定義
- `inputHistoryRingBuffer` を定義（`sessionLogRingBuffer` と同一構造）
  - ※ Go のジェネリクス（1.18+）でリングバッファを共通化できる場合は共通型を検討。コピーでも可。

#### 1-2. 入力履歴ロジック (`app_input_history.go`)
以下の関数を新設する:

```go
// initInputHistory — アプリ起動時にJSONL履歴ファイルを作成
func (a *App) initInputHistory()

// cleanupOldInputHistory — 古い履歴ファイルのローテーション
func (a *App) cleanupOldInputHistory()

// writeInputHistoryEntry — リングバッファ+JSONL書き込み+pingイベント発火
func (a *App) writeInputHistoryEntry(entry InputHistoryEntry)

// GetInputHistory — Wailsバインド: フロントエンドからの取得API
func (a *App) GetInputHistory() []InputHistoryEntry

// GetInputHistoryFilePath — Wailsバインド: ファイルパス取得
func (a *App) GetInputHistoryFilePath() string
```

**設計ポイント（エラーログと同一パターン）:**
- `sync.RWMutex` による排他制御
- `writeInputHistoryEntry` 内でリングバッファ `push` → JSONL ファイル追記 → `app:input-history-updated` イベント発火
- イベント発火のスロットリング（`inputHistoryEmitMinInterval = 50ms`）

#### 1-3. `App` 構造体へのフィールド追加 (`app.go`)

```go
// 入力履歴関連フィールド
inputHistoryMu        sync.RWMutex
inputHistoryBuf       inputHistoryRingBuffer
inputHistoryFile      *os.File
inputHistorySeq       uint64
inputHistoryLastEmit  time.Time
```

#### 1-4. 入力キャプチャのフック箇所

`SendInput` と `SendSyncInput` （`app_pane_api.go`） の内部で、入力をバックエンドの `writeInputHistoryEntry` に記録する。

```go
// SendInput の既存処理の後に追加
func (a *App) SendInput(paneID string, input string) error {
    sessions, err := a.requireSessionsWithPaneID(&paneID)
    if err != nil {
        return err
    }
    if err := sessions.WriteToPane(paneID, input); err != nil {
        slog.Debug("[PANE] SendInput failed", "paneID", paneID, "err", err)
        return err
    }
    // --- 入力履歴の記録 ---
    a.writeInputHistoryEntry(InputHistoryEntry{
        Timestamp: time.Now().Format("20060102150405"),
        PaneID:    paneID,
        Input:     input,
        Source:    "keyboard",
    })
    return nil
}
```

**`SendSyncInput` にも同様に追加**（`Source: "sync-input"`）。

#### 1-5. ライフサイクルへの統合
- `app_lifecycle.go` の `startup` / `shutdown` に `initInputHistory` / `closeInputHistory` を追加
- `initInputHistory` 内で `cleanupOldInputHistory` を呼び出し、古いファイルを自動ローテーション

---

### Step 2: フロントエンド API 更新

`wails dev` 実行後に自動生成されるバインディングから `GetInputHistory` / `GetInputHistoryFilePath` を `api.ts` にエクスポートする。

---

### Step 3: Zustand ストア (`inputHistoryStore.ts`)

`errorLogStore.ts` をテンプレートとして、以下を実装:

```typescript
import { create } from "zustand";

export interface InputHistoryEntry {
    seq: number;
    ts: string;
    pane_id: string;
    input: string;
    source: string;
    session: string;
}

const MAX_ENTRIES = 10000;

interface InputHistoryState {
    entries: InputHistoryEntry[];
    unreadCount: number;
    lastReadSeq: number;
    setEntries: (entries: InputHistoryEntry[]) => void;
    markAllRead: () => void;
}

export const useInputHistoryStore = create<InputHistoryState>((set) => ({
    entries: [],
    unreadCount: 0,
    lastReadSeq: 0,
    setEntries: (incoming) =>
        set((state) => {
            // errorLogStore.ts と同一のバリデーション+正規化ロジック
            // ...
        }),
    markAllRead: () =>
        set((state) => {
            const lastReadSeq = state.entries.at(-1)?.seq ?? state.lastReadSeq;
            return { unreadCount: 0, lastReadSeq };
        }),
}));
```

---

### Step 4: バックエンドイベント購読 (`useBackendSync.ts`)

既存の `useBackendSync` に以下を追加:

#### 4-1. `BackendEventMap` にイベント型を追加
```typescript
"app:input-history-updated": null;  // ping-onlyイベント
```

#### 4-2. fetchとデバウンスの追加
`fetchErrorLog` と同一のパターンで `fetchInputHistory` を実装:

```typescript
let inputHistoryDebounceTimer: ReturnType<typeof setTimeout> | null = null;
let inputHistoryRetryTimer: ReturnType<typeof setTimeout> | null = null;
let inputHistoryFetchSeq = 0;

const fetchInputHistory = (attempt = 0) => {
    if (!isMountedRef.current) return;
    const seq = ++inputHistoryFetchSeq;
    void api.GetInputHistory()
        .then((result) => {
            if (!isMountedRef.current || seq !== inputHistoryFetchSeq) return;
            if (inputHistoryRetryTimer != null) {
                clearTimeout(inputHistoryRetryTimer);
                inputHistoryRetryTimer = null;
            }
            useInputHistoryStore.getState().setEntries(result ?? []);
        })
        .catch((err: unknown) => {
            // エラーログと同一のリトライロジック
        });
};

onEvent("app:input-history-updated", () => {
    if (!isMountedRef.current) return;
    if (inputHistoryDebounceTimer != null) {
        clearTimeout(inputHistoryDebounceTimer);
    }
    inputHistoryDebounceTimer = setTimeout(fetchInputHistory, ERROR_LOG_DEBOUNCE_MS);
});

fetchInputHistory(); // 初回ロード
```

#### 4-3. クリーンアップの追加
`return` 関数内にタイマーの `clearTimeout` を追加。

---

### Step 5: カスタムフック (`useInputHistory.ts`)

`useErrorLog.ts` をテンプレートとして:

```typescript
export function useInputHistory() {
    const entries = useInputHistoryStore((s) => s.entries);
    const unreadCount = useInputHistoryStore((s) => s.unreadCount);
    const markAllRead = useInputHistoryStore((s) => s.markAllRead);
    // ... copyAll, copyEntry, registerBodyElement, formatTimestamp
    return { entries, unreadCount, markAllRead, copyAll, copyEntry, registerBodyElement, formatTimestamp };
}
```

---

### Step 6: ビューコンポーネント (`InputHistoryView.tsx`)

`ErrorLogView.tsx` をテンプレートとして:

- ヘッダー: 「Input History」タイトル ＋ Copy All ＋ Close
- ボディ: エントリ一覧（タイムスタンプ、ペインID、入力内容、ソース）
- 空状態: 「No input history」
- `Ctrl+C` コピー対応
- Copy-on-select 対応
- 自動スクロール（末尾追従）

#### 表示形式
```
2026-02-23 07:50:35  [%5]  ls -la  keyboard
2026-02-23 07:50:40  [%5]  cd /tmp  keyboard
2026-02-23 07:50:45  [%5]  echo "hello"  paste
```

---

### Step 7: アイコンコンポーネント (`InputHistoryIcon.tsx`)

時計マーク付きのターミナルアイコンなどを SVG で作成。既存アイコン（`ErrorLogIcon.tsx` 等）と同一の props インターフェース `{ size?: number }` に準拠。

---

### Step 8: Viewer レジストリへの登録 (`views/input-history/index.ts`)

```typescript
import { registerView } from "../../viewerRegistry";
import { InputHistoryIcon } from "../../icons/InputHistoryIcon";
import { InputHistoryView } from "./InputHistoryView";

registerView({
    id: "input-history",
    icon: InputHistoryIcon,
    label: "Input History",
    component: InputHistoryView,
    shortcut: "Ctrl+Shift+H",
    position: "bottom",     // エラーログと同じく下部配置
});
```

### Step 9: ViewerSystem へのインポート (`ViewerSystem.tsx`)

```typescript
// Side-effect imports: 各ビューの自己登録
import "./views/input-history";  // 追加
```

---

### Step 10: ActivityStrip のバッジ対応 (`ActivityStrip.tsx`)

エラーログの未読バッジと同様に、入力履歴の未読バッジを表示:

```tsx
// 既存の unreadCount と同列に追加
const inputHistoryUnreadCount = useInputHistoryStore((s) => s.unreadCount);

// renderViewButton 内
{view.id === "input-history" && inputHistoryUnreadCount > 0 && (
    <span className="viewer-strip-badge"/>
)}
```

---

## 5. 入力キャプチャの設計詳細

### キャプチャ対象

| 入力ソース | キャプチャ箇所 | `source` 値 |
|---|---|---|
| 通常キーボード入力 | `SendInput` (Go) | `"keyboard"` |
| 同期入力モード | `SendSyncInput` (Go) | `"sync-input"` |
| ファイルドロップ | `SendInput` (Go、`useFileDrop.ts` 経由) | `"keyboard"`（送信元区別が必要なら `LogFrontendEvent` 類似の専用エンドポイントを検討） |

### 入力の集約（バッチング）

ターミナルの `onData` はキー1つごとに発火するため、そのまま全てを記録すると膨大なエントリ数になる可能性がある。

**対策案（段階的に検討）:**
1. **初期実装**: すべての `SendInput` 呼び出しを記録する（シンプル＋確実）
2. **改善版**: バックエンド側で短時間（100ms〜200ms）の入力をバッファリングし、まとめて1エントリとして記録する（JSONL 肥大化防止）
3. **改善版**: Enterキー（`\r`, `\n`）で区切り、コマンド単位で記録する（より直感的な履歴）

→ **初期実装では案2を推奨**。 `writeInputHistoryEntry` 内にタイマーベースのバッチング機構を設け、連続するキーストロークを集約する。

```go
// 入力バッチング（100ms窓）
const inputBatchWindow = 100 * time.Millisecond

// writeInputHistoryEntry 内部で paneID ごとに入力をバッファリングし、
// 100ms間追加入力がなければフラッシュして1エントリとして記録
```

### 入力内容の保存サイズ制限

```go
const (
    inputHistoryMaxInputLen   = 4000 // rune: 1エントリあたりの入力文字数上限
    inputHistoryMaxEntries    = 10000 // リングバッファ容量
    inputHistoryMaxFiles      = 50    // ファイルローテーション上限
)
```

### 制御文字の表示処理

ターミナル入力にはエスケープシーケンスや制御文字が含まれる。フロントエンドで表示する際には:
- `\r` / `\n` → `↵` に変換
- `\t` → `⇥` に変換
- `\x03` (Ctrl+C) → `^C` に変換
- その他の制御文字 → `^X` 形式に変換
- ANSI エスケープシーケンス → 除去（表示上不要なため）

---

## 6. ファイル構成一覧（新規作成・変更対象）

### 新規作成ファイル

| ファイルパス | 役割 |
|---|---|
| `myT-x/app_input_history.go` | バックエンドロジック（JSONL永続化、リングバッファ、API） |
| `myT-x/app_input_history_types.go` | 型定義（`InputHistoryEntry`, リングバッファ） |
| `myT-x/app_input_history_test.go` | ユニットテスト |
| `myT-x/frontend/src/stores/inputHistoryStore.ts` | Zustandストア |
| `myT-x/frontend/src/components/viewer/views/input-history/InputHistoryView.tsx` | ビューコンポーネント |
| `myT-x/frontend/src/components/viewer/views/input-history/useInputHistory.ts` | カスタムフック |
| `myT-x/frontend/src/components/viewer/views/input-history/index.ts` | Viewerレジストリ登録 |
| `myT-x/frontend/src/components/viewer/icons/InputHistoryIcon.tsx` | アイコンコンポーネント |
| `myT-x/frontend/tests/inputHistoryStore.test.ts` | ストアのユニットテスト |

### 変更対象ファイル

| ファイルパス | 変更内容 |
|---|---|
| `myT-x/app.go` | `App` 構造体に入力履歴関連フィールドを追加 |
| `myT-x/app_lifecycle.go` | `startup` / `shutdown` に `initInputHistory` / `closeInputHistory` を追加 |
| `myT-x/app_pane_api.go` | `SendInput` / `SendSyncInput` に `writeInputHistoryEntry` 呼び出しを追加 |
| `myT-x/frontend/src/api.ts` | `GetInputHistory`, `GetInputHistoryFilePath` のエクスポート追加 |
| `myT-x/frontend/src/hooks/useBackendSync.ts` | `app:input-history-updated` イベントの購読・fetchロジック追加 |
| `myT-x/frontend/src/components/viewer/ViewerSystem.tsx` | `import "./views/input-history"` を追加 |
| `myT-x/frontend/src/components/viewer/ActivityStrip.tsx` | 入力履歴の未読バッジを追加 |

---

## 7. テスト計画

### Go ユニットテスト (`app_input_history_test.go`)
- `writeInputHistoryEntry` がリングバッファに正しく追加されること
- `GetInputHistory` がスナップショットを正しく返すこと
- JSONL ファイルへの書き込みが正しいフォーマットであること
- ファイルローテーションが `inputHistoryMaxFiles` を超えた場合に古いファイルを削除すること
- 入力バッチングが正しく集約されること
- `Seq` がモノトニック増加すること
- 入力文字数制限がルーンカウントで正しく適用されること

### フロントエンド ユニットテスト (`inputHistoryStore.test.ts`)
- `setEntries` がバリデーション＋正規化を正しく適用すること
- `markAllRead` が `unreadCount` を 0 にリセットすること
- 初回ロード時に既存エントリが既読扱いになること
- `seq` リグレッション時に `lastReadSeq` がリセットされること

### 統合テスト（手動確認）
1. ターミナルでコマンドを入力 → Input History パネルに表示されること
2. `input-history/input-*.jsonl` に永続化されていること
3. アプリ再起動後にパネルを開くと前セッションの履歴が表示されること
4. Copy All / 個別コピーが動作すること
5. 未読バッジが正しく表示・リセットされること
6. ショートカット `Ctrl+Shift+H` でパネルがトグルされること

---

## 8. 今後の拡展案（スコープ外）

- **検索・フィルタ機能**: ペインID、セッション名、入力内容でフィルタ
- **コマンド補完連携**: 入力履歴から過去のコマンドをペインに挿入
- **タイムライン表示**: エラーログと入力履歴を統合したタイムライン
- **入力再実行**: 履歴エントリをクリックして同じ入力を再送
