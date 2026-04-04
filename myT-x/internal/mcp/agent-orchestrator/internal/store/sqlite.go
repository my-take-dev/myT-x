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

type queryRower interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// scanner is a common interface for sql.Row and sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

const taskSelectColumns = `task_id, agent_name, assignee_pane_id, sender_pane_id,
sender_name, sender_instance_id, send_message_id, send_response_id,
status, sent_at, completed_at, acknowledged_at, cancelled_at, cancel_reason,
progress_pct, progress_note, progress_updated_at, expires_at, group_id, is_now_session`

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
// status, sent_at, completed_at, acknowledged_at, cancelled_at, cancel_reason,
// progress_pct, progress_note, progress_updated_at, expires_at, group_id,
// is_now_session.
func scanTask(sc scanner) (domain.Task, error) {
	var t domain.Task
	var assigneePaneID, senderPaneID, senderName, senderInstanceID sql.NullString
	var sendMessageID, sendResponseID, completedAt sql.NullString
	var acknowledgedAt, cancelledAt, cancelReason sql.NullString
	var progressPct sql.NullInt64
	var progressNote, progressUpdatedAt, expiresAt, groupID sql.NullString
	if err := sc.Scan(
		&t.ID,
		&t.AgentName,
		&assigneePaneID,
		&senderPaneID,
		&senderName,
		&senderInstanceID,
		&sendMessageID,
		&sendResponseID,
		&t.Status,
		&t.SentAt,
		&completedAt,
		&acknowledgedAt,
		&cancelledAt,
		&cancelReason,
		&progressPct,
		&progressNote,
		&progressUpdatedAt,
		&expiresAt,
		&groupID,
		&t.IsNowSession,
	); err != nil {
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
	if acknowledgedAt.Valid {
		t.AcknowledgedAt = acknowledgedAt.String
	}
	if cancelledAt.Valid {
		t.CancelledAt = cancelledAt.String
	}
	if cancelReason.Valid {
		t.CancelReason = cancelReason.String
	}
	if progressPct.Valid {
		value := int(progressPct.Int64)
		t.ProgressPct = &value
	}
	if progressNote.Valid {
		t.ProgressNote = progressNote.String
	}
	if progressUpdatedAt.Valid {
		t.ProgressUpdatedAt = progressUpdatedAt.String
	}
	if expiresAt.Valid {
		t.ExpiresAt = expiresAt.String
	}
	if groupID.Valid {
		t.GroupID = groupID.String
	}
	return t, nil
}

func scanAgentStatus(sc scanner) (domain.AgentStatus, error) {
	var status domain.AgentStatus
	var currentTaskID, note, updatedAt sql.NullString
	if err := sc.Scan(&status.AgentName, &status.Status, &currentTaskID, &note, &updatedAt); err != nil {
		return domain.AgentStatus{}, err
	}
	if currentTaskID.Valid {
		status.CurrentTaskID = currentTaskID.String
	}
	if note.Valid {
		status.Note = note.String
	}
	if updatedAt.Valid {
		status.UpdatedAt = updatedAt.String
	}
	return status, nil
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
			sender_pane_id TEXT,
			sender_name  TEXT,
			sender_instance_id TEXT,
			send_message_id  TEXT,
			send_response_id TEXT,
			status       TEXT NOT NULL DEFAULT 'pending',
			sent_at      TEXT NOT NULL DEFAULT (datetime('now')),
			completed_at TEXT,
			acknowledged_at TEXT,
			cancelled_at TEXT,
			cancel_reason TEXT,
			progress_pct INTEGER,
			progress_note TEXT,
			progress_updated_at TEXT,
			expires_at TEXT,
			group_id TEXT,
			is_now_session INTEGER NOT NULL DEFAULT 0
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
		`CREATE TABLE IF NOT EXISTS agent_status (
			agent_name TEXT PRIMARY KEY REFERENCES agents(name) ON DELETE CASCADE,
			status TEXT NOT NULL,
			current_task_id TEXT,
			note TEXT,
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS task_groups (
			group_id TEXT PRIMARY KEY,
			group_label TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE IF NOT EXISTS task_dependencies (
			task_id TEXT NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
			depends_on_task_id TEXT NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
			dependency_order INTEGER NOT NULL,
			PRIMARY KEY (task_id, depends_on_task_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_task_deps_depends ON task_dependencies(depends_on_task_id)`,
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
		{
			table:  "tasks",
			column: "acknowledged_at",
			stmt:   `ALTER TABLE tasks ADD COLUMN acknowledged_at TEXT`,
		},
		{
			table:  "tasks",
			column: "cancelled_at",
			stmt:   `ALTER TABLE tasks ADD COLUMN cancelled_at TEXT`,
		},
		{
			table:  "tasks",
			column: "cancel_reason",
			stmt:   `ALTER TABLE tasks ADD COLUMN cancel_reason TEXT`,
		},
		{
			table:  "tasks",
			column: "progress_pct",
			stmt:   `ALTER TABLE tasks ADD COLUMN progress_pct INTEGER`,
		},
		{
			table:  "tasks",
			column: "progress_note",
			stmt:   `ALTER TABLE tasks ADD COLUMN progress_note TEXT`,
		},
		{
			table:  "tasks",
			column: "progress_updated_at",
			stmt:   `ALTER TABLE tasks ADD COLUMN progress_updated_at TEXT`,
		},
		{
			table:  "tasks",
			column: "expires_at",
			stmt:   `ALTER TABLE tasks ADD COLUMN expires_at TEXT`,
		},
		{
			table:  "tasks",
			column: "group_id",
			stmt:   `ALTER TABLE tasks ADD COLUMN group_id TEXT`,
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete agents by pane_id %q: begin tx: %w", paneID, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM agent_status WHERE agent_name IN (SELECT name FROM agents WHERE pane_id = ?)`,
		paneID,
	); err != nil {
		return fmt.Errorf("delete agent statuses by pane_id %q: %w", paneID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM agents WHERE pane_id = ?`, paneID); err != nil {
		return fmt.Errorf("delete agents by pane_id %q: %w", paneID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete agents by pane_id %q: commit: %w", paneID, err)
	}
	return nil
}

func (s *Store) UpsertAgentStatus(ctx context.Context, status domain.AgentStatus) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO agent_status (agent_name, status, current_task_id, note, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(agent_name) DO UPDATE SET
			status=excluded.status,
			current_task_id=excluded.current_task_id,
			note=excluded.note,
			updated_at=excluded.updated_at`,
		status.AgentName,
		status.Status,
		nullableString(status.CurrentTaskID),
		nullableString(status.Note),
		status.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert agent status %q: %w", status.AgentName, err)
	}
	return nil
}

func (s *Store) GetAgentStatus(ctx context.Context, agentName string) (domain.AgentStatus, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT agent_name, status, current_task_id, note, updated_at FROM agent_status WHERE agent_name = ?`,
		agentName,
	)
	status, err := scanAgentStatus(row)
	if err != nil {
		return domain.AgentStatus{}, wrapNotFound(err, fmt.Sprintf("get agent status %q", agentName))
	}
	return status, nil
}

func (s *Store) ListAgentStatuses(ctx context.Context) ([]domain.AgentStatus, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT agent_name, status, current_task_id, note, updated_at FROM agent_status ORDER BY agent_name`)
	if err != nil {
		return nil, fmt.Errorf("list agent statuses: %w", err)
	}
	defer rows.Close()

	statuses := make([]domain.AgentStatus, 0)
	for rows.Next() {
		status, scanErr := scanAgentStatus(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan agent status: %w", scanErr)
		}
		statuses = append(statuses, status)
	}
	return statuses, rows.Err()
}

func insertTask(ctx context.Context, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, task domain.Task) error {
	_, err := executor.ExecContext(
		ctx,
		`INSERT INTO tasks (
			task_id, agent_name, assignee_pane_id, sender_pane_id, sender_name,
			sender_instance_id, send_message_id, status, sent_at, expires_at, group_id, is_now_session
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
		task.ID,
		task.AgentName,
		task.AssigneePaneID,
		nullableString(task.SenderPaneID),
		task.SenderName,
		nullableString(task.SenderInstanceID),
		nullableString(task.SendMessageID),
		task.Status,
		task.SentAt,
		nullableString(task.ExpiresAt),
		nullableString(task.GroupID),
	)
	if err != nil {
		return fmt.Errorf("create task %q: %w", task.ID, err)
	}
	return nil
}

func insertTaskDependencies(ctx context.Context, tx *sql.Tx, taskID string, dependencyTaskIDs []string) error {
	if len(dependencyTaskIDs) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO task_dependencies (task_id, depends_on_task_id, dependency_order) VALUES (?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("create task dependencies %q: prepare: %w", taskID, err)
	}
	defer stmt.Close()

	for idx, dependencyTaskID := range dependencyTaskIDs {
		if _, err := stmt.ExecContext(ctx, taskID, dependencyTaskID, idx); err != nil {
			return fmt.Errorf("create task dependencies %q -> %q: %w", taskID, dependencyTaskID, err)
		}
	}
	return nil
}

func validateDependencyTaskIDs(ctx context.Context, query queryRower, taskID string, dependencyTaskIDs []string) error {
	seen := make(map[string]struct{}, len(dependencyTaskIDs))
	for _, dependencyTaskID := range dependencyTaskIDs {
		if dependencyTaskID == taskID {
			return fmt.Errorf("task %q cannot depend on itself", taskID)
		}
		if _, ok := seen[dependencyTaskID]; ok {
			return fmt.Errorf("duplicate dependency task %q", dependencyTaskID)
		}
		seen[dependencyTaskID] = struct{}{}

		var exists int
		if err := query.QueryRowContext(ctx, `SELECT 1 FROM tasks WHERE task_id = ?`, dependencyTaskID).Scan(&exists); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("dependency task %q not found", dependencyTaskID)
			}
			return fmt.Errorf("lookup dependency task %q: %w", dependencyTaskID, err)
		}
	}
	return nil
}

// CreateTask はタスクを作成する。
func (s *Store) CreateTask(ctx context.Context, task domain.Task) error {
	// is_now_session=1: Tasks created in the current session are always marked as
	// "now session". EndSessionByInstanceID bulk-updates is_now_session=0 when the
	// MCP instance terminates, so that subsequent sessions start with a clean view.
	return insertTask(ctx, s.db, task)
}

// CreateTaskGroup persists batch metadata for grouped task dispatch.
func (s *Store) CreateTaskGroup(ctx context.Context, group domain.TaskGroup) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO task_groups (group_id, group_label, created_at) VALUES (?, ?, ?)`,
		group.ID,
		nullableString(group.Label),
		group.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create task group %q: %w", group.ID, err)
	}
	return nil
}

// DeleteTaskGroup removes persisted batch metadata when no task was created.
func (s *Store) DeleteTaskGroup(ctx context.Context, groupID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM task_groups WHERE group_id = ?`, groupID)
	if err != nil {
		return fmt.Errorf("delete task group %q: %w", groupID, err)
	}
	return nil
}

// CreateTaskWithDependencies stores a task and its dependency edges atomically.
func (s *Store) CreateTaskWithDependencies(ctx context.Context, task domain.Task, dependencyTaskIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create task with dependencies %q: begin tx: %w", task.ID, err)
	}
	defer tx.Rollback()

	if err := validateDependencyTaskIDs(ctx, tx, task.ID, dependencyTaskIDs); err != nil {
		return fmt.Errorf("create task with dependencies %q: %w", task.ID, err)
	}
	if err := insertTask(ctx, tx, task); err != nil {
		return err
	}
	if err := insertTaskDependencies(ctx, tx, task.ID, dependencyTaskIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create task with dependencies %q: commit: %w", task.ID, err)
	}
	return nil
}

// CreateTaskDependencies stores dependency edges for a blocked task.
func (s *Store) CreateTaskDependencies(ctx context.Context, taskID string, dependencyTaskIDs []string) error {
	if len(dependencyTaskIDs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create task dependencies %q: begin tx: %w", taskID, err)
	}
	defer tx.Rollback()

	if err := validateDependencyTaskIDs(ctx, tx, taskID, dependencyTaskIDs); err != nil {
		return fmt.Errorf("create task dependencies %q: %w", taskID, err)
	}
	if err := insertTaskDependencies(ctx, tx, taskID, dependencyTaskIDs); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("create task dependencies %q: commit: %w", taskID, err)
	}
	return nil
}

// GetTask はタスクIDでタスクを取得する。
func (s *Store) GetTask(ctx context.Context, taskID string) (domain.Task, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT `+taskSelectColumns+` FROM tasks WHERE task_id = ?`,
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
	query := `SELECT ` + taskSelectColumns + ` FROM tasks WHERE 1=1`
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

// GetTaskDependencies returns dependency task IDs in the original input order.
func (s *Store) GetTaskDependencies(ctx context.Context, taskID string) ([]string, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT depends_on_task_id FROM task_dependencies WHERE task_id = ? ORDER BY dependency_order`,
		taskID,
	)
	if err != nil {
		return nil, fmt.Errorf("get task dependencies %q: %w", taskID, err)
	}
	defer rows.Close()

	dependencyTaskIDs := make([]string, 0)
	for rows.Next() {
		var dependencyTaskID string
		if err := rows.Scan(&dependencyTaskID); err != nil {
			return nil, fmt.Errorf("scan task dependency %q: %w", taskID, err)
		}
		dependencyTaskIDs = append(dependencyTaskIDs, dependencyTaskID)
	}
	return dependencyTaskIDs, rows.Err()
}

type dependencyResolution int

const (
	dependencyResolutionReady dependencyResolution = iota
	dependencyResolutionWaiting
	dependencyResolutionBroken
)

func listBlockedTasksForActivation(ctx context.Context, tx *sql.Tx, agentName string) ([]domain.Task, error) {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT `+taskSelectColumns+` FROM tasks
		WHERE status = ?
		  AND (? = '' OR agent_name = ?)
		ORDER BY sent_at ASC`,
		domain.TaskStatusBlocked,
		agentName,
		agentName,
	)
	if err != nil {
		return nil, fmt.Errorf("query blocked tasks: %w", err)
	}
	defer rows.Close()

	blockedTasks := make([]domain.Task, 0)
	for rows.Next() {
		task, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan blocked task: %w", scanErr)
		}
		blockedTasks = append(blockedTasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate blocked tasks: %w", err)
	}
	return blockedTasks, nil
}

func classifyDependencyResolution(ctx context.Context, tx *sql.Tx, taskID string) (dependencyResolution, string, string, error) {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT dep.task_id, dep.status
		FROM task_dependencies d
		LEFT JOIN tasks dep ON dep.task_id = d.depends_on_task_id
		WHERE d.task_id = ?
		ORDER BY d.dependency_order ASC`,
		taskID,
	)
	if err != nil {
		return dependencyResolutionWaiting, "", "", fmt.Errorf("query dependencies for %q: %w", taskID, err)
	}
	defer rows.Close()

	hasDependency := false
	waiting := false
	for rows.Next() {
		hasDependency = true
		var dependencyTaskID sql.NullString
		var dependencyStatus sql.NullString
		if err := rows.Scan(&dependencyTaskID, &dependencyStatus); err != nil {
			return dependencyResolutionWaiting, "", "", fmt.Errorf("scan dependency for %q: %w", taskID, err)
		}
		if !dependencyTaskID.Valid {
			return dependencyResolutionBroken, "", "", nil
		}
		switch dependencyStatus.String {
		case domain.TaskStatusCompleted:
			continue
		case domain.TaskStatusCancelled, domain.TaskStatusFailed, domain.TaskStatusAbandoned, domain.TaskStatusExpired:
			return dependencyResolutionBroken, dependencyTaskID.String, dependencyStatus.String, nil
		default:
			waiting = true
		}
	}
	if err := rows.Err(); err != nil {
		return dependencyResolutionWaiting, "", "", fmt.Errorf("iterate dependencies for %q: %w", taskID, err)
	}
	if !hasDependency {
		return dependencyResolutionReady, "", "", nil
	}
	if waiting {
		return dependencyResolutionWaiting, "", "", nil
	}
	return dependencyResolutionReady, "", "", nil
}

func dependencyFailureCancelReason(dependencyTaskID, dependencyStatus string) string {
	if dependencyTaskID == "" {
		return "dependency task is not available"
	}
	if dependencyStatus == "" {
		return fmt.Sprintf("dependency task %s is not available", dependencyTaskID)
	}
	return fmt.Sprintf("dependency task %s ended with status %s", dependencyTaskID, dependencyStatus)
}

func cancelBlockedTask(ctx context.Context, tx *sql.Tx, taskID string, now string, reason string) error {
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE tasks
		 SET status = ?, cancelled_at = ?, cancel_reason = ?, completed_at = ?
		 WHERE task_id = ? AND status = ?`,
		domain.TaskStatusCancelled,
		now,
		nullableString(reason),
		now,
		taskID,
		domain.TaskStatusBlocked,
	); err != nil {
		return fmt.Errorf("cancel blocked task %q: %w", taskID, err)
	}
	return nil
}

// ActivateReadyTasks transitions blocked tasks to pending once all dependencies are completed.
func (s *Store) ActivateReadyTasks(ctx context.Context, now string, agentName string) ([]domain.Task, int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("activate ready tasks: begin tx: %w", err)
	}
	defer tx.Rollback()

	blockedTasks, err := listBlockedTasksForActivation(ctx, tx, agentName)
	if err != nil {
		return nil, 0, fmt.Errorf("activate ready tasks: %w", err)
	}

	activated := make([]domain.Task, 0, len(blockedTasks))
	stillBlocked := 0
	for _, task := range blockedTasks {
		resolution, dependencyTaskID, dependencyStatus, err := classifyDependencyResolution(ctx, tx, task.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("activate ready tasks: %w", err)
		}
		switch resolution {
		case dependencyResolutionBroken:
			if err := cancelBlockedTask(ctx, tx, task.ID, now, dependencyFailureCancelReason(dependencyTaskID, dependencyStatus)); err != nil {
				return nil, 0, fmt.Errorf("activate ready tasks: %w", err)
			}
			continue
		case dependencyResolutionWaiting:
			stillBlocked++
			continue
		}
		if task.ExpiresAt != "" && task.ExpiresAt <= now {
			if _, err := tx.ExecContext(
				ctx,
				`UPDATE tasks SET status = ?, completed_at = ? WHERE task_id = ? AND status = ?`,
				domain.TaskStatusExpired,
				now,
				task.ID,
				domain.TaskStatusBlocked,
			); err != nil {
				return nil, 0, fmt.Errorf("activate ready tasks: expire %q: %w", task.ID, err)
			}
			continue
		}
		if _, err := tx.ExecContext(
			ctx,
			`UPDATE tasks SET status = ? WHERE task_id = ? AND status = ?`,
			domain.TaskStatusPending,
			task.ID,
			domain.TaskStatusBlocked,
		); err != nil {
			return nil, 0, fmt.Errorf("activate ready tasks: activate %q: %w", task.ID, err)
		}
		task.Status = domain.TaskStatusPending
		activated = append(activated, task)
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("activate ready tasks: commit: %w", err)
	}
	return activated, stillBlocked, nil
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

func (s *Store) AcknowledgeTask(ctx context.Context, taskID string, acknowledgedAt string) error {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status != domain.TaskStatusPending {
		return fmt.Errorf("task %q is %s", taskID, task.Status)
	}
	if task.AcknowledgedAt != "" {
		return nil
	}

	result, err := s.db.ExecContext(
		ctx,
		`UPDATE tasks
		 SET acknowledged_at = ?
		 WHERE task_id = ? AND status = 'pending' AND acknowledged_at IS NULL`,
		acknowledgedAt,
		taskID,
	)
	if err != nil {
		return fmt.Errorf("acknowledge task %q: %w", taskID, err)
	}
	n, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("acknowledge task %q: %w", taskID, rowsErr)
	}
	if n == 0 {
		latest, getErr := s.GetTask(ctx, taskID)
		if getErr != nil {
			return fmt.Errorf("task %q not found", taskID)
		}
		if latest.AcknowledgedAt != "" {
			return nil
		}
		return fmt.Errorf("task %q is %s", taskID, latest.Status)
	}
	return nil
}

func (s *Store) CancelTask(ctx context.Context, taskID string, cancelledAt string, reason string) error {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status == domain.TaskStatusCancelled {
		return nil
	}
	if task.Status != domain.TaskStatusPending && task.Status != domain.TaskStatusBlocked {
		return fmt.Errorf("task %q is %s", taskID, task.Status)
	}

	result, err := s.db.ExecContext(
		ctx,
		`UPDATE tasks
		 SET status = 'cancelled', cancelled_at = ?, cancel_reason = ?, completed_at = ?
		 WHERE task_id = ? AND status IN ('pending', 'blocked')`,
		cancelledAt,
		nullableString(reason),
		cancelledAt,
		taskID,
	)
	if err != nil {
		return fmt.Errorf("cancel task %q: %w", taskID, err)
	}
	n, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("cancel task %q: %w", taskID, rowsErr)
	}
	if n == 0 {
		latest, getErr := s.GetTask(ctx, taskID)
		if getErr != nil {
			return fmt.Errorf("task %q not found", taskID)
		}
		if latest.Status == domain.TaskStatusCancelled {
			return nil
		}
		return fmt.Errorf("task %q is %s", taskID, latest.Status)
	}
	return nil
}

func (s *Store) UpdateTaskProgress(ctx context.Context, taskID string, progressPct *int, progressNote *string, progressUpdatedAt string) error {
	task, err := s.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status != domain.TaskStatusPending {
		return fmt.Errorf("task %q is %s", taskID, task.Status)
	}

	progressPctValue := any(nil)
	if progressPct != nil {
		progressPctValue = *progressPct
	}
	progressNoteValue := any(nil)
	if progressNote != nil {
		progressNoteValue = *progressNote
	}

	result, err := s.db.ExecContext(
		ctx,
		`UPDATE tasks
		 SET progress_pct = COALESCE(?, progress_pct),
			progress_note = CASE WHEN ? IS NULL THEN progress_note ELSE ? END,
			progress_updated_at = ?
		 WHERE task_id = ? AND status = 'pending'`,
		progressPctValue,
		progressNoteValue,
		progressNoteValue,
		progressUpdatedAt,
		taskID,
	)
	if err != nil {
		return fmt.Errorf("update task progress %q: %w", taskID, err)
	}
	n, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("update task progress %q: %w", taskID, rowsErr)
	}
	if n == 0 {
		latest, getErr := s.GetTask(ctx, taskID)
		if getErr != nil {
			return fmt.Errorf("task %q not found", taskID)
		}
		return fmt.Errorf("task %q is %s", taskID, latest.Status)
	}
	return nil
}

func (s *Store) ExpirePendingTasks(ctx context.Context, now string) (int64, error) {
	result, err := s.db.ExecContext(
		ctx,
		`UPDATE tasks
		 SET status = 'expired', completed_at = ?
		 WHERE status = 'pending' AND expires_at IS NOT NULL AND expires_at <= ?`,
		now,
		now,
	)
	if err != nil {
		return 0, fmt.Errorf("expire pending tasks: %w", err)
	}
	return result.RowsAffected()
}

// AbandonTasksByPaneID は指定ペインIDのエージェントに紐づく pending / blocked タスクを abandoned に更新する。
func (s *Store) AbandonTasksByPaneID(ctx context.Context, paneID string) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE tasks SET status = 'abandoned'
		 WHERE status IN ('pending', 'blocked')
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
		`SELECT `+taskSelectColumns+` FROM tasks WHERE send_message_id = ?`,
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

func (s *Store) GetResponse(ctx context.Context, id string) (domain.TaskMessage, error) {
	var msg domain.TaskMessage
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, content, created_at FROM send_responses WHERE id = ?`, id,
	).Scan(&msg.ID, &msg.Content, &msg.CreatedAt)
	if err != nil {
		return domain.TaskMessage{}, wrapNotFound(err, fmt.Sprintf("get response %q", id))
	}
	return msg, nil
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("cleanup stale agents: begin tx: %w", err)
	}
	defer tx.Rollback()

	var (
		statusQuery string
		agentQuery  string
		args        []any
	)
	if len(activeInstanceIDs) == 0 {
		statusQuery = `DELETE FROM agent_status`
		agentQuery = `DELETE FROM agents`
	} else {
		placeholders := strings.Repeat("?,", len(activeInstanceIDs))
		placeholders = placeholders[:len(placeholders)-1]
		args = make([]any, len(activeInstanceIDs))
		for i, id := range activeInstanceIDs {
			args[i] = id
		}
		staleAgentCondition := `mcp_instance_id IS NULL OR (mcp_instance_id IS NOT NULL AND mcp_instance_id NOT IN (` + placeholders + `))`
		statusQuery = `DELETE FROM agent_status WHERE agent_name IN (SELECT name FROM agents WHERE ` + staleAgentCondition + `)`
		agentQuery = `DELETE FROM agents WHERE ` + staleAgentCondition
	}

	if _, err := tx.ExecContext(ctx, statusQuery, args...); err != nil {
		return 0, fmt.Errorf("cleanup stale agent statuses: %w", err)
	}
	result, err := tx.ExecContext(ctx, agentQuery, args...)
	if err != nil {
		return 0, fmt.Errorf("cleanup stale agents: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("cleanup stale agents: commit: %w", err)
	}
	return result.RowsAffected()
}

// CleanupStaleTasks は生存リストに含まれないインスタンスの pending / blocked タスクを abandoned に更新する。
// sender_instance_id が NULL（旧形式）の pending / blocked タスクも古いデータとして abandoned に更新する。
func (s *Store) CleanupStaleTasks(ctx context.Context, activeInstanceIDs []string) (int64, error) {
	if len(activeInstanceIDs) == 0 {
		// No active instances: all pending / blocked tasks are stale, abandon everything.
		result, err := s.db.ExecContext(ctx,
			`UPDATE tasks SET status = 'abandoned', is_now_session = 0 WHERE status IN ('pending', 'blocked')`)
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
		 WHERE status IN ('pending', 'blocked')
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
