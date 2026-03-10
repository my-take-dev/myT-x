package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"myT-x/internal/mcp/agent-orchestrator/domain"

	_ "modernc.org/sqlite"
)

func wrapNotFound(err error, msg string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", msg, domain.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// Store は SQLite3 アクセス層を提供する。
type Store struct {
	db *sql.DB
}

// Open は SQLite3 データベースを開く。
func Open(dbPath string) (*Store, error) {
	normalizedPath, err := normalizeDBPath(dbPath)
	if err != nil {
		return nil, fmt.Errorf("normalize db path: %w", err)
	}

	dir := filepath.Dir(normalizedPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", sqliteDSN(normalizedPath))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return &Store{db: db}, nil
}

// Migrate はテーブルとマイグレーションを適用する。
func (s *Store) Migrate() error {
	if err := createTables(s.db); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	return nil
}

// New は SQLite3 データベースを開き、テーブルを自動作成する。
func New(dbPath string) (*Store, error) {
	st, err := Open(dbPath)
	if err != nil {
		return nil, err
	}

	if err := st.Migrate(); err != nil {
		if closeErr := st.Close(); closeErr != nil {
			return nil, fmt.Errorf("migrate database: %w (close database: %v)", err, closeErr)
		}
		return nil, err
	}

	return st, nil
}

func normalizeDBPath(dbPath string) (string, error) {
	if strings.TrimSpace(dbPath) == "" {
		return "", fmt.Errorf("db path is required")
	}

	cleaned := filepath.Clean(dbPath)
	if strings.ContainsRune(cleaned, rune(0)) || strings.ContainsAny(cleaned, "?#") {
		return "", fmt.Errorf("invalid db path %q", dbPath)
	}

	return cleaned, nil
}

func sqliteDSN(dbPath string) string {
	return dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
}

func createTables(db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			name       TEXT PRIMARY KEY,
			pane_id    TEXT NOT NULL,
			role       TEXT,
			skills     TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			task_id      TEXT PRIMARY KEY,
			agent_name   TEXT NOT NULL REFERENCES agents(name),
			assignee_pane_id TEXT,
			sender_name  TEXT,
			label        TEXT,
			status       TEXT NOT NULL DEFAULT 'pending',
			sent_at      TEXT NOT NULL DEFAULT (datetime('now')),
			completed_at TEXT,
			notes        TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS config (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}

	migrations := []struct {
		table  string
		column string
		stmt   string
	}{
		{
			table:  "tasks",
			column: "sender_name",
			stmt:   `ALTER TABLE tasks ADD COLUMN sender_name TEXT`,
		},
		{
			table:  "tasks",
			column: "assignee_pane_id",
			stmt:   `ALTER TABLE tasks ADD COLUMN assignee_pane_id TEXT`,
		},
	}
	for _, m := range migrations {
		hasColumn, err := tableHasColumn(db, m.table, m.column)
		if err != nil {
			return fmt.Errorf("inspect migration %s.%s: %w", m.table, m.column, err)
		}
		if hasColumn {
			continue
		}
		if _, err := db.Exec(m.stmt); err != nil {
			return fmt.Errorf("apply migration %q: %w", m.stmt, err)
		}
	}

	return nil
}

func tableHasColumn(db *sql.DB, tableName string, columnName string) (bool, error) {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, tableName)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return false, err
		}
		if name == columnName {
			return true, nil
		}
	}

	return false, rows.Err()
}

// UpsertAgent はエージェントを登録または更新する。
func (s *Store) UpsertAgent(ctx context.Context, agent domain.Agent) error {
	skillsJSON, err := json.Marshal(agent.Skills)
	if err != nil {
		return fmt.Errorf("marshal skills: %w", err)
	}

	_, err = s.db.ExecContext(
		ctx,
		`INSERT INTO agents (name, pane_id, role, skills, created_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET pane_id=excluded.pane_id, role=excluded.role, skills=excluded.skills`,
		agent.Name, agent.PaneID, agent.Role, string(skillsJSON), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("upsert agent: %w", err)
	}
	return nil
}

// GetAgent は名前でエージェントを取得する。
func (s *Store) GetAgent(ctx context.Context, name string) (domain.Agent, error) {
	var a domain.Agent
	var skillsJSON sql.NullString
	err := s.db.QueryRowContext(
		ctx,
		`SELECT name, pane_id, role, skills, created_at FROM agents WHERE name = ?`, name,
	).Scan(&a.Name, &a.PaneID, &a.Role, &skillsJSON, &a.CreatedAt)
	if err != nil {
		return domain.Agent{}, wrapNotFound(err, fmt.Sprintf("get agent %q", name))
	}
	if skillsJSON.Valid && skillsJSON.String != "" {
		if jsonErr := json.Unmarshal([]byte(skillsJSON.String), &a.Skills); jsonErr != nil {
			return domain.Agent{}, fmt.Errorf("unmarshal skills: %w", jsonErr)
		}
	}
	return a, nil
}

// GetAgentByPaneID は pane_id でエージェントを取得する。
func (s *Store) GetAgentByPaneID(ctx context.Context, paneID string) (domain.Agent, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT name, pane_id, role, skills, created_at FROM agents WHERE pane_id = ? ORDER BY name`,
		paneID,
	)
	if err != nil {
		return domain.Agent{}, fmt.Errorf("get agent by pane_id %q: %w", paneID, err)
	}
	defer rows.Close()

	var agents []domain.Agent
	for rows.Next() {
		var a domain.Agent
		var skillsJSON sql.NullString
		if err := rows.Scan(&a.Name, &a.PaneID, &a.Role, &skillsJSON, &a.CreatedAt); err != nil {
			return domain.Agent{}, fmt.Errorf("scan agent by pane_id %q: %w", paneID, err)
		}
		if skillsJSON.Valid && skillsJSON.String != "" {
			if jsonErr := json.Unmarshal([]byte(skillsJSON.String), &a.Skills); jsonErr != nil {
				return domain.Agent{}, fmt.Errorf("unmarshal skills for pane_id %q: %w", paneID, jsonErr)
			}
		}
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		return domain.Agent{}, fmt.Errorf("iterate agents by pane_id %q: %w", paneID, err)
	}
	if len(agents) == 0 {
		return domain.Agent{}, fmt.Errorf("get agent by pane_id %q: %w", paneID, domain.ErrNotFound)
	}
	if len(agents) > 1 {
		return domain.Agent{}, fmt.Errorf("multiple agents registered for pane_id %q", paneID)
	}
	return agents[0], nil
}

// ListAgents は全エージェント情報を取得する。
func (s *Store) ListAgents(ctx context.Context) ([]domain.Agent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, pane_id, role, skills, created_at FROM agents ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []domain.Agent
	for rows.Next() {
		var a domain.Agent
		var skillsJSON sql.NullString
		if err := rows.Scan(&a.Name, &a.PaneID, &a.Role, &skillsJSON, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		if skillsJSON.Valid && skillsJSON.String != "" {
			if jsonErr := json.Unmarshal([]byte(skillsJSON.String), &a.Skills); jsonErr != nil {
				return nil, fmt.Errorf("unmarshal skills: %w", jsonErr)
			}
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// DeleteAgentsByPaneID は指定ペインIDに紐づくエージェントを削除する。
func (s *Store) DeleteAgentsByPaneID(ctx context.Context, paneID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM agents WHERE pane_id = ?`, paneID)
	if err != nil {
		return fmt.Errorf("delete agents by pane_id %q: %w", paneID, err)
	}
	return nil
}

// CreateTask はタスクを作成する。
func (s *Store) CreateTask(ctx context.Context, task domain.Task) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO tasks (task_id, agent_name, assignee_pane_id, sender_name, label, status, sent_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.AgentName, task.AssigneePaneID, task.SenderName, task.Label, task.Status, task.SentAt,
	)
	if err != nil {
		return fmt.Errorf("create task %q: %w", task.ID, err)
	}
	return nil
}

// GetTask はタスクIDでタスクを取得する。
func (s *Store) GetTask(ctx context.Context, taskID string) (domain.Task, error) {
	var t domain.Task
	var assigneePaneID, senderName, completedAt, notes sql.NullString
	err := s.db.QueryRowContext(
		ctx,
		`SELECT task_id, agent_name, assignee_pane_id, sender_name, label, status, sent_at, completed_at, notes FROM tasks WHERE task_id = ?`,
		taskID,
	).Scan(&t.ID, &t.AgentName, &assigneePaneID, &senderName, &t.Label, &t.Status, &t.SentAt, &completedAt, &notes)
	if err != nil {
		return domain.Task{}, wrapNotFound(err, fmt.Sprintf("get task %q", taskID))
	}
	if assigneePaneID.Valid {
		t.AssigneePaneID = assigneePaneID.String
	}
	if senderName.Valid {
		t.SenderName = senderName.String
	}
	if completedAt.Valid {
		t.CompletedAt = completedAt.String
	}
	if notes.Valid {
		t.Notes = notes.String
	}
	return t, nil
}

// ListTasks はフィルタ条件に基づいてタスクを取得する。
func (s *Store) ListTasks(ctx context.Context, filter domain.TaskFilter) ([]domain.Task, error) {
	query := `SELECT task_id, agent_name, assignee_pane_id, sender_name, label, status, sent_at, completed_at, notes FROM tasks WHERE 1=1`
	var args []any

	if filter.Status != "" && filter.Status != "all" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if filter.AgentName != "" {
		query += ` AND agent_name = ?`
		args = append(args, filter.AgentName)
	}
	query += ` ORDER BY sent_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []domain.Task
	for rows.Next() {
		var t domain.Task
		var assigneePaneID, senderName, completedAt, notes sql.NullString
		if err := rows.Scan(&t.ID, &t.AgentName, &assigneePaneID, &senderName, &t.Label, &t.Status, &t.SentAt, &completedAt, &notes); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		if assigneePaneID.Valid {
			t.AssigneePaneID = assigneePaneID.String
		}
		if senderName.Valid {
			t.SenderName = senderName.String
		}
		if completedAt.Valid {
			t.CompletedAt = completedAt.String
		}
		if notes.Valid {
			t.Notes = notes.String
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// CompleteTask はタスクを完了状態に更新する。
func (s *Store) CompleteTask(ctx context.Context, taskID string, notes string, completedAt string) error {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE tasks SET status = 'completed', completed_at = ?, notes = ? WHERE task_id = ? AND status = 'pending'`,
		completedAt, notes, taskID,
	)
	if err != nil {
		return fmt.Errorf("complete task %q: %w", taskID, err)
	}
	n, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("complete task %q: %w", taskID, rowsErr)
	}
	if n == 0 {
		task, getErr := s.GetTask(ctx, taskID)
		if getErr != nil {
			return fmt.Errorf("task %q not found", taskID)
		}
		return fmt.Errorf("task %q is %s", taskID, task.Status)
	}
	return nil
}

// MarkTaskFailed はタスクを failed 状態に更新する。
func (s *Store) MarkTaskFailed(ctx context.Context, taskID string, notes string) error {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE tasks SET status = 'failed', completed_at = ?, notes = ? WHERE task_id = ? AND status = 'pending'`,
		time.Now().UTC().Format(time.RFC3339), notes, taskID,
	)
	if err != nil {
		return fmt.Errorf("mark task failed %q: %w", taskID, err)
	}
	n, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("mark task failed %q: %w", taskID, rowsErr)
	}
	if n == 0 {
		task, getErr := s.GetTask(ctx, taskID)
		if getErr != nil {
			return fmt.Errorf("task %q not found", taskID)
		}
		return fmt.Errorf("task %q is %s", taskID, task.Status)
	}
	return nil
}

// AbandonTasksByPaneID は指定ペインIDのエージェントに紐づく pending タスクを abandoned に更新する。
func (s *Store) AbandonTasksByPaneID(ctx context.Context, paneID string) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE tasks SET status = 'abandoned'
		 WHERE status = 'pending'
		 AND (
		 	assignee_pane_id = ?
		 	OR agent_name IN (SELECT name FROM agents WHERE pane_id = ?)
		 )`,
		paneID, paneID,
	)
	if err != nil {
		return fmt.Errorf("abandon tasks by pane_id %q: %w", paneID, err)
	}
	return nil
}

// GetConfig は設定値を取得する。
func (s *Store) GetConfig(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&value)
	if err != nil {
		return "", fmt.Errorf("get config %q: %w", key, err)
	}
	return value, nil
}

// SetConfig は設定値を登録または更新する。
func (s *Store) SetConfig(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set config %q: %w", key, err)
	}
	return nil
}

// Close はデータベース接続を閉じる。
func (s *Store) Close() error {
	return s.db.Close()
}
