# MCP オーケストレーター ツールリファレンス

myT-x 内蔵のオーケストレーション MCP が提供する全 18 ツールの詳細仕様です。
Agent Teams の概要は [Agent Teams](agent-teams.md) を参照してください。

---

## 概要

Agent Orchestrator MCP は、複数の AI エージェント間のタスク管理・通信を行うサーバーです。
各ペインの AI エージェントは MCP ツールを通じて、タスクの送受信・ステータス管理・チームメンバーの追加などを行えます。

### 典型的なワークフロー

1. `register_agent` — 自分をエージェントとして登録する（**必須**。全ツール利用の前提条件）
2. `get_my_tasks` — 自分宛の pending タスクを確認する
3. `get_task_message` — `send_message_id` を指定してタスクのメッセージ本文を取得する
4. `acknowledge_task` — 受領確認が必要なら `task_id` を指定して受領を記録する
5. タスクを実行する
6. `send_response` — `task_id` を指定してタスクに応答する（タスク完了も同時記録）

### ベストプラクティス

- 最初に必ず `register_agent` を実行する。他ツールは登録後でないと利用できない
- `send_response` の `task_id` は必須。省略するとタスクが完了状態にならない
- `include_response_instructions=true`（デフォルト）で送ると、相手に `send_response` の使い方が自動付与される
- 他エージェントへの相談は `send_task` で直接送信できる（orchestrator 経由は不要）
- `capture_pane` で他エージェントの画面を確認し、進捗やエラーを把握できる
- `add_member` で動的にメンバーを追加できる。追加後は自動でブートストラップメッセージが送信される
- `depends_on` 付きタスクを送った場合、依存タスク完了後に `activate_ready_tasks` を呼んで活性化する

---

## タスクステータス一覧

| ステータス | 説明 |
|-----------|------|
| `pending` | 実行待ち |
| `blocked` | 依存タスクの完了待ち |
| `completed` | 応答済み・完了 |
| `failed` | 送信・実行エラー |
| `abandoned` | タイムアウト（暗黙） |
| `cancelled` | 送信者が明示的にキャンセル |
| `expired` | TTL 超過 |

## エージェントステータス一覧

| ステータス | 説明 |
|-----------|------|
| `idle` | 待機中・タスク受付可 |
| `busy` | 作業中・新規タスク受付不可 |
| `working` | 作業中・新規タスク受付可 |

---

## ツール一覧

### 1. register_agent

エージェントのペイン ID と名前を紐付け、ロール・得意分野を登録する。
同名で再呼び出しすると情報を更新する。

**実行権限:** 誰でも実行可能（登録不要）

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `name` | Yes | string | エージェント名（英数字・`._-`、最大 64 文字） |
| `pane_id` | Yes | string | tmux ペイン ID（例: `%1`） |
| `role` | No | string | 役割（最大 120 文字） |
| `skills` | No | array | 得意分野（最大 20 件）。`[{"name":"Go","description":"..."}]` 形式推奨 |

**戻り値:** `name`, `pane_id`, `role`, `skills`, `pane_title`, (`warning`)

**注意事項:**
- 全ツール利用の前提条件。最初に必ず実行すること
- 同じ `pane_id` に既に別エージェントが登録されている場合は上書きされる
- 登録・更新は caller の pane に関係なく実行できる

---

### 2. list_agents

全エージェント情報を取得し、tmux list-panes と突合して全ペインの状態を返す。

**実行権限:** 登録済みエージェントのみ

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| （なし） | — | — | — |

**戻り値:** `registered_agents`（name, pane_id, role, skills, status）, `unregistered_panes`, (`orchestrator`), (`warning`)

**注意事項:**
- `registered_agents` には status（`idle` / `busy` / `working` / `unknown`）が含まれる

---

### 3. send_task

指定エージェントにタスクを送信する（ユニキャスト）。

**実行権限:** 誰でも送信可能（orchestrator 経由不要）

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `agent_name` | Yes | string | 宛先エージェント名 |
| `from_agent` | Yes | string | 自分の登録済みエージェント名（返信先） |
| `message` | Yes | string | 送信メッセージ（最大 8000 文字） |
| `include_response_instructions` | No | boolean | 応答テンプレート自動付与（デフォルト: `true`） |
| `expires_after_minutes` | No | integer | タスク有効期限（1〜1440 分） |
| `depends_on` | No | array | 依存タスク ID 配列（最大 20 件） |

**戻り値:** `task_id`, `agent_name`, `pane_id`, `sender_pane_id`, `sent_at`

**注意事項:**
- 送信成功時に `task_id` が返る。相手はこの `task_id` で `send_response` する
- デフォルトでメッセージ末尾に応答方法テンプレートが自動付与される
- `depends_on` を指定すると `blocked` で作成され、依存完了後に `activate_ready_tasks` で活性化する
- 存在しない依存先はエラーになる
- 送信失敗時はタスクが `failed` 状態になる

---

### 4. send_tasks

複数エージェントへ一括でタスクを送信し、`group_id` でまとめる。

**実行権限:** 登録済みエージェントのみ

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `from_agent` | Yes | string | 自分の登録済みエージェント名（返信先） |
| `group_label` | No | string | グループラベル（最大 120 文字） |
| `tasks` | Yes | array | 送信対象配列（1〜10 件） |

`tasks` の各要素:

| フィールド | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `agent_name` | Yes | string | 宛先エージェント名 |
| `message` | Yes | string | 送信メッセージ（最大 8000 文字） |
| `include_response_instructions` | No | boolean | 応答テンプレート自動付与（デフォルト: `true`） |
| `expires_after_minutes` | No | integer | タスク有効期限（1〜1440 分） |

**戻り値:** `group_id`, `results`（agent_name + task_id or error）, `summary`（sent, failed）

**注意事項:**
- 成功要素は `task_id` / `agent_name`、失敗要素は `agent_name` / `error` のみ返す
- バッチ内タスク間の `depends_on` 指定は未サポート。依存関係が必要な場合は `send_task` を個別に使う

---

### 5. get_my_tasks

自分宛のタスク一覧を取得する（受信箱）。

**実行権限:** 登録済みエージェントのみ（caller 名と `agent_name` が一致する場合のみ）

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `agent_name` | Yes | string | 自分のエージェント名 |
| `status_filter` | No | string | `pending` / `blocked` / `completed` / `all` / `failed` / `abandoned` / `cancelled` / `expired`（デフォルト: `pending`） |

**戻り値:** `agent_name`, `tasks`（task_id, status, sent_at, is_now_session, send_message_id, sender_pane_id, completed_at）, `response_instructions`

**注意事項:**
- 戻り値の各タスクに `send_message_id` が含まれる。これを `get_task_message` に渡してメッセージ本文を取得する
- `response_instructions`（返信手順）も戻り値に含まれる

---

### 6. get_task_message

`send_message_id` からタスクのメッセージ本文とメタデータを取得する。

**実行権限:** 登録済みエージェントのみ（caller 名と `agent_name` が一致する場合のみ）

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `agent_name` | Yes | string | 自分のエージェント名 |
| `send_message_id` | Yes | string | 取得対象の send_message_id（`m-` プレフィックス） |

**戻り値:** `task_id`, `agent_name`, `send_message_id`, `status`, `sent_at`, `is_now_session`, `message`（content, created_at）, (`sender_pane_id`), (`completed_at`)

**注意事項:**
- `get_my_tasks` で取得した `send_message_id` を指定して使う
- メッセージ本文を読むにはこのツールを使う
- タスクの進捗・依存関係・応答内容の確認には `get_task_detail` を使う

---

### 7. get_task_detail

単一タスクの詳細状態を取得する。

**実行権限:** 送信者・担当者・trusted caller

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `task_id` | Yes | string | 対象の task_id |

**戻り値:** `task_id`, `status`, `agent_name`, (`completed_at`), (`acknowledged_at`), (`cancelled_at`), (`cancel_reason`), (`progress_pct`), (`progress_note`), (`progress_updated_at`), (`expires_at`), (`depends_on`), (`response`（content, created_at）)

**注意事項:**
- `completed` タスクは `response.content` と `response.created_at` を含む
- メッセージ本文は含まない。本文取得には `get_task_message`（担当者）を使う

---

### 8. acknowledge_task

担当タスクの受領を記録する（任意）。

**実行権限:** task assignee のみ

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `agent_name` | Yes | string | 自分のエージェント名 |
| `task_id` | Yes | string | 受領する task_id |

**戻り値:** `task_id`, `agent_name`, `acknowledged_at`

**注意事項:**
- 省略してもタスク処理に影響しない
- 送信者が `get_task_detail` で `acknowledged_at` を確認できるようになる

---

### 9. send_response

タスク送信者にメッセージを返信し、対象タスクを `completed` に更新する。

**実行権限:** pending 状態の task_id を持つ担当者のみ

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `task_id` | Yes | string | 応答対象の task_id |
| `message` | Yes | string | 返信メッセージ（最大 8000 文字） |

**戻り値:** `sent_to`, `sent_to_name`, (`warning`), (`task_id`, `task_status`, `completed_at`)

**注意事項:**
- `task_id` を省略するとエラー。タスクを完了できなくなるので注意
- 送信者のペインにメッセージが送られ、タスクが `completed` になる

---

### 10. update_status

自分のエージェント状態を更新する。

**実行権限:** 自分自身のみ

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `agent_name` | Yes | string | 自分のエージェント名 |
| `status` | Yes | string | `idle`（待機中・受付可）/ `busy`（作業中・受付不可）/ `working`（作業中・受付可） |
| `current_task_id` | No | string | 現在作業中の task_id（空文字でクリア） |
| `note` | No | string | 補足メモ（最大 200 文字） |

**戻り値:** `agent_name`, `status`, `updated_at`

**注意事項:**
- 他エージェントが `get_agent_status` で確認し、タスク送信先の選定に使う

---

### 11. get_agent_status

特定エージェントの最新ステータスを取得する。

**実行権限:** 登録済みエージェントなら誰でも

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `agent_name` | Yes | string | 対象エージェント名 |

**戻り値:** `agent_name`, `status`, (`current_task_id`), (`note`), (`seconds_since_update`)

---

### 12. list_all_tasks

全タスクの状態一覧を取得する（チーム全体の監視用）。

**実行権限:** 登録済みエージェントなら誰でも

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `status_filter` | No | string | `pending` / `blocked` / `completed` / `all` / `failed` / `abandoned` / `cancelled` / `expired`（デフォルト: `all`） |
| `assignee_name` | No | string | 担当者（assignee）でフィルタ |

**戻り値:** `tasks`（task_id, agent_name, status, sent_at, is_now_session, send_message_id, sender_pane_id, completed_at）, `summary`（pending, blocked, completed, failed, abandoned, cancelled, expired）

**注意事項:**
- `get_my_tasks` は自分宛タスクだけを返す受信箱。`list_all_tasks` は全エージェントのタスクを含む全体監視ビュー

---

### 13. activate_ready_tasks

`blocked` タスクの依存関係を評価し、全依存が完了したタスクを `pending` に切り替えて配信する。

**実行権限:** 登録済みエージェントなら誰でも

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `assignee_name` | No | string | 担当者（assignee）でフィルタ |

**戻り値:** `activated`（task_id, agent_name）, `still_blocked`

**注意事項:**
- `send_response` でタスクを完了した後に呼ぶと、そのタスクに依存していた `blocked` タスクが活性化される
- `cancelled` / `failed` / `abandoned` / `expired` / 不整合な依存先を持つ `blocked` タスクは自動で `cancelled` に整理される

---

### 14. cancel_task

送信済みの `pending` または `blocked` タスクをキャンセルする。

**実行権限:** 送信者のみ

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `task_id` | Yes | string | キャンセルする task_id |
| `reason` | No | string | キャンセル理由（最大 500 文字） |

**戻り値:** `task_id`, `status`

---

### 15. update_task_progress

担当タスクの進捗率または進捗メモを更新する。

**実行権限:** task assignee のみ

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `task_id` | Yes | string | 対象の task_id |
| `progress_pct` | No | integer | 進捗率（0〜100） |
| `progress_note` | No | string | 進捗メモ（最大 500 文字） |

**戻り値:** `task_id`, `progress_updated_at`, (`progress_pct`)

**注意事項:**
- `progress_pct` または `progress_note` のいずれかは必須

---

### 16. capture_pane

指定エージェントのペイン表示内容を取得する（スクリーンキャプチャ）。

**実行権限:** 登録済みエージェントなら誰でも

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `agent_name` | Yes | string | 対象エージェント名 |
| `lines` | No | integer | 取得行数（1〜200、デフォルト: 50） |

**戻り値:** `agent_name`, `pane_id`, `lines`, `content`, `warning`

**注意事項:**
- 相手の進捗確認・エラー確認に使用する

---

### 17. add_member

新メンバーを動的に追加する。ペイン分割 → CLI 起動 → ブートストラップメッセージ送信を一括実行する。

**実行権限:** 登録済みエージェントのみ

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `pane_title` | Yes | string | メンバー表示名（最大 30 文字） |
| `role` | Yes | string | 役割（最大 120 文字） |
| `command` | Yes | string | CLI コマンド（例: `claude`）（最大 100 文字） |
| `args` | No | array | コマンド引数配列（最大 20 件、各最大 200 文字） |
| `custom_message` | No | string | 追加指示メッセージ（最大 2000 文字） |
| `skills` | No | array | 得意分野配列（`register_agent` と同形式、最大 20 件） |
| `team_name` | No | string | チーム名（最大 64 文字、デフォルト: `動的チーム`） |
| `split_from` | No | string | 分割元ペイン ID（デフォルト: 呼び出し元ペイン） |
| `split_direction` | No | string | `horizontal` / `vertical`（デフォルト: `horizontal`） |
| `bootstrap_delay_ms` | No | integer | CLI 起動後の待ち時間 ms（1000〜30000、デフォルト: 3000） |

**戻り値:** `pane_id`, `pane_title`, `agent_name`, (`warnings`)

**注意事項:**
- `split_from` を省略すると呼び出し元のペインを分割する
- Claude CLI（claude, claude.exe, claude-code*）はブラケットペーストモードで送信される
- ペインタイトル設定やブートストラップ送信の失敗は warning として返され、処理は続行される
- 追加されたメンバーは自動でブートストラップメッセージを受信し、`register_agent` の指示を受ける

---

### 18. help

オーケストレーター MCP の使い方ヘルプを返す。

**実行権限:** 誰でも（登録不要）

| パラメータ | 必須 | 型 | 説明 |
|-----------|------|-----|------|
| `topic` | No | string | ツール名を指定すると詳細ヘルプを返す。省略時は全体概要 |

---

## 入力バリデーション制約

| 制約 | 値 |
|------|-----|
| エージェント名の最大長 | 64 文字 |
| エージェント名の形式 | `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$` |
| ロールの最大長 | 120 文字 |
| スキル名の最大長 | 100 文字 |
| スキル説明の最大長 | 400 文字 |
| エージェントあたりの最大スキル数 | 20 件 |
| メッセージの最大長 | 8000 文字 |
| キャプチャの最大行数 | 200 行 |
| ペインタイトルの最大長 | 30 文字 |
| コマンドの最大長 | 100 文字 |
| カスタムメッセージの最大長 | 2000 文字 |
| チーム名の最大長 | 64 文字 |
| コマンド引数の最大数 | 20 件 |
| コマンド引数の最大長（各） | 200 文字 |
| ステータスメモの最大長 | 200 文字 |
| キャンセル理由の最大長 | 500 文字 |
| 進捗メモの最大長 | 500 文字 |
| グループラベルの最大長 | 120 文字 |
| バッチタスクの最大数 | 10 件 |
| 依存タスクの最大数 | 20 件 |
| task_id の形式 | `t-[A-Za-z0-9]+` |
| send_message_id の形式 | `m-[A-Za-z0-9]+` |

---

## 次のステップ

- [Agent Teams](agent-teams.md) — AI チーム開発の全体像
- [タスクスケジューラ](task-scheduler.md) — タスクの自動実行
- [ビューアシステム](viewer-system.md) — MCP Manager を含む全ビューア
