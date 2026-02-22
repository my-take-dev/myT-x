# 速度改善案: WebSocket化 + 防御的コーディング並列開発プラン

## Context

次世代AIモデル（1,000トークン/秒 x 10ペイン = 10,000トークン/秒）を前提に、現在のWails IPC (`EventsEmit`) ベースのペインデータ転送を **ローカルWebSocketバイナリストリーム** に移行する。Wails IPCのJSONシリアライズ + WebViewブリッジのオーバーヘッドがボトルネックとなっている。

**変更範囲**: 高頻度ペインデータ(`pane:data:<paneID>`)のみWebSocket化。低頻度イベント(snapshot, config, worker-panic等)は既存Wails IPCを維持。

**併せて**: `defensive-coding-checklist` SKILL.md に完全準拠し、既知のDRY違反・タイマーリーク・停止順序の問題も修正する。

---

## アーキテクチャ概要

```
[変更前] OutputFlushManager → emit callback → EventsEmit("pane:data:"+paneID, string)
                                                    ↓ Wails IPC (JSON + WebViewブリッジ)
                                              Frontend EventsOn

[変更後] OutputFlushManager → emit callback → wsHub.BroadcastPaneData(paneID, []byte)
                                                    ↓ WebSocket Binary (ゼロJSON)
                                              Frontend paneDataStream.onmessage
```

### バイナリプロトコル (Server → Client)
```
[1 byte: paneID長 N (uint8)] [N bytes: paneID (ASCII)] [残り: 生ターミナル出力]
```

### テキストプロトコル (Client → Server / Server → Client)
```json
Client → Server: {"action":"subscribe","paneIds":["%0","%1"]}
Client → Server: {"action":"unsubscribe","paneIds":["%0"]}
Server → Client: {"type":"error","message":"..."}
```

### 接続モデル
- デスクトップアプリ = クライアント1台 → **シングルコネクション設計**
- ポート: `:0`（OS自動割当）→ `App.GetWebSocketURL()` でフロントエンドに通知
- `gorilla/websocket v1.5.3` (既にgo.mod indirect依存)

---

## 並列開発タスク

### Phase 1: インフラ構築 (3エージェント並列)

#### Agent A: `internal/workerutil/` パッケージ新規作成

**新規ファイル:**
- `myT-x/internal/workerutil/recovery.go`
- `myT-x/internal/workerutil/recovery_test.go`

**実装内容:**
```go
// RunWithPanicRecovery は長寿命goroutineのパニックリカバリ+指数バックオフリトライを提供する。
// startPaneFeedWorker, startIdleMonitor, WebSocket Hubの3箇所で共通利用する。
func RunWithPanicRecovery(ctx context.Context, name string, wg *sync.WaitGroup, fn func(ctx context.Context), opts RecoveryOptions)

type RecoveryOptions struct {
    InitialBackoff    time.Duration // default: 100ms
    MaxBackoff        time.Duration // default: 5s
    MaxRetries        int           // default: 10
    OnPanic           func(worker string, attempt int)  // パニック時の通知コールバック
    OnFatal           func(worker string, maxRetries int) // リトライ上限到達時
    IsShutdown        func() bool   // runtimeContext nil check等のシャットダウン検知
}
```

**既存コードの再利用:**
- `app_panic_recovery.go` の `recoverBackgroundPanic()`, `nextPanicRestartBackoff()` の定数とロジックを移植
- `initialPanicRestartBackoff`, `maxPanicRestartBackoff`, `maxPanicRestartRetries` を `workerutil` に移動し、元ファイルからは参照

**防御的コーディングチェックリスト:**
- #60: `defer recover()` 必須
- #61: 指数バックオフ + リトライ上限 + 終了条件チェック(`IsShutdown`)
- #64: `context.WithCancel` の cancel が全exitパスで呼ばれること
- #66: `time.NewTimer` + drain パターン（`time.After` 禁止）
- #111: DRY - 2箇所の同一ロジックを1箇所に集約
- #29: エラーラップ `fmt.Errorf("workerutil: ...: %w", err)`
- #74: 定数に「なぜこの値か」の根拠コメント

**テスト要件:**
- 正常終了（ctx cancel）
- パニック発生→リカバリ→リトライ
- MaxRetries到達→OnFatal呼出
- IsShutdown=true → 即時停止
- バックオフ間隔の指数増加検証
- #126: リトライexhaustionパスのテスト

**触らないこと:** app_pane_feed.go, app_lifecycle.go（Phase 2で統合）

---

#### Agent B: `internal/wsserver/` パッケージ新規作成

**新規ファイル:**
- `myT-x/internal/wsserver/hub.go` - WebSocketサーバー本体
- `myT-x/internal/wsserver/protocol.go` - バイナリフレームエンコード/デコード
- `myT-x/internal/wsserver/hub_test.go`
- `myT-x/internal/wsserver/protocol_test.go`

**hub.go 実装内容:**
```go
type Hub struct {
    mu           sync.RWMutex
    conn         *websocket.Conn   // シングルコネクション
    subscribed   map[string]bool   // paneID → subscribed
    writeMu      sync.Mutex        // gorilla/websocket WriteMessage直列化
    addr         string            // "127.0.0.1:<port>"
    server       *http.Server
    closeOnce    sync.Once
    closed       chan struct{}
    logger       *slog.Logger
}

func NewHub(opts HubOptions) *Hub
func (h *Hub) Start(ctx context.Context) error     // http.Server起動、bgWGにAdd
func (h *Hub) Stop() error                         // graceful shutdown
func (h *Hub) URL() string                         // "ws://127.0.0.1:<port>/ws"
func (h *Hub) BroadcastPaneData(paneID string, data []byte) // OutputFlushManagerから呼ばれる
func (h *Hub) handleWS(w http.ResponseWriter, r *http.Request) // Upgrade + read/write pump
```

**protocol.go 実装内容:**
```go
func EncodePaneData(paneID string, data []byte) []byte  // バイナリフレーム構築
func DecodePaneData(frame []byte) (paneID string, data []byte, err error) // フレーム解析
```

**設計上の重要ポイント:**
- `BroadcastPaneData` は OutputFlushManager goroutine から呼ばれる → `writeMu` で直列化
- readPump: テキストフレーム(JSON) → subscribe/unsubscribe処理
- writePump不要（BroadcastPaneDataから直接Write、writeMuで保護）
- Upgrader: `CheckOrigin: func(r *http.Request) bool { return true }` (ローカルホスト限定)
- サーバーは `127.0.0.1:0` にバインド（外部からアクセス不可）
- `http.Server` の `BaseContext` に app context を設定

**防御的コーディングチェックリスト:**
- #29: `fmt.Errorf("wsserver: ...: %w", err)` エラーラップ統一
- #49: goroutineにapp context渡し
- #54: mu保持中にWriteMessage呼ばない（writeMuは別ロック）
- #56: ロック順序コメント（mu → writeMuの順、逆禁止）
- #60: handleWS内で `defer recover()`
- #66: ping/pong用 `time.NewTimer` + `defer timer.Stop()`
- #136: `defer conn.Close()` 必須
- #138: `sync.Once` で二重Close防止
- #74: ポート0の根拠コメント、バッファサイズの根拠コメント

**テスト要件:**
- サーバー起動→URL取得→クライアント接続→subscribe→BroadcastPaneData→受信検証
- subscribe/unsubscribe のライフサイクル
- 接続なし時のBroadcastPaneData（パニックしない、ログのみ）
- Stop()によるgraceful shutdown
- EncodePaneData/DecodePaneData ラウンドトリップ（テーブル駆動）
- 空paneID、空data、255バイトpaneIDの境界値テスト
- #128: 数値境界テスト

**触らないこと:** app_events.go, app_lifecycle.go（Phase 2で統合）

---

#### Agent C: フロントエンド WebSocket クライアント

**新規ファイル:**
- `myT-x/frontend/src/services/paneDataStream.ts` - モジュールスコープシングルトン

**変更ファイル:**
- `myT-x/frontend/src/hooks/useTerminalEvents.ts` - EventsOn → WS handler切替
- `myT-x/frontend/src/hooks/useBackendSync.ts` - WS接続ライフサイクル追加

**paneDataStream.ts 実装内容:**
```typescript
// モジュールスコープ（React state外）でWebSocket接続を管理。
// 理由: 全ペインで共有する単一接続であり、React再レンダリングを避ける。

let ws: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let reconnectAttempt = 0;
const INITIAL_BACKOFF_MS = 100;
const MAX_BACKOFF_MS = 5000;
const MAX_RECONNECT_RETRIES = 10;

const paneHandlers = new Map<string, (data: Uint8Array) => void>();
const textDecoder = new TextDecoder("utf-8");

export function connect(url: string): void     // WS接続開始
export function disconnect(): void              // WS切断 + タイマークリア
export function registerPaneHandler(paneId: string, handler: (data: Uint8Array) => void): () => void
export function unregisterPaneHandler(paneId: string): void
export function isConnected(): boolean
```

**onmessageハンドラ:**
```typescript
ws.onmessage = (event) => {
    if (!(event.data instanceof ArrayBuffer)) return;
    const view = new Uint8Array(event.data);
    if (view.length < 2) return; // 最小: 1byte長 + 1byte paneID
    const paneIdLen = view[0];
    if (view.length < 1 + paneIdLen) return;
    const paneId = textDecoder.decode(view.subarray(1, 1 + paneIdLen));
    const data = view.subarray(1 + paneIdLen);
    const handler = paneHandlers.get(paneId);
    if (handler) handler(data);
};
```

**再接続ロジック:**
- 指数バックオフ: 100ms → 200ms → 400ms → ... → 5000ms cap
- 最大10回リトライ
- リトライ到達時: `useNotificationStore.getState().addNotification("ターミナル出力の接続に失敗しました", "error")` でUI通知
- 再接続成功時: 全registeredハンドラのpaneIDで自動re-subscribe

**useTerminalEvents.ts 変更:**
```typescript
// 変更前:
const cancelPaneEvent = EventsOn(paneEvent, (data: string) => {
    if (typeof data === "string") { enqueuePendingWrite(data); }
});

// 変更後:
const unregister = registerPaneHandler(paneId, (rawData: Uint8Array) => {
    const text = textDecoder.decode(rawData);
    enqueuePendingWrite(text);
});
```
- `EventsOn` import削除（このファイルからのみ。useBackendSync.tsでは引き続き使用）
- cleanup: `cancelPaneEvent()` → `unregister()`
- 既存のRAFバッチング、IME composition、visibilityガード、disposedフラグは**全て維持**

**useBackendSync.ts 変更:**
```typescript
// useEffect冒頭に追加:
const initWs = async () => {
    try {
        const url = await api.GetWebSocketURL();
        if (url && isMountedRef.current) {
            connect(url);
        }
    } catch (err) {
        console.warn("[DEBUG-WS] GetWebSocketURL failed, falling back to EventsOn", err);
        // NOTE: WS未接続時はuseTerminalEventsのregisterPaneHandlerが
        // データを受け取れないが、これは起動直後の一時的な状態。
        // 再接続ロジックが自動リカバリする。
    }
};
void initWs();

// cleanup追加:
return () => {
    isMountedRef.current = false;
    disconnect(); // WS切断
    // ... 既存cleanup ...
};
```

**防御的コーディングチェックリスト:**
- #84: catch内でaddNotification（console.warnだけでなくUI通知）
- #87: 失敗時のUI通知（再接続失敗、接続エラー）
- #95: async handler内 try/catch
- #96: reconnectTimer の clearTimeout をdisconnect()で確実実行
- #99: TypeScript非nullアサーション(`!`)禁止 → 明示的ガード
- #102: グローバル変数(`window`, `document`)をシャドウしない
- #88: モジュールスコープ変数のHMR安全性確認（vite HMRでモジュール再実行される→状態リセット対応）

**テスト要件:**
- protocol.test.ts: EncodePaneData相当のバイナリフレーム解析テスト（TypeScript側）
- 再接続バックオフ計算ロジックのユニットテスト

**触らないこと:** stores/, components/, vite.config.ts, api.ts（`GetWebSocketURL`はWails自動生成）

---

### Phase 2: バックエンド統合 (1エージェント、Phase 1完了後)

#### Agent D: App統合 + DRYリファクタリング

**変更ファイル:**
- `myT-x/app.go` - Hub フィールド追加、`GetWebSocketURL()` メソッド追加
- `myT-x/app_lifecycle.go` - startup/shutdown に Hub 起動/停止追加、startIdleMonitor を workerutil化
- `myT-x/app_pane_feed.go` - startPaneFeedWorker を workerutil化
- `myT-x/app_panic_recovery.go` - 定数を workerutil に移動、関数は workerutil 内の関数を呼ぶラッパーに
- `myT-x/app_events.go` - `ensureOutputFlusher()` の emit callback を WS broadcast に変更
- `myT-x/internal/config/config.go` - Config に `WebSocketPort` フィールド追加

**app.go 変更:**
```go
import "myT-x/internal/wsserver"

type App struct {
    // ... 既存フィールド ...
    wsHub *wsserver.Hub  // WebSocket hub for pane data streaming
}

// GetWebSocketURL returns the WebSocket URL for frontend pane data streaming.
// Called by frontend on mount to establish the binary data channel.
func (a *App) GetWebSocketURL() string {
    if a.wsHub == nil {
        return ""
    }
    return a.wsHub.URL()
}
```

**app_lifecycle.go startup() 変更:**
```go
// startPaneFeedWorker の前に追加:
hub := wsserver.NewHub(wsserver.HubOptions{
    Addr: fmt.Sprintf("127.0.0.1:%d", cfg.WebSocketPort), // 0 = OS自動割当
})
if err := hub.Start(ctx); err != nil {
    runtimeLogger.Errorf(ctx, "websocket server failed: %v", err)
    a.addPendingConfigLoadWarning(
        "Failed to start WebSocket server. Terminal output may be slower. Error: " + err.Error(),
    )
} else {
    runtimeLogger.Infof(ctx, "websocket server listening: %s", hub.URL())
}
a.wsHub = hub
```

**app_lifecycle.go shutdown() 変更:**
```go
// pipeServer.Stop() の後に追加:
if a.wsHub != nil {
    if err := a.wsHub.Stop(); err != nil {
        runtimeLogger.Warningf(logCtx, "websocket server stop failed: %v", err)
    }
}
```

**app_events.go ensureOutputFlusher() 変更:**
```go
// 変更前:
a.emitRuntimeEventWithContext(ctx, "pane:data:"+paneID, string(flushed))

// 変更後:
if a.wsHub != nil && a.wsHub.HasActiveConnection() {
    a.wsHub.BroadcastPaneData(paneID, flushed)
} else {
    // フォールバック: WS未接続時は既存Wails IPCを使用
    a.emitRuntimeEventWithContext(ctx, "pane:data:"+paneID, string(flushed))
}
```

**startPaneFeedWorker / startIdleMonitor のworkerutil化:**
```go
// 変更前: 80行の同一パニックリカバリループ x 2箇所
// 変更後:
workerutil.RunWithPanicRecovery(ctx, "pane-feed-worker", &a.bgWG, func(ctx context.Context) {
    for {
        select {
        case <-ctx.Done():
            return
        case item := <-ch:
            paneStates.FeedTrimmed(item.paneID, item.chunk)
            putFeedBuffer(item.poolPtr)
        }
    }
}, workerutil.RecoveryOptions{
    OnPanic: func(worker string, attempt int) {
        rtCtx := a.runtimeContext()
        if rtCtx != nil {
            a.emitRuntimeEventWithContext(rtCtx, "tmux:worker-panic", map[string]any{"worker": worker})
        }
    },
    OnFatal: func(worker string, maxRetries int) {
        if fatalCtx := a.runtimeContext(); fatalCtx != nil {
            a.emitRuntimeEventWithContext(fatalCtx, "tmux:worker-fatal", map[string]any{
                "worker": worker, "maxRetries": maxRetries,
            })
        }
    },
    IsShutdown: func() bool { return a.runtimeContext() == nil },
})
```

**stopPaneFeedWorker の修正 (#65):**
```go
func (a *App) stopPaneFeedWorker() {
    if a.paneFeedStop != nil {
        a.paneFeedStop()
        a.paneFeedStop = nil
    }
    // NOTE: bgWG.Wait() はshutdown()内で一括実行される。
    // ここでは cancel() → channel drain の順序を維持する。
    // paneFeedWorker のgoroutineは cancel() 後に ctx.Done() で終了し、
    // channel への送信は既に停止している前提。
    for {
        select {
        case item := <-a.paneFeedCh:
            putFeedBuffer(item.poolPtr)
        default:
            return
        }
    }
}
```

**config.go 変更:**
```go
type Config struct {
    // ... 既存フィールド ...
    // WebSocketPort is the port for the local WebSocket server used for
    // high-throughput pane data streaming. 0 = OS auto-assign (recommended).
    WebSocketPort int `yaml:"websocket_port" json:"websocket_port"`
}
```

**防御的コーディングチェックリスト:**
- #54: outputMu 保持中に wsHub.BroadcastPaneData を呼ばない（ensureOutputFlusher のコールバックはロック外で実行される）
- #55: a.wsHub アクセスは startup/shutdown からのみ（シングルスレッド初期化）
- #65: stopPaneFeedWorker の cancel→drain 順序が正しいことをコメントで明示
- #111: DRY - パニックリカバリ2箇所を workerutil に集約
- #136: Hub.Stop() を shutdown で確実呼出
- #29: エラーラップ統一
- #42: WebSocketPort は int 型（`omitempty` 不使用、0がデフォルト値として有効）

**触らないこと:** internal/wsserver/, internal/workerutil/（Phase 1で完成済み）

---

### Phase 3: テスト・ビルド・検証

#### Agent E: 統合テスト + ビルド検証

1. `go build ./...` でコンパイルエラーなし確認
2. `go vet ./...` で静的解析パス
3. `go test ./internal/workerutil/...` - workerutilテスト実行
4. `go test ./internal/wsserver/...` - wsserverテスト実行
5. `go test ./...` - 全テスト実行
6. 既存テストが壊れていないことを確認

#### Agent F: self-review (必須)

`self-review` スキルを実行し、全指摘をクリアするまで修正を繰り返す。

---

## 重要な設計判断

| 判断 | 理由 |
|------|------|
| pane:dataのみWS化 | 変更範囲最小化。snapshot等は低頻度でWails IPCで十分 |
| シングルコネクション | デスクトップアプリ = クライアント1台。マルチコネクション管理は不要な複雑性 |
| ポート`:0` | ポート競合回避。フロントエンドは `GetWebSocketURL()` API で動的取得 |
| gorilla/websocket | go.mod に既にindirect依存。新規依存追加なし |
| モジュールスコープWS管理 | React Context不要。再レンダリング回避。useTerminalSetup.tsのwebglUnavailableと同じパターン |
| WS未接続時Wails IPCフォールバック | 起動順序問題のグレースフルデグラデーション |
| TextDecoder(frontend) | ArrayBuffer → string のネイティブ高速変換 |

---

## 全エージェント共通指示（レビュアー完封用）

各エージェントに以下を徹底指示:

1. **全ての公開関数にgodoc** — 同語反復禁止（#80）、振る舞い記述（#82）
2. **定数に根拠コメント** — 「なぜこの値か」を必ず記載（#74）
3. **エラーラップ統一** — `fmt.Errorf("パッケージ名: 操作名: %w", err)`（#29）
4. **ロック順序コメント** — struct定義に明記（#56）
5. **デバッグログ** — `slog.Debug("[DEBUG-WS]"` 等のプレフィックス統一（#77）。開発完了まで消さない
6. **テーブル駆動テスト** — 正常系 + 全エラーreturnパスの異常系（#113）
7. **境界値テスト** — 空入力、0、MaxUint8、255バイトpaneID等（#128）
8. **`_ = func()` 禁止** — エラーは処理/ログ/返却（#4）
9. **エンコーディング** — UTF-8 BOM無し

---

## 検証手順

1. **ビルド**: `go build ./...` + `go vet ./...`
2. **ユニットテスト**: `go test ./internal/workerutil/... ./internal/wsserver/...`
3. **全テスト**: `go test ./...`（既存テスト含む）
4. **GoLand**: `build_project` + `get_file_problems` で全ファイル検査
5. **フロントエンド**: `cd myT-x/frontend && npm run build` でTypeScriptコンパイルエラーなし
6. **self-review**: スキル実行→全クリアまで修正ループ

---

## ファイル一覧

### 新規作成
| ファイル | Agent | 概要 |
|---------|-------|------|
| `myT-x/internal/workerutil/recovery.go` | A | パニックリカバリ共通ヘルパー |
| `myT-x/internal/workerutil/recovery_test.go` | A | テスト |
| `myT-x/internal/wsserver/hub.go` | B | WebSocketサーバー本体 |
| `myT-x/internal/wsserver/protocol.go` | B | バイナリフレームエンコード/デコード |
| `myT-x/internal/wsserver/hub_test.go` | B | テスト |
| `myT-x/internal/wsserver/protocol_test.go` | B | テスト |
| `myT-x/frontend/src/services/paneDataStream.ts` | C | WS接続管理シングルトン |

### 変更
| ファイル | Agent | 概要 |
|---------|-------|------|
| `myT-x/app.go` | D | wsHub フィールド + GetWebSocketURL() |
| `myT-x/app_lifecycle.go` | D | Hub startup/shutdown + workerutil化 |
| `myT-x/app_pane_feed.go` | D | workerutil化 |
| `myT-x/app_panic_recovery.go` | D | 定数・ロジックを workerutil に移動 |
| `myT-x/app_events.go` | D | emit callback を WS broadcast に変更 |
| `myT-x/internal/config/config.go` | D | WebSocketPort フィールド追加 |
| `myT-x/go.mod` | D | gorilla/websocket を direct 依存に昇格 |
| `myT-x/frontend/src/hooks/useTerminalEvents.ts` | C | EventsOn → registerPaneHandler |
| `myT-x/frontend/src/hooks/useBackendSync.ts` | C | WS connect/disconnect ライフサイクル |

### 更新が必要な付随ファイル
| ファイル | Agent | 概要 |
|---------|-------|------|
| `config.yaml` (実使用設定) | D | `websocket_port: 0` 追記 |
| `README.md` | D | WebSocket設定項目の説明追記 |
