# Skills利用統計ダッシュボード — データ取得元調査プラン

## Context

`C:\Users\mytakedev\.claude` 配下のデータを活用し、Claude Codeのスキル（`/skill-name`）の利用頻度・傾向を可視化するダッシュボードを構築する。
本プランは「どのファイルから何を取得できるか」のデータ取得元マッピングと優先順位を定める。

---

## データソース一覧（優先度順）

### 1位: `history.jsonl`（最重要・スキル呼び出し直接記録）

| 項目 | 値 |
|------|-----|
| パス | `C:\Users\mytakedev\.claude\history.jsonl` |
| 形式 | JSONL（1行1レコード） |
| 行数 | 約4,058行（2026-01〜04） |

**取得できる情報:**
- スキル呼び出しコマンド（例: `/pr-review-toolkit:review-pr`, `/self-review`）
- 呼び出しタイムスタンプ（Unix ms）
- 呼び出し元プロジェクト（`cwd` パス）
- セッションID（`sessionId`）

**レコード構造:**
```json
{
  "display": "/pr-review-toolkit:review-pr C:\\path",
  "timestamp": 1769306046171,
  "project": "C:\\Users\\mytakedev\\dev-myT-x\\myT-x",
  "sessionId": "faafed8e-5d6e-4bdb-ae55-7c4486ddbb11",
  "pastedContents": {}
}
```

**スキル呼び出しの抽出ルール:**
- `display` が `/` で始まる行を抽出
- 組み込みコマンド（`/clear`, `/exit`, `/model`, `/status`, `/help`, `/fast`）を除外
- 残りがスキル呼び出し

---

### 2位: `stats-cache.json`（日次集計・トレンドライン用）

| 項目 | 値 |
|------|-----|
| パス | `C:\Users\mytakedev\.claude\stats-cache.json` |
| 形式 | JSON |
| サイズ | 約10.5 KB |

**取得できる情報:**
- 日付ごとの `messageCount`（メッセージ数）
- 日付ごとの `sessionCount`（セッション数）
- 日付ごとの `toolCallCount`（ツール呼び出し数）

**用途:** スキル呼び出し数を全体の活動量と比較するためのベースライン。

---

### 3位: `usage-data/facets/`（セッション品質・成功率）

| 項目 | 値 |
|------|-----|
| パス | `C:\Users\mytakedev\.claude\usage-data\facets\` |
| 形式 | JSON（セッションUUID単位） |
| ファイル数 | 101+件 |

**取得できる情報:**
- セッションの目標カテゴリ（`goal_categories`）
- 達成度（`outcome`: `fully_achieved` / `partially_achieved` / `not_achieved`）
- ユーザー満足度（`user_satisfaction_counts`）
- Claudeの有用性評価（`claude_helpfulness`）
- 摩擦の有無（`friction_counts`, `friction_detail`）

**用途:** スキルが呼び出されたセッションのIDで JOIN し、スキル別の成功率・満足度を集計。

---

### 4位: `usage-data/session-meta/`（詳細セッションメタ）

| 項目 | 値 |
|------|-----|
| パス | `C:\Users\mytakedev\.claude\usage-data\session-meta\` |
| 形式 | JSON（セッションUUID単位） |
| ファイル数 | 1,764+件 |

**用途:** facets より大きい母数のセッション品質データ。同様に sessionId で JOIN 可能。

---

### 5位: `projects/*/subagents/agent-*.jsonl`（詳細ツール呼び出しログ）

| 項目 | 値 |
|------|-----|
| パス | `C:\Users\mytakedev\.claude\projects\[project]\[session]\subagents\agent-*.jsonl` |
| 形式 | JSONL |

**取得できる情報:**
- Skill ツール（`Skill` tool）呼び出しのフル引数
- `skill` パラメータ値（呼び出されたスキル名）
- `args` パラメータ値（スキルへの引数）
- 実行タイムスタンプ

**用途:** `history.jsonl` が `/command` 単位であるのに対し、こちらは `Skill` tool 呼び出し単位で集計可能。ただし全ファイルのスキャンコストが高い。

---

## ダッシュボードで可視化できる指標

| 指標 | 取得元 |
|------|--------|
| スキル別呼び出し回数（全期間） | `history.jsonl` |
| スキル別呼び出し回数（日次/週次トレンド） | `history.jsonl` × `stats-cache.json` |
| 最も使われるスキル TOP N | `history.jsonl` |
| スキル別プロジェクト分布 | `history.jsonl` |
| スキル呼び出しの時間帯分布 | `history.jsonl`（timestamp から変換） |
| スキル別セッション成功率 | `history.jsonl` ⋈ `usage-data/facets/`（sessionId） |
| 組み込みコマンド vs スキル比率 | `history.jsonl` |
| 日次アクティビティ（参考ベースライン） | `stats-cache.json` |

---

## 除外する組み込みコマンド一覧（スキル呼び出しから除外）

```
/clear, /exit, /model, /status, /help, /fast, /compact, /config,
/cost, /doctor, /init, /login, /logout, /memory, /mcp, /pr-create,
/pr-diff, /review, /release-notes, /reset, /resume, /ultrareview,
/vim, /schedule, /loop, /install
```

---

## 実装アプローチ（次フェーズ用）

### Option A: スタンドアロン Python スクリプト（シンプル）
- `history.jsonl` をパースして Pandas で集計 → HTML/Plotly ダッシュボード出力
- `stats-cache.json` でベースラインを追加
- 実行コスト: 低

### Option B: myT-x 内 UsageDashboard 機能として統合（推奨）
- 既存の `internal/usagedashboard/` パッケージに `SkillUsageRepository` を追加
- Wails の `usage-dashboard` ビューに「スキルタブ」を追加
- `history.jsonl` をバックエンドで読み取り、フロントエンドへ送信
- 実装コスト: 中

### Option C: 既存 `usage-data/report.html` の拡張
- 現在の HTML レポートを参考に、スキル統計セクションを追加
- 実装コスト: 低〜中

---

## 検証方法

1. `history.jsonl` から `/` 始まり行を抽出し、件数とユニーク名を確認
2. 組み込みコマンド除外後のスキル名リストを目視チェック
3. `sessionId` で `usage-data/facets/` と JOIN できる件数を確認（カバレッジ率）
4. `stats-cache.json` の日付範囲と `history.jsonl` の日付範囲が一致しているか確認

---

## 重要ファイルパス

| ファイル | 目的 |
|---------|------|
| `C:\Users\mytakedev\.claude\history.jsonl` | スキル呼び出し生データ（最重要） |
| `C:\Users\mytakedev\.claude\stats-cache.json` | 日次集計ベースライン |
| `C:\Users\mytakedev\.claude\usage-data\facets\` | セッション品質（成功率） |
| `C:\Users\mytakedev\.claude\usage-data\session-meta\` | 詳細セッションメタ |
| `C:\Users\mytakedev\.claude\usage-data\report.html` | 既存レポート（参考） |
| `C:\Users\mytakedev\.claude\skills\` | インストール済みスキル一覧 |
