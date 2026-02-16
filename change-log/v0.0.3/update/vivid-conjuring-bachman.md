# myT-x パフォーマンス改善調査レポート & 実装計画

## Context

myT-xの操作速度改善・負荷軽減のため、Go backend / React frontend / スナップショット/デルタパイプライン の3軸で包括的に調査を実施。最大のボトルネックは**スナップショット生成・比較パイプライン**であり、ペイン数増加に伴いCPU使用率・メモリ確保量が指数的に増大する構造を確認した。

---

## 調査結果サマリ（重要度順）

### A. スナップショット/デルタパイプライン（最大ボトルネック）

| # | 問題 | ファイル | 影響度 |
|---|------|---------|--------|
| A1 | **イベント毎にemitSnapshot()を即時実行** - 10ペイン一括作成で10回のフルスナップショット | `app_events.go:78-80, 83-95` | CRITICAL |
| A2 | **Snapshot()でセッション/ウィンドウ/ペイン全体をディープコピー** - 10sess×5win×10pane = 500+ヒープ確保 | `session_manager_snapshot.go:9-63` | CRITICAL |
| A3 | **snapshotMu.Lock()をデルタ比較全体で保持** - 全ツリー走査中にロック | `app_snapshot_delta.go:136-137` | CRITICAL |
| A4 | **syncPaneStates()が毎回全ペインをウォーク** - O(全ペイン数)、出力のみの変更時も実行 | `app_events.go:234-254` | HIGH |
| A5 | **payloadSizeBytes()がスナップショット構造を再帰走査** - デルタ比較と同じ構造を二重走査 | `app_snapshot_metrics.go:25-41` | HIGH |
| A6 | **recordSnapshotEmission()がsnapshotMuを再取得** - emitSnapshot内で2回ロック取得 | `app_snapshot_metrics.go:183-184` | MEDIUM |
| A7 | **セッションソートが毎スナップショットで実行** - O(n log n)、変更なしでも | `session_manager_snapshot.go:13-19` | LOW |
| A8 | **ActivePaneIDs()でfmt.Sprintf毎回実行** | `session_manager_snapshot.go:74` | LOW |

### B. 出力バッファリング & ターミナルI/O

| # | 問題 | ファイル | 影響度 |
|---|------|---------|--------|
| B1 | **ペイン毎に16msティッカー + ゴルーチン** - 50ペイン = 50ゴルーチン + 50タイマー | `output_buffer.go:61-81` | HIGH |
| B2 | **16msフラッシュ間隔 = 62.5回/秒/ペイン** - 50ペインで最大3,125 Wailsイベント/秒 | `output_buffer.go`, `app_events.go:129,141` | HIGH |
| B3 | **フラッシュ毎にUpdateActivityByPaneID呼び出し** - idle→active遷移でemitSnapshot発火 | `app_events.go:137-139` | MEDIUM |
| B4 | **paneFeedCh (4096) が高負荷時にフォールバック** | `app.go:85` | LOW |

### C. ロック競合 & メモリ確保

| # | 問題 | ファイル | 影響度 |
|---|------|---------|--------|
| C1 | **PaneEnv深コピーがIPCコマンド毎に実行** | `command_router.go:91-103` | MEDIUM |
| C2 | **Config深コピーがアクセス毎に実行** - セッション作成で6+回 | `app_config_state.go:7-10` | MEDIUM |
| C3 | **envマップコピーがCOWなし** - 50ペイン×100変数 = 5,000確保/スナップショット | `session_manager_helpers.go:9-17` | MEDIUM |

### D. フロントエンド（React + xterm.js）

| # | 問題 | ファイル | 影響度 |
|---|------|---------|--------|
| D1 | **TerminalPaneがReact.memo未使用** - 親再レンダリングでxterm.jsを含む全コンポーネントが再評価 | `TerminalPane.tsx:35` | HIGH |
| D2 | **pane:dataイベントでterm.write()を即時実行** - RAF未使用、高頻度出力時にCPUスパイク | `TerminalPane.tsx:331-339` | HIGH |
| D3 | **onWriteParsedでスクロール状態を毎回更新** - setScrollAtBottom()が毎write発火 | `TerminalPane.tsx:342-349` | HIGH |
| D4 | **usePrefixKeyModeの依存が7個** - Zustandストア変更毎にkeydownリスナー再登録 | `usePrefixKeyMode.ts:10-17` | MEDIUM |
| D5 | **ResizeObserverがペイン毎にO(n)** - ウィンドウリサイズで全ペイン同時発火 | `TerminalPane.tsx:363-368` | MEDIUM |
| D6 | **StatusBarが1秒間隔で無条件ポーリング** | `StatusBar.tsx:25-27` | MEDIUM |
| D7 | **Ctrl+スクロールでfontSize変更が無制限** - 全ペインの同期fit()が即時実行 | `TerminalPane.tsx:405-410` | MEDIUM |
| D8 | **LayoutNodeViewのcallback propsがuseCallback未使用** | `LayoutNodeView.tsx:150-162` | LOW |

### E. ポーリング

| # | 問題 | ファイル | 影響度 |
|---|------|---------|--------|
| E1 | **1秒間隔のアイドル監視ポーリング** - 全セッションを毎秒チェック、86,400回/日 | `app_lifecycle.go:269-278` | LOW |

---

## 実装計画（優先度順）

### Phase 1: スナップショットイベントコアレッシング（最高インパクト）

**対象**: A1, A3, A5, A6
**推定効果**: バースト時のスナップショットCPUを70-90%削減

#### 変更ファイル
- `myT-x/app.go` - コアレッシングタイマーフィールド追加
- `myT-x/app_events.go` - `emitSnapshot()`直接呼び出しを`requestSnapshot()`に置換
- `myT-x/app_snapshot_delta.go` - snapshotMuスコープ縮小（キャッシュ読み書きのみロック、比較はロック外）
- `myT-x/app_snapshot_metrics.go` - snapshotMuから独立したメトリクスロック化、payloadSizeBytesの軽量化

#### アプローチ
1. **requestSnapshot()**: 50msデバウンスウィンドウでコアレッシング。連続イベントは1回のemitSnapshotに集約
2. **即時バイパス**: session-created等の即時可視化が必要なイベントはデバウンスをスキップ
3. **snapshotDelta()のロック縮小**: Lock→キャッシュコピー→Unlock→比較→Lock→キャッシュ更新→Unlock
4. **metricsを別ミューテックスに分離**: snapshotMuの二重取得を排除

#### テスト
- コアレッシング動作テスト（単発、バースト、即時バイパス）
- デルタ比較の正確性テスト（ロック外比較でも結果一致）
- スナップショット発行頻度ベンチマーク

---

### Phase 2: スナップショットディープコピー最適化

**対象**: A2, A4, A7, A8
**推定効果**: スナップショット毎のメモリ確保を60-80%削減

#### 変更ファイル
- `myT-x/internal/tmux/session_manager.go` - 世代カウンター追加、ソート済みキャッシュ
- `myT-x/internal/tmux/session_manager_snapshot.go` - 差分スナップショット（変更セッションのみコピー）
- `myT-x/app_events.go` - syncPaneStatesをトポロジ変更時のみ実行

#### アプローチ
1. **世代ベース差分追跡**: SessionManagerに`generation uint64`追加。各ミューテーション（セッション追加/削除、ペイン変更等）で+1
2. **差分Snapshot()**: 前回スナップショット以降に変更されたセッションのみコピー。未変更はキャッシュ返却
3. **ソート済みキャッシュ**: AddSession/RemoveSession時のみ再ソート
4. **syncPaneStatesのトポロジゲート**: トポロジ世代変更時のみフル走査。出力のみ変更時はスキップ
5. **IDString()プリキャッシュ**: ペイン作成時に一度だけfmt.Sprintf実行、フィールドに保存

#### テスト
- 世代カウンターの全ミューテーション種別でのインクリメントテスト
- キャッシュスナップショットの正確性テスト
- syncPaneStatesスキップ条件テスト

---

### Phase 3: 出力バッファ統合

**対象**: B1, B2, B3
**推定効果**: Wailsイベント発行率50-70%削減、ゴルーチン数をO(n)からO(1)に削減

#### 変更ファイル
- `myT-x/internal/terminal/output_buffer.go` - 共有フラッシュマネージャーにリデザイン
- `myT-x/app_events.go` - ensureOutputBuffer/フラッシュコールバック適応
- `myT-x/app.go` - `outputBuffers map`を単一共有マネージャーに置換

#### アプローチ
1. **共有OutputFlushManager**: ペイン毎バッファの単一管理者。1ゴルーチン + 1ティッカーで全ペインをフラッシュ
2. **バッチアクティビティ更新**: フラッシュスイープ時に全アクティブIDを一括収集 → idle→active遷移を1回のrequestSnapshotに集約
3. **適応的フラッシュ間隔**: アクティブペインが少ない場合は間隔を延長

#### テスト
- 共有マネージャーの正確なフラッシュテスト
- Stop()で保留データが全て排出されるテスト
- 50ペイン同時出力のイベント発行率ベンチマーク

---

### Phase 4: フロントエンドReact最適化

**対象**: D1, D2, D3, D4, D5, D6, D7
**推定効果**: React再レンダリング30-50%削減、ターミナル出力のスムーズネス向上
**依存**: なし（バックエンドフェーズと並行実施可能）

#### 変更ファイル
- `myT-x/frontend/src/components/TerminalPane.tsx` - RAF batching, スクロールスロットル, memo化
- `myT-x/frontend/src/hooks/usePrefixKeyMode.ts` - Zustand依存を3個に削減
- `myT-x/frontend/src/components/StatusBar.tsx` - イベント駆動に変更
- `myT-x/frontend/src/components/LayoutNodeView.tsx` - callback props memo化

#### アプローチ
1. **RAF-batched terminal writes**: `pane:data`イベントでチャンクを配列に蓄積、requestAnimationFrameで一括write
2. **スクロール状態スロットル**: onWriteParsedのupdateScrollStateを100msスロットル
3. **TerminalPaneのexport memo化**: React.memoで囲む（useEffect内部のイベントリスナーは影響なし）
4. **usePrefixKeyMode**: sessions/activeSessionをuseRef化、useEffect依存を削減
5. **StatusBar**: setIntervalを10秒に延長 + Wailsイベント(session変更時)でトリガー
6. **fontSize debounce**: Ctrl+スクロールを50msデバウンス

#### テスト
- ターミナル出力の視覚的動作確認
- prefix keyモードの正常動作確認
- StatusBar更新タイミング確認

---

### Phase 5: Config/Envキャッシュ最適化

**対象**: C1, C2, C3
**推定効果**: IPCコマンド毎のメモリ確保10-20%削減

#### 変更ファイル
- `myT-x/app_config_state.go` - 世代ベースキャッシュ
- `myT-x/internal/tmux/command_router.go` - PaneEnvスナップショットキャッシュ
- `myT-x/internal/tmux/session_manager_helpers.go` - COWラッパー（オプション）

#### アプローチ
1. **Configキャッシュ**: `configGeneration atomic.Uint64`追加。setConfig時+1。getConfigは世代一致時はキャッシュ返却
2. **PaneEnvキャッシュ**: 同パターン。UpdatePaneEnvで世代+1
3. **COW envマップ**（長期的）: frozen flagで書き込み時のみコピー

---

### Phase 6: アイドル監視最適化

**対象**: E1
**推定効果**: 微小（アーキテクチャ改善）

#### 変更ファイル
- `myT-x/app_lifecycle.go` - 適応的ポーリング間隔

#### アプローチ
- セッション0件時はティッカー停止
- 全セッションidle時は5秒間隔に延長
- アクティブセッションがある場合のみ1秒間隔

---

## 実施順序とフェーズ依存

```
Phase 1 (コアレッシング)     ← 全フェーズの基盤
    |
    v
Phase 2 (スナップショット最適化) ← Phase 1のコアレッシングで効果倍増
    |
    v
Phase 3 (出力バッファ統合)    ← Phase 1のコアレッシングが必要
    |
Phase 4 (フロントエンド)      ← 独立、Phase 1-3と並行可能
    |
Phase 5 (Config/Envキャッシュ) ← 独立
    |
Phase 6 (アイドル監視)        ← Phase 1に依存
```

## 推定合計効果（50ペイン環境）

| 指標 | 現状 | 改善後見込み |
|------|------|------------|
| バースト時スナップショット回数 | N回/イベント | 1回/50ms窓 |
| スナップショットCPU | O(全セッション×全ペイン) | O(変更セッションのみ) |
| Wailsイベント/秒 | ~3,125 (50pane×62.5) | ~500-1,000 |
| 出力フラッシュゴルーチン数 | 50 | 1 |
| snapshotMuロック保持時間 | フルツリー比較中 | キャッシュ読み書きのみ |
| フロントエンド再レンダリング | 全変更で全体 | 変更対象のみ |

## 検証方法

1. **Go pprof**: `go tool pprof`でCPU/メモリプロファイリング（各Phase前後で比較）
2. **スナップショットメトリクス**: 既存の`[snapshot-metrics]`ログで発行頻度・バイト数を比較
3. **React DevTools Profiler**: 再レンダリング回数・時間を計測
4. **手動テスト**: 10+ペイン作成 → 高頻度出力（`yes`コマンド等）→ UI応答性確認
5. **既存テスト**: `go test ./...` で全テスト通過を確認
