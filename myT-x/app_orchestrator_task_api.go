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
	TaskID         string `json:"task_id"`
	AgentName      string `json:"agent_name"`
	SenderPaneID   string `json:"sender_pane_id"`
	AssigneePaneID string `json:"assignee_pane_id"`
	SenderName     string `json:"sender_name"`
	Status         string `json:"status"`
	SentAt         string `json:"sent_at"`
	CompletedAt    string `json:"completed_at"`
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
		`SELECT task_id, agent_name, COALESCE(assignee_pane_id,''), COALESCE(sender_pane_id,''),
		        COALESCE(sender_name,''), status, sent_at, COALESCE(completed_at,'')
		 FROM tasks WHERE is_now_session = 1 ORDER BY sent_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("[DEBUG-canvas] list tasks: %w", err)
	}
	defer rows.Close()

	var result []OrchestratorTask
	for rows.Next() {
		var t OrchestratorTask
		if err := rows.Scan(&t.TaskID, &t.AgentName, &t.AssigneePaneID, &t.SenderPaneID,
			&t.SenderName, &t.Status, &t.SentAt, &t.CompletedAt); err != nil {
			return nil, fmt.Errorf("[DEBUG-canvas] scan task: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
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
