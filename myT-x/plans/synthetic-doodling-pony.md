# Go 1.26 対応による品質改善 — 実装プラン

## Context

`task/Go1.26対応による品質改善.md` に記載された Go 1.26 機能の適用タスク。
CLAUDE.md の規約に従い、`go fix ./...` によるコードモダン化を最初に実施し、
その後に手動マイグレーション、テスト、self-review を行う。

**go.mod は既に Go 1.26** — コンパイラ更新は不要。コード側の最適化が対象。

---

## 適用判定サマリ

| タスク文書の項目 | 適用 | 理由 |
|----------------|------|------|
| `go fix ./...` モダン化 | **YES** | CLAUDE.md 必須。24個のモダナイザで自動変換 |
| `errors.AsType[T]` 移行 | **YES** | 1箇所。型安全・3倍高速 |
| `slog.MultiHandler` | **NO** | 現状シングルハンドラのみ。マルチ出力の要件なし |
| `reflect.Value.Fields()` | **NO** | 16箇所あるが全てテストのフィールド数ガード。イテレーション用途なし |
| Green Tea GC / エスケープ解析 | **不要** | Go 1.26 デフォルト有効。コード変更なしで恩恵を受ける |
| PGO プロファイル | **延期** | プロファイリング基盤が未整備。別タスクとして切り出し推奨 |
| WebSocket フレームプーリング | **NO** | Go 1.26 機能ではなくパフォーマンス最適化。別タスク |

---

## 実装フェーズ

### Phase 0: ベースライン確認

```
go test ./...   → 全パス確認
go build ./...  → ビルド確認
```

### Phase 1: `go fix ./...` 自動モダン化 (順次・最優先)

**コマンド**: `go fix ./...` (myT-x ディレクトリで実行)

**予測される主な変換 (~20-30ファイル)**:

| モダナイザ | 予測変換数 | 対象例 |
|-----------|-----------|--------|
| `rangeint` | ~8箇所 | `for i := 0; i < N; i++` → `for i := range N` |
| `slicessort` | 6箇所 | `sort.Slice(...)` → `slices.SortFunc(...)` |
| `stringscut` | 3箇所 | `strings.SplitN(s, sep, 2)` → `strings.Cut(s, sep)` |
| `waitgroup` | ~5箇所 | `wg.Add(1)+go+defer wg.Done()` → `wg.Go(func(){...})` |
| `testingcontext` | ~30箇所 | テスト内 `context.Background()` → `t.Context()` |

**変換後の必須手順**:
1. `git diff` で全変更をレビュー
2. `stringscut` 変換は要注意: `SplitN` → `Cut` で戻り値パターンが変わる
3. `waitgroup` 変換は要注意: struct フィールドの wg や bulk `wg.Add(N)` は未変換の可能性
4. `go build ./...` → `go test ./...` → 全パス確認

### Phase 2: 手動マイグレーション (Phase 1 完了後、並列可)

#### 2a. `errors.AsType[T]` 移行
- **ファイル**: `internal/ipc/pipe_client.go:85`
- **変更前**: `var opErr *net.OpError; if errors.As(err, &opErr) { ... }`
- **変更後**: `if opErr, ok := errors.AsType[*net.OpError](err); ok { ... }`

#### 2b. `go fix` 未変換の `wg.Add/Done` パターン確認
- **ファイル**: `internal/ipc/pipe_server.go` (struct フィールド wg)
- **ファイル**: `app_context_test.go` (bulk `wg.Add(2)`)
- 1:1 パターンのみ `wg.Go` に変換。bulk-Add は可読性を優先し維持

### Phase 3: テスト実行・修正

```
go build ./...   → ビルド確認
go vet ./...     → 静的解析
go test ./...    → 全テスト実行
```

フロントエンドテストも確認:
```
cd frontend && npx vitest run
```

失敗テストがあれば修正 (go fix の変換に起因するもの含む)

### Phase 4: self-review

`self-review` スキルで最終検証。全項目クリアまで繰り返し。

---

## 対象ファイル一覧 (手動修正)

| ファイル | 修正内容 |
|---------|---------|
| `internal/ipc/pipe_client.go` | `errors.As` → `errors.AsType[T]` |
| `internal/ipc/pipe_server.go` | `wg.Add/Done` → `wg.Go` (go fix 未変換分) |
| `app_context_test.go` | `wg.Add/Done` → `wg.Go` (go fix 未変換分) |

残りは `go fix ./...` が自動変換。

---

## 検証方法

1. `go fix ./...` 実行前後の `git diff` で変換内容を確認
2. `go build ./...` — コンパイルエラーなし
3. `go vet ./...` — 静的解析エラーなし
4. `go test ./...` — 全パッケージパス
5. `npx vitest run` — フロントエンド全テストパス
6. `self-review` — 全項目クリア
