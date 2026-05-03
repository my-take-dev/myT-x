package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	agentorchestrator "myT-x/internal/mcp/agent-orchestrator"
	"myT-x/internal/mcp/agent-orchestrator/domain"
	"myT-x/internal/orchestrator"
	"myT-x/internal/orchestratorstorage"

	_ "modernc.org/sqlite"
)

var errOrchestratorDBNotReady = errors.New("orchestrator db is not initialized")

var orchestratorDBStat = os.Stat

// OrchestratorTask はフロントエンドに返すタスク情報。
type OrchestratorTask struct {
	TaskID          string `json:"task_id"`
	AgentName       string `json:"agent_name"`
	AssigneePaneID  string `json:"assignee_pane_id"`
	SenderPaneID    string `json:"sender_pane_id"`
	SenderName      string `json:"sender_name"`
	Status          string `json:"status"`
	SentAt          string `json:"sent_at"`
	CompletedAt     string `json:"completed_at"`
	MessagePreview  string `json:"message_preview"`  // 依頼メッセージ冒頭80文字
	ResponsePreview string `json:"response_preview"` // 応答メッセージ冒頭80文字
}

// OrchestratorTaskDetail contains task payload content or stored-payload metadata.
type OrchestratorTaskDetail struct {
	TaskID                string   `json:"task_id"`
	AgentName             string   `json:"agent_name"`
	SenderName            string   `json:"sender_name"`
	Status                string   `json:"status"`
	SentAt                string   `json:"sent_at"`
	CompletedAt           string   `json:"completed_at"`
	MessageContent        string   `json:"message_content"`
	MessagePreview        string   `json:"message_preview"`
	MessageStorageMode    string   `json:"message_storage_mode"`
	MessageArtifactPaths  []string `json:"message_artifact_paths"`
	MessagePartCount      int      `json:"message_part_count"`
	MessageContentChars   int      `json:"message_content_chars"`
	MessageSHA256         string   `json:"message_sha256"`
	ResponseContent       string   `json:"response_content"`
	ResponsePreview       string   `json:"response_preview"`
	ResponseStorageMode   string   `json:"response_storage_mode"`
	ResponseArtifactPaths []string `json:"response_artifact_paths"`
	ResponsePartCount     int      `json:"response_part_count"`
	ResponseContentChars  int      `json:"response_content_chars"`
	ResponseSHA256        string   `json:"response_sha256"`
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
	db, cleanup, ok, err := a.openOrchestratorDBOptional(sessionName)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []OrchestratorTask{}, nil
	}
	defer cleanup()

	ctx := context.Background()
	rows, err := db.QueryContext(ctx,
		`SELECT t.task_id, t.agent_name, COALESCE(t.assignee_pane_id,''), COALESCE(t.sender_pane_id,''),
		        COALESCE(t.sender_name,''), t.status, t.sent_at, COALESCE(t.completed_at,''),
		        COALESCE(NULLIF(SUBSTR(m.content_preview, 1, 80), ''), SUBSTR(m.content, 1, 80), ''),
		        COALESCE(NULLIF(SUBSTR(r.content_preview, 1, 80), ''), SUBSTR(r.content, 1, 80), '')
		 FROM tasks t
		 LEFT JOIN send_messages m ON t.send_message_id = m.id
		 LEFT JOIN send_responses r ON t.send_response_id = r.id
		 WHERE t.is_now_session = 1 ORDER BY t.sent_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list orchestrator tasks: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("[WARN-canvas] failed to close orchestrator task rows", "error", closeErr)
		}
	}()

	var result []OrchestratorTask
	for rows.Next() {
		var t OrchestratorTask
		if err := rows.Scan(&t.TaskID, &t.AgentName, &t.AssigneePaneID, &t.SenderPaneID,
			&t.SenderName, &t.Status, &t.SentAt, &t.CompletedAt,
			&t.MessagePreview, &t.ResponsePreview); err != nil {
			return nil, fmt.Errorf("scan orchestrator task: %w", err)
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

// GetOrchestratorTaskDetail returns task payload content or stored-payload metadata.
func (a *App) GetOrchestratorTaskDetail(sessionName, taskID string) (*OrchestratorTaskDetail, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	projectRoot, err := a.resolveOrchestratorProjectRoot(sessionName)
	if err != nil {
		return nil, err
	}

	db, cleanup, ok, err := a.openOrchestratorDBOptional(sessionName)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	defer cleanup()

	ctx := context.Background()
	row := db.QueryRowContext(ctx,
		`SELECT t.task_id, t.agent_name, COALESCE(t.sender_name,''), t.status,
		        t.sent_at, COALESCE(t.completed_at,''),
		        COALESCE(m.content, ''), COALESCE(m.content_preview, ''), COALESCE(m.storage_mode, ''),
		        COALESCE(m.artifact_paths_json, '[]'), COALESCE(m.part_count, 0), COALESCE(m.content_chars, 0), COALESCE(m.sha256, ''),
		        COALESCE(r.content, ''), COALESCE(r.content_preview, ''), COALESCE(r.storage_mode, ''),
		        COALESCE(r.artifact_paths_json, '[]'), COALESCE(r.part_count, 0), COALESCE(r.content_chars, 0), COALESCE(r.sha256, '')
		 FROM tasks t
		 LEFT JOIN send_messages m ON t.send_message_id = m.id
		 LEFT JOIN send_responses r ON t.send_response_id = r.id
		 WHERE t.task_id = ? AND t.is_now_session = 1`, taskID,
	)

	var (
		d                         OrchestratorTaskDetail
		messageArtifactPathsJSON  string
		responseArtifactPathsJSON string
	)
	if err := row.Scan(
		&d.TaskID,
		&d.AgentName,
		&d.SenderName,
		&d.Status,
		&d.SentAt,
		&d.CompletedAt,
		&d.MessageContent,
		&d.MessagePreview,
		&d.MessageStorageMode,
		&messageArtifactPathsJSON,
		&d.MessagePartCount,
		&d.MessageContentChars,
		&d.MessageSHA256,
		&d.ResponseContent,
		&d.ResponsePreview,
		&d.ResponseStorageMode,
		&responseArtifactPathsJSON,
		&d.ResponsePartCount,
		&d.ResponseContentChars,
		&d.ResponseSHA256,
	); err != nil {
		return nil, fmt.Errorf("get orchestrator task detail: %w", err)
	}
	d.MessageArtifactPaths, err = parseArtifactPaths(projectRoot, messageArtifactPathsJSON)
	if err != nil {
		return nil, fmt.Errorf("parse message artifact paths: %w", err)
	}
	d.ResponseArtifactPaths, err = parseArtifactPaths(projectRoot, responseArtifactPathsJSON)
	if err != nil {
		return nil, fmt.Errorf("parse response artifact paths: %w", err)
	}
	return &d, nil
}

// ListOrchestratorAgents は現在のセッションの登録エージェント一覧を返す。
// Provisional registrations are included so freshly enlisted panes appear immediately.
func (a *App) ListOrchestratorAgents(sessionName string) ([]OrchestratorAgent, error) {
	db, cleanup, ok, err := a.openOrchestratorDBOptional(sessionName)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []OrchestratorAgent{}, nil
	}
	defer cleanup()

	ctx := context.Background()
	rows, err := db.QueryContext(ctx,
		`SELECT name, pane_id, COALESCE(role,'') FROM agents ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list orchestrator agents: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("[WARN-canvas] failed to close orchestrator agent rows", "error", closeErr)
		}
	}()

	var result []OrchestratorAgent
	for rows.Next() {
		var ag OrchestratorAgent
		if err := rows.Scan(&ag.Name, &ag.PaneID, &ag.Role); err != nil {
			return nil, fmt.Errorf("scan orchestrator agent: %w", err)
		}
		result = append(result, ag)
	}
	return result, rows.Err()
}

// openOrchestratorDB はセッション名からオーケストレーターDBを開く。
func (a *App) openOrchestratorDB(sessionName string) (*sql.DB, func(), error) {
	return a.openOrchestratorDBWithMode(sessionName, "ro")
}

func (a *App) openOrchestratorDBOptional(sessionName string) (*sql.DB, func(), bool, error) {
	db, cleanup, err := a.openOrchestratorDB(sessionName)
	if err != nil {
		if errors.Is(err, errOrchestratorDBNotReady) {
			return nil, nil, false, nil
		}
		return nil, nil, false, err
	}
	return db, cleanup, true, nil
}

func (a *App) openOrchestratorDBWritable(sessionName string) (*sql.DB, func(), error) {
	dbPath, err := a.resolveOrchestratorDBPathForSession(sessionName)
	if err != nil {
		return nil, nil, err
	}
	if err := agentorchestrator.EnsureDatabase(dbPath); err != nil {
		return nil, nil, fmt.Errorf("initialize orchestrator db: %w", err)
	}
	return openOrchestratorDBPath(dbPath, "rwc")
}

func (a *App) openOrchestratorDBWithMode(sessionName string, mode string) (*sql.DB, func(), error) {
	dbPath, err := a.resolveOrchestratorDBPath(sessionName)
	if err != nil {
		return nil, nil, err
	}
	return openOrchestratorDBPath(dbPath, mode)
}

func openOrchestratorDBPath(dbPath string, mode string) (*sql.DB, func(), error) {
	dsn := dbPath + "?mode=" + mode
	if mode == "ro" {
		dsn += "&_pragma=busy_timeout(5000)"
	} else {
		dsn += "&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open orchestrator db: %w", err)
	}

	return db, func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Warn("[WARN-canvas] failed to close orchestrator db", "error", closeErr)
		}
	}, nil
}

func (a *App) resolveOrchestratorProjectRoot(sessionName string) (string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return "", fmt.Errorf("session name is required")
	}

	snapshot, err := a.sessionService.FindSessionSnapshotByName(sessionName)
	if err != nil {
		return "", err
	}

	rootPath, err := orchestrator.ResolveSourceRootPath(snapshot)
	if err != nil {
		return "", err
	}
	return rootPath, nil
}

func (a *App) resolveOrchestratorDBPath(sessionName string) (string, error) {
	dbPath, err := a.resolveOrchestratorDBPathForSession(sessionName)
	if err != nil {
		return "", err
	}

	// I-8: DBファイルの存在確認
	if _, statErr := orchestratorDBStat(dbPath); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			return "", fmt.Errorf("%w: %w", errOrchestratorDBNotReady, statErr)
		}
		return "", fmt.Errorf("stat orchestrator db: %w", statErr)
	}
	return dbPath, nil
}

func (a *App) resolveOrchestratorDBPathForSession(sessionName string) (string, error) {
	rootPath, err := a.resolveOrchestratorProjectRoot(sessionName)
	if err != nil {
		return "", err
	}
	return a.resolveOrchestratorDBPathForProjectRoot(rootPath)
}

func (a *App) resolveOrchestratorDBPathForProjectRoot(rootPath string) (string, error) {
	configDir, err := appConfigDirProvider(a)()
	if err != nil {
		return "", err
	}
	return orchestratorstorage.DBPath(configDir, rootPath)
}

func parseArtifactPaths(projectRoot string, raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var paths []string
	if err := json.Unmarshal([]byte(raw), &paths); err != nil {
		return nil, err
	}
	return domain.ResolveArtifactPaths(projectRoot, paths), nil
}
