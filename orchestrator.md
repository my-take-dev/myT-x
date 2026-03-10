# MCP Agent Orchestrator - 運用マニュアル

tmux 上の複数対話型 AI 間で、登録、タスク送信、返信、状態確認を行うための実務向けガイド。

## 基本方針

- 固定のペイン番号を前提にしない。`pane_id` は毎回変わり得るため、過去の `%0`, `%1`, `%2` などをそのまま使わない。
- 作業開始前に必ず現在の登録状況を確認する。`list_agents()` を見ずに `send_task` や `send_response` を始めない。
- 登録が空、または不足している場合は、登録方法を先にユーザーへ確認する。
- 登録完了前に本作業へ入らない。
- **`list_agents()` は登録済みエージェントのみ実行可能**。未登録状態では呼べないため、まず自身を `register_agent` してから使う。

## 最初に必ず行う確認

1. まず自身を `register_agent()` で登録する（未登録の場合）。
2. `list_agents()` を実行し、現在の登録済みエージェントを確認する。
3. 依頼の送信元として使うエージェント名と、宛先として使うエージェント名が登録済みか確認する。
4. 未登録、誤登録、または登録が不足している場合は、そのまま作業を続けず登録方針を確認する。

## 登録が空または不足しているときのルール

登録が空のケースは普通に起こる。DB リセット、再起動、別セッション起動直後を常に想定する。

その場合は、次のどちらで進めるかをユーザーに確認してから登録する。

1. 最初に指示を受けた対話型 AI が、現在起動している各 AI の `pane_id` と名前対応を確認し、まとめて `register_agent` を実行する。
2. 既に起動している各対話型 AI が、それぞれ自分の実行コンテキストで自分自身を `register_agent` する。

確認せずに勝手に片方へ決め打ちしない。

ユーザー確認の要点:

- 登録を一括で行うか、各 AI に自己登録させるか
- どの名前で登録するか
- どの AI を送信元として使うか

## 推奨フロー

### 1. 自身の登録

`list_agents()` は登録済みエージェントのみ実行可能なため、未登録の場合はまず自身を登録する。

```text
register_agent(name="<self_name>", pane_id="<current_pane_id>")
```

### 2. 状態確認

`list_agents()`

- 自身の登録後にこれを実行する
- 登録済み一覧と未登録ペインを確認する
- 期待する AI 名が見えない場合は追加登録に進む

### 3. 登録方針の確認

他のエージェントの登録が不足している場合は、ユーザーに確認する。

確認例:

`現在の登録が空です。最初に指示を受けた私が全 AI を登録しますか、それとも各 AI に自己登録させますか。登録後に作業へ進みます。`

### 4. 登録

登録方法はどちらでもよいが、必ず現在の `pane_id` と名前対応を確認してから実行する。

例:

```text
register_agent(name="<agent_name>", pane_id="<current_pane_id>", role="<role>", skills=["<skill1>", "<skill2>"])
```

登録時の注意:

- `pane_id` は毎回現在値を使う
- 同じ `pane_id` に別名を登録すると既存登録は置き換わる
- `orchestrator` は予約名のため、別ペインから上書きできない

### 5. 再確認

登録後にもう一度 `list_agents()` を実行し、今回使う送信元と宛先が見えていることを確認する。

### 6. タスク送信

```text
send_task(agent_name="<target_agent>", from_agent="<sender_agent>", message="<request>", task_label="<label>")
```

送信時の注意:

- `from_agent` は返信先になるため必須
- `from_agent` は登録済み名でなければならない
- エージェント同士の直接通信は可能
- 本文末尾には具体的な `task_id` と `send_response(...)` 例を含む応答方法テンプレートが自動付与される

### 7. 受信側の確認

```text
get_my_tasks(agent_name="<self_agent_name>")
```

- 受信側は自分宛タスクを確認する
- `task_id` を特定して返信に使う
- 受信本文にも `task_id=<...>` と `send_response(task_id="...", message="...")` が出るため、通常は本文だけでも返信できる

### 8. 返信

```text
send_response(task_id="<task_id>", message="<reply>")
```

- 返信は受信した本人が行う
- 第三者が代理で完了させない
- `send_response` は返信と完了記録を同時に行う
- 受信本文中の `task_id=<...>` をそのまま使う

### 9. 状態確認

```text
check_tasks(status_filter="all")
```

- `pending` から `completed` への遷移を確認する
- 長文返信テスト時も必ずここまで見る

## ツール別の使い方

### `register_agent`

用途:

- エージェント名と現在の `pane_id` を紐付ける
- 再起動後の再登録
- 誤登録の修正

基本形:

```text
register_agent(name="<agent_name>", pane_id="<current_pane_id>")
```

補足:

- `role` と `skills` は必要なときだけ付ける
- 誰でも登録・更新できる

### `list_agents`

用途:

- 作業前の現況確認
- 登録漏れの検出
- 再登録後の検証

運用上の扱い:

- 登録済みエージェントのみ実行可能。未登録なら先に `register_agent` する
- 自身の登録後、最初に必ず使う
- 迷ったら再度使う

### `send_task`

用途:

- 作業依頼
- リマインド
- 他エージェントへの相談

補足:

- 応答方法テンプレートは既定で自動付与される（`include_response_instructions` を `false` にすると抑制可能）
- 自動付与テンプレートには具体的な `task_id` と `send_response(task_id="...", message="...")` が含まれる
- `from_agent` は返信先そのもの
- `task_label` は管理用ラベル（オプション、最大120文字）。省略時はメッセージ先頭50文字が自動設定される

### `get_my_tasks`

用途:

- 自分宛タスク確認
- `task_id` の再取得

補足:

- 受信側本人が使う前提
- 本文側に `task_id` が出ているため、通常は `get_my_tasks` がなくても返信できる
- ACL 挙動に疑義がある場合でも、運用上は本人確認前提で扱う

### `send_response`

用途:

- 依頼元への返信
- タスク完了記録

補足:

- 本文は短文でも長文でもよい
- 少なくとも短文、約500文字、約1000文字の返信は運用確認済み

### `check_tasks`

用途:

- タスク全体の監視
- タスクステータスの確認（`pending` / `completed` / `failed` / `abandoned`）
- 無応答時の切り分け

補足:

- `status_filter` でフィルタリングできる（オプション、デフォルト: `"all"`）。有効値: `"all"` / `"pending"` / `"completed"` / `"failed"` / `"abandoned"`
- `agent_name` で特定エージェントのタスクのみ取得可能

### `capture_pane`

用途:

- 相手ペインの状況確認

補足:

- current tmux shim では `warning` のみ返り、内容取得できないことがある
- 使えない場合でも、疎通本体の成否とは分けて判断する

## 運用上の注意

- 過去のペイン番号を資料からコピペしない
- 未登録なら自身を `register_agent` してから `list_agents()` を起点にする
- 登録が空なら、登録方法をユーザーに確認してから進める
- 送信前に、送信元と宛先の両方が登録済みであることを確認する
- 返信は必ず受信者本人が `send_response` で返す
- 疎通確認では `send_task` の成功だけで終わらず、`check_tasks()` で `completed` まで確認する

## 最低限の実行順

1. 自身が未登録なら `register_agent(...)` で自身を登録
2. `list_agents()`
3. 他のエージェントの登録が不足していればユーザーに登録方針を確認
4. 必要な `register_agent(...)` を実行
5. `list_agents()` で再確認
6. `send_task(...)`
7. 受信側で `get_my_tasks(...)`
8. 受信側で `send_response(...)`
9. `check_tasks(status_filter="all")`

この順を崩さない限り、実運用でのミスはかなり減らせる。
