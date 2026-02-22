# モデル置換 `from: "ALL"` ワイルドカード対応

## Context

モデル置換機能（`agent_model.from` → `agent_model.to`）は現在、特定のモデル名のみマッチする。
ユーザーが `from` に `ALL` を指定した場合、全てのモデルを `to` に一括置換する機能を追加する。
オーバーライドの優先度は変更しない（overrides > from→to/ALL）。

## 変更ファイル

### 1. `myT-x/cmd/tmux-shim/model_transform.go` （コアロジック）

**a) `modelTransformer` 構造体に `matchAll bool` フィールド追加**

```go
type modelTransformer struct {
    modelFrom    string
    modelTo      string
    matchAll     bool           // true when From == "ALL" (case-insensitive)
    modelPattern *regexp.Regexp
    overrides    []modelOverrideRule
}
```

**b) ヘルパー関数追加**

```go
func isAllModelFrom(from string) bool {
    return strings.EqualFold(strings.TrimSpace(from), "ALL")
}
```

**c) `newModelTransformer()` 修正**

- `modelFrom` が `ALL` の場合は `matchAll = true` に設定し、`modelPattern` は作成しない
- nil ガードに `!matchAll` 条件を追加

```go
if transformer.modelFrom != "" && transformer.modelTo != "" {
    if isAllModelFrom(transformer.modelFrom) {
        transformer.matchAll = true
    } else {
        transformer.modelPattern = regexp.MustCompile(
            `(?i)(--model\s+)` + regexp.QuoteMeta(transformer.modelFrom) + `(\s|$)`,
        )
    }
}
// ...
if transformer.modelPattern == nil && !transformer.matchAll && len(transformer.overrides) == 0 {
    return nil
}
```

**d) `transform()` 修正**

- override チェック後、`matchAll` の場合は既存の `applyModelOverride(args, t.modelTo)` を再利用

```go
func (t *modelTransformer) transform(args []string) {
    if len(args) == 0 {
        return
    }
    if overrideModel, ok := t.findOverrideModel(args); ok {
        if t.applyModelOverride(args, overrideModel) {
            return
        }
    }
    if t.matchAll {
        t.applyModelOverride(args, t.modelTo)
        return
    }
    if t.modelPattern != nil {
        t.applyFromToReplacement(args)
    }
}
```

**設計ポイント**: `applyModelOverride` は既に全フラグ形式（inline/tokenized/`--model=`）で任意のモデル値を置換する機能を持っている。ALL モードではこれを再利用し、新しい正規表現は不要。

---

### 2. `myT-x/cmd/tmux-shim/model_transform_test.go` （テスト）

**`TestApplyModelTransformAllWildcard`** テーブル駆動テスト:

| テストケース | from | args | 期待動作 |
|-------------|------|------|---------|
| ALL replaces any model (inline) | `ALL` | `--model claude-opus-4-6` | sonnet に置換 |
| ALL replaces any model (tokenized) | `ALL` | `--model`, `claude-opus-4-6` | sonnet に置換 |
| ALL replaces any model (`--model=`) | `ALL` | `--model=claude-opus-4-6` | sonnet に置換 |
| Case insensitivity: `all` | `all` | `--model claude-opus-4-6` | 置換される |
| Case insensitivity: `All` | `All` | `--model claude-opus-4-6` | 置換される |
| Override wins over ALL | `ALL` + override | `--agent-name security --model X` | override が勝つ |
| ALL with no --model flag | `ALL` | `--agent-name foo` | 変更なし |
| ALL with empty args | `ALL` | `[]` | 変更なし |
| ALL replaces multiple --model | `ALL` | `--model X --flag y --model Z` | 両方置換 |
| ALL with empty --model= | `ALL` | `--model=` | スキップ（防御ガード） |

**`TestIsAllModelFrom`** ヘルパー関数テスト:
- `"ALL"`, `"all"`, `"All"`, `" ALL "` → true
- `"ALLX"`, `"model"`, `""` → false

---

### 3. `myT-x/frontend/src/components/settings/AgentModelSettings.tsx` （UI）

説明テキストに ALL ワイルドカードの記述を追加:

```
Claude Codeが子エージェントを起動する際のモデル自動置換設定。
子プロセスの --model フラグを置換元から置換先に変更します。
置換元に「ALL」を指定すると、全モデルを置換先に一括変更します。
```

from/to 下部の説明文も更新:
```
fromとtoは両方同時に指定が必要です。fromに「ALL」を指定すると全モデル置換。
```

---

### 4. `myT-x/internal/config/config_test.go` （バリデーションテスト）

`TestNormalizeAndValidateAgentModel` に `from: "ALL"` ケース追加:
- `from: "ALL", to: "claude-sonnet-4-5"` → エラーなし
- `from: " ALL ", to: "claude-sonnet-4-5"` → トリム後にエラーなし

---

### 5. ドキュメント更新

- `develop-README.md`: agent_model セクションに ALL ワイルドカードの説明追加
- `.claude/skills/agent-command-customization/SKILL.md`: 処理優先度の説明に ALL 分岐を追記

## 実行順序

```
1. model_transform.go       ← コアロジック実装
2. model_transform_test.go  ← テスト作成・実行
3. config_test.go           ← バリデーションテスト追加
4. AgentModelSettings.tsx   ← UI テキスト更新（並列可）
5. ドキュメント更新          ← develop-README.md, SKILL.md（並列可）
6. self-review              ← 全体レビュー
```

## 検証方法

1. `go test ./cmd/tmux-shim/... -run TestApplyModelTransformAllWildcard` — ALL ワイルドカードテスト
2. `go test ./cmd/tmux-shim/... -run TestIsAllModelFrom` — ヘルパーテスト
3. `go test ./internal/config/... -run TestNormalizeAndValidateAgentModel` — バリデーションテスト
4. `go test ./...` — 全テストパス確認
5. `go build ./...` — ビルド確認
