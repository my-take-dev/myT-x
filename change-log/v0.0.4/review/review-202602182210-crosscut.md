# Cross-Cutting Review: myT-x v0.0.4 差分 (53ファイル)

レビュー日時: 2026-02-18 22:10
対象: diff_app_core.txt / diff_internal_tmux.txt / diff_shim.txt / diff_frontend.txt / diff_tests.txt

---

## 1. サイレント障害ハンター

### [Critical] C-1: tmuxStore に `activeWindowId` が追加されたが、どのイベントハンドラからもセットされない

- **ファイル**: `frontend/src/stores/tmuxStore.ts` (TmuxState / setActiveWindowId)
- **ファイル**: `frontend/src/hooks/useBackendSync.ts` (全ハンドラ)

`tmuxStore.ts` に `activeWindowId: string | null` と `setActiveWindowId` が新規追加されているが、
`useBackendSync.ts` のどのイベントハンドラ (`tmux:snapshot`, `tmux:snapshot-delta`, `tmux:active-session` 等) からも
`setActiveWindowId` が呼ばれていない。初期値 `null` のまま永続的に放置される。

**リスク**: 将来この値を参照するコンポーネントが追加された場合、常に `null` を返し、フォールバックロジックに依存する形で動作が不安定になる。意図的な将来準備であっても、TODO コメントまたは未使用警告の抑制コメントが必要。

**修正案**: (a) snapshot/snapshot-delta ハンドラ内で activeWindowId を更新するか、(b) 未使用なら削除するか、(c) TODO コメントを追加。

---

### [Important] I-1: `BackendEventMap` の全ペイロードが `unknown` で型保証が弱い

- **ファイル**: `frontend/src/hooks/useBackendSync.ts` (BackendEventMap)

```typescript
interface BackendEventMap {
    "config:load-failed": unknown;
    "tmux:snapshot": unknown;
    // ... all unknown
}
```

`onEvent` ジェネリクス化 (I-37) は良い改善だが、全値が `unknown` のため、実質的な型安全性は旧 `...data: any[]` から向上していない。`payload` を受け取った後、各ハンドラ内で `asObject` / `asArray` による手動パースが従来通り必要。

**修正案**: 最低限、以下のような具体的ペイロード型を定義すると段階的にタイプセーフにできる:
```typescript
"tmux:snapshot": SessionSnapshot[];
"tmux:snapshot-delta": Partial<SessionSnapshotDelta>;
"tmux:active-session": { name?: unknown };
```

---

### [Important] I-2: `splitWindowResolved` の extraEnv マージに `isBlockedEnvironmentKey` チェックが追加されたが、`buildPaneEnv` にはない

- **ファイル**: `internal/tmux/command_router_handlers_pane.go` (splitWindowResolved 内 extraEnv ループ)
- **ファイル**: `internal/tmux/command_router.go` (buildPaneEnv)

`splitWindowResolved` では extraEnv を合流させる際に `isBlockedEnvironmentKey` をチェックしているが、
`buildPaneEnv` は `copyEnvMap(reqEnv)` をそのまま利用しており、`reqEnv` に PATH / SYSTEMROOT 等のブロック対象キーが含まれていても除外されない。

**影響箇所**: `handleNewSession` と `handleNewWindow` は `buildPaneEnv(req.Env, ...)` を使用。`req.Env` は shim 経由のリクエスト環境変数であり、通常これらのキーは含まれないが、防御としては不統一。

**修正案**: `buildPaneEnv` 内の `copyEnvMap` 後にブロックキー除外を追加するか、`copyEnvMap` 自体にフィルタオプションを追加。

---

### [Suggestion] S-1: `handleListPanes` のソート削除後、出力順序が暗黙的にウィンドウ登録順に依存

- **ファイル**: `internal/tmux/command_router_handlers_pane.go` (handleListPanes)

旧コードはセッションID → ウィンドウID → ペインインデックスで明示的にソートしていた。新コードは value copy のため `Window=nil` でソート不能となり、削除された。出力順は `ListPanesByWindowTarget` のウィンドウ走査順（session.Windows のスライス順）に暗黙的に依存する。

現開発フェーズでは後方互換不要だが、shim 経由の `list-panes` を利用するスクリプト（Claude Code 等）が順序に依存している場合、挙動変更となる。

**修正案**: `copyPaneSlice` で Index をコピーしているため、value copy に対して Index ベースのソートを追加すれば安全。

---

### [Suggestion] S-2: `emitLayoutChangedForSession` のログレベルが Debug → Warn に変更

- **ファイル**: `internal/tmux/command_router_handlers_pane.go` (emitLayoutChangedForSession)

```go
-		slog.Debug("["+debugTag+"] "+message, "session", sessionName)
+		slog.Warn("["+debugTag+"] "+message, "session", sessionName)
```

セッション削除直後のレイアウトイベント発行失敗は正常な遷移パスで発生しうる（kill-session → layout snapshot の間にセッションが消える）。Warn レベルにすると、正常なセッション終了操作でワーニングログが出力され、運用時のログノイズになる。

**修正案**: セッション・ペイン不在のケースは Debug のまま維持し、layout snapshot 取得の _実際のエラー_ のみ Warn とする分離が望ましい。

---

## 2. 型設計分析

### [Important] I-3: `RemoveWindow` / `RemoveWindowByID` の戻り値が 3 → 4 に拡張

- **ファイル**: `internal/tmux/session_manager_windows.go` (RemoveWindow, RemoveWindowByID, removeWindowAtIndexLocked)

戻り値に `survivingWindowID int` が追加された。全呼び出し元の更新確認:

| 呼び出し元 | 更新済み |
|---|---|
| `handleKillWindow` (command_router_handlers_window.go) | ✅ |
| `handleNewWindow` rollbackWindow (command_router_handlers_window.go) | ✅ |
| `TestAddWindowDefaultNameUsesUniqueWindowID` (session_manager_test.go) | ✅ |

戻り値数の変更はコンパイルエラーで検出されるため、漏れリスクは低い。

---

### [Important] I-4: `ListPanesByWindowTarget` の戻り値が `[]*TmuxPane` → `[]TmuxPane` に変更

- **ファイル**: `internal/tmux/session_manager_pane_io.go` (ListPanesByWindowTarget, copyPaneSlice)

安全な値コピーへの変更は適切。呼び出し元確認:

| 呼び出し元 | 対応状況 |
|---|---|
| `handleListPanes` | ✅ `for i := range panes { &panes[i] }` で対応 |
| `resolveDirectionalPane` (旧) | ✅ `ResolveDirectionalPane` に置換済み |

value copy では `Window=nil`, `Terminal=nil` のため、コピー後にこれらを参照するコードがないことを確認済み。

---

### [Suggestion] S-3: `DirectionalPaneDirection` の型が `int` ベース

- **ファイル**: `internal/tmux/session_manager_targets.go` (DirectionalPaneDirection)

```go
type DirectionalPaneDirection int
const (
    DirPrev DirectionalPaneDirection = iota
    DirNext
    DirNone
)
```

Go の iota enum は型安全性が限定的（任意の int を代入可能）。現在は呼び出し元が `resolveDirectionalPane` の 1 箇所のみのため問題ないが、公開型として拡張される場合は `String()` メソッドの追加を推奨。

---

## 3. 副作用分析

### [Important] I-5: `handleNewSession` が `-d` フラグに関わらず `session-created` を発行

- **ファイル**: `internal/tmux/command_router_handlers_session.go` (handleNewSession)

```go
// I-16: Emit session-created regardless of -d flag.
// The -d flag controls focus (detach), not whether the session was created.
```

旧コードは `-d` (detached) 時に `session-created` を発行しなかった。新コードはフラグに関わらず発行する。

**影響分析**:
- `session-created` は `snapshotEventPolicies` に `{trigger: true, bypassDebounce: true}` で登録
- フロントエンドの `useBackendSync.ts` は `session-created` を直接ハンドルしない（snapshot/delta 経由で更新）
- Agent Teams 連携で `-d` による子プロセス生成時、以前は snapshot が省略されていたのが、毎回 snapshot + delta が発行される

**結論**: 正しい修正。`-d` は「フォアグラウンドにしない」であり「イベントを抑制する」ではない。フロントエンド側への副作用はない。

---

### [Important] I-6: `cloneSessionForRead` の nil ペインフィルタリングと ActivePN 再計算

- **ファイル**: `internal/tmux/session_manager_sessions.go` (cloneSessionForRead)

```go
windowCopy.Panes = make([]*TmuxPane, 0, len(window.Panes))
for srcIdx, pane := range window.Panes {
    if pane == nil { continue }
    if srcIdx == window.ActivePN {
        windowCopy.ActivePN = len(windowCopy.Panes)
    }
    // ...
    windowCopy.Panes = append(windowCopy.Panes, paneCopy)
}
```

**動作変更**: 旧コードは nil ペインも含めて固定長スライスを生成（nil ホールあり）。新コードは nil を除外しコンパクトなスライスを生成。

**リスク**: `ActivePN` がもともと nil ペインのインデックスを指していた場合、新コードでは `windowCopy.ActivePN = 0`（初期値のまま）になる。これは自然なフォールバックだが、テストで明示的にカバーされているか確認が必要。

**確認事項**: `TestCloneSessionSnapshotsIndependence` テストでは nil ペインを含むケースがテストされていない。nil ペインの ActivePN マッピングテストを追加推奨。

---

### [Suggestion] S-4: `ApplyLayoutPreset` のエラーメッセージ変更

- **ファイル**: `myT-x/app_pane_api.go` (ApplyLayoutPreset)

旧コードでは `session.ActiveWindowID < 0` の場合に `"session has no windows"` を返していたが、新コードでは `ApplyLayoutPresetToActiveWindow` 内から同じメッセージが返る。エラーの発生源は変わるが、メッセージは同じなので呼び出し元への影響はない。

---

## 4. コード簡素化

### [Positive] P-1: `requireSessionsWithPaneID` による 7 メソッドの共通化

- **ファイル**: `myT-x/app_guards.go` (requireSessionsWithPaneID)
- **ファイル**: `myT-x/app_pane_api.go` (SendInput, SendSyncInput, ResizePane, FocusPane, RenamePane, KillPane, GetPaneEnv)

```go
func (a *App) requireSessionsWithPaneID(paneID *string) (*SessionManager, error) {
    *paneID = strings.TrimSpace(*paneID)
    if *paneID == "" { return nil, errors.New("pane id is required") }
    return a.requireSessions()
}
```

`TrimSpace + empty check + requireSessions` の繰り返しパターンが 7 箇所で統一されている。`SplitPane` が対象外である理由もコメントで明記されている。

---

### [Positive] P-2: `bestEffortSendKeys` / `buildPaneEnv` によるハンドラ簡素化

- **ファイル**: `internal/tmux/command_router.go` (bestEffortSendKeys, buildPaneEnv)

3 箇所の send-keys ブートストラップ（new-session, new-window, split-window）と 2 箇所の env 構築が共通化。defensive copy (`argsWithEnter := make(...)`) も一元管理。

---

### [Positive] P-3: `snapshotEventPolicies` によるデータ駆動化

- **ファイル**: `myT-x/app_events.go` (snapshotEventPolicies)

2 つの switch 文が 1 つの map に統合。新イベント追加時にエントリを 1 行追加するだけで trigger/bypassDebounce の両方が設定できる。

---

### [Positive] P-4: `resolveSessionState` によるストア更新ロジック共通化

- **ファイル**: `frontend/src/stores/tmuxStore.ts` (resolveSessionState)

`setSessions` と `applySessionDelta` で重複していた `buildOrder → sortByOrder → activeSession フォールバック` が 1 関数に集約。

---

### [Positive] P-5: `usePrefixKeyMode` の Ref 同期パターン廃止

- **ファイル**: `frontend/src/hooks/usePrefixKeyMode.ts`

旧: `sessions, activeSession, zoomPaneId` を個別 useRef + useEffect で同期（4 つの Ref + 4 つの Effect）
新: `useTmuxStore.getState()` をハンドラ内で直接呼び出し

stale closure リスクの根本排除 + コード量削減。`prefixModeRef` のみ timer callback のために Ref 維持。設計判断のコメントも明確。

---

## 5. コメント精度

### [Suggestion] S-5: `resolveSessionTargetLocked` の Locked suffixが実際のロック要件と不一致

- **ファイル**: `internal/tmux/session_manager_targets.go` (resolveSessionTargetLocked)

```go
// resolveSessionTargetLocked resolves a session from target. Read-only operation.
// Despite the "Locked" suffix, this is safe under either RLock or Lock.
```

`Locked` suffix は session_manager_windows.go の命名規則（I-11）では「caller must hold m.mu (write lock)」を意味する。しかしこの関数はコメントで「RLock or Lock どちらでも安全」と述べている。

**修正案**: `resolveSessionTargetUnderLock` や suffix なし（純粋関数に近いため）に変更するか、命名規則に「Locked = requires any lock (R or W)」の追加定義を加える。

---

### [Suggestion] S-6: `copySnapshotCache` の Safety コメントが SessionSnapshot のフィールド追加時の更新を要求

- **ファイル**: `myT-x/app_snapshot_delta.go` (copySnapshotCache)

```go
// IMPORTANT: if SessionSnapshot gains mutable pointer or slice fields that are
// modified after snapshot collection, this function must be updated to perform
// a deep copy of those fields.
```

これは有益なコメントだが、フィールド追加時に開発者がこのコメントを発見する保証がない。`TestSnapshotFieldCounts` のようなフィールド数ガードテストを copySnapshotCache にも追加すると確実。

---

## 6. ロック・並行処理の横断分析

### [Important] I-7: `ResolveTarget` の RLock → Lock アップグレード間の競合ウィンドウ

- **ファイル**: `internal/tmux/session_manager_targets.go` (ResolveTarget)

```go
func (m *SessionManager) ResolveTarget(target string, callerPaneID int) (*TmuxPane, error) {
    pane, needsRepair, err := m.resolveTargetRLocked(target, callerPaneID)
    if !needsRepair { return pane, nil }
    return m.resolveTargetWriteLocked(target, callerPaneID)
}
```

RLock 解放 → Lock 取得の間に別ゴルーチンがセッション/ペインを変更する可能性がある。
`resolveTargetWriteLocked` 内で完全に再解決しているため、ロジック的には安全。

**しかし**: RLock パスで返された `pane` ポインタは、Lock パスに進む場合に使われない（再解決されるため破棄）。
これは正しい実装だが、`needsRepair=true` の場合に RLock パスの pane を return しないことが重要。
現コードは `if !needsRepair { return pane, nil }` で正しくガードされている。✅

---

### [Important] I-8: `ResolveDirectionalPane` が Lock を使用する理由の整合性

- **ファイル**: `internal/tmux/session_manager_targets.go` (ResolveDirectionalPane)

```go
func (m *SessionManager) ResolveDirectionalPane(callerPaneID int, direction DirectionalPaneDirection) (*TmuxPane, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
```

旧 `resolveDirectionalPane` は 3 回の個別ロック取得（ResolveTarget, ListPanesByWindowTarget, ResolveTarget）による TOCTOU があった。
新コードは 1 回の Lock で全操作をアトミックに実行。
ただし、内部で `m.defaultPaneLocked()` を呼ぶ可能性があり、これは `activeWindowInSessionLocked` → `markStateMutationLocked` を呼ぶ可能性がある。
Lock（Write Lock）を使用しているのはこの書き込み可能性のためであり、正当。✅

---

### [Important] I-9: `app.go` のロック順序ドキュメントと実装の整合性

- **ファイル**: `myT-x/app.go` (コメント)

```go
// Lock ordering (outer -> inner):
//   snapshotDeltaMu -> snapshotMu  (snapshotDelta acquires snapshotMu while holding snapshotDeltaMu)
```

`snapshotDelta()` メソッド (app_snapshot_delta.go) を確認:
```go
a.snapshotDeltaMu.Lock()   // outer
...
a.snapshotMu.Lock()         // inner
```

ロック順序の宣言と実装が一致。✅

旧コメントで `snapshotMu, snapshotDeltaMu` が independent locks リストに含まれていたのが、
新コメントでは除外されている。整合性あり。✅

---

### [Suggestion] S-7: `activePaneInSessionRLocked` の needsRepair シグナリングの網羅性

- **ファイル**: `internal/tmux/session_manager_targets.go` (activePaneInSessionRLocked)

```go
func (m *SessionManager) activePaneInSessionRLocked(session *TmuxSession) (*TmuxPane, bool, error) {
    activeWindow, fallback := findWindowByID(session.Windows, session.ActiveWindowID)
    if activeWindow != nil {
        pane, err := activePaneInWindow(activeWindow)
        return pane, false, err  // needsRepair=false
    }
    // ActiveWindowID is stale → needsRepair=true
```

`findWindowByID` が `activeWindow=nil, fallback=nil` を返す場合（全 Window が nil）：
→ `return nil, false, errors.New("session has no windows")`

ここで `needsRepair=false` で error を返すが、全 Window が nil というのは異常状態。
Lock パスでの修復も不要だが、この状態自体のログ出力が望ましい。

---

## Summary / タスク整理

### 修正タスク一覧

| # | 種別 | 概要 | ファイル | 並列作業可否 |
|---|------|------|----------|-------------|
| C-1 | Critical | `activeWindowId` が未使用: 削除 or TODO追加 or ハンドラ実装 | tmuxStore.ts, useBackendSync.ts | 単独作業 |
| I-1 | Important | BackendEventMap の payload 型を具体化 | useBackendSync.ts | C-1 と並列可 |
| I-2 | Important | `buildPaneEnv` に blockedKey フィルタ追加 | command_router.go | 単独作業可 |
| I-5 | Important | `-d` フラグ変更の Agent Teams 影響確認 | command_router_handlers_session.go | 確認のみ、他と並列可 |
| I-6 | Important | cloneSessionForRead の nil ActivePN テスト追加 | session_manager_sessions.go, テスト | I-2 と並列可 |
| S-1 | Suggestion | handleListPanes の出力順序安定化 | command_router_handlers_pane.go | 単独作業可 |
| S-2 | Suggestion | emitLayoutChanged のログレベル見直し | command_router_handlers_pane.go | S-1 と並列可 |
| S-5 | Suggestion | resolveSessionTargetLocked の命名修正 | session_manager_targets.go | 単独作業可 |
| S-6 | Suggestion | copySnapshotCache のフィールドガードテスト | app_snapshot_delta.go, テスト | I-6 と並列可 |

### 並列作業マトリクス

```
グループA (フロントエンド): C-1, I-1 → 並列OK
グループB (バックエンド env): I-2 → 単独
グループC (テスト追加): I-6, S-6 → 並列OK  
グループD (ログ/命名): S-2, S-5 → 並列OK
グループE (リスト順序): S-1 → 単独
グループF (確認のみ): I-5 → 他全てと並列OK
```

### Positive Points

| # | 概要 |
|---|------|
| P-1 | `requireSessionsWithPaneID` で 7 メソッドのボイラープレート統一 |
| P-2 | `bestEffortSendKeys` / `buildPaneEnv` で 3+2 箇所のロジック集約 |
| P-3 | `snapshotEventPolicies` のデータ駆動化で拡張性向上 |
| P-4 | `resolveSessionState` でストア更新ロジック DRY 化 |
| P-5 | `usePrefixKeyMode` の Ref 同期パターン廃止で stale closure リスク排除 |
| — | `ResolveDirectionalPane` による 3-lock TOCTOU 解消 |
| — | `removeWindowAtIndexLocked` の survivingWindowID 返却による TOCTOU 解消 |
| — | `cloneSessionForRead` の nil フィルタリングによるパニック防止 |
| — | `copyPaneSlice` による値コピー化でロック外データ競合防止 |
| — | `useBackendSync` の `Promise.allSettled` 化で初期化の独立性確保 |
| — | `ConfirmDialog` のフォーカストラップ追加 (a11y 改善) |
| — | `SearchBar` の `safeAddonOp` による disposed addon 安全対策 |
| — | ロック順序ドキュメントの正確な更新 (`snapshotDeltaMu -> snapshotMu`) |
| — | `SessionSnapshot.Clone()` メソッド抽出でテスト容易性向上 |

---

全体評価: **大規模なリファクタリングとして高品質**。TOCTOU 修正、値コピー化、ロジック集約が体系的に行われている。Critical は 1 件のみ（未使用状態の新フィールド）で、コア機能への影響はない。
