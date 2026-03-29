package store

import (
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/hex"
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

// unmarshalSkillsJSON は JSON 文字列から []domain.Skill をパースする。
// 新形式 [{"name":"x","description":"y"}] と旧形式 ["x","y"] の両方に対応する。
func unmarshalSkillsJSON(raw string) ([]domain.Skill, error) {
	var skills []domain.Skill
	newErr := json.Unmarshal([]byte(raw), &skills)
	if newErr == nil {
		return skills, nil
	}
	// 旧形式: []string → []Skill{Name: s} に変換
	var legacy []string
	if legacyErr := json.Unmarshal([]byte(raw), &legacy); legacyErr != nil {
		return nil, fmt.Errorf("unmarshal skills: new format: %w, legacy format: %v", newErr, legacyErr)
	}
	skills = make([]domain.Skill, len(legacy))
	for i, s := range legacy {
		skills[i] = domain.Skill{Name: s}
	}
	return skills, nil
}

func wrapNotFound(err error, msg string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", msg, domain.ErrNotFound)
	}
	return fmt.Errorf("%s: %w", msg, err)
}

// nullableString converts a non-empty string to sql.NullString for INSERT.
func nullableString(s string) any {
	if s != "" {
		return s
	}
	return nil
}

// scanner is a common interface for sql.Row and sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

// scanAgent scans a single agent row from the given scanner.
// Column order: name, pane_id, role, skills, created_at, mcp_instance_id.
func scanAgent(sc scanner) (domain.Agent, error) {
	var a domain.Agent
	var skillsJSON, mcpInstanceID sql.NullString
	if err := sc.Scan(&a.Name, &a.PaneID, &a.Role, &skillsJSON, &a.CreatedAt, &mcpInstanceID); err != nil {
		return domain.Agent{}, err
	}
	if mcpInstanceID.Valid {
		a.MCPInstanceID = mcpInstanceID.String
	}
	if skillsJSON.Valid && skillsJSON.String != "" {
		skills, jsonErr := unmarshalSkillsJSON(skillsJSON.String)
		if jsonErr != nil {
			return domain.Agent{}, fmt.Errorf("unmarshal skills: %w", jsonErr)
		}
		a.Skills = skills
	}
	return a, nil
}

// scanTask scans a single task row from the given scanner.
// Column order: task_id, agent_name, assignee_pane_id, sender_pane_id,
// sender_name, sender_instance_id, send_message_id, send_response_id,
// status, sent_at, completed_at, is_now_session.
func scanTask(sc scanner) (domain.Task, error) {
	var t domain.Task
	var assigneePaneID, senderPaneID, senderName, senderInstanceID, sendMessageID, sendResponseID, completedAt sql.NullString
	if err := sc.Scan(&t.ID, &t.AgentName, &assigneePaneID, &senderPaneID, &senderName, &senderInstanceID, &sendMessageID, &sendResponseID, &t.Status, &t.SentAt, &completedAt, &t.IsNowSession); err != nil {
		return domain.Task{}, err
	}
	if assigneePaneID.Valid {
		t.AssigneePaneID = assigneePaneID.String
	}
	if senderPaneID.Valid {
		t.SenderPaneID = senderPaneID.String
	}
	if senderName.Valid {
		t.SenderName = senderName.String
	}
	if senderInstanceID.Valid {
		t.SenderInstanceID = senderInstanceID.String
	}
	if sendMessageID.Valid {
		t.SendMessageID = sendMessageID.String
	}
	if sendResponseID.Valid {
		t.SendResponseID = sendResponseID.String
	}
	if completedAt.Valid {
		t.CompletedAt = completedAt.String
	}
	return t, nil
}

// generateID generates a random hex ID with the given prefix.
func generateID(prefix string) (string, error) {
	b := make([]byte, 6)
	if _, err := crand.Read(b); err != nil {
		return "", fmt.Errorf("generate %s id: %w", prefix, err)
	}
	return prefix + hex.EncodeToString(b), nil
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
			send_message_id  TEXT,
			send_response_id TEXT,
			status       TEXT NOT NULL DEFAULT 'pending',
			sent_at      TEXT NOT NULL DEFAULT (datetime('now')),
			completed_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS send_messages (
			id         TEXT PRIMARY KEY,
			content    TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS send_responses (
			id         TEXT PRIMARY KEY,
			content    TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS config (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS mcp_instances (
			instance_id TEXT PRIMARY KEY,
			started_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt[:40], err)
		}
	}

	// TODO(GA): Migrate to schema_version based migration management.
	// Current column-existence checks work but don't support column renames/deletes
	// and have linear startup cost as migrations grow.
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
		{
			table:  "agents",
			column: "mcp_instance_id",
			stmt:   `ALTER TABLE agents ADD COLUMN mcp_instance_id TEXT`,
		},
		{
			table:  "tasks",
			column: "sender_instance_id",
			stmt:   `ALTER TABLE tasks ADD COLUMN sender_instance_id TEXT`,
		},
		{
			table:  "tasks",
			column: "send_message_id",
			stmt:   `ALTER TABLE tasks ADD COLUMN send_message_id TEXT`,
		},
		{
			table:  "tasks",
			column: "send_response_id",
			stmt:   `ALTER TABLE tasks ADD COLUMN send_response_id TEXT`,
		},
		{
			table:  "tasks",
			column: "sender_pane_id",
			stmt:   `ALTER TABLE tasks ADD COLUMN sender_pane_id TEXT`,
		},
		{
			table:  "tasks",
			column: "is_now_session",
			stmt:   `ALTER TABLE tasks ADD COLUMN is_now_session INTEGER NOT NULL DEFAULT 0`,
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
		`INSERT INTO agents (name, pane_id, role, skills, mcp_instance_id, created_at) VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET pane_id=excluded.pane_id, role=excluded.role, skills=excluded.skills, mcp_instance_id=excluded.mcp_instance_id`,
		agent.Name, agent.PaneID, agent.Role, string(skillsJSON), nullableString(agent.MCPInstanceID), time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("upsert agent: %w", err)
	}
	return nil
}

// GetAgent は名前でエージェントを取得する。
func (s *Store) GetAgent(ctx context.Context, name string) (domain.Agent, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT name, pane_id, role, skills, created_at, mcp_instance_id FROM agents WHERE name = ?`, name,
	)
	a, err := scanAgent(row)
	if err != nil {
		return domain.Agent{}, wrapNotFound(err, fmt.Sprintf("get agent %q", name))
	}
	return a, nil
}

// GetAgentByPaneID は pane_id でエージェントを取得する。
func (s *Store) GetAgentByPaneID(ctx context.Context, paneID string) (domain.Agent, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT name, pane_id, role, skills, created_at, mcp_instance_id FROM agents WHERE pane_id = ? ORDER BY name`,
		paneID,
	)
	if err != nil {
		return domain.Agent{}, fmt.Errorf("get agent by pane_id %q: %w", paneID, err)
	}
	defer rows.Close()

	var agents []domain.Agent
	for rows.Next() {
		a, scanErr := scanAgent(rows)
		if scanErr != nil {
			return domain.Agent{}, fmt.Errorf("scan agent by pane_id %q: %w", paneID, scanErr)
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
	rows, err := s.db.QueryContext(ctx, `SELECT name, pane_id, role, skills, created_at, mcp_instance_id FROM agents ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var agents []domain.Agent
	for rows.Next() {
		a, scanErr := scanAgent(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan agent: %w", scanErr)
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
	// is_now_session=1: Tasks created in the current session are always marked as
	// "now session". EndSessionByInstanceID bulk-updates is_now_session=0 when the
	// MCP instance terminates, so that subsequent sessions start with a clean view.
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO tasks (task_id, agent_name, assignee_pane_id, sender_pane_id, sender_name, sender_instance_id, send_message_id, status, sent_at, is_now_session) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
		task.ID, task.AgentName, task.AssigneePaneID, nullableString(task.SenderPaneID), task.SenderName, nullableString(task.SenderInstanceID), nullableString(task.SendMessageID), task.Status, task.SentAt,
	)
	if err != nil {
		return fmt.Errorf("create task %q: %w", task.ID, err)
	}
	return nil
}

// GetTask はタスクIDでタスクを取得する。
func (s *Store) GetTask(ctx context.Context, taskID string) (domain.Task, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT task_id, agent_name, assignee_pane_id, sender_pane_id, sender_name, sender_instance_id, send_message_id, send_response_id, status, sent_at, completed_at, is_now_session FROM tasks WHERE task_id = ?`,
		taskID,
	)
	t, err := scanTask(row)
	if err != nil {
		return domain.Task{}, wrapNotFound(err, fmt.Sprintf("get task %q", taskID))
	}
	return t, nil
}

// ListTasks はフィルタ条件に基づいてタスクを取得する。
func (s *Store) ListTasks(ctx context.Context, filter domain.TaskFilter) ([]domain.Task, error) {
	query := `SELECT task_id, agent_name, assignee_pane_id, sender_pane_id, sender_name, sender_instance_id, send_message_id, send_response_id, status, sent_at, completed_at, is_now_session FROM tasks WHERE 1=1`
	var args []any

	if filter.Status != "" && filter.Status != "all" {
		query += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if filter.AgentName != "" {
		query += ` AND agent_name = ?`
		args = append(args, filter.AgentName)
	}
	if filter.IsNowSession != nil {
		if *filter.IsNowSession {
			query += ` AND is_now_session = 1`
		} else {
			query += ` AND is_now_session = 0`
		}
	}
	query += ` ORDER BY sent_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []domain.Task
	for rows.Next() {
		t, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan task: %w", scanErr)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// CompleteTask はタスクを完了状態に更新する。
func (s *Store) CompleteTask(ctx context.Context, taskID string, responseID string, completedAt string) error {
	var respID any
	if responseID != "" {
		respID = responseID
	}
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE tasks SET status = 'completed', completed_at = ?, send_response_id = ? WHERE task_id = ? AND status = 'pending'`,
		completedAt, respID, taskID,
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
func (s *Store) MarkTaskFailed(ctx context.Context, taskID string) error {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE tasks SET status = 'failed', completed_at = ? WHERE task_id = ? AND status = 'pending'`,
		time.Now().UTC().Format(time.RFC3339), taskID,
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

// EndSessionByInstanceID は指定インスタンスIDに関連するタスクの is_now_session を false に更新する。
// sender（送信者）側と assignee（担当者）側の両方を更新する。
func (s *Store) EndSessionByInstanceID(ctx context.Context, instanceID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET is_now_session = 0
		 WHERE sender_instance_id = ?
		    OR assignee_pane_id IN (SELECT pane_id FROM agents WHERE mcp_instance_id = ?)`,
		instanceID, instanceID,
	)
	if err != nil {
		return fmt.Errorf("end session by instance_id %q: %w", instanceID, err)
	}
	return nil
}

// SaveMessage は送信メッセージを保存する。
func (s *Store) SaveMessage(ctx context.Context, msg domain.TaskMessage) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO send_messages (id, content, created_at) VALUES (?, ?, ?)`,
		msg.ID, msg.Content, msg.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save message %q: %w", msg.ID, err)
	}
	return nil
}

// GetMessage は送信メッセージを取得する。
func (s *Store) GetMessage(ctx context.Context, id string) (domain.TaskMessage, error) {
	var msg domain.TaskMessage
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, content, created_at FROM send_messages WHERE id = ?`, id,
	).Scan(&msg.ID, &msg.Content, &msg.CreatedAt)
	if err != nil {
		return domain.TaskMessage{}, wrapNotFound(err, fmt.Sprintf("get message %q", id))
	}
	return msg, nil
}

// GetTaskBySendMessageID は send_message_id でタスクを取得する。
func (s *Store) GetTaskBySendMessageID(ctx context.Context, sendMessageID string) (domain.Task, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT task_id, agent_name, assignee_pane_id, sender_pane_id, sender_name, sender_instance_id, send_message_id, send_response_id, status, sent_at, completed_at, is_now_session FROM tasks WHERE send_message_id = ?`,
		sendMessageID,
	)
	t, err := scanTask(row)
	if err != nil {
		return domain.Task{}, wrapNotFound(err, fmt.Sprintf("get task by send_message_id %q", sendMessageID))
	}
	return t, nil
}

// SaveResponse は応答メッセージを保存する。
func (s *Store) SaveResponse(ctx context.Context, msg domain.TaskMessage) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO send_responses (id, content, created_at) VALUES (?, ?, ?)`,
		msg.ID, msg.Content, msg.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save response %q: %w", msg.ID, err)
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

// RegisterInstance は MCP インスタンスを生存リストに登録する。
func (s *Store) RegisterInstance(ctx context.Context, instanceID string) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO mcp_instances (instance_id, started_at) VALUES (?, ?)`,
		instanceID, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("register instance %q: %w", instanceID, err)
	}
	return nil
}

// UnregisterInstance は MCP インスタンスを生存リストから除去する。
func (s *Store) UnregisterInstance(ctx context.Context, instanceID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM mcp_instances WHERE instance_id = ?`, instanceID)
	if err != nil {
		return fmt.Errorf("unregister instance %q: %w", instanceID, err)
	}
	return nil
}

// ListActiveInstances は生存中の MCP インスタンスIDを返す。
func (s *Store) ListActiveInstances(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT instance_id FROM mcp_instances`)
	if err != nil {
		return nil, fmt.Errorf("list active instances: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan instance id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CleanupStaleAgents は生存リストに含まれないインスタンスのエージェントを削除する。
// mcp_instance_id が NULL（旧形式）のエージェントも古いデータとして削除する。
func (s *Store) CleanupStaleAgents(ctx context.Context, activeInstanceIDs []string) (int64, error) {
	if len(activeInstanceIDs) == 0 {
		// No active instances: all agents are stale, delete everything.
		result, err := s.db.ExecContext(ctx, `DELETE FROM agents`)
		if err != nil {
			return 0, fmt.Errorf("cleanup stale agents: %w", err)
		}
		return result.RowsAffected()
	}
	placeholders := strings.Repeat("?,", len(activeInstanceIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(activeInstanceIDs))
	for i, id := range activeInstanceIDs {
		args[i] = id
	}
	result, err := s.db.ExecContext(ctx,
		`DELETE FROM agents WHERE mcp_instance_id IS NULL OR (mcp_instance_id IS NOT NULL AND mcp_instance_id NOT IN (`+placeholders+`))`,
		args...)
	if err != nil {
		return 0, fmt.Errorf("cleanup stale agents: %w", err)
	}
	return result.RowsAffected()
}

// CleanupStaleTasks は生存リストに含まれないインスタンスの pending タスクを abandoned に更新する。
// sender_instance_id が NULL（旧形式）の pending タスクも古いデータとして abandoned に更新する。
func (s *Store) CleanupStaleTasks(ctx context.Context, activeInstanceIDs []string) (int64, error) {
	if len(activeInstanceIDs) == 0 {
		// No active instances: all pending tasks are stale, abandon everything.
		result, err := s.db.ExecContext(ctx,
			`UPDATE tasks SET status = 'abandoned', is_now_session = 0 WHERE status = 'pending'`)
		if err != nil {
			return 0, fmt.Errorf("cleanup stale tasks: %w", err)
		}
		return result.RowsAffected()
	}
	placeholders := strings.Repeat("?,", len(activeInstanceIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(activeInstanceIDs))
	for i, id := range activeInstanceIDs {
		args[i] = id
	}
	result, err := s.db.ExecContext(ctx,
		`UPDATE tasks SET status = 'abandoned', is_now_session = 0
		 WHERE status = 'pending'
		   AND (sender_instance_id IS NULL OR sender_instance_id NOT IN (`+placeholders+`))`,
		args...)
	if err != nil {
		return 0, fmt.Errorf("cleanup stale tasks: %w", err)
	}
	return result.RowsAffected()
}

// Close はデータベース接続を閉じる。
func (s *Store) Close() error {
	return s.db.Close()
}
