package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// OrchestratorTask はフロントエンドに返すタスク情報。
type OrchestratorTask struct {
	TaskID          string `json:"task_id"`
	AgentName       string `json:"agent_name"`
	SenderPaneID    string `json:"sender_pane_id"`
	AssigneePaneID  string `json:"assignee_pane_id"`
	SenderName      string `json:"sender_name"`
	Status          string `json:"status"`
	SentAt          string `json:"sent_at"`
	CompletedAt     string `json:"completed_at"`
	MessagePreview  string `json:"message_preview"`  // 依頼メッセージ冒頭80文字
	ResponsePreview string `json:"response_preview"` // 応答メッセージ冒頭80文字
}

// OrchestratorTaskDetail はタスクの詳細情報（メッセージ全文含む）。
type OrchestratorTaskDetail struct {
	TaskID          string `json:"task_id"`
	AgentName       string `json:"agent_name"`
	SenderName      string `json:"sender_name"`
	Status          string `json:"status"`
	SentAt          string `json:"sent_at"`
	CompletedAt     string `json:"completed_at"`
	MessageContent  string `json:"message_content"`
	ResponseContent string `json:"response_content"`
}

// OrchestratorAgent はフロントエンドに返すエージェント情報。
type OrchestratorAgent struct {
	Name   string `json:"name"`
	PaneID string `json:"pane_id"`
	Role   string `json:"role"`
}

// PaneProcessStatus はペインのプロセス実行状態。
type PaneProcessStatus struct {
	PaneID          string `json:"pane_id"`
	HasChildProcess bool   `json:"has_child_process"`
}

// ListOrchestratorTasks は現在のセッションのタスク一覧を返す（isNowSession=true のみ）。
func (a *App) ListOrchestratorTasks(sessionName string) ([]OrchestratorTask, error) {
	db, cleanup, err := a.openOrchestratorDB(sessionName)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	ctx := context.Background()
	rows, err := db.QueryContext(ctx,
		`SELECT t.task_id, t.agent_name, COALESCE(t.assignee_pane_id,''), COALESCE(t.sender_pane_id,''),
		        COALESCE(t.sender_name,''), t.status, t.sent_at, COALESCE(t.completed_at,''),
		        COALESCE(SUBSTR(m.content, 1, 80), ''),
		        COALESCE(SUBSTR(r.content, 1, 80), '')
		 FROM tasks t
		 LEFT JOIN send_messages m ON t.send_message_id = m.id
		 LEFT JOIN send_responses r ON t.send_response_id = r.id
		 WHERE t.is_now_session = 1 ORDER BY t.sent_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("[DEBUG-canvas] list tasks: %w", err)
	}
	defer rows.Close()

	var result []OrchestratorTask
	for rows.Next() {
		var t OrchestratorTask
		if err := rows.Scan(&t.TaskID, &t.AgentName, &t.AssigneePaneID, &t.SenderPaneID,
			&t.SenderName, &t.Status, &t.SentAt, &t.CompletedAt,
			&t.MessagePreview, &t.ResponsePreview); err != nil {
			return nil, fmt.Errorf("[DEBUG-canvas] scan task: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// GetOrchestratorTaskDetail はタスクの詳細情報（メッセージ全文含む）を返す。
func (a *App) GetOrchestratorTaskDetail(sessionName, taskID string) (*OrchestratorTaskDetail, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}

	db, cleanup, err := a.openOrchestratorDB(sessionName)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	ctx := context.Background()
	row := db.QueryRowContext(ctx,
		`SELECT t.task_id, t.agent_name, COALESCE(t.sender_name,''), t.status,
		        t.sent_at, COALESCE(t.completed_at,''),
		        COALESCE(m.content, ''), COALESCE(r.content, '')
		 FROM tasks t
		 LEFT JOIN send_messages m ON t.send_message_id = m.id
		 LEFT JOIN send_responses r ON t.send_response_id = r.id
		 WHERE t.task_id = ? AND t.is_now_session = 1`, taskID,
	)

	var d OrchestratorTaskDetail
	if err := row.Scan(&d.TaskID, &d.AgentName, &d.SenderName, &d.Status,
		&d.SentAt, &d.CompletedAt, &d.MessageContent, &d.ResponseContent); err != nil {
		return nil, fmt.Errorf("[DEBUG-canvas] get task detail: %w", err)
	}
	return &d, nil
}

// ListOrchestratorAgents は現在のセッションの登録エージェント一覧を返す。
// mcp_instance_id IS NOT NULL でフィルタし、有効なインスタンスに紐づくエージェントのみ返す。
func (a *App) ListOrchestratorAgents(sessionName string) ([]OrchestratorAgent, error) {
	db, cleanup, err := a.openOrchestratorDB(sessionName)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	ctx := context.Background()
	rows, err := db.QueryContext(ctx,
		`SELECT name, pane_id, COALESCE(role,'') FROM agents WHERE mcp_instance_id IS NOT NULL ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("[DEBUG-canvas] list agents: %w", err)
	}
	defer rows.Close()

	var result []OrchestratorAgent
	for rows.Next() {
		var ag OrchestratorAgent
		if err := rows.Scan(&ag.Name, &ag.PaneID, &ag.Role); err != nil {
			return nil, fmt.Errorf("[DEBUG-canvas] scan agent: %w", err)
		}
		result = append(result, ag)
	}
	return result, rows.Err()
}

// openOrchestratorDB はセッション名からオーケストレーターDBを開く。
func (a *App) openOrchestratorDB(sessionName string) (*sql.DB, func(), error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return nil, nil, fmt.Errorf("session name is required")
	}

	snapshot, err := a.findSessionSnapshotByName(sessionName)
	if err != nil {
		return nil, nil, err
	}

	rootPath, err := resolveOrchestratorSourceRootPath(snapshot)
	if err != nil {
		return nil, nil, err
	}

	dbPath := filepath.Join(rootPath, ".myT-x", "orchestrator.db")

	// I-8: DBファイルの存在確認
	if _, statErr := os.Stat(dbPath); statErr != nil {
		return nil, nil, fmt.Errorf("[DEBUG-canvas] orchestrator db not found: %w", statErr)
	}

	// I-8: _busy_timeout=5000 で他プロセスのロック待ちに対応
	db, err := sql.Open("sqlite", dbPath+"?mode=ro&_busy_timeout=5000")
	if err != nil {
		return nil, nil, fmt.Errorf("[DEBUG-canvas] open orchestrator db: %w", err)
	}

	// M-4: クローズエラーをログ出力
	return db, func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Warn("[WARN-canvas] failed to close orchestrator db", "error", closeErr)
		}
	}, nil
}
