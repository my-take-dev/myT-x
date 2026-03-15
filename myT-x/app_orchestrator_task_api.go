package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"myT-x/internal/tmux"

	"golang.org/x/sys/windows"
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

	rows, err := db.Query(
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
func (a *App) ListOrchestratorAgents(sessionName string) ([]OrchestratorAgent, error) {
	db, cleanup, err := a.openOrchestratorDB(sessionName)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	rows, err := db.Query(
		`SELECT name, pane_id, COALESCE(role,'') FROM agents ORDER BY name`,
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

// GetPaneProcessStatus はセッション内の全ペインのプロセス実行状態を返す。
// シェルプロセスに子プロセスが存在する場合に HasChildProcess=true を返す。
func (a *App) GetPaneProcessStatus(sessionName string) ([]PaneProcessStatus, error) {
	sessions, err := a.requireSessions()
	if err != nil {
		return nil, err
	}

	panePIDs, err := sessions.GetSessionPanePIDs(sessionName)
	if err != nil {
		return nil, fmt.Errorf("[DEBUG-canvas] get pane PIDs: %w", err)
	}

	// プロセスツリーのスナップショットを一度だけ取得して全ペインを評価する。
	childPIDs, snapshotErr := buildChildPIDSet(panePIDs)
	if snapshotErr != nil {
		// スナップショット取得失敗時は全て false で返す。
		result := make([]PaneProcessStatus, len(panePIDs))
		for i, p := range panePIDs {
			result[i] = PaneProcessStatus{PaneID: p.PaneID}
		}
		return result, nil
	}

	result := make([]PaneProcessStatus, len(panePIDs))
	for i, p := range panePIDs {
		result[i] = PaneProcessStatus{
			PaneID:          p.PaneID,
			HasChildProcess: childPIDs[uint32(p.PID)],
		}
	}
	return result, nil
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
	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, nil, fmt.Errorf("[DEBUG-canvas] open orchestrator db: %w", err)
	}

	return db, func() { _ = db.Close() }, nil
}

// buildChildPIDSet は指定ペインPIDのいずれかを親に持つ子プロセスが存在するか判定する。
// 戻り値は shellPID → true のマップ（子プロセスが存在するPIDのみ true）。
func buildChildPIDSet(panePIDs []tmux.PanePIDInfo) (map[uint32]bool, error) {
	if len(panePIDs) == 0 {
		return nil, nil
	}

	// 対象PIDのセットを構築
	targetPIDs := make(map[uint32]struct{}, len(panePIDs))
	for _, p := range panePIDs {
		if p.PID > 0 {
			targetPIDs[uint32(p.PID)] = struct{}{}
		}
	}
	if len(targetPIDs) == 0 {
		return nil, nil
	}

	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("[DEBUG-canvas] CreateToolhelp32Snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)

	result := make(map[uint32]bool, len(targetPIDs))

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = windows.Process32First(snap, &entry)
	for err == nil {
		if _, ok := targetPIDs[entry.ParentProcessID]; ok {
			if entry.ProcessID != entry.ParentProcessID {
				result[entry.ParentProcessID] = true
			}
		}
		err = windows.Process32Next(snap, &entry)
	}

	return result, nil
}
