package mcptool

import "sort"

// helpOverview は全体概要のヘルプ情報を返す。
func helpOverview() map[string]any {
	return map[string]any{
		"title":       "Agent Orchestrator MCP ヘルプ",
		"description": "エージェント間のタスク管理・通信を行うオーケストレーターMCPサーバーです。複数のAIエージェントが協調して作業を進めるための基盤を提供します。",
		"available_tools": []string{
			"register_agent", "list_agents", "send_task",
			"get_my_tasks", "get_my_task", "send_response",
			"check_tasks", "capture_pane", "add_member", "help",
		},
		"workflow": map[string]any{
			"title": "典型的なワークフロー",
			"steps": []string{
				"1. register_agent: 自分をエージェントとして登録する（必須・全ツール利用の前提条件）",
				"2. get_my_tasks: 自分宛のpendingタスクを確認する",
				"3. get_my_task: send_message_id を指定してタスクのメッセージ本文を取得する",
				"4. タスクを実行する",
				"5. send_response: task_id を指定してタスクに応答する（タスク完了も同時記録）",
			},
			"additional": []string{
				"send_task: 他エージェントにタスクを送信（誰でも送信可能）",
				"check_tasks: 全タスクの状態を俯瞰的に確認",
				"list_agents: 全エージェント情報と未登録ペインを確認",
				"capture_pane: 他エージェントのペイン表示を取得（進捗・エラー確認用）",
				"add_member: 新メンバーを動的に追加（ペイン分割・CLI起動・ブートストラップ）",
			},
		},
		"best_practices": []string{
			"最初に必ず register_agent を実行してください。他ツールは登録後でないと利用できません",
			"send_response の task_id は必須です。省略するとタスクが完了状態になりません",
			"include_response_instructions=true（デフォルト）で送ると、相手にsend_responseの使い方が自動付与されます",
			"他エージェントへの相談はsend_taskで直接送信できます（orchestrator経由は不要）",
			"capture_pane で他エージェントの画面を確認し、進捗やエラーを把握できます",
			"add_member で動的にメンバーを追加できます。追加後は自動でブートストラップメッセージが送信されます",
		},
		"tip": "topic パラメータで各ツールの詳細ヘルプを取得できます（例: topic=\"send_task\"）",
	}
}

// helpForTool は指定ツールの詳細ヘルプ情報を返す。
// 存在しないツール名の場合は nil, false を返す。
func helpForTool(topic string) (map[string]any, bool) {
	h, ok := toolHelps[topic]
	if !ok {
		return nil, false
	}
	return h, true
}

// availableTopics は help で指定可能なトピック一覧をソート済みで返す。
func availableTopics() []string {
	topics := make([]string, 0, len(toolHelps))
	for k := range toolHelps {
		topics = append(topics, k)
	}
	sort.Strings(topics)
	return topics
}

var toolHelps = map[string]map[string]any{
	"register_agent": {
		"tool":        "register_agent",
		"description": "エージェントのペインIDと名前を紐付け、ロール・得意分野をSQLiteに記録する。同名で再呼び出しすると情報を更新する。",
		"parameters": map[string]any{
			"name（必須）":    "エージェント名（英数字・._-、最大64文字）",
			"pane_id（必須）": "tmux ペインID（例: \"%1\"）",
			"role（任意）":    "役割（最大120文字）",
			"skills（任意）":  "得意分野の配列（最大20件）。オブジェクト形式 [{\"name\":\"Go\",\"description\":\"...\"}] 推奨",
		},
		"notes": []string{
			"全ツール利用の前提条件。最初に必ず実行すること",
			"同じ pane_id に既に別エージェントが登録されている場合は上書きされる",
			"登録・更新は caller の pane に関係なく実行できる",
		},
	},
	"list_agents": {
		"tool":        "list_agents",
		"description": "全エージェント情報を取得し、tmux list-panes と突合して全ペインの状態を返す。",
		"parameters":  map[string]any{},
		"notes": []string{
			"registered_agents（登録済み）と unregistered_panes（未登録ペイン）を返す",
			"登録済みエージェントのみ実行可能",
		},
	},
	"send_task": {
		"tool":        "send_task",
		"description": "エージェント間でメッセージを送信し、SQLiteにタスクを記録する。",
		"parameters": map[string]any{
			"agent_name（必須）":                    "宛先エージェント名",
			"from_agent（必須）":                    "送信元エージェント名（自分の登録名）",
			"message（必須）":                       "送信メッセージ（最大8000文字）",
			"include_response_instructions（任意）": "応答テンプレート自動付与（デフォルト: true）",
		},
		"notes": []string{
			"誰でも送信可能（orchestrator経由不要）",
			"送信成功時に task_id が返る。相手はこの task_id で send_response する",
			"デフォルトでメッセージ末尾に応答方法テンプレートが自動付与される",
			"送信失敗時はタスクが failed 状態になる",
		},
	},
	"get_my_tasks": {
		"tool":        "get_my_tasks",
		"description": "自分宛のタスク情報と応答方法をSQLiteから取得する。",
		"parameters": map[string]any{
			"agent_name（必須）":    "自分のエージェント名",
			"status_filter（任意）": "\"pending\" / \"completed\" / \"all\" / \"failed\" / \"abandoned\"（デフォルト: pending）",
		},
		"notes": []string{
			"呼び出し元の登録名と agent_name が一致する場合のみ返す",
			"戻り値の各タスクに send_message_id が含まれる。これを get_my_task に渡してメッセージ本文を取得できる",
			"response_instructions（返信手順）も戻り値に含まれる",
		},
	},
	"get_my_task": {
		"tool":        "get_my_task",
		"description": "send_message_id から自分宛タスクのメッセージ本文とメタデータを取得する。",
		"parameters": map[string]any{
			"agent_name（必須）":      "自分のエージェント名",
			"send_message_id（必須）": "取得対象の send_message_id（m- プレフィックス）",
		},
		"notes": []string{
			"get_my_tasks で取得した send_message_id を指定して使う",
			"呼び出し元の登録名と agent_name が一致する場合のみ返す",
			"戻り値に message.content（メッセージ本文）と message.created_at が含まれる",
		},
	},
	"send_response": {
		"tool":        "send_response",
		"description": "タスク送信者にメッセージを返信し、対象タスクを completed に更新する。",
		"parameters": map[string]any{
			"task_id（必須）": "対応する task_id",
			"message（必須）": "返信メッセージ（最大8000文字）",
		},
		"notes": []string{
			"pending 状態の task_id を持つ担当者のみ実行可能",
			"task_id を省略するとエラー。タスクを完了できなくなるので注意",
			"送信者のペインにメッセージが送られ、タスクが completed になる",
		},
	},
	"check_tasks": {
		"tool":        "check_tasks",
		"description": "全タスクの状態をSQLiteから取得する。",
		"parameters": map[string]any{
			"status_filter（任意）": "\"pending\" / \"completed\" / \"all\" / \"failed\" / \"abandoned\"（デフォルト: all）",
			"agent_name（任意）":    "特定エージェントのタスクのみ取得",
		},
		"notes": []string{
			"登録済みエージェントであれば誰でも実行可能",
			"戻り値の summary に pending/completed/failed/abandoned 件数を含む",
			"全体の進捗把握に便利",
		},
	},
	"capture_pane": {
		"tool":        "capture_pane",
		"description": "指定エージェントのペイン表示内容を取得する。",
		"parameters": map[string]any{
			"agent_name（必須）": "対象エージェント名",
			"lines（任意）":      "取得行数（1-200、デフォルト: 50）",
		},
		"notes": []string{
			"登録済みエージェントであれば誰でも実行可能",
			"相手の進捗確認・エラー確認に使用",
		},
	},
	"add_member": {
		"tool":        "add_member",
		"description": "新メンバーを動的に追加する。ペイン分割→CLI起動→ブートストラップメッセージ送信を一括実行。",
		"parameters": map[string]any{
			"pane_title（必須）":         "メンバー表示名（最大30文字）",
			"role（必須）":               "役割（最大120文字）",
			"command（必須）":            "CLIコマンド（例: \"claude\"）（最大100文字）",
			"args（任意）":               "コマンド引数配列（最大20件）",
			"custom_message（任意）":     "追加指示メッセージ（最大2000文字）",
			"skills（任意）":             "得意分野配列（register_agentと同形式、最大20件）",
			"team_name（任意）":          "チーム名（最大64文字、デフォルト: \"動的チーム\"）",
			"split_from（任意）":         "分割元ペインID（デフォルト: 呼び出し元ペイン）",
			"split_direction（任意）":    "\"horizontal\" / \"vertical\"（デフォルト: \"horizontal\"）",
			"bootstrap_delay_ms（任意）": "CLI起動後の待ち時間ms（1000-30000、デフォルト: 3000）",
		},
		"notes": []string{
			"登録済みエージェントのみ実行可能",
			"split_from を省略すると呼び出し元のペインを分割する",
			"Claude CLI（claude, claude.exe, claude-code*）はブラケットペーストモードで送信される",
			"ペインタイトル設定やブートストラップ送信の失敗はwarningとして返され、処理は続行される",
			"追加されたメンバーは自動でブートストラップメッセージを受信し、register_agent の指示を受ける",
		},
	},
	"help": {
		"tool":        "help",
		"description": "オーケストレーターMCPの使い方ヘルプを返す。",
		"parameters": map[string]any{
			"topic（任意）": "ヘルプトピック（ツール名を指定。省略時は全体概要）",
		},
		"notes": []string{
			"登録不要で誰でも利用可能",
			"topic を省略すると全体概要とワークフローを返す",
			"ツール名を指定するとそのツールの詳細ヘルプを返す",
		},
	},
}
