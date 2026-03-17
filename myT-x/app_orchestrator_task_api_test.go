package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"myT-x/internal/tmux"

	_ "modernc.org/sqlite"
)

func newOrchestratorTaskTestApp(t *testing.T) *App {
	t.Helper()

	app := NewApp()
	app.configPath = filepath.Join(t.TempDir(), "config.yaml")
	app.sessions = tmux.NewSessionManager()
	return app
}

// createOrchestratorTaskTestDB creates a temporary SQLite database with agents and tasks tables.
// Returns the database path.
func createOrchestratorTaskTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()

	tmpDir := t.TempDir()
	dbDir := filepath.Join(tmpDir, ".myT-x")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	dbPath := filepath.Join(dbDir, "orchestrator.db")
	dsn := dbPath + "?mode=rwc&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}

	// Create schema
	schema := []string{
		`CREATE TABLE agents (
			name            TEXT PRIMARY KEY,
			pane_id         TEXT NOT NULL,
			role            TEXT,
			skills          TEXT,
			mcp_instance_id TEXT,
			created_at      TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE TABLE tasks (
			task_id           TEXT PRIMARY KEY,
			agent_name        TEXT NOT NULL REFERENCES agents(name),
			assignee_pane_id  TEXT,
			sender_pane_id    TEXT,
			sender_name       TEXT,
			status            TEXT NOT NULL DEFAULT 'pending',
			sent_at           TEXT NOT NULL DEFAULT (datetime('now')),
			completed_at      TEXT,
			is_now_session    INTEGER NOT NULL DEFAULT 0
		)`,
	}

	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			t.Fatalf("schema creation error: %v", err)
		}
	}

	return db, tmpDir
}

// createOrchestratorTestSession creates a test session with a root path pointing to tmpDir.
func createOrchestratorTestSession(t *testing.T, app *App, sessionName, tmpDir string) tmux.SessionSnapshot {
	t.Helper()

	session, _, err := app.sessions.CreateSession(sessionName, "main", 120, 40)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := app.sessions.SetRootPath(sessionName, tmpDir); err != nil {
		t.Fatalf("SetRootPath() error = %v", err)
	}

	snapshot, ok := app.sessions.GetSession(session.Name)
	if !ok {
		t.Fatalf("GetSession(%q) returned not found", sessionName)
	}
	return tmux.SessionSnapshot{
		ID:             snapshot.ID,
		Name:           snapshot.Name,
		ActiveWindowID: snapshot.ActiveWindowID,
		Windows:        app.sessions.Snapshot()[0].Windows,
		RootPath:       tmpDir,
	}
}

func TestOrchestratorTaskFieldCount(t *testing.T) {
	if got := reflect.TypeFor[OrchestratorTask]().NumField(); got != 8 {
		t.Fatalf("OrchestratorTask has %d fields (expected 8). Update ListOrchestratorTasks scan logic and this constant.", got)
	}
}

func TestOpenOrchestratorDB(t *testing.T) {
	tests := []struct {
		name        string
		setupDB     bool
		sessionName string
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "valid session with db file",
			setupDB:     true,
			sessionName: "valid-session",
			wantErr:     false,
		},
		{
			name:        "missing session",
			setupDB:     false,
			sessionName: "missing-session",
			wantErr:     true,
			errMsg:      "not found",
		},
		{
			name:        "empty session name",
			setupDB:     true,
			sessionName: "",
			wantErr:     true,
			errMsg:      "session name is required",
		},
		{
			name:        "whitespace session name",
			setupDB:     true,
			sessionName: "   ",
			wantErr:     true,
			errMsg:      "session name is required",
		},
		{
			name:        "session exists but db file missing",
			setupDB:     false,
			sessionName: "no-db-session",
			wantErr:     true,
			errMsg:      "orchestrator db not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newOrchestratorTaskTestApp(t)

			var tmpDir string
			if tt.setupDB || !tt.wantErr {
				// Create DB for this test case
				db, dir := createOrchestratorTaskTestDB(t)
				tmpDir = dir
				db.Close()
			} else if tt.sessionName != "" && tt.sessionName != "   " {
				// Create session without DB
				tmpDir = t.TempDir()
			}

			// Create session for non-empty session names
			if tt.sessionName != "" && strings.TrimSpace(tt.sessionName) != "" {
				if tt.setupDB {
					// Use tmpDir where DB was created
					createOrchestratorTestSession(t, app, tt.sessionName, tmpDir)
				} else {
					// Use separate tmpDir without DB
					createOrchestratorTestSession(t, app, tt.sessionName, tmpDir)
				}
			}

			db, cleanup, err := app.openOrchestratorDB(tt.sessionName)

			if (err != nil) != tt.wantErr {
				t.Fatalf("openOrchestratorDB() error = %v, wantErr = %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Fatalf("openOrchestratorDB() error = %v, want to contain %q", err, tt.errMsg)
				}
				return
			}

			if db == nil {
				t.Fatal("openOrchestratorDB() returned nil db")
			}

			// Verify DB is accessible
			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM agents").Scan(&count); err != nil {
				t.Fatalf("query agents failed: %v", err)
			}

			if cleanup != nil {
				cleanup()
			} else {
				t.Fatal("cleanup function is nil")
			}
		})
	}
}

func TestListOrchestratorTasks(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		setupData   []struct {
			agentName string
			tasks     []struct {
				taskID         string
				assigneePaneID string
				senderPaneID   string
				senderName     string
				status         string
				sentAt         string
				completedAt    string
				isNowSession   int
			}
		}
		wantCount  int
		wantErr    bool
		checkTasks func(t *testing.T, tasks []OrchestratorTask)
	}{
		{
			name:        "empty tasks table",
			sessionName: "empty-session",
			setupData: []struct {
				agentName string
				tasks     []struct {
					taskID         string
					assigneePaneID string
					senderPaneID   string
					senderName     string
					status         string
					sentAt         string
					completedAt    string
					isNowSession   int
				}
			}{
				{
					agentName: "agent1",
					tasks: []struct {
						taskID         string
						assigneePaneID string
						senderPaneID   string
						senderName     string
						status         string
						sentAt         string
						completedAt    string
						isNowSession   int
					}{},
				},
			},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:        "single task with is_now_session = 1",
			sessionName: "single-task-session",
			setupData: []struct {
				agentName string
				tasks     []struct {
					taskID         string
					assigneePaneID string
					senderPaneID   string
					senderName     string
					status         string
					sentAt         string
					completedAt    string
					isNowSession   int
				}
			}{
				{
					agentName: "agent1",
					tasks: []struct {
						taskID         string
						assigneePaneID string
						senderPaneID   string
						senderName     string
						status         string
						sentAt         string
						completedAt    string
						isNowSession   int
					}{
						{
							taskID:         "task-1",
							assigneePaneID: "%1",
							senderPaneID:   "%2",
							senderName:     "sender1",
							status:         "pending",
							sentAt:         "2024-01-01T10:00:00Z",
							completedAt:    "",
							isNowSession:   1,
						},
					},
				},
			},
			wantCount: 1,
			wantErr:   false,
			checkTasks: func(t *testing.T, tasks []OrchestratorTask) {
				if len(tasks) != 1 {
					t.Fatalf("expected 1 task, got %d", len(tasks))
				}
				if tasks[0].TaskID != "task-1" {
					t.Fatalf("TaskID = %q, want task-1", tasks[0].TaskID)
				}
				if tasks[0].SenderName != "sender1" {
					t.Fatalf("SenderName = %q, want sender1", tasks[0].SenderName)
				}
			},
		},
		{
			name:        "multiple tasks ordered by sent_at descending",
			sessionName: "multi-task-session",
			setupData: []struct {
				agentName string
				tasks     []struct {
					taskID         string
					assigneePaneID string
					senderPaneID   string
					senderName     string
					status         string
					sentAt         string
					completedAt    string
					isNowSession   int
				}
			}{
				{
					agentName: "agent1",
					tasks: []struct {
						taskID         string
						assigneePaneID string
						senderPaneID   string
						senderName     string
						status         string
						sentAt         string
						completedAt    string
						isNowSession   int
					}{
						{
							taskID:         "task-1",
							assigneePaneID: "%1",
							senderPaneID:   "%2",
							senderName:     "sender1",
							status:         "pending",
							sentAt:         "2024-01-01T10:00:00Z",
							completedAt:    "",
							isNowSession:   1,
						},
						{
							taskID:         "task-2",
							assigneePaneID: "%1",
							senderPaneID:   "%2",
							senderName:     "sender2",
							status:         "completed",
							sentAt:         "2024-01-01T11:00:00Z",
							completedAt:    "2024-01-01T11:30:00Z",
							isNowSession:   1,
						},
						{
							taskID:         "task-3",
							assigneePaneID: "%1",
							senderPaneID:   "%2",
							senderName:     "sender3",
							status:         "pending",
							sentAt:         "2024-01-01T09:00:00Z",
							completedAt:    "",
							isNowSession:   1,
						},
					},
				},
			},
			wantCount: 3,
			wantErr:   false,
			checkTasks: func(t *testing.T, tasks []OrchestratorTask) {
				if len(tasks) != 3 {
					t.Fatalf("expected 3 tasks, got %d", len(tasks))
				}
				// Must be ordered DESC by sent_at
				if tasks[0].TaskID != "task-2" {
					t.Fatalf("first task = %q, want task-2 (latest sent_at)", tasks[0].TaskID)
				}
				if tasks[1].TaskID != "task-1" {
					t.Fatalf("second task = %q, want task-1", tasks[1].TaskID)
				}
				if tasks[2].TaskID != "task-3" {
					t.Fatalf("third task = %q, want task-3 (oldest)", tasks[2].TaskID)
				}
			},
		},
		{
			name:        "tasks with is_now_session filter",
			sessionName: "filter-session",
			setupData: []struct {
				agentName string
				tasks     []struct {
					taskID         string
					assigneePaneID string
					senderPaneID   string
					senderName     string
					status         string
					sentAt         string
					completedAt    string
					isNowSession   int
				}
			}{
				{
					agentName: "agent1",
					tasks: []struct {
						taskID         string
						assigneePaneID string
						senderPaneID   string
						senderName     string
						status         string
						sentAt         string
						completedAt    string
						isNowSession   int
					}{
						{
							taskID:         "task-now-1",
							assigneePaneID: "%1",
							senderPaneID:   "%2",
							senderName:     "sender1",
							status:         "pending",
							sentAt:         "2024-01-01T10:00:00Z",
							completedAt:    "",
							isNowSession:   1,
						},
						{
							taskID:         "task-old-1",
							assigneePaneID: "%1",
							senderPaneID:   "%2",
							senderName:     "sender2",
							status:         "pending",
							sentAt:         "2024-01-01T10:01:00Z",
							completedAt:    "",
							isNowSession:   0,
						},
						{
							taskID:         "task-now-2",
							assigneePaneID: "%1",
							senderPaneID:   "%2",
							senderName:     "sender3",
							status:         "pending",
							sentAt:         "2024-01-01T10:02:00Z",
							completedAt:    "",
							isNowSession:   1,
						},
					},
				},
			},
			wantCount: 2,
			wantErr:   false,
			checkTasks: func(t *testing.T, tasks []OrchestratorTask) {
				if len(tasks) != 2 {
					t.Fatalf("expected 2 tasks with is_now_session=1, got %d", len(tasks))
				}
				for _, task := range tasks {
					if task.TaskID == "task-old-1" {
						t.Fatal("task-old-1 should not be included (is_now_session=0)")
					}
				}
			},
		},
		{
			name:        "tasks with null fields",
			sessionName: "null-fields-session",
			setupData: []struct {
				agentName string
				tasks     []struct {
					taskID         string
					assigneePaneID string
					senderPaneID   string
					senderName     string
					status         string
					sentAt         string
					completedAt    string
					isNowSession   int
				}
			}{
				{
					agentName: "agent1",
					tasks: []struct {
						taskID         string
						assigneePaneID string
						senderPaneID   string
						senderName     string
						status         string
						sentAt         string
						completedAt    string
						isNowSession   int
					}{
						{
							taskID:         "task-1",
							assigneePaneID: "",
							senderPaneID:   "",
							senderName:     "",
							status:         "pending",
							sentAt:         "2024-01-01T10:00:00Z",
							completedAt:    "",
							isNowSession:   1,
						},
					},
				},
			},
			wantCount: 1,
			wantErr:   false,
			checkTasks: func(t *testing.T, tasks []OrchestratorTask) {
				if len(tasks) != 1 {
					t.Fatalf("expected 1 task, got %d", len(tasks))
				}
				task := tasks[0]
				if task.AssigneePaneID != "" {
					t.Fatalf("AssigneePaneID should be empty, got %q", task.AssigneePaneID)
				}
				if task.SenderPaneID != "" {
					t.Fatalf("SenderPaneID should be empty, got %q", task.SenderPaneID)
				}
				if task.SenderName != "" {
					t.Fatalf("SenderName should be empty, got %q", task.SenderName)
				}
			},
		},
		{
			name:        "missing session",
			sessionName: "nonexistent-session",
			setupData: []struct {
				agentName string
				tasks     []struct {
					taskID         string
					assigneePaneID string
					senderPaneID   string
					senderName     string
					status         string
					sentAt         string
					completedAt    string
					isNowSession   int
				}
			}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newOrchestratorTaskTestApp(t)

			// Setup database and session
			db, tmpDir := createOrchestratorTaskTestDB(t)
			defer db.Close()

			// Only create session if not an error test case
			if !tt.wantErr {
				createOrchestratorTestSession(t, app, tt.sessionName, tmpDir)
			}

			// Insert test data
			for _, agentData := range tt.setupData {
				_, err := db.Exec(
					"INSERT INTO agents (name, pane_id, role) VALUES (?, ?, ?)",
					agentData.agentName, "%pane-"+agentData.agentName, "test-role",
				)
				if err != nil {
					t.Fatalf("insert agent error: %v", err)
				}

				for _, task := range agentData.tasks {
					_, err := db.Exec(
						`INSERT INTO tasks (task_id, agent_name, assignee_pane_id, sender_pane_id, sender_name, status, sent_at, completed_at, is_now_session)
						 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
						task.taskID,
						agentData.agentName,
						task.assigneePaneID,
						task.senderPaneID,
						task.senderName,
						task.status,
						task.sentAt,
						task.completedAt,
						task.isNowSession,
					)
					if err != nil {
						t.Fatalf("insert task error: %v", err)
					}
				}
			}
			db.Close()

			// Call the function
			tasks, err := app.ListOrchestratorTasks(tt.sessionName)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ListOrchestratorTasks() error = %v, wantErr = %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(tasks) != tt.wantCount {
				t.Fatalf("ListOrchestratorTasks() returned %d tasks, want %d", len(tasks), tt.wantCount)
			}

			if tt.checkTasks != nil {
				tt.checkTasks(t, tasks)
			}
		})
	}
}

func TestListOrchestratorAgents(t *testing.T) {
	tests := []struct {
		name        string
		sessionName string
		setupData   []struct {
			name   string
			paneID string
			role   string
		}
		wantCount   int
		wantErr     bool
		checkAgents func(t *testing.T, agents []OrchestratorAgent)
	}{
		{
			name:        "empty agents table",
			sessionName: "empty-agents-session",
			setupData: []struct {
				name   string
				paneID string
				role   string
			}{},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:        "single agent",
			sessionName: "single-agent-session",
			setupData: []struct {
				name   string
				paneID string
				role   string
			}{
				{
					name:   "agent1",
					paneID: "%1",
					role:   "developer",
				},
			},
			wantCount: 1,
			wantErr:   false,
			checkAgents: func(t *testing.T, agents []OrchestratorAgent) {
				if len(agents) != 1 {
					t.Fatalf("expected 1 agent, got %d", len(agents))
				}
				if agents[0].Name != "agent1" {
					t.Fatalf("Name = %q, want agent1", agents[0].Name)
				}
				if agents[0].PaneID != "%1" {
					t.Fatalf("PaneID = %q, want %%1", agents[0].PaneID)
				}
				if agents[0].Role != "developer" {
					t.Fatalf("Role = %q, want developer", agents[0].Role)
				}
			},
		},
		{
			name:        "multiple agents ordered by name",
			sessionName: "multi-agent-session",
			setupData: []struct {
				name   string
				paneID string
				role   string
			}{
				{name: "zulu-agent", paneID: "%3", role: "reviewer"},
				{name: "alpha-agent", paneID: "%1", role: "developer"},
				{name: "bravo-agent", paneID: "%2", role: "tester"},
			},
			wantCount: 3,
			wantErr:   false,
			checkAgents: func(t *testing.T, agents []OrchestratorAgent) {
				if len(agents) != 3 {
					t.Fatalf("expected 3 agents, got %d", len(agents))
				}
				// Must be ordered by name
				if agents[0].Name != "alpha-agent" {
					t.Fatalf("first agent = %q, want alpha-agent", agents[0].Name)
				}
				if agents[1].Name != "bravo-agent" {
					t.Fatalf("second agent = %q, want bravo-agent", agents[1].Name)
				}
				if agents[2].Name != "zulu-agent" {
					t.Fatalf("third agent = %q, want zulu-agent", agents[2].Name)
				}
			},
		},
		{
			name:        "agents with empty role",
			sessionName: "empty-role-session",
			setupData: []struct {
				name   string
				paneID string
				role   string
			}{
				{name: "agent-no-role", paneID: "%1", role: ""},
				{name: "agent-with-role", paneID: "%2", role: "developer"},
			},
			wantCount: 2,
			wantErr:   false,
			checkAgents: func(t *testing.T, agents []OrchestratorAgent) {
				if len(agents) != 2 {
					t.Fatalf("expected 2 agents, got %d", len(agents))
				}
				found := false
				for _, agent := range agents {
					if agent.Name == "agent-no-role" && agent.Role == "" {
						found = true
						break
					}
				}
				if !found {
					t.Fatal("agent-no-role with empty role not found")
				}
			},
		},
		{
			name:        "agents with special characters in name",
			sessionName: "special-chars-session",
			setupData: []struct {
				name   string
				paneID string
				role   string
			}{
				{name: "agent-with-dashes", paneID: "%1", role: "role1"},
				{name: "agent_with_underscores", paneID: "%2", role: "role2"},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:        "missing session",
			sessionName: "nonexistent-agents-session",
			setupData: []struct {
				name   string
				paneID string
				role   string
			}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := newOrchestratorTaskTestApp(t)

			// Setup database and session
			db, tmpDir := createOrchestratorTaskTestDB(t)
			defer db.Close()

			// Only create session if not an error test case
			if !tt.wantErr {
				createOrchestratorTestSession(t, app, tt.sessionName, tmpDir)
			}

			// Insert test data with mcp_instance_id to satisfy the IS NOT NULL filter.
			for _, agentData := range tt.setupData {
				_, err := db.Exec(
					"INSERT INTO agents (name, pane_id, role, mcp_instance_id) VALUES (?, ?, ?, ?)",
					agentData.name,
					agentData.paneID,
					agentData.role,
					"test-instance-1",
				)
				if err != nil {
					t.Fatalf("insert agent error: %v", err)
				}
			}
			db.Close()

			// Call the function
			agents, err := app.ListOrchestratorAgents(tt.sessionName)

			if (err != nil) != tt.wantErr {
				t.Fatalf("ListOrchestratorAgents() error = %v, wantErr = %v", err, tt.wantErr)
			}

			if tt.wantErr {
				return
			}

			if len(agents) != tt.wantCount {
				t.Fatalf("ListOrchestratorAgents() returned %d agents, want %d", len(agents), tt.wantCount)
			}

			if tt.checkAgents != nil {
				tt.checkAgents(t, agents)
			}
		})
	}
}

func TestOpenOrchestratorDBCleanup(t *testing.T) {
	app := newOrchestratorTaskTestApp(t)
	db, tmpDir := createOrchestratorTaskTestDB(t)
	db.Close()

	createOrchestratorTestSession(t, app, "cleanup-test", tmpDir)

	db, cleanup, err := app.openOrchestratorDB("cleanup-test")
	if err != nil {
		t.Fatalf("openOrchestratorDB() error = %v", err)
	}

	if db == nil || cleanup == nil {
		t.Fatal("db and cleanup should not be nil")
	}

	// Verify cleanup doesn't panic
	cleanup()

	// Cleanup should close the connection, so next query should fail
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM agents").Scan(&count)
	if err == nil {
		t.Fatal("expected error after cleanup, but query succeeded")
	}
}

func TestListOrchestratorTasksColumnsCorrect(t *testing.T) {
	app := newOrchestratorTaskTestApp(t)
	db, tmpDir := createOrchestratorTaskTestDB(t)

	createOrchestratorTestSession(t, app, "column-test", tmpDir)

	// Insert agent
	_, err := db.Exec(
		"INSERT INTO agents (name, pane_id, role) VALUES (?, ?, ?)",
		"test-agent", "%pane-1", "test-role",
	)
	if err != nil {
		t.Fatalf("insert agent error: %v", err)
	}

	// Insert task with all fields populated
	_, err = db.Exec(
		`INSERT INTO tasks (task_id, agent_name, assignee_pane_id, sender_pane_id, sender_name, status, sent_at, completed_at, is_now_session)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"task-1", "test-agent", "%assignee", "%sender", "sender-name", "completed", "2024-01-01T10:00:00Z", "2024-01-01T11:00:00Z", 1,
	)
	if err != nil {
		t.Fatalf("insert task error: %v", err)
	}
	db.Close()

	tasks, err := app.ListOrchestratorTasks("column-test")
	if err != nil {
		t.Fatalf("ListOrchestratorTasks() error = %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if task.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want task-1", task.TaskID)
	}
	if task.AgentName != "test-agent" {
		t.Fatalf("AgentName = %q, want test-agent", task.AgentName)
	}
	if task.AssigneePaneID != "%assignee" {
		t.Fatalf("AssigneePaneID = %q, want %%assignee", task.AssigneePaneID)
	}
	if task.SenderPaneID != "%sender" {
		t.Fatalf("SenderPaneID = %q, want %%sender", task.SenderPaneID)
	}
	if task.SenderName != "sender-name" {
		t.Fatalf("SenderName = %q, want sender-name", task.SenderName)
	}
	if task.Status != "completed" {
		t.Fatalf("Status = %q, want completed", task.Status)
	}
	if task.SentAt != "2024-01-01T10:00:00Z" {
		t.Fatalf("SentAt = %q, want 2024-01-01T10:00:00Z", task.SentAt)
	}
	if task.CompletedAt != "2024-01-01T11:00:00Z" {
		t.Fatalf("CompletedAt = %q, want 2024-01-01T11:00:00Z", task.CompletedAt)
	}
}

func TestListOrchestratorAgentsColumnsCorrect(t *testing.T) {
	app := newOrchestratorTaskTestApp(t)
	db, tmpDir := createOrchestratorTaskTestDB(t)

	createOrchestratorTestSession(t, app, "agent-column-test", tmpDir)

	// Insert agent with all fields (mcp_instance_id required for IS NOT NULL filter)
	_, err := db.Exec(
		"INSERT INTO agents (name, pane_id, role, mcp_instance_id) VALUES (?, ?, ?, ?)",
		"full-agent", "%pane-full", "senior-developer", "test-instance-1",
	)
	if err != nil {
		t.Fatalf("insert agent error: %v", err)
	}
	db.Close()

	agents, err := app.ListOrchestratorAgents("agent-column-test")
	if err != nil {
		t.Fatalf("ListOrchestratorAgents() error = %v", err)
	}

	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	agent := agents[0]
	if agent.Name != "full-agent" {
		t.Fatalf("Name = %q, want full-agent", agent.Name)
	}
	if agent.PaneID != "%pane-full" {
		t.Fatalf("PaneID = %q, want %%pane-full", agent.PaneID)
	}
	if agent.Role != "senior-developer" {
		t.Fatalf("Role = %q, want senior-developer", agent.Role)
	}
}
